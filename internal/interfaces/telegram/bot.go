package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"test/internal/domain"
	"test/internal/usecase"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var errMissingBotToken = errors.New("BOT_TOKEN is not set")

type Bot struct {
	service     *usecase.NoteService
	token       string
	parseMode   string
	addRequests map[int64]addNoteRequest
}

type addNoteRequest struct {
	title string
	step  addNoteStep
}
type addNoteStep string

const (
	addStepTitle addNoteStep = "title"
	addStepText  addNoteStep = "text"
)

func New(service *usecase.NoteService, token string) *Bot {
	return &Bot{service: service, token: token, parseMode: tgbotapi.ModeMarkdown, addRequests: make(map[int64]addNoteRequest)}
}

func (b *Bot) Start(ctx context.Context) error {
	if b.token == "" {
		return errMissingBotToken
	}
	bot, err := tgbotapi.NewBotAPI(b.token)
	if err != nil {
		return err
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)
	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Offset: 0, Timeout: 30})
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

func (b *Bot) handleMessage(ctx context.Context, userID int64, text string) string {
	if text == "" {
		return "Пришлите команду или текст заметки. Используйте /help для справки."
	}
	if req, ok := b.addRequests[userID]; ok && !strings.HasPrefix(text, "/") {
		return b.handleAddFlowInput(ctx, userID, text, req)
	}
	if !strings.HasPrefix(text, "/") {
		return b.handleNoteLookupByTitle(ctx, userID, text)
	}
	fields := strings.Fields(text)
	cmd := fields[0]
	switch cmd {
	case "/start":
		return "Привет! Я помогу хранить ваши заметки. Введите /help для списка команд."
	case "/help":
		return helpMessage()
	case "/login":
		if len(fields) < 3 {
			return "Используйте /login <логин> <пароль>"
		}
		ok, err := b.service.Login(ctx, userID, fields[1], fields[2])
		if err != nil {
			return "Не удалось сохранить авторизацию. Попробуйте позже."
		}
		if !ok {
			return "Неверный логин или пароль."
		}
		return "Авторизация успешна. Теперь можно работать с заметками."
	default:
		if err := b.service.EnsureAuthorized(ctx, userID); err != nil {
			if errors.Is(err, usecase.ErrUnauthorized) {
				return "Сначала выполните /login <логин> <пароль>."
			}
			return "Не удалось проверить авторизацию."
		}
		return b.handleAuthorized(ctx, userID, cmd, fields)
	}
}

func (b *Bot) handleAuthorized(ctx context.Context, userID int64, cmd string, fields []string) string {
	switch cmd {
	case "/add":
		b.addRequests[userID] = addNoteRequest{step: addStepTitle}
		return "Введите название заметки."
	case "/list":
		notes, err := b.service.ListNotes(ctx, userID)
		if err != nil {
			return "Не удалось получить заметки. Попробуйте позже."
		}
		if len(notes) == 0 {
			return "У вас пока нет заметок. Добавьте через /add."
		}
		links, err := b.service.ListLinks(ctx, userID)
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
		deleted, err := b.service.DeleteNote(ctx, userID, id)
		if err != nil {
			return "Не удалось удалить заметку. Попробуйте позже."
		}
		if !deleted {
			return "Заметка с таким номером не найдена."
		}
		return "Заметка помечена как удаленная."
	case "/clear":
		if err := b.service.ClearNotes(ctx, userID); err != nil {
			return "Не удалось очистить заметки. Попробуйте позже."
		}
		return "Все заметки помечены как удаленные."
	case "/link":
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
		link, err := b.service.AddLink(ctx, userID, fromID, toID)
		if err != nil {
			return "Не удалось добавить связь. Попробуйте позже."
		}
		return fmt.Sprintf("Связь #%d добавлена.", link.ID)
	case "/link_edit":
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
		updated, err := b.service.UpdateLink(ctx, userID, uint(linkID), uint(newToID))
		if err != nil {
			return "Не удалось обновить связь."
		}
		if !updated {
			return "Связь не найдена."
		}
		return "Связь обновлена."
	case "/link_delete":
		if len(fields) < 2 {
			return "Используйте /link_delete <link_id>"
		}
		linkID, err := strconv.Atoi(fields[1])
		if err != nil || linkID <= 0 {
			return "link_id должен быть положительным числом"
		}
		deleted, err := b.service.DeleteLink(ctx, userID, uint(linkID))
		if err != nil {
			return "Не удалось удалить связь."
		}
		if !deleted {
			return "Связь не найдена."
		}
		return "Связь удалена."
	default:
		return "Неизвестная команда. Используйте /help."
	}
}

func (b *Bot) handleAddFlowInput(ctx context.Context, userID int64, text string, request addNoteRequest) string {
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
		note, err := b.service.AddNote(ctx, userID, request.title, input)
		if err != nil {
			return "Не удалось сохранить заметку. Попробуйте позже."
		}
		return fmt.Sprintf("Заметка #%d сохранена.", note.ID)
	default:
		delete(b.addRequests, userID)
		return "Процесс добавления заметки сброшен. Введите /add и попробуйте снова."
	}
}

func (b *Bot) handleNoteLookupByTitle(ctx context.Context, userID int64, text string) string {
	if err := b.service.EnsureAuthorized(ctx, userID); err != nil {
		if errors.Is(err, usecase.ErrUnauthorized) {
			return "Сначала выполните /login <логин> <пароль>."
		}
		return "Не удалось проверить авторизацию."
	}
	note, found, err := b.service.GetNoteByTitle(ctx, userID, text)
	if err != nil {
		return "Не удалось найти заметку. Попробуйте позже."
	}
	if !found {
		return "Неизвестная команда. Используйте /help."
	}
	links, err := b.service.ListLinksForNote(ctx, userID, int(note.ID))
	if err != nil {
		return "Не удалось получить связи заметки."
	}
	linkedTitles := b.linkedTitlesByIDs(ctx, userID, links)
	return strings.Join([]string{fmt.Sprintf("Название: %s", note.Title), fmt.Sprintf("Текст: %s", note.Text), fmt.Sprintf("Связи: %s", strings.Join(linkedTitles, ", "))}, "\n")
}

func (b *Bot) linkedTitlesByIDs(ctx context.Context, userID int64, links []domain.NoteLink) []string {
	if len(links) == 0 {
		return []string{"нет"}
	}
	notes, err := b.service.ListNotes(ctx, userID)
	if err != nil {
		return []string{"недоступно"}
	}
	titles := make(map[uint]string, len(notes))
	for _, n := range notes {
		titles[n.ID] = n.Title
	}
	out := make([]string, 0, len(links))
	for _, l := range links {
		if t, ok := titles[l.ToID]; ok {
			out = append(out, t)
		} else {
			out = append(out, fmt.Sprintf("#%d", l.ToID))
		}
	}
	return out
}

func formatNotesWithLinks(notes []domain.Note, links []domain.NoteLink) string {
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
		line := fmt.Sprintf("%d. [%s](%s) — %s", note.ID, note.Title, "https://t.me/share/url?url=&text="+url.QueryEscape(note.Title), note.Text)
		if linked := linksMap[note.ID]; len(linked) > 0 {
			parts := make([]string, 0, len(linked))
			for _, id := range linked {
				parts = append(parts, strconv.FormatUint(uint64(id), 10))
			}
			line = fmt.Sprintf("%s (связи: %s)", line, strings.Join(parts, ", "))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

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
