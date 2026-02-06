# Telegram Notes Bot (Go)

Telegram-бот и HTTP API для ведения личных заметок. Хранение работает через PostgreSQL и GORM.

## Возможности

- Добавление заметок через `/add`.
- Просмотр списка `/list`.
- Удаление заметки через смену статуса на `deleted` (без физического удаления записи).
- Массовая пометка заметок как удаленных через `/clear`.
- Создание, редактирование и удаление связей между заметками.
- Авторизация через логин и пароль.
- Ответы бота форматируются с поддержкой Markdown.

## Переменные окружения

- `BOT_TOKEN` — токен Telegram-бота.
- `DATABASE_URL` — строка подключения к PostgreSQL.
- `HTTP_ADDR` — адрес HTTP API (по умолчанию `:8080`).
- `API_USER` / `API_PASSWORD` — учетные данные для HTTP API (Basic Auth).
- `BOT_LOGIN` / `BOT_PASSWORD` — логин и пароль для Telegram-бота.

## Запуск

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

```text
/start
/login bot secret
/add купить молоко
/list
/link 1 2
/link_edit 1 3
/link_delete 1
/delete 1
/clear
```

## Примеры HTTP API

```bash
# Список заметок
curl -u api:secret "http://localhost:8080/notes?user_id=123"

# Создание заметки
curl -u api:secret -X POST "http://localhost:8080/notes?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"text":"заметка"}'

# Создание связи
curl -u api:secret -X POST "http://localhost:8080/notes/1/links?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"to_id":2}'

# Редактирование связи
curl -u api:secret -X PATCH "http://localhost:8080/links/1?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"to_id":3}'

# Удаление связи
curl -u api:secret -X DELETE "http://localhost:8080/links/1?user_id=123"

# Пометить заметку как удаленную
curl -u api:secret -X DELETE "http://localhost:8080/notes/1?user_id=123"
```
