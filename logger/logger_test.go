package logger

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// assertPanics asserts that calling fn panics.
func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic but none occurred")
		}
	}()
	fn()
}

// captureLastEvent initialises logging with EnableFile: true in a temp dir,
// runs fn, then reads and returns the last JSON event from the log file.
func captureLastEvent(t *testing.T, cfg Config, fn func()) map[string]any {
	t.Helper()
	cfg.LogDir = t.TempDir()
	cfg.EnableFile = true
	InitConfig(cfg)
	fn()
	return readLastEvent(t, filepath.Join(cfg.LogDir, "app.log"))
}

// --- Core functions ---

func TestInfoEmitsInfoLevel(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
		EnableSeq:   false,
	}, func() {
		Info(context.Background(), "fn", "hello info")
	})
	assertEq(t, ev["level"], "info")
	assertEq(t, ev["message"], "hello info")
}

func TestErrorEmitsErrorLevel(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
		EnableSeq:   false,
	}, func() {
		Error(context.Background(), "fn", "/path", "something broke")
	})
	assertEq(t, ev["level"], "error")
	assertEq(t, ev["message"], "something broke")
}

func TestLogIncludesStrategyFieldsAndRedactsMetadata(t *testing.T) {
	tmp := t.TempDir()

	InitConfig(Config{
		Service:        "test-service",
		Env:            "test",
		ServiceVersion: "1.2.3",
		LogDir:         tmp,
		EnableFile:     true,
		ConsoleJSON:    true,
		EnableSeq:      false,
		RedactKeys:     []string{"token", "password"},
	})

	ctx := context.Background()
	ctx = WithTraceID(ctx, "corr-123")

	Warn(ctx, "fn", "/path", "something happened",
		WithComponent("Participant"),
		WithEvent("SagaStepRetrying"),
		WithDurationMs(10),
		WithMetadata(map[string]any{
			"order_id": "order-1",
			"token":    "secret",
		}),
	)

	ev := readLastEvent(t, filepath.Join(tmp, "app.log"))

	assertEq(t, ev["service"], "test-service")
	assertEq(t, ev["environment"], "test")
	assertEq(t, ev["service_version"], "1.2.3")
	assertEq(t, ev["trace_id"], "corr-123")
	assertEq(t, ev["component"], "Participant")
	assertEq(t, ev["event"], "SagaStepRetrying")
	assertEq(t, ev["function"], "fn")
	assertEq(t, ev["error_path"], "/path")
	assertEq(t, ev["duration_ms"], float64(10)) // json decodes numbers as float64

	if _, ok := ev["timestamp"]; !ok {
		t.Fatalf("expected timestamp field to be present; got keys=%v", keys(ev))
	}
	if _, ok := ev["level"]; !ok {
		t.Fatalf("expected level field to be present; got keys=%v", keys(ev))
	}

	md, _ := ev["metadata"].(map[string]any)
	if md == nil {
		t.Fatalf("expected metadata object to be present")
	}
	assertEq(t, md["order_id"], "order-1")
	assertEq(t, md["token"], "[REDACTED]")
}

func TestCriticalLevelEmitsCritical(t *testing.T) {
	tmp := t.TempDir()

	InitConfig(Config{
		Service:     "test-service",
		Env:         "test",
		LogDir:      tmp,
		EnableFile:  true,
		ConsoleJSON: true,
		EnableSeq:   false,
	})

	ctx := context.Background()
	ctx = WithTraceID(ctx, "corr-123")

	Critical(ctx, "fn", "/path", "boom")

	ev := readLastEvent(t, filepath.Join(tmp, "app.log"))
	assertEq(t, ev["level"], "critical")
}

// --- Panic guards ---

func TestWarnPanicsOnEmptyFnName(t *testing.T) {
	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	assertPanics(t, func() {
		Warn(context.Background(), "", "/path", "msg")
	})
}

func TestErrorPanicsOnEmptyErrorPath(t *testing.T) {
	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	assertPanics(t, func() {
		Error(context.Background(), "fn", "", "msg")
	})
}

func TestCriticalPanicsOnEmptyArgs(t *testing.T) {
	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	assertPanics(t, func() {
		Critical(context.Background(), "", "", "msg")
	})
}

// --- Context ---

func TestWithTraceIDGeneratesUUIDWhenEmpty(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		ctx := WithTraceID(context.Background(), "")
		Info(ctx, "fn", "msg")
	})
	id, _ := ev["trace_id"].(string)
	if id == "" {
		t.Fatal("expected non-empty trace_id when empty string passed to WithTraceID")
	}
}

func TestTraceIDFromContextGeneratesWhenMissing(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg")
	})
	id, _ := ev["trace_id"].(string)
	if id == "" {
		t.Fatal("expected auto-generated trace_id for bare context")
	}
}

func TestTraceIDFromContextNilContext(t *testing.T) {
	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	// Should not panic; nil context treated as background.
	defer func() {
		if r := recover(); r != nil {
			// A panic here is a bug — nil context support is expected.
			t.Fatalf("unexpected panic with nil context: %v", r)
		}
	}()
	//nolint:staticcheck // intentional nil context test
	ctx, id := TraceIDFromContext(nil)
	_ = ctx
	if id == "" {
		t.Fatal("expected non-empty trace_id for nil context")
	}
}

// --- Config ---

func TestInitConfigPanicsOnEmptyService(t *testing.T) {
	assertPanics(t, func() {
		InitConfig(Config{})
	})
}

func TestConfigOrDefaultPanicsWhenNotInitialised(t *testing.T) {
	// Temporarily wipe global state.
	saved := globalConfig
	globalConfig = Config{}
	t.Cleanup(func() { globalConfig = saved })

	assertPanics(t, func() {
		ConfigOrDefault()
	})
}

func TestLogDirFallsBackToEnvVar(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOG_DIR", tmp)

	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	cfg := ConfigOrDefault()
	if cfg.LogDir != tmp {
		t.Fatalf("expected LogDir=%q from env, got %q", tmp, cfg.LogDir)
	}
}

func TestEnvDefaultsToDevelopment(t *testing.T) {
	InitConfig(Config{Service: "svc", ConsoleJSON: true})
	cfg := ConfigOrDefault()
	if cfg.Env != "development" {
		t.Fatalf("expected Env=development, got %q", cfg.Env)
	}
}

// --- EnableFile ---

func TestEnableFileFalseDoesNotCreateFile(t *testing.T) {
	tmp := t.TempDir()
	InitConfig(Config{
		Service:    "svc",
		Env:        "test",
		LogDir:     tmp,
		EnableFile: false,
		EnableSeq:  false,
	})
	Info(context.Background(), "fn", "msg")

	path := filepath.Join(tmp, "app.log")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no app.log when EnableFile=false")
	}
}

func TestEnableFileTrueCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	InitConfig(Config{
		Service:    "svc",
		Env:        "test",
		LogDir:     tmp,
		EnableFile: true,
		EnableSeq:  false,
	})
	Info(context.Background(), "fn", "msg")

	path := filepath.Join(tmp, "app.log")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected app.log to exist when EnableFile=true: %v", err)
	}
}

// --- Seq env vars ---

func TestSeqConfigPopulatedFromEnvVars(t *testing.T) {
	t.Setenv("SEQ_ENABLE", "true")
	t.Setenv("SEQ_URL", "http://seq-test")
	t.Setenv("SEQ_API_KEY", "testkey")

	InitConfig(Config{Service: "svc", Env: "test", ConsoleJSON: true})
	cfg := ConfigOrDefault()

	if !cfg.EnableSeq {
		t.Fatal("expected EnableSeq=true from SEQ_ENABLE env var")
	}
	if cfg.SeqURL != "http://seq-test" {
		t.Fatalf("expected SeqURL=http://seq-test, got %q", cfg.SeqURL)
	}
	if cfg.SeqAPIKey != "testkey" {
		t.Fatalf("expected SeqAPIKey=testkey, got %q", cfg.SeqAPIKey)
	}
}

// --- WithException ---

func TestWithExceptionNilDoesNothing(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg", WithException(nil))
	})
	if ev["exception"] != nil {
		t.Fatal("expected exception=null for nil error")
	}
}

func TestWithExceptionSetsFields(t *testing.T) {
	testErr := &testError{msg: "boom"}
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Error(context.Background(), "fn", "/path", "err", WithException(testErr))
	})
	exc, _ := ev["exception"].(map[string]any)
	if exc == nil {
		t.Fatal("expected exception object")
	}
	if exc["message"] != "boom" {
		t.Fatalf("expected exception.message=boom, got %v", exc["message"])
	}
}

func TestWithExceptionContextCanceled(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Error(context.Background(), "fn", "/path", "err", WithException(context.Canceled))
	})
	exc, _ := ev["exception"].(map[string]any)
	if exc == nil {
		t.Fatal("expected exception object")
	}
	if exc["type"] != "context.Canceled" {
		t.Fatalf("expected type=context.Canceled, got %v", exc["type"])
	}
}

func TestWithExceptionDeadlineExceeded(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Error(context.Background(), "fn", "/path", "err", WithException(context.DeadlineExceeded))
	})
	exc, _ := ev["exception"].(map[string]any)
	if exc == nil {
		t.Fatal("expected exception object")
	}
	if exc["type"] != "context.DeadlineExceeded" {
		t.Fatalf("expected type=context.DeadlineExceeded, got %v", exc["type"])
	}
}

// --- redactMap ---

func TestRedactMapNilInput(t *testing.T) {
	result := redactMap([]string{"token"}, nil)
	if result != nil {
		t.Fatal("expected nil result for nil input map")
	}
}

func TestRedactMapEmptyRedactKeys(t *testing.T) {
	m := map[string]any{"token": "secret"}
	result := redactMap([]string{}, m)
	if result["token"] != "secret" {
		t.Fatal("expected no redaction when redact keys are empty")
	}
}

func TestRedactMapCaseInsensitive(t *testing.T) {
	m := map[string]any{"TOKEN": "secret"}
	result := redactMap([]string{"token"}, m)
	if result["TOKEN"] != "[REDACTED]" {
		t.Fatalf("expected TOKEN to be redacted case-insensitively, got %v", result["TOKEN"])
	}
}

// --- Option helpers ---

func TestWithRetryCountApplied(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg", WithRetryCount(3))
	})
	assertEq(t, ev["retry_count"], float64(3))
}

func TestWithTopicApplied(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg", WithTopic("orders"))
	})
	assertEq(t, ev["topic"], "orders")
}

func TestWithMessageIDApplied(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg", WithMessageID("msg-999"))
	})
	assertEq(t, ev["message_id"], "msg-999")
}

func TestWithTraceApplied(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Info(context.Background(), "fn", "msg", WithTrace("trace-abc", "span-xyz"))
	})
	assertEq(t, ev["trace_id"], "trace-abc")
	assertEq(t, ev["span_id"], "span-xyz")
}

func TestWithExceptionApplied(t *testing.T) {
	ev := captureLastEvent(t, Config{
		Service:     "svc",
		Env:         "test",
		ConsoleJSON: true,
	}, func() {
		Error(context.Background(), "fn", "/path", "err",
			WithException(&testError{msg: "applied"}))
	})
	exc, _ := ev["exception"].(map[string]any)
	if exc == nil {
		t.Fatal("expected exception to be set")
	}
	assertEq(t, exc["message"], "applied")
}

// --- helpers ---

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func readLastEvent(t *testing.T, path string) map[string]any {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}
	defer f.Close()

	var last string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line != "" {
			last = line
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan log file: %v", err)
	}
	if last == "" {
		t.Fatalf("expected at least one log line")
	}

	var ev map[string]any
	if err := json.Unmarshal([]byte(last), &ev); err != nil {
		t.Fatalf("unmarshal log json: %v; line=%q", err, last)
	}
	return ev
}

func assertEq(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("got=%v (%T) want=%v (%T)", got, got, want, want)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
