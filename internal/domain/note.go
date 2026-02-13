package domain

import "time"

type NoteStatus string

const (
	NoteStatusActive  NoteStatus = "active"
	NoteStatusDeleted NoteStatus = "deleted"
)

type Note struct {
	ID        uint
	UserID    int64
	Title     string
	Text      string
	Status    NoteStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NoteLink struct {
	ID        uint
	UserID    int64
	FromID    uint
	ToID      uint
	CreatedAt time.Time
	UpdatedAt time.Time
}
