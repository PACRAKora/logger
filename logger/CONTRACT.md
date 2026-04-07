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
- `trace_id`
- `message`

#### Required on `warn`, `error`, `critical`

- `function` — name of the function where the log originates
- `error_path` — logical route/path identifier (e.g. `/saga/processPayment`)

#### Strongly recommended

- `span_id` — current span ID (set via context or OpenTelemetry)
- `parent_id` — parent span ID for distributed tracing correlation
- `correlation_id` — business transaction ID spanning multiple traces/services
- `component` (`Orchestrator` | `Participant` | `Consumer` | `API`)
- `event` (especially for saga lifecycle)
- `actor_id`, `actor_type`, `actor_ip` — flat who-fields, directly filterable in Seq/Kibana/Grafana

#### For saga execution

- `retry_count` (when retrying)
- `duration_ms` (latency/performance events)

#### Structured payloads

- `metadata`: object with business context (order_id, amount, customer_id, etc.)
- `exception`: object: `{ "type": "...", "message": "...", "stack": "..." }`

#### Audit fields (Who / What)

All audit fields are flat top-level strings for direct filterability in Seq/Kibana/Grafana.

**Who:**
- `actor_id` — user ID, service account, or job ID performing the action
- `actor_type` — `"user"` | `"service"` | `"scheduler"` | `"system"`
- `actor_ip` — originating IP address

**What:**
- `action` — `"create"` | `"update"` | `"delete"` | `"read"`
- `resource_type` — resource category, e.g. `"payment"` | `"order"` | `"account"`
- `resource_id` — specific entity acted upon
- `outcome` — `"success"` | `"failure"` | `"partial"`

#### NATS/messaging fields (optional)

- `subscribe_subject` — NATS subject the service consumed this event from
- `publish_subject` — NATS subject the service is publishing to
- `received_payload` — inbound message body (JSON object or string); sensitive keys are redacted
- `response_payload` — outbound response body (JSON object or string); sensitive keys are redacted

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
ctx = logger.WithTraceID(ctx, "trace-a1b2c3d4")
ctx = logger.WithSpanID(ctx, "span-00112233")   // optional: manual span propagation

logger.Warn(ctx, "processPayment", "/saga/processPayment", "retrying payment",
    logger.WithEvent("SagaStepRetrying"),
    logger.WithRetryCount(1),
    logger.WithDurationMs(245),
    logger.WithParentID("span-parent-0000"),
    logger.WithMetadata(map[string]any{
        "order_id":  "order-2024-001",
        "amount":    99.99,
        "currency":  "ZMW",
        "component": "Participant",
    }),
)

// NATS-specific fields
logger.Info(ctx, "handlePaymentEvent", "received payment event",
    logger.WithSubscribeSubject("payments.process"),
    logger.WithPublishSubject("payments.result"),
    logger.WithReceivedPayload(inboundBytes),
    logger.WithResponsePayload(outboundBytes),
)
```

### OpenTelemetry (optional)

If your service uses OpenTelemetry, build with `-tags=otel` and add:

```go
logger.Warn(ctx, "processPayment", "/saga/processPayment", "retrying payment",
    logger.WithTraceFromContext(ctx),
)
```

This extracts `trace_id` from the active OTel span. When not using OTel, set `trace_id` and `span_id` manually via `WithTraceID` / `WithSpanID` on the context.

### Seq integration

Seq receives logs over **GELF UDP** (not HTTP). Configure via environment variables:

```
SEQ_ENABLE=true
SEQ_URL=localhost:12201   # must be a GELF UDP input address
SEQ_API_KEY=...           # unused with GELF transport; kept for backwards compatibility
```

Field mapping to GELF extras (`_` prefix):

| Contract field | GELF extra key |
|----------------|---------------|
| `trace_id` | `_TraceId` |
| `span_id` | `_SpanId` |
| `parent_id` | `_ParentId` |
| `metadata.*` | `_<key>` (flattened) |
| all others | `_<field_name>` |

Special mappings: `message` → GELF `Short`, `exception` → GELF `Full` (`type: message\nstack`). `timestamp`, `level`, and `exception` are excluded from extras.

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
