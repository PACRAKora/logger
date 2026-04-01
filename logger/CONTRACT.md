## Logging contract (Saga-based microservices)

This document is the **cross-language logging contract** for saga-based microservices (Orchestrator pattern), aligned across **Go**, **C# (.NET)**, and **Python**.

### Core requirements

- **Structured logs only**: JSON
- **Field naming**: **snake_case**
- **Trace is mandatory**: every log line must include `trace_id`
- **Event naming**: use `event` for business/saga events (e.g., `SagaStarted`, `SagaStepFailed`)
- **No secrets/PII**: never log credentials; use redaction where possible

### Standard fields

#### Required on every log

- `timestamp`
- `level` (`info` | `warn` | `error` | `critical`)
- `service`
- `environment`
- `service_version`
- `trace_id`
- `message`

#### Strongly recommended

- `span_id` (OpenTelemetry)
- `component` (`Orchestrator` | `Participant` | `Consumer` | `API`)
- `event` (especially for saga lifecycle)

#### For saga execution

- `retry_count` (when retrying)
- `duration_ms` (latency/performance events)

#### Structured payloads

- `metadata`: object with business context (order_id, amount, customer_id, etc.)
- `exception`: object: `{ "type": "...", "message": "...", "stack": "..." }`

### Propagation contract

#### HTTP headers

- `X-Trace-Id` → `trace_id`

#### Messaging headers

- `trace_id`

### Canonical saga events (`event`)

- `SagaStarted`
- `SagaStepCompleted`
- `SagaStepFailed`
- `SagaStepRetrying`
- `SagaAborted`
- `SagaCompleted`
- `CompensationTriggered`
- `CompensationFailed`

---

## Go example (this package)

```go
ctx := context.Background()
ctx = logging.WithTraceID(ctx, "trace-a1b2c3d4")

logging.Warn(ctx, "processPayment", "/saga/processPayment", "retrying payment",
    logging.WithComponent("Participant"),
    logging.WithEvent("SagaStepRetrying"),
    logging.WithRetryCount(1),
    logging.WithDurationMs(245),
    logging.WithMetadata(map[string]any{
        "order_id": "order-2024-001",
        "amount": 99.99,
        "currency": "ZMW",
    }),
)
```

### OpenTelemetry (optional)

If your service uses OpenTelemetry, build with `-tags=otel` and add:

```go
logging.Warn(ctx, "processPayment", "/saga/processPayment", "retrying payment",
    logging.WithTraceFromContext(ctx),
)
```

---

## C# (.NET) example (Serilog)

The key is to **enrich** every log with the contract fields and keep names in **snake_case**.

```csharp
using Serilog;
using Serilog.Events;
using Serilog.Formatting.Json;

var logger = new LoggerConfiguration()
    .MinimumLevel.Information()
    .Enrich.WithProperty("service", "payment-service")
    .Enrich.WithProperty("environment", Environment.GetEnvironmentVariable("APP_ENV") ?? "production")
    .Enrich.WithProperty("service_version", Environment.GetEnvironmentVariable("SERVICE_VERSION") ?? "unknown")
    // trace_id should be added per-request via middleware/enrichers
    .WriteTo.Console(new JsonFormatter(renderMessage: true))
    .CreateLogger();

Log.Logger = logger;

Log.ForContext("trace_id", "trace-a1b2c3d4")
   .ForContext("component", "Participant")
   .ForContext("event", "SagaStepFailed")
   .ForContext("metadata", new { order_id = "order-2024-001", amount = 99.99, currency = "ZMW" }, destructureObjects: true)
   .Error("payment processing failed");
```

### Level mapping

- `Information` → `info`
- `Warning` → `warn`
- `Error` → `error`
- `Fatal` → `critical` (only for system failure / saga corruption / compensation failure)

---

## Python example (standard library `logging`)

```python
import json
import logging
import uuid
from datetime import datetime, timezone


class JSONFormatter(logging.Formatter):
    """Emits contract-compliant snake_case JSON log lines."""

    def __init__(self, service: str, environment: str, service_version: str):
        super().__init__()
        self.service = service
        self.environment = environment
        self.service_version = service_version

    def format(self, record: logging.LogRecord) -> str:
        level_map = {
            logging.DEBUG: "debug",
            logging.INFO: "info",
            logging.WARNING: "warn",
            logging.ERROR: "error",
            logging.CRITICAL: "critical",
        }
        entry = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "level": level_map.get(record.levelno, "info"),
            "service": self.service,
            "environment": self.environment,
            "service_version": self.service_version,
            "trace_id": getattr(record, "trace_id", str(uuid.uuid4())),
            "message": record.getMessage(),
        }
        return json.dumps(entry)


# Setup
handler = logging.StreamHandler()
handler.setFormatter(JSONFormatter(
    service="payment-service",
    environment="production",
    service_version="1.0.0",
))
logger = logging.getLogger("payment-service")
logger.addHandler(handler)
logger.setLevel(logging.INFO)

# Usage
extra = {"trace_id": "trace-a1b2c3d4"}
logger.warning("retrying payment", extra=extra)
```
