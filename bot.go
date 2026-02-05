package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot отвечает за обработку сообщений Telegram.
type TelegramBot struct {
	store     *NotesStore
	token     string
	login     string
	password  string
	parseMode string
}

// NewTelegramBot создает новый бот с доступом к хранилищу.
func NewTelegramBot(store *NotesStore, token, login, password string) *TelegramBot {
	return &TelegramBot{
		store:     store,
		token:     token,
		login:     login,
		password:  password,
		parseMode: tgbotapi.ModeMarkdown,
	}
}

// Start запускает цикл получения обновлений.
func (b *TelegramBot) Start(ctx context.Context) error {
	if b.token == "" {
		return errMissingBotToken
	}

	bot, err := tgbotapi.NewBotAPI(b.token)
	if err != nil {
		return err
	}

	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			userID := update.Message.From.ID
			text := strings.TrimSpace(update.Message.Text)

			reply := b.handleMessage(ctx, userID, text)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
			msg.ParseMode = b.parseMode
			if _, err := bot.Send(msg); err != nil {
				log.Printf("send message error: %v", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// handleMessage маршрутизирует команду пользователя.
func (b *TelegramBot) handleMessage(ctx context.Context, userID int64, text string) string {
	if text == "" {
		return "Пришлите команду или текст заметки. Используйте /help для справки."
	}

	fields := strings.Fields(text)
	command := fields[0]

	switch command {
	case "/start":
		return startMessage()
	case "/help":
		return helpMessage()
	case "/login":
		return b.handleLogin(ctx, userID, fields)
	default:
		return b.handleAuthorized(ctx, userID, command, text, fields)
	}
}

// handleLogin авторизует пользователя по логину и паролю.
func (b *TelegramBot) handleLogin(ctx context.Context, userID int64, fields []string) string {
	if len(fields) < 3 {
		return "Используйте /login <логин> <пароль>"
	}
	if fields[1] != b.login || fields[2] != b.password {
		return "Неверный логин или пароль."
	}
	if err := b.store.AuthorizeUser(ctx, userID); err != nil {
		return "Не удалось сохранить авторизацию. Попробуйте позже."
	}
	return "Авторизация успешна. Теперь можно работать с заметками."
}

// handleAuthorized выполняет команды, требующие авторизации.
func (b *TelegramBot) handleAuthorized(ctx context.Context, userID int64, command, text string, fields []string) string {
	authorized, err := b.store.IsUserAuthorized(ctx, userID)
	if err != nil {
		return "Не удалось проверить авторизацию."
	}
	if !authorized {
		return "Сначала выполните /login <логин> <пароль>."
	}

	switch command {
	case "/add":
		payload := strings.TrimSpace(strings.TrimPrefix(text, command))
		if payload == "" {
			return "Добавьте текст заметки: /add купить молоко"
		}
		note, err := b.store.AddNote(ctx, userID, payload)
		if err != nil {
			return "Не удалось сохранить заметку. Попробуйте позже."
		}
		return fmt.Sprintf("Заметка #%d сохранена.", note.ID)
	case "/list":
		notes, err := b.store.ListNotes(ctx, userID)
		if err != nil {
			return "Не удалось получить заметки. Попробуйте позже."
		}
		if len(notes) == 0 {
			return "У вас пока нет заметок. Добавьте через /add."
		}
		links, err := b.store.ListLinks(ctx, userID)
		if err != nil {
			return "Не удалось получить связи между заметками."
		}
		return formatNotesWithLinks(notes, links)
	case "/delete":
		if len(fields) < 2 {
			return "Укажите номер заметки: /delete 2"
		}
		id, err := strconv.Atoi(fields[1])
		if err != nil || id <= 0 {
			return "Номер заметки должен быть числом: /delete 2"
		}
		deleted, err := b.store.DeleteNote(ctx, userID, id)
		if err != nil {
			return "Не удалось удалить заметку. Попробуйте позже."
		}
		if !deleted {
			return "Заметка с таким номером не найдена."
		}
		return "Заметка удалена."
	case "/clear":
		if err := b.store.ClearNotes(ctx, userID); err != nil {
			return "Не удалось очистить заметки. Попробуйте позже."
		}
		return "Все заметки удалены."
	case "/link":
		return b.handleLink(ctx, userID, fields)
	default:
		return "Неизвестная команда. Используйте /help."
	}
}

// handleLink создает связь между заметками пользователя.
func (b *TelegramBot) handleLink(ctx context.Context, userID int64, fields []string) string {
	if len(fields) < 3 {
		return "Укажите две заметки: /link 1 2"
	}
	fromID, err := strconv.Atoi(fields[1])
	if err != nil || fromID <= 0 {
		return "Первый номер должен быть числом: /link 1 2"
	}
	toID, err := strconv.Atoi(fields[2])
	if err != nil || toID <= 0 {
		return "Второй номер должен быть числом: /link 1 2"
	}
	if err := b.store.AddLink(ctx, userID, fromID, toID); err != nil {
		return "Не удалось добавить связь. Попробуйте позже."
	}
	return "Связь добавлена."
}

// formatNotesWithLinks формирует список заметок с указанием связей.
func formatNotesWithLinks(notes []Note, links []NoteLink) string {
	linksMap := make(map[int][]int)
	for _, link := range links {
		linksMap[link.FromID] = append(linksMap[link.FromID], link.ToID)
	}
	for _, ids := range linksMap {
		sort.Ints(ids)
	}

	lines := make([]string, 0, len(notes)+1)
	lines = append(lines, "Ваши заметки:")
	for _, note := range notes {
		line := fmt.Sprintf("%d. %s", note.ID, note.Text)
		if linked := linksMap[note.ID]; len(linked) > 0 {
			line = fmt.Sprintf("%s (связи: %s)", line, joinInts(linked))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// joinInts форматирует список чисел в строку.
func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}

// startMessage возвращает приветственное сообщение.
func startMessage() string {
	return "Привет! Я помогу хранить ваши заметки. Введите /help для списка команд."
}

// helpMessage возвращает справку по командам бота.
func helpMessage() string {
	return strings.Join([]string{
		"Доступные команды:",
		"/login <логин> <пароль> — авторизация",
		"/add <текст> — добавить заметку",
		"/list — список заметок",
		"/link <id1> <id2> — связь между заметками",
		"/delete <номер> — удалить заметку",
		"/clear — удалить все заметки",
		"/help — справка",
	}, "\n")
}
