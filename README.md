# Telegram Quotes Bot

Telegram-бот на Go, ежедневно отправляющий цитаты великих людей через API [Forismatic](http://forismatic.com/).

## Возможности

- Команды `/start`, `/stop`, `/quote`
- Ежедневная рассылка цитат в заданное время
- Хранение подписчиков в JSON-файле
- Автоматические повторы запросов к Forismatic при ошибках

## Подготовка

```bash
git clone https://github.com/beglov/go-quotes-telegram-bot.git
cd go-quotes-telegram-bot
go test ./...
```

## Конфигурация

| Переменная | Обязательна | Описание | Значение по умолчанию |
|------------|-------------|----------|------------------------|
| `TELEGRAM_BOT_TOKEN` | Да | Токен Telegram-бота | — |
| `DAILY_TIME` | Нет | Время рассылки в формате `HH:MM` | `08:00` |
| `SUBSCRIBERS_FILE` | Нет | Путь к файлу с подписчиками | `data/subscribers.json` |

## Запуск

```bash
export TELEGRAM_BOT_TOKEN=ваш_токен
export DAILY_TIME=08:00
go run ./cmd/bot
```

## Тестирование

```bash
go test ./...
```

## Структура проекта

```
cmd/bot            // точка входа приложения
internal/bot       // логика Telegram-бота
internal/quotes    // клиент Forismatic
internal/scheduler // планировщик рассылки
internal/subscribers // файловое хранилище подписчиков
```


