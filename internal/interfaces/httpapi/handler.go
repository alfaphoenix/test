package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"test/internal/usecase"
)

var errInvalidUserID = errors.New("invalid user_id")

type Handler struct {
	service *usecase.NoteService
}

func NewHandler(service *usecase.NoteService, apiUser, apiPassword string) http.Handler {
	h := &Handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("/notes", h.handleNotes)
	mux.HandleFunc("/notes/by-title", h.handleNoteByTitle)
	mux.HandleFunc("/notes/", h.handleNoteByID)
	mux.HandleFunc("/links/", h.handleLinkByID)
	return Logging(BasicAuth(apiUser, apiPassword, mux))
}

func (h *Handler) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListNotes(w, r)
	case http.MethodPost:
		h.handleCreateNote(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleNoteByTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(r.URL.Query().Get("title"))
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	note, found, err := h.service.GetNoteByTitle(r.Context(), userID, title)
	if err != nil {
		http.Error(w, "failed to get note", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *Handler) handleNoteByID(w http.ResponseWriter, r *http.Request) {
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
		h.handleDeleteNote(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "links" {
		h.handleLinks(w, r, id)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) handleLinkByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/links/")
	linkID, err := strconv.Atoi(idStr)
	if err != nil || linkID <= 0 {
		http.Error(w, "invalid link id", http.StatusBadRequest)
		return
	}
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var payload struct {
			ToID uint `json:"to_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ToID == 0 {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		updated, err := h.service.UpdateLink(r.Context(), userID, uint(linkID), payload.ToID)
		if err != nil {
			http.Error(w, "failed to update link", http.StatusInternalServerError)
			return
		}
		if !updated {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		deleted, err := h.service.DeleteLink(r.Context(), userID, uint(linkID))
		if err != nil {
			http.Error(w, "failed to delete link", http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.Error(w, "link not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleListNotes(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	notes, err := h.service.ListNotes(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to list notes", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}
func (h *Handler) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var payload struct{ Title, Text string }
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Text = strings.TrimSpace(payload.Text)
	if payload.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if payload.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	note, err := h.service.AddNote(r.Context(), userID, payload.Title, payload.Text)
	if err != nil {
		http.Error(w, "failed to save note", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}
func (h *Handler) handleDeleteNote(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	deleted, err := h.service.DeleteNote(r.Context(), userID, id)
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
func (h *Handler) handleLinks(w http.ResponseWriter, r *http.Request, fromID int) {
	userID, err := userIDFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		links, err := h.service.ListLinksForNote(r.Context(), userID, fromID)
		if err != nil {
			http.Error(w, "failed to list links", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, links)
	case http.MethodPost:
		var payload struct {
			ToID int `json:"to_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ToID <= 0 {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		link, err := h.service.AddLink(r.Context(), userID, fromID, payload.ToID)
		if err != nil {
			http.Error(w, "failed to add link", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, link)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func userIDFromQuery(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidUserID
	}
	return id, nil
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
