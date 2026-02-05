# Telegram Notes Bot (Go)

Telegram-бот и HTTP API для ведения личных заметок. Все данные хранятся в PostgreSQL.

## Возможности

- Добавление заметок через `/add`.
- Просмотр списка `/list`.
- Удаление по номеру `/delete`.
- Полная очистка `/clear`.
- Связи между заметками через `/link`.
- Авторизация через логин и пароль.
- Ответы бота форматируются с поддержкой Markdown.

## Переменные окружения

- `BOT_TOKEN` — токен Telegram-бота.
- `DATABASE_URL` — строка подключения к PostgreSQL.
- `HTTP_ADDR` — адрес HTTP API (по умолчанию `:8080`).
- `API_USER` / `API_PASSWORD` — учетные данные для HTTP API (Basic Auth).
- `BOT_LOGIN` / `BOT_PASSWORD` — логин и пароль для Telegram-бота.

## Запуск

1. Создайте бота через [@BotFather](https://t.me/BotFather) и получите токен.
2. Подготовьте PostgreSQL и строку подключения.
3. Запустите приложение:

```bash
export BOT_TOKEN="ваш_токен"
export DATABASE_URL="postgres://user:pass@localhost:5432/notes?sslmode=disable"
export API_USER="api"
export API_PASSWORD="secret"
export BOT_LOGIN="bot"
export BOT_PASSWORD="secret"

go run .
```

## Пример команд Telegram

```
/start
/login bot secret
/add купить молоко
/list
/link 1 2
/delete 1
/clear
```

## Примеры HTTP API

```bash
curl -u api:secret "http://localhost:8080/notes?user_id=123"

curl -u api:secret -X POST "http://localhost:8080/notes?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"text":"заметка"}'

curl -u api:secret -X POST "http://localhost:8080/notes/1/links?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"to_id":2}'

curl -u api:secret -X DELETE "http://localhost:8080/notes/1?user_id=123"
```
