# pacra-logger

Structured logging library for Go microservices, built on [zerolog](https://github.com/rs/zerolog).
Designed for **Saga-based orchestrator patterns** in distributed systems.

## Install

```bash
go get github.com/PACRAKora/logger/logger@v1.0.0
```

## Quick start

```go
import (
    "context"
    "os"
    logger "github.com/PACRAKora/logger/logger"
)

func main() {
    logger.InitConfig(logger.Config{
        Service:        os.Getenv("SERVICE_NAME"),
        Env:            os.Getenv("APP_ENV"),        // defaults to "development"
        ServiceVersion: os.Getenv("SERVICE_VERSION"),
        RedactKeys:     []string{"password", "token", "api_key", "authorization"},
        // Set EnableFile: true to write logs to LogDir/app.log (local dev / VM)
        // Seq is auto-enabled when SEQ_ENABLE=true, SEQ_URL, and SEQ_API_KEY are in the environment
    })

    ctx := logger.WithTraceID(context.Background(), "") // generate a fresh trace ID

    logger.Info(ctx, "main", "application started")

    logger.Warn(ctx, "processOrder", "/orders/processOrder", "retrying step",
        logger.WithEvent("SagaStepRetrying"),
        logger.WithComponent("Participant"),
        logger.WithTopic("orders"),
        logger.WithMessageID("msg-123"),
        logger.WithRetryCount(1),
        logger.WithDurationMs(245),
        logger.WithMetadata(map[string]any{"order_id": "order-2024-001"}),
    )

    logger.Error(ctx, "processOrder", "/orders/processOrder", "payment failed",
        logger.WithEvent("SagaStepFailed"),
        logger.WithComponent("Orchestrator"),
        logger.WithException(err),
    )
}
```

## Features

- **Multi-output** — stderr (pretty or JSON), optional file (`app.log`), optional [Seq](https://datalust.co/seq)
- **Redaction** — sensitive keys (e.g. `password`, `token`) masked in `metadata` before any write
- **Saga fields** — `component`, `event`, `topic`, `message_id`, `retry_count`, `duration_ms`
- **Trace propagation** — `trace_id` auto-generated or injected; forward via `X-Trace-Id` header
- **Custom `critical` level** — above Fatal (level 99) for highest-severity alerts
- **OpenTelemetry** — build with `-tags=otel` to extract `trace_id`/`span_id` from an active OTel span
- **Cloud-native defaults** — file logging off by default; logs to stderr for container log collectors

## Configuration via environment variables

| Variable | Effect |
|----------|--------|
| `LOG_DIR` | Override log file directory (used when `EnableFile: true`) |
| `SEQ_ENABLE=true` | Enable Seq output |
| `SEQ_URL` | Seq server URL |
| `SEQ_API_KEY` | Seq API key |

```go
// Minimal production config — everything else comes from env vars
logger.InitConfig(logger.Config{
    Service: os.Getenv("SERVICE_NAME"),
})
```

## Build & test

```bash
go build ./...
go test ./...
go test -tags=otel ./...
cd example/app && go run .
```

## Docs

- [logger/README.md](logger/README.md) — full strategy docs, schema, and usage guide
- [logger/CONTRACT.md](logger/CONTRACT.md) — cross-language logging contract (Go, C#, Python)

## License

MIT © 2026 PACRA / PACRAKora
