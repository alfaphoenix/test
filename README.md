# Telegram Notes Bot (Go)

Telegram-бот и HTTP API для ведения личных заметок.
Проект хранит данные в PostgreSQL через GORM и запускает два интерфейса одновременно:
- Telegram-бот для интерактивной работы с заметками;
- HTTP API c Basic Auth для интеграций и автоматизации.

## Что умеет сервис

- Авторизация пользователей в боте по логину/паролю (`/login`).
- Добавление заметок (`/add`).
- Просмотр списка активных заметок (`/list`) с кликабельными названиями.
- Мягкое удаление заметки (`/delete`) — статус меняется на `deleted`, запись не удаляется физически.
- Массовая очистка заметок (`/clear`) — все активные заметки пользователя помечаются как удаленные.
- Создание, редактирование и удаление связей между заметками (`/link`, `/link_edit`, `/link_delete`).
- Управление заметками и связями через HTTP API.


## Архитектура (Clean Architecture)

Проект разделен на слои:

- `internal/domain` — доменные сущности и контракты репозиториев.
- `internal/usecase` — бизнес-логика приложения (сценарии работы с заметками).
- `internal/infrastructure/postgres` — реализация репозитория на PostgreSQL/GORM.
- `internal/interfaces/httpapi` — HTTP transport (handlers + middleware).
- `internal/interfaces/telegram` — Telegram transport (бот и команды).
- `main.go` — слой композиции зависимостей (composition root).

Зависимости направлены внутрь: `interfaces -> usecase -> domain`, а инфраструктура подключается через интерфейсы.

## Технологии

- Go 1.24+
- PostgreSQL
- GORM
- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)

## Быстрый старт

### 1) Подготовьте `.env`

Создайте файл `.env` в корне проекта:

```env
BOT_TOKEN=change_me
DATABASE_URL=postgres://user:pass@localhost:5432/notes?sslmode=disable
HTTP_ADDR=:8080
API_USER=api
API_PASSWORD=secret
BOT_LOGIN=bot
BOT_PASSWORD=secret
```

### 2) Запустите PostgreSQL

Убедитесь, что БД доступна по `DATABASE_URL`.

Пример через Docker:

```bash
docker run --name notes-postgres \
  -e POSTGRES_USER=user \
  -e POSTGRES_PASSWORD=pass \
  -e POSTGRES_DB=notes \
  -p 5432:5432 \
  -d postgres:16
```

### 3) Установите зависимости и запустите проект

```bash
go mod download
go run .
```

При старте приложение автоматически выполняет миграции таблиц:
- `notes`
- `note_links`
- `authorized_users`

## Переменные окружения

| Переменная     | Обязательна | Описание |
|---|---:|---|
| `BOT_TOKEN`    | Да  | Telegram Bot Token. Без него бот не запустится. |
| `DATABASE_URL` | Да  | Строка подключения к PostgreSQL. |
| `HTTP_ADDR`    | Нет | Адрес HTTP-сервера (по умолчанию `:8080`). |
| `API_USER`     | Да  | Логин для Basic Auth HTTP API. |
| `API_PASSWORD` | Да  | Пароль для Basic Auth HTTP API. |
| `BOT_LOGIN`    | Да  | Логин команды `/login` в Telegram. |
| `BOT_PASSWORD` | Да  | Пароль команды `/login` в Telegram. |

## Telegram-команды

Перед выполнением команд с заметками пользователь должен авторизоваться:

```text
/login <логин> <пароль>
```

Доступные команды:

| Команда | Описание | Пример |
|---|---|---|
| `/start` | Приветствие и краткий гайд | `/start` |
| `/help` | Справка по командам | `/help` |
| `/login <login> <password>` | Авторизация пользователя | `/login bot secret` |
| `/add` | Создать заметку (бот последовательно запросит название и текст) | `/add` |
| `/list` | Показать активные заметки и связи | `/list` |
| `/delete <note_id>` | Пометить заметку как удаленную | `/delete 1` |
| `/clear` | Пометить все заметки как удаленные | `/clear` |
| `/link <from_id> <to_id>` | Создать связь между заметками | `/link 1 2` |
| `/link_edit <link_id> <new_to_id>` | Изменить связь | `/link_edit 1 3` |
| `/link_delete <link_id>` | Удалить связь | `/link_delete 1` |

## HTTP API

### Общие правила

- Все HTTP-запросы требуют Basic Auth (`API_USER` / `API_PASSWORD`).
- Для операций обязательно передавать `user_id` query-параметром.
- Ответы API — JSON (кроме некоторых ошибок `http.Error`, где возвращается plain text).

Базовый URL по умолчанию: `http://localhost:8080`.

### Список эндпоинтов

| Метод | Путь | Описание |
|---|---|---|
| `GET` | `/notes?user_id=<id>` | Список активных заметок пользователя |
| `GET` | `/notes/by-title?user_id=<id>&title=<title>` | Получить заметку по названию |
| `POST` | `/notes?user_id=<id>` | Создание заметки |
| `DELETE` | `/notes/{note_id}?user_id=<id>` | Мягкое удаление заметки |
| `POST` | `/notes/{note_id}/links?user_id=<id>` | Создание связи от заметки `note_id` к `to_id` |
| `PATCH` | `/links/{link_id}?user_id=<id>` | Изменение поля `to_id` у связи |
| `DELETE` | `/links/{link_id}?user_id=<id>` | Удаление связи |

### Примеры запросов

```bash
# 1) Получить список заметок
curl -u api:secret "http://localhost:8080/notes?user_id=123"

# 2) Создать заметку
curl -u api:secret -X POST "http://localhost:8080/notes?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"title":"Покупки","text":"купить молоко"}'

# 3) Получить заметку по названию
curl -u api:secret "http://localhost:8080/notes/by-title?user_id=123&title=%D0%9F%D0%BE%D0%BA%D1%83%D0%BF%D0%BA%D0%B8"

# 4) Удалить заметку (soft delete)
curl -u api:secret -X DELETE "http://localhost:8080/notes/1?user_id=123"

# 5) Создать связь
curl -u api:secret -X POST "http://localhost:8080/notes/1/links?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"to_id":2}'

# 6) Изменить связь
curl -u api:secret -X PATCH "http://localhost:8080/links/1?user_id=123" \
  -H "Content-Type: application/json" \
  -d '{"to_id":3}'

# 7) Удалить связь
curl -u api:secret -X DELETE "http://localhost:8080/links/1?user_id=123"
```

## Структура проекта

- `main.go` — точка входа и wiring зависимостей.
- `internal/config/config.go` — загрузка конфигурации из окружения.
- `internal/domain` — доменные сущности и интерфейсы.
- `internal/usecase` — application service для заметок/связей/авторизации.
- `internal/infrastructure/postgres/store.go` — GORM репозиторий и миграции.
- `internal/interfaces/httpapi` — HTTP API и middleware.
- `internal/interfaces/telegram/bot.go` — Telegram-логика и команды.

## Особенности поведения

- Удаление заметок выполняется через смену статуса (`active` → `deleted`).
- Связь можно создать/изменить только между существующими **активными** заметками пользователя.
- HTTP API и бот работают с одним и тем же хранилищем данных.

## Проверка качества

Базовые команды для локальной проверки:

```bash
go test ./...
go vet ./...
```
