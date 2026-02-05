package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// API описывает HTTP API для работы с заметками.
type API struct {
	store *NotesStore
	auth  AuthMiddleware
}

// NewAPI создает API с заданным хранилищем и учетными данными.
func NewAPI(store *NotesStore, user, password string) *API {
	return &API{
		store: store,
		auth:  AuthMiddleware{User: user, Password: password},
	}
}

// Handler возвращает http.Handler со всеми маршрутами API.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/notes", a.handleNotes)
	mux.HandleFunc("/notes/", a.handleNoteByID)

	return LoggingMiddleware(a.auth.Wrap(mux))
}

// handleNotes обрабатывает создание и получение списка заметок.
func (a *API) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleListNotes(w, r)
	case http.MethodPost:
		a.handleCreateNote(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNoteByID маршрутизирует запросы для конкретной заметки.
func (a *API) handleNoteByID(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/notes/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		a.handleDeleteNote(w, r, id)
		return
	}

	if len(parts) == 2 && parts[1] == "links" {
		a.handleLinks(w, r, id)
		return
	}

	http.NotFound(w, r)
}

// handleListNotes возвращает список заметок пользователя.
func (a *API) handleListNotes(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	notes, err := a.store.ListNotes(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to list notes", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, notes)
}

// handleCreateNote создает заметку пользователя.
func (a *API) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	payload.Text = strings.TrimSpace(payload.Text)
	if payload.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	note, err := a.store.AddNote(r.Context(), userID, payload.Text)
	if err != nil {
		http.Error(w, "failed to save note", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, note)
}

// handleDeleteNote удаляет заметку пользователя.
func (a *API) handleDeleteNote(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	deleted, err := a.store.DeleteNote(r.Context(), userID, id)
	if err != nil {
		http.Error(w, "failed to delete note", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleLinks создает и возвращает связи между заметками.
func (a *API) handleLinks(w http.ResponseWriter, r *http.Request, fromID int) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		links, err := a.store.ListLinksForNote(r.Context(), userID, fromID)
		if err != nil {
			http.Error(w, "failed to list links", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, links)
	case http.MethodPost:
		var payload struct {
			ToID int `json:"to_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if payload.ToID <= 0 {
			http.Error(w, "to_id is required", http.StatusBadRequest)
			return
		}
		if err := a.store.AddLink(r.Context(), userID, fromID, payload.ToID); err != nil {
			http.Error(w, "failed to add link", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// userIDFromQuery извлекает идентификатор пользователя из параметров запроса.
func userIDFromQuery(r *http.Request) (int64, error) {
	value := r.URL.Query().Get("user_id")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidUserID
	}
	return id, nil
}

// writeJSON сериализует ответ в JSON.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
