package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"net/url"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot отвечает за обработку сообщений Telegram.
type TelegramBot struct {
	store       *NotesStore
	token       string
	login       string
	password    string
	parseMode   string
	addRequests map[int64]addNoteRequest
}

// addNoteRequest хранит состояние пошагового создания заметки через /add.
type addNoteRequest struct {
	title string
	step  addNoteStep
}

// addNoteStep описывает шаг пошагового добавления заметки.
type addNoteStep string

const (
	addStepTitle addNoteStep = "title"
	addStepText  addNoteStep = "text"
)

// NewTelegramBot создает новый бот с доступом к хранилищу.
func NewTelegramBot(store *NotesStore, token, login, password string) *TelegramBot {
	return &TelegramBot{
		store:       store,
		token:       token,
		login:       login,
		password:    password,
		parseMode:   tgbotapi.ModeMarkdown,
		addRequests: make(map[int64]addNoteRequest),
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

	if request, ok := b.addRequests[userID]; ok && !strings.HasPrefix(text, "/") {
		return b.handleAddFlowInput(ctx, userID, text, request)
	}
	if !strings.HasPrefix(text, "/") {
		return b.handleNoteLookupByTitle(ctx, userID, text)
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
		b.addRequests[userID] = addNoteRequest{step: addStepTitle}
		return "Введите название заметки."
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
		return "Заметка помечена как удаленная."
	case "/clear":
		if err := b.store.ClearNotes(ctx, userID); err != nil {
			return "Не удалось очистить заметки. Попробуйте позже."
		}
		return "Все заметки помечены как удаленные."
	case "/link":
		return b.handleLinkCreate(ctx, userID, fields)
	case "/link_edit":
		return b.handleLinkEdit(ctx, userID, fields)
	case "/link_delete":
		return b.handleLinkDelete(ctx, userID, fields)
	default:
		return "Неизвестная команда. Используйте /help."
	}
}

// handleLinkCreate создает связь между заметками пользователя.
func (b *TelegramBot) handleLinkCreate(ctx context.Context, userID int64, fields []string) string {
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
	link, err := b.store.AddLink(ctx, userID, fromID, toID)
	if err != nil {
		return "Не удалось добавить связь. Попробуйте позже."
	}
	return fmt.Sprintf("Связь #%d добавлена.", link.ID)
}

// handleLinkEdit редактирует существующую связь.
func (b *TelegramBot) handleLinkEdit(ctx context.Context, userID int64, fields []string) string {
	if len(fields) < 3 {
		return "Используйте /link_edit <link_id> <new_to_id>"
	}
	linkID, err := strconv.Atoi(fields[1])
	if err != nil || linkID <= 0 {
		return "link_id должен быть положительным числом"
	}
	newToID, err := strconv.Atoi(fields[2])
	if err != nil || newToID <= 0 {
		return "new_to_id должен быть положительным числом"
	}
	updated, err := b.store.UpdateLink(ctx, userID, uint(linkID), uint(newToID))
	if err != nil {
		return "Не удалось обновить связь."
	}
	if !updated {
		return "Связь не найдена."
	}
	return "Связь обновлена."
}

// handleLinkDelete удаляет связь между заметками.
func (b *TelegramBot) handleLinkDelete(ctx context.Context, userID int64, fields []string) string {
	if len(fields) < 2 {
		return "Используйте /link_delete <link_id>"
	}
	linkID, err := strconv.Atoi(fields[1])
	if err != nil || linkID <= 0 {
		return "link_id должен быть положительным числом"
	}
	deleted, err := b.store.DeleteLink(ctx, userID, uint(linkID))
	if err != nil {
		return "Не удалось удалить связь."
	}
	if !deleted {
		return "Связь не найдена."
	}
	return "Связь удалена."
}

// handleNoteLookupByTitle ищет заметку по названию и выводит подробности.
func (b *TelegramBot) handleNoteLookupByTitle(ctx context.Context, userID int64, text string) string {
	authorized, err := b.store.IsUserAuthorized(ctx, userID)
	if err != nil {
		return "Не удалось проверить авторизацию."
	}
	if !authorized {
		return "Сначала выполните /login <логин> <пароль>."
	}

	note, found, err := b.store.GetNoteByTitle(ctx, userID, text)
	if err != nil {
		return "Не удалось найти заметку. Попробуйте позже."
	}
	if !found {
		return "Неизвестная команда. Используйте /help."
	}

	links, err := b.store.ListLinksForNote(ctx, userID, int(note.ID))
	if err != nil {
		return "Не удалось получить связи заметки."
	}
	linkedTitles := b.linkedTitlesByIDs(ctx, userID, links)

	return strings.Join([]string{
		fmt.Sprintf("Название: %s", note.Title),
		fmt.Sprintf("Текст: %s", note.Text),
		fmt.Sprintf("Связи: %s", strings.Join(linkedTitles, ", ")),
	}, "\n")
}

// linkedTitlesByIDs возвращает список названий связанных заметок.
func (b *TelegramBot) linkedTitlesByIDs(ctx context.Context, userID int64, links []NoteLink) []string {
	if len(links) == 0 {
		return []string{"нет"}
	}

	notes, err := b.store.ListNotes(ctx, userID)
	if err != nil {
		return []string{"недоступно"}
	}
	titlesByID := make(map[uint]string, len(notes))
	for _, note := range notes {
		titlesByID[note.ID] = note.Title
	}

	titles := make([]string, 0, len(links))
	for _, link := range links {
		title, ok := titlesByID[link.ToID]
		if !ok {
			titles = append(titles, fmt.Sprintf("#%d", link.ToID))
			continue
		}
		titles = append(titles, title)
	}
	return titles
}

// handleAddFlowInput принимает данные для пошагового добавления заметки.
func (b *TelegramBot) handleAddFlowInput(ctx context.Context, userID int64, text string, request addNoteRequest) string {
	input := strings.TrimSpace(text)
	if input == "" {
		if request.step == addStepTitle {
			return "Название не должно быть пустым. Введите название заметки."
		}
		return "Текст не должен быть пустым. Введите текст заметки."
	}

	switch request.step {
	case addStepTitle:
		b.addRequests[userID] = addNoteRequest{title: input, step: addStepText}
		return "Теперь введите текст заметки."
	case addStepText:
		delete(b.addRequests, userID)
		note, err := b.store.AddNote(ctx, userID, request.title, input)
		if err != nil {
			return "Не удалось сохранить заметку. Попробуйте позже."
		}
		return fmt.Sprintf("Заметка #%d сохранена.", note.ID)
	default:
		delete(b.addRequests, userID)
		return "Процесс добавления заметки сброшен. Введите /add и попробуйте снова."
	}
}

// formatNotesWithLinks формирует список заметок с указанием связей.
func formatNotesWithLinks(notes []Note, links []NoteLink) string {
	linksMap := make(map[uint][]uint)
	for _, link := range links {
		linksMap[link.FromID] = append(linksMap[link.FromID], link.ToID)
	}
	for id := range linksMap {
		sort.Slice(linksMap[id], func(i, j int) bool { return linksMap[id][i] < linksMap[id][j] })
	}

	lines := make([]string, 0, len(notes)+1)
	lines = append(lines, "Ваши заметки:")
	for _, note := range notes {
		line := fmt.Sprintf("%d. [%s](%s) — %s", note.ID, note.Title, noteTitleShareLink(note.Title), note.Text)
		if linked := linksMap[note.ID]; len(linked) > 0 {
			line = fmt.Sprintf("%s (связи: %s)", line, joinUints(linked))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// joinUints форматирует список чисел в строку.
func joinUints(values []uint) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatUint(uint64(value), 10))
	}
	return strings.Join(parts, ", ")
}

// noteTitleShareLink формирует ссылку, которая подставляет название в сообщение чата.
func noteTitleShareLink(title string) string {
	return "https://t.me/share/url?url=&text=" + url.QueryEscape(title)
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
		"/add — добавить заметку (сначала название, потом текст)",
		"/list — список заметок (названия кликабельны)",
		"/link <id1> <id2> — создать связь",
		"/link_edit <link_id> <new_to_id> — редактировать связь",
		"/link_delete <link_id> — удалить связь",
		"/delete <номер> — пометить заметку удаленной",
		"/clear — пометить все заметки удаленными",
		"/help — справка",
	}, "\n")
}
