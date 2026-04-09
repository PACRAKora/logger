# Logging Strategy for Saga-Based Microservice Architecture

**Tech Stack:** C# (.NET) and Go  
**Architecture Pattern:** Saga (Orchestrator-based)  
**Last Updated:** February 13, 2026

---

## Table of Contents

1. [Purpose](#1-purpose)
2. [Core Logging Principles](#2-core-logging-principles)
3. [Standard Log Schema](#3-standard-log-schema)
4. [Correlation and Tracing](#4-correlation-and-tracing)
5. [Observability Integration](#5-observability-integration)
6. [Success Metrics](#6-success-metrics)

---

## 1. Purpose

This document defines the standardized logging strategy for a distributed microservice architecture using the Saga pattern.

### Goals

* Enable distributed debugging across services and languages (C# and Go)
* Ensure end-to-end request traceability in saga workflows
* Standardize log structure across all microservices
* Facilitate root cause analysis for saga failures
* Enable business analytics on transaction flows

---

## 2. Core Logging Principles

### 2.1 Structured Logging Only

**All logs MUST be JSON structured logs.**

```csharp
// C#
Log.Information("Order processed {Event} for OrderId={OrderId} SagaId={SagaId}", 
    "OrderCompleted", orderId, sagaId);
```

```go
// Go
log.Info().
    Str("Event", "OrderCompleted").
    Str("OrderId", orderId).
    Str("SagaId", sagaId).
    Msg("Order processed")
```

---

### 2.2 Correlation is Mandatory

Every log entry MUST include correlation identifiers:

| Field | Purpose | Example |
|-------|---------|---------|
| `correlationId` | Tracks entire request lifecycle across all services | `corr-a1b2c3d4` |
| `referenceId` | Business-level transaction identifier | `case-2024-001` |
| `traceId` | OpenTelemetry distributed trace ID | `4bf92f3577b34da6a3ce929d0e0e4736` |

**How correlation works:**

```
User Request → Frontend Service (generates correlationId)
    ↓
Order Orchestrator (receives correlationId)
    ↓
Inventory Service (receives both IDs)
    ↓
Payment Service (receives both IDs)
    ↓
All logs share the same correlationId
```

---

### 2.3 Consistent Log Levels

| Level | When to Use | Examples |
|-------|-------------|----------|
| **Information** | Normal saga operation | Saga started, step completed, saga finished |
| **Warning** | Degraded performance or retries | Slow response, retry attempt, duplicate detection |
| **Error** | Saga step failure (recoverable) | Payment declined, inventory unavailable |
| **Critical** | System failure or saga corruption | Database unavailable, saga state invalid, compensation failed |

**Never use DEBUG or TRACE in production** unless temporarily enabled for troubleshooting.

---

### 2.4 Contextual Data is Required

Every log must answer:
- **What** happened? (event name)
- **Where** did it happen? (service, component)
- **When** did it happen? (timestamp)
- **Why** did it happen? (business context)
- **Who** initiated it? (user, system, saga)

Minimal context fields:
```json
{
  "timestamp": "2026-02-13T10:15:30.123Z",
  "level": "Information",
  "service": "payment-service",
  "component": "SagaParticipant",
  "event": "PaymentProcessed",
  "correlationId": "corr-456"
}
```

---

## 3. Standard Log Schema

All services (C# and Go) MUST follow this structure:

```json
{
  "timestamp": "2026-02-13T10:15:30.123Z",
  "level": "Information | Warning | Error | Critical",
  "service": "payment-service",
  "serviceVersion": "1.2.3",
  "environment": "production | staging | development",
  "component": "Orchestrator | Participant | Consumer | API",
  
  "correlationId": "corr-a1b2c3d4",
  "transactionId": "order-2024-001",
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  
  "event": "PaymentProcessed",
  "message": "Payment processed successfully",
  
  "metadata": {
    "orderId": "order-2024-001",
    "amount": 99.99,
    "currency": "ZMW",
    "customerId": "cust-789"
  },
  
  "performance": {
    "durationMs": 245,
    "retryCount": 0
  },
  
  "exception": {
    "type": "System.TimeoutException",
    "message": "Payment gateway timeout"
  }
}
```

---

## 4. Correlation and Tracing

### 4.1 Correlation ID Flow

The correlation ID is generated at the **entry point** (API Gateway or first service) and propagated through:

1. **HTTP Headers** (for synchronous calls):
   ```
   X-Correlation-Id: corr-a1b2c3d4
   X-Saga-Id: saga-order-123456
   X-Transaction-Id: order-2024-001
   ```

2. **Message Headers** (for async messaging):
   ```json
   {
     "headers": {
       "correlationId": "corr-a1b2c3d4",
       "sagaId": "saga-order-123456",
       "transactionId": "order-2024-001"
     }
   }
   ```


---

## 5. Observability Integration

Logs are ONE of the THREE pillars of observability:

### 5.1 The Three Pillars

```
┌──────────────────────────────────────┐
│         OBSERVABILITY                │
├──────────────┬──────────────┬────────┤
│    LOGS      │   METRICS    │ TRACES │
└──────────────┴──────────────┴────────┘
```

**Logs:** What happened? (Events, errors, messages)  
**Metrics:** How much? (CPU, memory, request rate, error rate)  
**Traces:** How long? (Request flow, latency breakdown)

### 5.2 Correlation Across Pillars

Use the **same correlation ID** in all three:

**Log Entry:**
```json
{
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "sagaId": "saga-123",
  "event": "PaymentProcessed"
}
```

**Trace Span:**
```json
{
  "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spanId": "00f067aa0ba902b7",
  "operationName": "ProcessPayment",
  "tags": {"sagaId": "saga-123"}
}
```

**Metric:**
```
saga_step_duration_seconds{saga_id="saga-123", step="ProcessPayment"} 0.245
```

---

## 6. Success Metrics

Your logging strategy is successful when you can answer these questions **in under 30 seconds:**

### Production Incident Response

1. **Which saga failed?**
   - Query: `event:"SagaAborted" AND timestamp:[now-1h TO now]`

2. **At which step did it fail?**
   - Query: `sagaId:"saga-123" AND event:"SagaStepFailed"`

3. **Why did it fail?**
   - Query: `sagaId:"saga-123" AND level:"Error"`

4. **Was compensation triggered?**
   - Query: `sagaId:"saga-123" AND event:"CompensationTriggered"`

5. **How many retries were attempted?**
   - Query: `sagaId:"saga-123" AND event:"SagaStepRetrying" | stats count`

6. **Which service is the root cause?**
   - Query: `sagaId:"saga-123" | group by service`

7. **How long did each step take?**
   - Query: `sagaId:"saga-123" | extract durationMs | visualize timeline`

8. **What was the customer impact?**
   - Query: `correlationId:"corr-456" AND level:"Error" | count by customerId`

### Business Analytics

1. **How many sagas completed successfully today?**
   - Query: `event:"SagaCompleted" AND timestamp:[now-1d TO now] | count`

2. **What's the average saga completion time?**
   - Query: `event:"SagaCompleted" | stats avg(totalDurationMs)`

3. **Which step fails most often?**
   - Query: `event:"SagaStepFailed" | stats count by step`

4. **What's the retry rate per step?**
   - Query: `event:"SagaStepRetrying" | stats count by step`

---

## Appendix A: Quick Reference

### Common Queries

**Find all failed sagas in last hour:**
```
event:"SagaAborted" AND timestamp:[now-1h TO now]
```

**Track specific saga:**
```
sagaId:"saga-order-123456"
```

**Find slow operations:**
```
event:"SlowExecution" AND durationMs:>2000
```

**Compensation failures (critical):**
```
event:"CompensationFailed"
```

---

## Appendix B: Implementation Checklist

- [ ] JSON structured logging configured in all services
- [ ] Correlation ID middleware implemented (C# and Go)
- [ ] All mandatory saga events logged
- [ ] Centralized log aggregation deployed
- [ ] Log retention policy defined
- [ ] Sensitive data masking implemented
- [ ] OpenTelemetry instrumentation added
- [ ] Alerting rules configured for critical events
- [ ] Dashboards created for saga monitoring
- [ ] Team trained on log querying
- [ ] Runbook created for common failure scenarios

---

## Appendix C: Alert Rules

### Critical Alerts (PagerDuty)

```yaml
- alert: SagaStateCorrupted
  expr: count(event{event="SagaStateCorrupted"}) > 0
  severity: critical
  
- alert: CompensationFailed
  expr: count(event{event="CompensationFailed"}) > 0
  severity: critical
  
- alert: DatabaseUnavailable
  expr: count(event{event="DatabaseUnavailable"}) > 0
  severity: critical
```

### Warning Alerts (Slack)

```yaml
- alert: HighCompensationRate
  expr: rate(event{event="CompensationTriggered"}[5m]) > 0.1
  severity: warning
  
- alert: HighRetryRate
  expr: rate(event{event="SagaStepRetrying"}[5m]) > 0.2
  severity: warning
```

---

**Document Version:** 1.0  
**Last Updated:** February 13, 2026  
**Next Review:** May 2026