### Logging package (`logger`)

This package provides a structured logging facility for an event-driven microservice architecture
based on `zerolog`.

- **Outputs**: console/stderr (always), optional file logs (`LogDir/app.log` when `EnableFile: true`), and optional Seq over HTTP.
- **Format**: JSON structured logs only (JSON lines for file output).
- **Levels**: `info`, `warn`, `error`, `critical`.
- **Structured fields** (always available): `service`, `environment`, `service_version`, `trace_id`, `message`, `level`, `timestamp`.
- **Common contextual fields**: `component`, `event`, `function`, `error_path`, `duration_ms`, `retry_count`, `metadata`, `exception`.

---

## Strategy alignment (Saga-based microservices)

This section highlights how to use this package in a saga (orchestrator-based) architecture and how it maps to the standardized logging strategy.

For the cross-language schema contract (Go + C# + Python examples), see `CONTRACT.md`.

### 0. Compliance checklist (package + service integration)

- **Structured logging only**: All production logs emitted via this package are JSON structured logs.
- **Trace is mandatory**:
  - **Required in every log**: `trace_id`
  - **Strongly recommended in every log** (strategy requirement): `span_id` when available
- **Consistent log levels**:
  - **Supported**: `info`, `warn`, `error`, `critical`
- **Contextual data**:
  - **Package-provided baseline**: `service`, `environment`, `service_version`, `timestamp`, `level`, `message`, plus `function` and `error_path`
  - **Supported**: `component` (Orchestrator/Participant/Consumer/API) and `metadata` object
- **Propagation**:
  - **Package supports**: trace ID propagation via `context.Context` (`WithTraceID`)
  - **Integration required**: HTTP/message header propagation must be implemented in each service (gateway, orchestrator, participants, consumers).
- **Observability integration**:
  - **Package supports**: emitting `trace_id` / `span_id` as optional fields if the service provides them
  - **Optional helper**: `WithTraceFromContext(ctx)` (build with `-tags=otel`) extracts IDs from OpenTelemetry context automatically.
- **Sensitive data masking**:
  - **Supported**: key-based redaction via `Config.RedactKeys` (applies to `metadata` and Seq properties).
- **Saga events**:
  - **Supported pattern**: use `event` for saga events like `SagaStarted`, `SagaStepCompleted`, `SagaStepFailed`, `CompensationTriggered`, `CompensationFailed`
  - **Integration required**: orchestrator/participants must emit these events consistently.
- **Centralized aggregation / retention / alerts / dashboards / runbooks**:
  - **Partially supported**: Seq output
  - **Integration required**: log aggregation deployment, retention policy, alert rules, dashboards, and runbooks are outside this package.

### 1. Standard schema mapping and gaps

Your strategy's canonical fields vs this package's current fields:

| Strategy field | Strategy meaning | Package field today | Status |
|---|---|---|---|
| `timestamp` | Event time | `timestamp` | Supported |
| `level` | Info/Warn/Error/Critical | `level` | Supported |
| `service` | Service name | `service` | Supported |
| `serviceVersion` | Deployed version | `service_version` | Supported |
| `environment` | prod/stage/dev | `environment` | Supported |
| `component` | Orchestrator/Participant/Consumer/API | `component` | Supported |
| `traceId` | End-to-end trace correlation | `trace_id` | Required |
| `spanId` | OTel span id | `span_id` | Supported (optional) |
| `event` | Event name | `event` | Supported |
| `message` | Human-readable description | `message` | Supported |
| `metadata` | Structured business context | `metadata` | Supported |
| `performance.durationMs` | Duration in ms | `duration_ms` | Supported |
| `performance.retryCount` | Retries attempted | `retry_count` | Supported (optional) |
| `exception` | Error details | `exception` | Supported |

**Important**: the package uses `snake_case` keys (e.g., `trace_id`, `span_id`). If your org standard mandates `camelCase` (`traceId`, etc.), either:

- Standardize on `snake_case` in the strategy for Go services, or
- Add a compatibility layer/field-aliasing in this package so emitted keys match the strategy exactly.

### 1. Configuration

Call `InitConfig` **once at startup** in each application that uses this package.

```go
import (
    "os"
    "time"

    "github.com/PACRAKora/logger/logger"
)

func main() {
    logging.InitConfig(logging.Config{
        // REQUIRED FIELD: unique per service
        Service: os.Getenv("SERVICE_NAME"),

        // CONFIGURABLE
        Env:         getenvDefault("APP_ENV", "production"),
        ServiceVersion: os.Getenv("SERVICE_VERSION"),
        ConsoleJSON: getenvDefault("LOG_CONSOLE_JSON", "true") == "true",

        // Leave LogDir empty to defer to LOG_DIR env var (recommended)
        LogDir: "",

        // EnableFile: true to write logs to LogDir/app.log (local dev / VM only)
        // Seq is auto-enabled from SEQ_ENABLE, SEQ_URL, SEQ_API_KEY env vars

        // Masking/redaction (recommended defaults)
        RedactKeys: []string{"password", "token", "authorization", "api_key"},

        TimeFormat: time.RFC3339,
    })

    // ...
}
```

Recommended environment variables (set per deployment):

- `SERVICE_NAME` – logical service name (e.g. `orders-service`).
- `APP_ENV` – `development`, `staging`, or `production`.
- `SERVICE_VERSION` – deployed version identifier (e.g. `1.2.3` or git SHA).
- `LOG_DIR` – absolute or project-root-relative path for logs (e.g. `/app/logs`).
- `LOG_CONSOLE_JSON` – `"true"` for JSON console, `"false"` for pretty output in development.
- `SEQ_ENABLE` – `"true"`/`"false"` to toggle Seq.
- `SEQ_URL` – e.g. `http://seq:5341`.
- `SEQ_API_KEY` – optional Seq API key.

If `LogDir` is empty in config:

- The package will first look at `LOG_DIR`.
- If `LOG_DIR` is not set, it falls back to `"logs"` in the current working directory.

### 2. Using trace IDs

Use contexts to propagate a `trace_id` across calls.

```go
import "github.com/PACRAKora/logger/logger"

// New request / message entry-point:
func handler(ctx context.Context) {
    // Generate or attach a trace id
    ctx = logging.WithTraceID(ctx, "")

    logging.Info(ctx, "handler", "started handling request")
}
```

If the context already has a trace id (e.g. from an HTTP gateway), use:

```go
ctx, traceID := logging.TraceIDFromContext(ctx)
_ = traceID // use for headers, etc.
```

#### 2.1 Strategy-required propagation (what services must do)

This package propagates trace ID via `context.Context`, but your strategy also requires propagating identifiers across boundaries.

**HTTP headers (sync calls)**:

- `X-Trace-Id`: propagate into `trace_id` (required)

**Message headers (async)**:

- `trace_id` (name depends on your broker conventions)

Services should:

- **Extract** these IDs at entry points (HTTP middleware, message consumer wrapper)
- **Attach** them to context
- **Ensure every log line** includes them (either as first-class fields in this package or via additional options you add/extend)

### 3. Emitting logs

```go
ctx := logging.WithTraceID(context.Background(), "")

// Info
logging.Info(ctx, "main", "application started")

// Warn (function and error_path are REQUIRED)
logging.Warn(ctx, "processEvent", "/handler/processEvent", "unexpected payload",
    logging.WithEvent("SagaStepRetrying"),
    logging.WithComponent("Participant"),
    logging.WithMetadata(map[string]any{"order_id": "order-2024-001"}),
    logging.WithTopic("user-events"),
)

// Error (function and error_path are REQUIRED)
logging.Error(ctx, "processEvent", "/handler/processEvent", "failed to persist event",
    logging.WithRetryCount(3),
    logging.WithMessageID("msg-123"),
    logging.WithException(errors.New("example failure")),
)
```

All three outputs (file, console, Seq if enabled) receive the same structured event.

#### 3.1 Saga event naming (strategy alignment)

Use `event` consistently for saga lifecycle events so production incident queries and business analytics are possible:

- `SagaStarted`
- `SagaStepCompleted`
- `SagaStepFailed`
- `SagaStepRetrying`
- `SagaAborted`
- `SagaCompleted`
- `CompensationTriggered`
- `CompensationFailed`

### 4. Where logs go

- **File**: `app.log` inside `LogDir` (JSON lines).
- **Console**:
  - Pretty human-readable in development when `Env == "development"` and `ConsoleJSON == false`.
  - JSON in all other cases.
- **Seq**:
  - Enabled when `EnableSeq == true` and `SeqURL` is non-empty.
  - Failures to reach Seq **never crash the app**; they are ignored.

### 5. Extension points

- `Config` (in `config.go`) – add new configurable knobs.
- `Option` and `With…` helpers (in `logger.go`) – add new optional fields.
- `seqWriter` (in `seq.go`) – customize Seq payloads, HTTP client, retries.
- `initLogger` (in `writer.go`) – add hooks, sampling, or additional writers.

---

## Build tags

### OpenTelemetry trace/span extraction

If you want `WithTraceFromContext(ctx)` support, build with:

```bash
go test -tags=otel ./...
```

This intentionally keeps OpenTelemetry as an optional dependency.
