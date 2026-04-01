package logger

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/rs/zerolog"
)

// Info logs an informational event.
func Info(ctx context.Context, fnName string, msg string, opts ...Option) {
	logWithLevel(ctx, zerolog.InfoLevel, fnName, "", msg, opts...)
}

// Critical logs a critical event.
// REQUIRED FIELD: fnName and errorPath must both be provided and non-empty.
func Critical(ctx context.Context, fnName, errorPath, msg string, opts ...Option) {
	if fnName == "" || errorPath == "" {
		panic("Critical requires non-empty function and error_path")
	}
	logWithLevel(ctx, criticalZerologLevel(), fnName, errorPath, msg, opts...)
}

// Warn logs a warning event.
// REQUIRED FIELD: fnName and errorPath must both be provided and non-empty.
func Warn(ctx context.Context, fnName, errorPath, msg string, opts ...Option) {
	if fnName == "" || errorPath == "" {
		panic("Warn requires non-empty function and error_path")
	}
	logWithLevel(ctx, zerolog.WarnLevel, fnName, errorPath, msg, opts...)
}

// Error logs an error event.
// REQUIRED FIELD: fnName and errorPath must both be provided and non-empty.
func Error(ctx context.Context, fnName, errorPath, msg string, opts ...Option) {
	if fnName == "" || errorPath == "" {
		panic("Error requires non-empty function and error_path")
	}
	logWithLevel(ctx, zerolog.ErrorLevel, fnName, errorPath, msg, opts...)
}

// Option allows adding optional metadata to an Event.
// EXTENSION POINT: Add more option helpers for additional fields.
type Option func(*Event)

// WithEnvironment sets the deployment environment, overriding the config default.
func WithEnvironment(env string) Option {
	return func(e *Event) {
		e.Environment = env
	}
}

// WithEvent sets the canonical event field.
func WithEvent(evName string) Option {
	return func(e *Event) {
		e.Event = evName
	}
}

// WithMetadata attaches arbitrary business-context key/value pairs; sensitive keys are redacted.
func WithMetadata(metadata map[string]any) Option {
	return func(e *Event) {
		e.Metadata = metadata
	}
}

// WithDurationMs records the operation duration in milliseconds.
func WithDurationMs(ms int64) Option {
	return func(e *Event) {
		e.DurationMs = ms
	}
}

// WithRetryCount records the number of retry attempts made.
func WithRetryCount(count int) Option {
	return func(e *Event) {
		e.RetryCount = count
	}
}

// WithSubscribeSubject sets the NATS subject the service consumed this event from.
func WithSubscribeSubject(subject string) Option {
	return func(e *Event) {
		e.SubscribeSubject = subject
	}
}

// WithPublishSubject sets the NATS subject the service is publishing this event to.
func WithPublishSubject(subject string) Option {
	return func(e *Event) {
		e.PublishSubject = subject
	}
}

// WithException populates the structured exception field.
// If err is nil, this option does nothing.
func WithException(err error) Option {
	return func(e *Event) {
		if err == nil {
			return
		}
		typ := fmt.Sprintf("%T", err)
		msg := err.Error()
		if errors.Is(err, context.Canceled) {
			typ = "context.Canceled"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			typ = "context.DeadlineExceeded"
		}
		e.Exception = &Exception{
			Type:    typ,
			Message: msg,
			Stack:   string(debug.Stack()),
		}
	}
}

func logWithLevel(ctx context.Context, level zerolog.Level, fnName, errorPath, msg string, opts ...Option) {
	cfg := ConfigOrDefault()
	ctx, traceID := TraceIDFromContext(ctx)
	_ = ctx // context is used for ID propagation; retained for future extension.

	ev := &Event{
		Service:     cfg.Service,
		Environment: cfg.Env,
		TraceID:     traceID,
		Function:    fnName,
		ErrorPath:   errorPath,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(ev)
		}
	}

	// Ensure environment is always present even if options override to empty.
	if ev.Environment == "" {
		ev.Environment = cfg.Env
	}

	logger := Logger()
	ze := logger.With().
		Str("trace_id", ev.TraceID).
		Str("function", ev.Function).
		Str("error_path", ev.ErrorPath).
		Str("event", ev.Event).
		Int("retry_count", ev.RetryCount).
		Int64("duration_ms", ev.DurationMs).
		Interface("metadata", redactMap(cfg.RedactKeys, ev.Metadata)).
		Interface("exception", ev.Exception).
		Logger()

	if ev.SubscribeSubject != "" {
		ze = ze.With().Str("subscribe_subject", ev.SubscribeSubject).Logger()
	}
	if ev.PublishSubject != "" {
		ze = ze.With().Str("publish_subject", ev.PublishSubject).Logger()
	}
	event := ze

	switch level {
	case zerolog.InfoLevel:
		event.Info().Msg(msg)
	case zerolog.WarnLevel:
		event.Warn().Msg(msg)
	case zerolog.ErrorLevel:
		event.Error().Msg(msg)
	default:
		// Critical uses a custom numeric level. Treat it as an error emission while keeping level alignment.
		if level == criticalZerologLevel() {
			event.WithLevel(zerolog.ErrorLevel).Str("level", "critical").Msg(msg)
			return
		}
		event.Log().Msg(msg)
	}
}

func criticalZerologLevel() zerolog.Level {
	// Use a custom level outside zerolog's built-in set; we still emit via ErrorLevel for writers.
	return zerolog.Level(99)
}

// redactMap masks values whose keys match redactKeys. Used by both the logger and seq writer.
func redactMap(redactKeys []string, m map[string]any) map[string]any {
	if len(m) == 0 || len(redactKeys) == 0 {
		return m
	}

	redactSet := make(map[string]struct{}, len(redactKeys))
	for _, k := range redactKeys {
		k = strings.TrimSpace(strings.ToLower(k))
		if k != "" {
			redactSet[k] = struct{}{}
		}
	}
	if len(redactSet) == 0 {
		return m
	}

	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, ok := redactSet[strings.ToLower(k)]; ok {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}
