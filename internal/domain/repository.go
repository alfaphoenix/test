package domain

import "context"

// Repository определяет контракт хранилища для use case слоя.
type Repository interface {
	Close() error
	AuthorizeUser(ctx context.Context, userID int64) error
	IsUserAuthorized(ctx context.Context, userID int64) (bool, error)
	AddNote(ctx context.Context, userID int64, title, text string) (Note, error)
	ListNotes(ctx context.Context, userID int64) ([]Note, error)
	GetNoteByTitle(ctx context.Context, userID int64, title string) (Note, bool, error)
	DeleteNote(ctx context.Context, userID int64, id int) (bool, error)
	ClearNotes(ctx context.Context, userID int64) error
	AddLink(ctx context.Context, userID int64, fromID, toID int) (NoteLink, error)
	UpdateLink(ctx context.Context, userID int64, linkID uint, toID uint) (bool, error)
	DeleteLink(ctx context.Context, userID int64, linkID uint) (bool, error)
	ListLinks(ctx context.Context, userID int64) ([]NoteLink, error)
	ListLinksForNote(ctx context.Context, userID int64, fromID int) ([]NoteLink, error)
}
