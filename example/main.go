package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	logger "github.com/golub15/slog_color"
)

// ──────────────────────────────────────────────────────────
// Вспомогательная функция для разделителей между секциями
// ──────────────────────────────────────────────────────────

func section(title string) {
	fmt.Printf("\n%s\n  %s\n%s\n\n",
		strings.Repeat("─", 60),
		title,
		strings.Repeat("─", 60),
	)
}

// ──────────────────────────────────────────────────────────
// Пример структуры для JSON-сериализации
// ──────────────────────────────────────────────────────────

type UserProfile struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type ServerConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TLS      bool   `json:"tls"`
	MaxConns int    `json:"max_conns"`
}

func main() {
	// ──────────────────────────────────────────────────────
	// 1. Базовое использование — все уровни логирования
	// ──────────────────────────────────────────────────────

	section("1. Базовое использование")

	log := slog.New(logger.NewColorHandler(os.Stdout))

	log.Debug("загрузка конфигурации", "path", "/etc/app/config.yaml")
	log.Info("сервер запущен", "port", 8080, "env", "production")
	log.Warn("высокое использование диска", "percent", 91.4, "mount", "/data")
	log.Error("не удалось подключиться к БД", "host", "db.local", "err", "connection refused")

	// ──────────────────────────────────────────────────────
	// 2. Постоянные атрибуты через With
	// ──────────────────────────────────────────────────────

	section("2. Постоянные атрибуты (With)")

	reqLog := log.With("request_id", "req-abc-123", "user_id", 42)

	reqLog.Info("авторизация пройдена")
	reqLog.Info("загрузка данных", "table", "products")
	reqLog.Warn("медленный запрос", "duration", 1200*time.Millisecond)

	// ──────────────────────────────────────────────────────
	// 3. Группы
	// ──────────────────────────────────────────────────────

	section("3. Группы (WithGroup)")

	httpLog := log.WithGroup("http")
	httpLog.Info("входящий запрос", "method", "GET", "path", "/api/users")
	httpLog.Info("ответ отправлен", "status", 200, "bytes", 4096)

	// Вложенные группы
	dbLog := log.WithGroup("storage").WithGroup("postgres")
	dbLog.Error("ошибка запроса", "table", "orders", "err", "deadlock detected")

	// ──────────────────────────────────────────────────────
	// 4. Группы атрибутов (slog.Group)
	// ──────────────────────────────────────────────────────

	section("4. Группы атрибутов (slog.Group)")

	log.Info("заказ создан",
		slog.Group("customer",
			slog.String("name", "Alice"),
			slog.Int("id", 7),
		),
		slog.Group("order",
			slog.String("sku", "WIDGET-9"),
			slog.Int("qty", 3),
			slog.Float64("total", 149.97),
		),
	)

	// ──────────────────────────────────────────────────────
	// 5. Разнообразие типов значений
	// ──────────────────────────────────────────────────────

	section("5. Типы значений")

	log.Info("все типы",
		"string", "hello",
		"int", 42,
		"float", 3.14159,
		"bool", true,
		"duration", 5*time.Second,
		"time", time.Now(),
	)

	// ──────────────────────────────────────────────────────
	// 6. Структуры → JSON
	// ──────────────────────────────────────────────────────

	section("6. Структуры (автоматический JSON)")

	user := UserProfile{
		ID:    1,
		Name:  "Bob",
		Email: "bob@example.com",
		Role:  "admin",
	}

	cfg := ServerConfig{
		Host:     "0.0.0.0",
		Port:     443,
		TLS:      true,
		MaxConns: 1000,
	}

	log.Info("профиль пользователя", "user", user)
	log.Info("конфигурация сервера", "config", cfg)

	// ──────────────────────────────────────────────────────
	// 7. Ошибки
	// ──────────────────────────────────────────────────────

	section("7. Ошибки")

	err := errors.New("file not found: /var/data/report.csv")
	log.Error("не удалось прочитать отчёт", "err", err, "retry", false)

	wrappedErr := fmt.Errorf("обработка платежа: %w", errors.New("insufficient funds"))
	log.Error("платёж отклонён", "err", wrappedErr, "order_id", "ORD-99")

	// ──────────────────────────────────────────────────────
	// 8. JSON-строки как значения
	// ──────────────────────────────────────────────────────

	section("8. JSON-строки")

	payload := `{"action":"purchase","items":["book","pen"]}`
	log.Info("получен webhook", "payload", payload)

	// ──────────────────────────────────────────────────────
	// 9. Хук на ошибки (SetHook)
	// ──────────────────────────────────────────────────────

	section("9. Хук на ошибки (SetHook)")

	handler := logger.NewColorHandler(os.Stdout)
	handler.SetHook(func(ctx context.Context, r slog.Record) {
		fmt.Printf("  ⚡ HOOK: перехвачена ошибка → %q\n", r.Message)
	})
	hookLog := slog.New(handler)

	hookLog.Info("обычное сообщение (хук НЕ сработает)")
	hookLog.Warn("предупреждение (хук НЕ сработает)")
	hookLog.Error("критическая ошибка (хук СРАБОТАЕТ)", "code", 500)

	// ──────────────────────────────────────────────────────
	// 10. Быстрый логгер для тестов
	// ──────────────────────────────────────────────────────

	section("10. Быстрый логгер (NewTestLogger)")

	testLog := logger.NewTestLogger()
	testLog.Info("запуск теста", "suite", "integration", "case", "happy path")
	testLog.Debug("проверка завершена", "assertions", 12, "passed", true)

	fmt.Println()
}
