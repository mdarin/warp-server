
# WebSocket TUI Implementation

Структура проекта

```text
web/
├── server.go
├── handler.go
└── static/
    └── index.html
```

## Использование в main.go

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    logs := make(chan string, 100)
    conditions := make(chan []controlloop.Condition, 10)

    // Запускаем Web UI в отдельной горутине
    go func() {
        if err := web.CreateWebTUI(ctx, ":8080", logs, conditions); err != nil {
            log.Printf("web UI error: %v", err)
        }
    }()

    // Или оригинальный TUI — интерфейсы совместимы по каналам
    // web.CreateTUI(cancel, g, logWriter, conditions)

    log.Printf("Web UI available at http://localhost:8080")
    // ... остальная логика
}
```
