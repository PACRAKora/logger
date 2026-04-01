//go:build !otel

package logger

import "context"

// WithTraceFromContext is a no-op unless built with -tags=otel.
func WithTraceFromContext(ctx context.Context) Option {
	return func(e *Event) {
		_ = ctx
	}
}

