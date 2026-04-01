package logger

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const (
	// CONFIGURABLE: Context key names can be changed if needed.
	ctxKeyTraceID ctxKey = "trace_id"
)

// WithTraceID attaches the given trace ID to the context.
// REQUIRED FIELD: Trace ID is used to tie together related log events.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}

// TraceIDFromContext extracts the trace ID from the context.
// If none is found, a new one is generated and attached.
func TraceIDFromContext(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	if v := ctx.Value(ctxKeyTraceID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return ctx, s
		}
	}
	id := uuid.NewString()
	ctx = context.WithValue(ctx, ctxKeyTraceID, id)
	return ctx, id
}
