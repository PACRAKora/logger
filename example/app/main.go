package main

import (
	"context"
	"errors"
	"time"

	logging "github.com/PACRAKora/logger/logger"
)

func main() {
	// Initialize logging configuration.
	// Seq is auto-enabled when SEQ_ENABLE=true, SEQ_URL, and SEQ_API_KEY are set in the environment.
	// Set EnableFile: true to write logs to a file (e.g. for local dev or VM deployments).
	logging.InitConfig(logging.Config{
		// CONFIGURABLE: Set from your service configuration.
		Service:     "pacra-logger-example",
		Env:         "development",
		ConsoleJSON: false,
		RedactKeys:  []string{"password", "token", "authorization", "api_key", "card_number", "cvv"},

		// CONFIGURABLE: Time format for pretty console output.
		TimeFormat: time.RFC3339,
	})

	ctx := context.Background()
	ctx = logging.WithTraceID(ctx, "")

	logging.Info(ctx, "main", "application started")

	logging.Warn(ctx, "processMessage", "/handlers/processMessage", "retrying message",
		logging.WithEvent("SagaStepRetrying"),
		logging.WithRetryCount(1),
		logging.WithDurationMs(245),
		logging.WithMetadata(map[string]any{"order_id": "order-2024-001"}),
	)

	logging.Error(ctx, "processMessage", "/handlers/processMessage", "failed to handle message",
		logging.WithEvent("SagaStepFailed"),
		logging.WithRetryCount(3),
		logging.WithException(errors.New("payment gateway timeout")),
	)

	logging.Critical(ctx, "processMessage", "/handlers/processMessage", "compensation failed",
		logging.WithEvent("CompensationFailed"),
		logging.WithException(errors.New("compensation outbox unavailable")),
	)
}
