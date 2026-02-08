# slog_color

Готовый цветной handler для стандартного `log/slog` в Go, который делает логи в терминале удобными для чтения с первого взгляда.

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Версия Go" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="Лицензия" />
</p>

---

## Предпросмотр

```text
[14:05:32] INF  server started        port=8080 env=production
[14:05:32] DBG  loading config         path=/etc/app.yaml
[14:05:33] WRN  slow query             query=SELECT * FROM users ms=1200
[14:05:33] ERR  connection lost        host=db.local err=timeout
```

Каждый элемент окрашивается в терминале:

| Элемент          | Цвет              |
|------------------|--------------------|
| `[время]`        | яркий синий        |
| `DBG`            | яркий голубой      |
| `INF`            | зелёный            |
| `WRN`            | яркий жёлтый       |
| `ERR`            | яркий красный      |
| сообщение (DBG)  | яркий голубой      |
| сообщение (INF)  | зелёный            |
| сообщение (WRN/ERR) | яркий белый     |
| ключ атрибута    | яркий зелёный      |
| значение атрибута| яркий жёлтый       |
| имя группы       | яркий синий        |

---

## Установка

```bash
go get github.com/golub15/slog_color
```

---

## Быстрый старт

```go
package main

import (
    "log/slog"
    "os"

    logger "github.com/golub15/slog_color"
)

func main() {
    log := slog.New(logger.NewColorHandler(os.Stdout))

    log.Info("server started", "port", 8080)
    log.Debug("cache hit", "key", "user:42")
    log.Warn("disk usage high", "percent", 91.4)
    log.Error("failed to connect", "host", "db.local", "err", "timeout")
}
```

Вывод:

```text
[12:30:45] INF server started port=8080
[12:30:45] DBG cache hit key=user:42
[12:30:45] WRN disk usage high percent=91.4
[12:30:45] ERR failed to connect host=db.local err=timeout
```

---

## Возможности

### Группы

Добавьте логическое пространство имён, которое будет префиксом каждого сообщения:

```go
httpLog := log.WithGroup("http")

httpLog.Info("request received", "method", "GET", "path", "/api/users")
httpLog.Warn("slow response", "ms", 1200)
```

```text
[12:30:45] INF http.request received method=GET path=/api/users
[12:30:45] WRN http.slow response ms=1200
```

Группы можно вкладывать друг в друга:

```go
dbLog := log.WithGroup("storage").WithGroup("postgres")

dbLog.Error("query failed", "table", "orders")
```

```text
[12:30:45] ERR storage.postgres.query failed table=orders
```

---

### Постоянные атрибуты

Привяжите пары ключ-значение, которые будут появляться в каждой последующей записи:

```go
reqLog := log.With("request_id", "abc-123", "user_id", 7)

reqLog.Info("authorized")
reqLog.Info("fetching data", "table", "products")
```

```text
[12:30:45] INF authorized request_id=abc-123 user_id=7
[12:30:45] INF fetching data request_id=abc-123 user_id=7 table=products
```

---

### Вложенные группы атрибутов

Используйте `slog.Group` для структурирования связанных полей под общим ключом:

```go
log.Info("order placed",
    slog.Group("customer",
        slog.String("name", "Alice"),
        slog.Int("id", 42),
    ),
    slog.Group("order",
        slog.String("sku", "WIDGET-9"),
        slog.Int("qty", 3),
    ),
)
```

```text
[12:30:45] INF order placed name=Alice id=42 sku=WIDGET-9 qty=3
```

---

### Поддержка типов значений

Все стандартные типы значений `slog` красиво форматируются:

```go
log.Info("event",
    "label",    "deploy",
    "count",    int64(42),
    "ratio",    3.14,
    "active",   true,
    "elapsed",  2*time.Second,
    "started",  time.Now(),
)
```

Сложные структуры автоматически сериализуются в JSON с отступами:

```go
type Config struct {
    Host string `json:"host"`
    Port int    `json:"port"`
}

log.Info("loaded config", "cfg", Config{Host: "localhost", Port: 5432})
```

```text
[12:30:45] INF loaded config cfg={
  "host": "localhost",
  "port": 5432
}
```

---

### Хук на ошибки

Зарегистрируйте callback, который срабатывает при каждой записи уровня `ERROR` и выше — идеально для алертов, метрик или трекинга ошибок:

```go
handler := logger.NewColorHandler(os.Stdout)

handler.SetHook(func(ctx context.Context, r slog.Record) {
    // Отправить в Sentry, увеличить счётчик Prometheus и т.д.
    alerting.Notify(ctx, r.Message)
})

log := slog.New(handler)
log.Error("payment failed", "order_id", "ORD-99")
// -> хук срабатывает с записью до вывода строки
```

---

### Быстрый логгер для тестов

Удобный однострочник для тестов и прототипов:

```go
log := logger.NewTestLogger()
log.Info("running test", "case", "happy path")
```

---

## Потокобезопасность

`ColorHandler` безопасен для конкурентного использования. Внутренняя запись защищена `sync.Mutex`, а `WithGroup` / `WithAttrs` возвращают новые неизменяемые копии handler'а, поэтому один логгер можно без опасений использовать из нескольких горутин.

---

## Справочник по API

| Функция / Метод | Описание |
|---|---|
| `NewColorHandler(w io.Writer)` | Создаёт новый handler, пишущий в `w` |
| `NewTestLogger()` | Сокращение: `slog.New(NewColorHandler(os.Stdout))` |
| `handler.SetHook(fn)` | Регистрирует callback для записей `>= ERROR` |
| `handler.WithGroup(name)` | Возвращает новый handler с добавленным префиксом группы |
| `handler.WithAttrs(attrs)` | Возвращает новый handler с предустановленными атрибутами |
| `handler.Enabled(ctx, level)` | Всегда возвращает `true` (логируются все уровни) |
| `handler.Handle(ctx, record)` | Форматирует и записывает цветную строку лога |

---

## Создано с помощью ИИ

При разработке этого проекта использовались инструменты искусственного интеллекта для генерации кода, тестов и документации.

---

## Лицензия

MIT
