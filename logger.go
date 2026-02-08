package logger

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
)

func NewTestLogger() *slog.Logger {
	return slog.New(NewColorHandler(os.Stdout))
}

// ColorHandler обрабатывает логи с цветовым форматированием
type ColorHandler struct {
	Writer io.Writer
	HookFn func(ctx context.Context, r slog.Record)
	groups []string    // текущие группы (в порядке добавления)
	attrs  []slog.Attr // накопленные атрибуты

	mu sync.Mutex
}

// NewColorHandler создает новый ColorHandler
func NewColorHandler(w io.Writer) *ColorHandler {
	return &ColorHandler{
		Writer: w,
		groups: []string{},
		attrs:  []slog.Attr{},
	}
}

// WithGroup реализует slog.HandlerWithGroup
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	// Создаем новый handler с добавленной группой
	newHandler := &ColorHandler{
		Writer: h.Writer,
		HookFn: h.HookFn,
		groups: make([]string, len(h.groups)),
		attrs:  h.attrs, // разделяем атрибуты
	}
	copy(newHandler.groups, h.groups)
	newHandler.groups = append(newHandler.groups, name)
	return newHandler
}

// WithAttrs реализует slog.HandlerWithAttrs
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Создаем новый handler с добавленными атрибутами
	newHandler := &ColorHandler{
		Writer: h.Writer,
		HookFn: h.HookFn,
		groups: h.groups, // разделяем группы
		attrs:  append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
	}
	return newHandler
}

// Enabled всегда возвращает true (логируем все уровни)
func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle - применяет цвета и форматирует запись с поддержкой групп
func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {

	buf := newBuffer()
	defer buf.Free()

	// Вызываем хук ДО обработки основным handler'ом
	if h.HookFn != nil && r.Level >= slog.LevelError {
		h.HookFn(ctx, r)
	}

	// Формируем временную метку
	timeStr := r.Time.Format(time.TimeOnly)

	// Выбираем цвет в зависимости от уровня логирования
	var levelColor *color.Color
	var msgColor *color.Color // цвет сообщения
	var levelStr string

	switch r.Level {
	case slog.LevelDebug:
		levelColor = color.New(color.FgHiCyan)
		msgColor = color.New(color.FgHiCyan) // подсвечиваем сообщение Debug
		levelStr = "DBG"
	case slog.LevelInfo:
		levelColor = color.New(color.FgGreen)
		msgColor = color.New(color.FgGreen) // подсвечиваем сообщение Info
		levelStr = "INF"
	case slog.LevelWarn:
		levelColor = color.New(color.FgHiYellow)
		msgColor = color.New(color.FgHiWhite)
		levelStr = "WRN"
	case slog.LevelError:
		levelColor = color.New(color.FgHiRed)
		msgColor = color.New(color.FgHiWhite)
		levelStr = "ERR"
	default:
		levelColor = color.New(color.FgWhite)
		msgColor = color.New(color.FgHiWhite)
		levelStr = "???"
	}

	// Собираем красивую строку
	_, err := color.New(color.FgHiBlue).Fprintf(buf, "[%s] ", timeStr)
	if err != nil {
		return err
	}
	_, err = levelColor.Fprintf(buf, "%-3s ", levelStr)
	if err != nil {
		return err
	}

	// Выводим группы в правильном порядке (слева направо)
	if len(h.groups) > 0 {
		for _, group := range h.groups {
			color.New(color.FgHiBlue).Fprintf(buf, "%s.", group)
		}
	}

	_, err = msgColor.Fprintf(buf, "%s", r.Message)
	if err != nil {
		return err
	}

	// Обрабатываем предварительно накопленные атрибуты (из WithAttrs)
	h.processAttrs(buf, h.attrs)

	// Обрабатываем атрибуты из записи
	r.Attrs(func(attr slog.Attr) bool {
		h.processAttr(buf, attr)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err = io.WriteString(buf, "\n")
	_, err = h.Writer.Write(*buf)

	return err
}

// processAttrs обрабатывает массив атрибутов
func (h *ColorHandler) processAttrs(buf *buffer, attrs []slog.Attr) {
	for _, attr := range attrs {
		h.processAttr(buf, attr)
	}
}

// processAttr обрабатывает один атрибут с учетом групп
func (h *ColorHandler) processAttr(buf *buffer, attr slog.Attr) {
	// Обрабатываем вложенные группы
	if attr.Value.Kind() == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		// Создаем временный handler для обработки группы
		groupHandler := &ColorHandler{
			Writer: buf,
			groups: append(h.groups[:len(h.groups):len(h.groups)], attr.Key),
		}
		for _, groupAttr := range groupAttrs {
			groupHandler.processAttr(buf, groupAttr)
		}
		return
	}

	// Выводим группы перед ключом (в правильном порядке)
	// if len(h.groups) > 0 {
	// 	color.New(color.FgHiBlue).Fprintf(h.Writer, " ")
	// 	for _, group := range h.groups {
	// 		color.New(color.FgHiBlue).Fprintf(h.Writer, "%s.", group)
	// 	}
	// }

	// Выводим ключ и значение
	color.New(color.FgHiGreen).Fprintf(buf, " %s=", attr.Key)
	color.New(color.FgHiYellow).Fprintf(buf, "%v", formatValue(attr.Value))
}

// formatValue форматирует значение атрибута
func formatValue(v slog.Value) interface{} {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		return formatAnyValue(v.Any())
	default:
		return v.Any()
	}
}

func formatAnyValue(value interface{}) interface{} {
	// Если значение уже является JSON-строкой, возвращаем как есть

	switch v := value.(type) {
	case error:
		return v
	}

	if str, ok := value.(string); ok {
		if isJSON(str) {
			return json.RawMessage(str)
		}
		return str
	}

	// Для сложных структур пытаемся преобразовать в JSON
	jsonBytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return value // Возвращаем как есть при ошибке
	}

	return string(jsonBytes)
}

func isJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

// SetHook устанавливает функцию хука для ошибок
func (h *ColorHandler) SetHook(fn func(ctx context.Context, r slog.Record)) {
	h.HookFn = fn
}
