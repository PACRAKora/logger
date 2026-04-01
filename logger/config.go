package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Config holds all logging configuration.
// CONFIGURABLE: All fields in this struct are intended to be set from application configuration.
type Config struct {
	// Service is the logical service name for this application.
	// REQUIRED FIELD: Must be provided and non-empty.
	Service string

	// Env represents the deployment environment (e.g. "development", "staging", "production").
	// CONFIGURABLE
	Env string

	// RedactKeys are case-insensitive field names that should be redacted from metadata/properties.
	// Example: ["password", "token", "authorization", "ssn"].
	// CONFIGURABLE
	RedactKeys []string

	// LogDir is the directory where log files will be written.
	// Only used when EnableFile is true.
	// CONFIGURABLE
	LogDir string

	// EnableFile writes logs to a file at LogDir/app.log.
	// Disabled by default; suitable for containerised deployments where stderr is collected.
	EnableFile bool

	// ConsoleJSON determines whether console output should be JSON (true)
	// or a human-friendly pretty format (false).
	// CONFIGURABLE
	ConsoleJSON bool

	// EnableSeq toggles sending logs to Seq over HTTP.
	// Can also be enabled by setting SEQ_ENABLE=true in the environment.
	// CONFIGURABLE
	EnableSeq bool

	// SeqURL is the base URL of the Seq server (e.g. "http://localhost:5341").
	// Can also be set via the SEQ_URL environment variable.
	// CONFIGURABLE
	SeqURL string

	// SeqAPIKey is an optional API key for authenticating with Seq.
	// Can also be set via the SEQ_API_KEY environment variable.
	// CONFIGURABLE
	SeqAPIKey string

	// TimeFormat is the layout used for timestamps when pretty-printing.
	// CONFIGURABLE
	TimeFormat string
}

var (
	globalConfig Config
)

// InitConfig initializes global logging configuration.
// This must be called once during application startup before using the logging package.
func InitConfig(cfg Config) {
	// REQUIRED FIELD: Service must be set and non-empty.
	if cfg.Service == "" {
		panic("logging.Config.Service must be set")
	}

	if cfg.LogDir == "" {
		// CONFIGURABLE: Allow overriding the logs directory via environment variable.
		// This enables a shared pattern across many projects, e.g. LOG_DIR=/app/logs or /var/log/my-service.
		if env := os.Getenv("LOG_DIR"); env != "" {
			cfg.LogDir = env
		} else {
			// CONFIGURABLE: Fallback logs directory if neither config nor LOG_DIR is provided.
			cfg.LogDir = "logs"
		}
	}

	if cfg.TimeFormat == "" {
		// CONFIGURABLE: Default time format used for pretty console writer.
		cfg.TimeFormat = time.RFC3339
	}

	if cfg.Env == "" {
		// CONFIGURABLE: Default environment if not provided.
		cfg.Env = "development"
	}

	// Auto-populate Seq settings from environment variables when not set in code.
	if !cfg.EnableSeq {
		cfg.EnableSeq = os.Getenv("SEQ_ENABLE") == "true"
	}
	if cfg.SeqURL == "" {
		cfg.SeqURL = os.Getenv("SEQ_URL")
	}
	if cfg.SeqAPIKey == "" {
		cfg.SeqAPIKey = os.Getenv("SEQ_API_KEY")
	}

	globalConfig = cfg

	// If InitConfig is called again (tests or re-init), ensure we rebuild the root logger.
	// This keeps behavior deterministic across multiple initializations within one process.
	rootLogger = zerolog.Logger{}
	loggerInitialized = false
}

// ConfigOrDefault returns the current globalConfig.
// It panics if InitConfig has not been called.
func ConfigOrDefault() Config {
	if globalConfig.Service == "" {
		panic("logging.InitConfig must be called before using the logging package")
	}
	return globalConfig
}
