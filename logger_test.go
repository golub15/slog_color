package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fatih/color"
)

// init отключает цветовой вывод для детерминированного тестирования
func init() {
	color.NoColor = true
}

// ──────────────────────────────────────────────────────────
// Хелперы
// ──────────────────────────────────────────────────────────

// newTestHandler создает handler с буфером для захвата вывода
func newTestHandler() (*ColorHandler, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	h := NewColorHandler(buf)
	return h, buf
}

// newTestRecord создает запись с фиксированным временем
func newTestRecord(level slog.Level, msg string) slog.Record {
	return slog.NewRecord(
		time.Date(2026, 2, 8, 12, 30, 45, 0, time.UTC),
		level,
		msg,
		0,
	)
}

// ──────────────────────────────────────────────────────────
// NewColorHandler / NewTestLogger
// ──────────────────────────────────────────────────────────

func TestNewColorHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewColorHandler(buf)

	if h == nil {
		t.Fatal("NewColorHandler вернул nil")
	}
	if h.Writer != buf {
		t.Error("Writer не совпадает с переданным")
	}
	if len(h.groups) != 0 {
		t.Error("groups должны быть пустыми при создании")
	}
	if len(h.attrs) != 0 {
		t.Error("attrs должны быть пустыми при создании")
	}
	if h.HookFn != nil {
		t.Error("HookFn должен быть nil при создании")
	}
}

func TestNewTestLogger(t *testing.T) {
	l := NewTestLogger()
	if l == nil {
		t.Fatal("NewTestLogger вернул nil")
	}
}

// ──────────────────────────────────────────────────────────
// Enabled
// ──────────────────────────────────────────────────────────

func TestEnabled(t *testing.T) {
	h, _ := newTestHandler()
	ctx := context.Background()

	levels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
		slog.Level(42), // произвольный уровень
	}

	for _, lvl := range levels {
		if !h.Enabled(ctx, lvl) {
			t.Errorf("Enabled(%v) = false, ожидалось true", lvl)
		}
	}
}

// ──────────────────────────────────────────────────────────
// Handle — уровни логирования
// ──────────────────────────────────────────────────────────

func TestHandle_LevelLabels(t *testing.T) {
	tests := []struct {
		level    slog.Level
		wantTag  string
		wantMsg  string
	}{
		{slog.LevelDebug, "DBG", "debug message"},
		{slog.LevelInfo, "INF", "info message"},
		{slog.LevelWarn, "WRN", "warning message"},
		{slog.LevelError, "ERR", "error message"},
		{slog.Level(42), "???", "unknown level"},
	}

	for _, tt := range tests {
		t.Run(tt.wantTag, func(t *testing.T) {
			h, buf := newTestHandler()
			r := newTestRecord(tt.level, tt.wantMsg)

			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle вернул ошибку: %v", err)
			}

			out := buf.String()
			if !strings.Contains(out, tt.wantTag) {
				t.Errorf("вывод не содержит метку уровня %q: %s", tt.wantTag, out)
			}
			if !strings.Contains(out, tt.wantMsg) {
				t.Errorf("вывод не содержит сообщение %q: %s", tt.wantMsg, out)
			}
		})
	}
}

func TestHandle_TimeFormat(t *testing.T) {
	h, buf := newTestHandler()
	r := newTestRecord(slog.LevelInfo, "test")

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	// time.TimeOnly = "15:04:05"
	if !strings.Contains(buf.String(), "12:30:45") {
		t.Errorf("вывод не содержит ожидаемое время: %s", buf.String())
	}
}

func TestHandle_NewlineAtEnd(t *testing.T) {
	h, buf := newTestHandler()
	r := newTestRecord(slog.LevelInfo, "msg")

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("вывод должен заканчиваться переводом строки")
	}
}

// ──────────────────────────────────────────────────────────
// Handle — атрибуты в записи
// ──────────────────────────────────────────────────────────

func TestHandle_RecordAttrs(t *testing.T) {
	h, buf := newTestHandler()
	r := newTestRecord(slog.LevelInfo, "with attrs")
	r.AddAttrs(
		slog.String("key", "value"),
		slog.Int("count", 42),
		slog.Bool("active", true),
	)

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"key=", "value", "count=", "42", "active=", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("вывод не содержит %q: %s", want, out)
		}
	}
}

// ──────────────────────────────────────────────────────────
// WithAttrs
// ──────────────────────────────────────────────────────────

func TestWithAttrs(t *testing.T) {
	h, buf := newTestHandler()
	h2 := h.WithAttrs([]slog.Attr{
		slog.String("service", "api"),
	})

	r := newTestRecord(slog.LevelInfo, "with preattrs")
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "service=") || !strings.Contains(out, "api") {
		t.Errorf("предварительные атрибуты не попали в вывод: %s", out)
	}
}

func TestWithAttrs_DoesNotMutateOriginal(t *testing.T) {
	h, _ := newTestHandler()
	_ = h.WithAttrs([]slog.Attr{slog.String("extra", "val")})

	if len(h.attrs) != 0 {
		t.Error("WithAttrs изменил атрибуты оригинального handler'а")
	}
}

// ──────────────────────────────────────────────────────────
// WithGroup
// ──────────────────────────────────────────────────────────

func TestWithGroup(t *testing.T) {
	h, buf := newTestHandler()
	h2 := h.WithGroup("request")

	r := newTestRecord(slog.LevelInfo, "grouped msg")
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "request.") {
		t.Errorf("группа не появилась в выводе: %s", out)
	}
}

func TestWithGroup_Nested(t *testing.T) {
	h, buf := newTestHandler()
	h2 := h.WithGroup("a").WithGroup("b").WithGroup("c")

	r := newTestRecord(slog.LevelInfo, "deep")
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	out := buf.String()
	// Все группы должны идти в порядке a.b.c.
	if !strings.Contains(out, "a.") || !strings.Contains(out, "b.") || !strings.Contains(out, "c.") {
		t.Errorf("вложенные группы не появились в выводе: %s", out)
	}
}

func TestWithGroup_DoesNotMutateOriginal(t *testing.T) {
	h, _ := newTestHandler()
	_ = h.WithGroup("grp")

	if len(h.groups) != 0 {
		t.Error("WithGroup изменил группы оригинального handler'а")
	}
}

// ──────────────────────────────────────────────────────────
// SetHook / HookFn
// ──────────────────────────────────────────────────────────

func TestSetHook_CalledOnError(t *testing.T) {
	h, _ := newTestHandler()

	var hookCalled bool
	var capturedMsg string
	h.SetHook(func(ctx context.Context, r slog.Record) {
		hookCalled = true
		capturedMsg = r.Message
	})

	r := newTestRecord(slog.LevelError, "something broke")
	_ = h.Handle(context.Background(), r)

	if !hookCalled {
		t.Error("хук не вызван при уровне Error")
	}
	if capturedMsg != "something broke" {
		t.Errorf("хук получил неверное сообщение: %q", capturedMsg)
	}
}

func TestSetHook_NotCalledBelowError(t *testing.T) {
	h, _ := newTestHandler()

	hookCalled := false
	h.SetHook(func(ctx context.Context, r slog.Record) {
		hookCalled = true
	})

	for _, lvl := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn} {
		hookCalled = false
		r := newTestRecord(lvl, "low level")
		_ = h.Handle(context.Background(), r)

		if hookCalled {
			t.Errorf("хук вызван при уровне %v, ожидалось только >= Error", lvl)
		}
	}
}

func TestSetHook_Nil(t *testing.T) {
	h, _ := newTestHandler()
	// Хук не установлен — не должно быть паники
	r := newTestRecord(slog.LevelError, "no hook")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку без хука: %v", err)
	}
}

// ──────────────────────────────────────────────────────────
// formatValue
// ──────────────────────────────────────────────────────────

func TestFormatValue(t *testing.T) {
	now := time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		val  slog.Value
		want string
	}{
		{"string", slog.StringValue("hello"), "hello"},
		{"int64", slog.Int64Value(123), "123"},
		{"uint64", slog.Uint64Value(456), "456"},
		{"float64", slog.Float64Value(3.14), "3.14"},
		{"bool_true", slog.BoolValue(true), "true"},
		{"bool_false", slog.BoolValue(false), "false"},
		{"duration", slog.DurationValue(5 * time.Second), "5s"},
		{"time", slog.TimeValue(now), "2026-02-08T12:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf("%v", formatValue(tt.val))
			if got != tt.want {
				t.Errorf("formatValue(%v) = %q, ожидалось %q", tt.val, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────
// formatAnyValue
// ──────────────────────────────────────────────────────────

func TestFormatAnyValue_Error(t *testing.T) {
	err := errors.New("test error")
	got := formatAnyValue(err)
	if gotErr, ok := got.(error); !ok || gotErr.Error() != "test error" {
		t.Errorf("formatAnyValue(error) = %v, ожидалось error 'test error'", got)
	}
}

func TestFormatAnyValue_JSONString(t *testing.T) {
	jsonStr := `{"key":"value"}`
	got := formatAnyValue(jsonStr)
	if _, ok := got.(json.RawMessage); !ok {
		t.Errorf("formatAnyValue(JSON-строка) должен вернуть json.RawMessage, получил %T", got)
	}
}

func TestFormatAnyValue_PlainString(t *testing.T) {
	s := "plain text"
	got := formatAnyValue(s)
	if gotStr, ok := got.(string); !ok || gotStr != s {
		t.Errorf("formatAnyValue(обычная строка) = %v, ожидалось %q", got, s)
	}
}

func TestFormatAnyValue_Struct(t *testing.T) {
	type data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	d := data{Name: "Bob", Age: 30}
	got := formatAnyValue(d)
	gotStr, ok := got.(string)
	if !ok {
		t.Fatalf("formatAnyValue(struct) вернул %T, ожидалось string (JSON)", got)
	}
	if !strings.Contains(gotStr, `"name"`) || !strings.Contains(gotStr, `"Bob"`) {
		t.Errorf("formatAnyValue(struct) неверный JSON: %s", gotStr)
	}
}

// ──────────────────────────────────────────────────────────
// isJSON
// ──────────────────────────────────────────────────────────

func TestIsJSON(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"a":1}`, true},
		{`[1,2,3]`, true},
		{`"hello"`, true},
		{`42`, true},
		{`not json`, false},
		{`{broken`, false},
		{``, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isJSON(tt.input); got != tt.want {
				t.Errorf("isJSON(%q) = %v, ожидалось %v", tt.input, got, tt.want)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────
// Группа-атрибут (вложенная группа в записи)
// ──────────────────────────────────────────────────────────

func TestHandle_GroupAttr(t *testing.T) {
	h, buf := newTestHandler()
	r := newTestRecord(slog.LevelInfo, "nested group")
	r.AddAttrs(slog.Group("user",
		slog.String("name", "Alice"),
		slog.Int("id", 7),
	))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle вернул ошибку: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "name=") || !strings.Contains(out, "Alice") {
		t.Errorf("вложенная группа: не найден name=Alice: %s", out)
	}
	if !strings.Contains(out, "id=") || !strings.Contains(out, "7") {
		t.Errorf("вложенная группа: не найден id=7: %s", out)
	}
}

// ──────────────────────────────────────────────────────────
// Конкурентность
// ──────────────────────────────────────────────────────────

func TestHandle_Concurrent(t *testing.T) {
	h, buf := newTestHandler()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			r := newTestRecord(slog.LevelInfo, fmt.Sprintf("msg-%d", n))
			_ = h.Handle(context.Background(), r)
		}(i)
	}

	wg.Wait()

	// Проверяем, что все сообщения записаны (по количеству строк)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != goroutines {
		t.Errorf("ожидалось %d строк, получено %d", goroutines, len(lines))
	}
}

// ──────────────────────────────────────────────────────────
// Интеграция через slog.Logger
// ──────────────────────────────────────────────────────────

func TestSlogLogger_Integration(t *testing.T) {
	h, buf := newTestHandler()
	l := slog.New(h)

	l.Info("hello world", "user", "test")

	out := buf.String()
	if !strings.Contains(out, "INF") {
		t.Errorf("интеграция: нет метки INF: %s", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("интеграция: нет сообщения: %s", out)
	}
	if !strings.Contains(out, "user=") || !strings.Contains(out, "test") {
		t.Errorf("интеграция: нет атрибута user=test: %s", out)
	}
}

func TestSlogLogger_WithGroupIntegration(t *testing.T) {
	h, buf := newTestHandler()
	l := slog.New(h).WithGroup("http")

	l.Warn("slow request", "path", "/api", "ms", 500)

	out := buf.String()
	if !strings.Contains(out, "WRN") {
		t.Errorf("интеграция WithGroup: нет метки WRN: %s", out)
	}
	if !strings.Contains(out, "http.") {
		t.Errorf("интеграция WithGroup: нет группы http.: %s", out)
	}
	if !strings.Contains(out, "path=") || !strings.Contains(out, "/api") {
		t.Errorf("интеграция WithGroup: нет атрибута path=/api: %s", out)
	}
}

func TestSlogLogger_WithAttrsIntegration(t *testing.T) {
	h, buf := newTestHandler()
	l := slog.New(h).With("env", "staging")

	l.Debug("starting")

	out := buf.String()
	if !strings.Contains(out, "DBG") {
		t.Errorf("интеграция With: нет метки DBG: %s", out)
	}
	if !strings.Contains(out, "env=") || !strings.Contains(out, "staging") {
		t.Errorf("интеграция With: нет атрибута env=staging: %s", out)
	}
}
