package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

var (
	rootLogger        zerolog.Logger
	loggerInitialized bool
)

// initLogger configures the root zerolog logger with console, optional file, and optional Seq outputs.
func initLogger() {
	cfg := ConfigOrDefault()

	// Standardize field names for strategy alignment.
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var consoleWriter io.Writer
	if cfg.Env == "development" && !cfg.ConsoleJSON {
		// Pretty console output in development.
		pretty := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: cfg.TimeFormat,
		}
		consoleWriter = pretty
	} else {
		// JSON console output in non-development or when explicitly requested.
		consoleWriter = os.Stderr
	}

	var writers []io.Writer
	writers = append(writers, consoleWriter)

	// Only open the log file if explicitly requested.
	if cfg.EnableFile {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			panic("failed to create logs directory: " + err.Error())
		}
		logFilePath := filepath.Join(cfg.LogDir, "app.log")
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic("failed to open log file: " + err.Error())
		}
		writers = append(writers, file)
	}

	if sw := newSeqWriter(cfg); sw != nil {
		// Seq writer integrated with zerolog.
		writers = append(writers, sw)
	}

	multi := zerolog.MultiLevelWriter(writers...)

	// CONFIGURABLE: Global logging level; default to info.
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// EXTENSION POINT: Attach hooks, sampling, or additional configuration here.
	rootLogger = zerolog.New(multi).With().
		Timestamp().
		Str("service", cfg.Service).
		Str("environment", cfg.Env).
		Logger()
	loggerInitialized = true
}

// Logger returns the configured root logger.
func Logger() zerolog.Logger {
	if !loggerInitialized {
		initLogger()
	}
	return rootLogger
}
