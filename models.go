package main

import "time"

// NoteStatus описывает состояние заметки.
type NoteStatus string

const (
	// NoteStatusActive означает, что заметка активна.
	NoteStatusActive NoteStatus = "active"
	// NoteStatusDeleted означает, что заметка помечена как удаленная.
	NoteStatusDeleted NoteStatus = "deleted"
)

// Note описывает заметку пользователя в базе данных.
type Note struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    int64      `gorm:"index;not null" json:"user_id"`
	Text      string     `gorm:"type:text;not null" json:"text"`
	Status    NoteStatus `gorm:"type:varchar(16);not null;default:'active';index" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// NoteLink описывает связь между двумя заметками одного пользователя.
type NoteLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    int64     `gorm:"index;not null" json:"user_id"`
	FromID    uint      `gorm:"index;not null" json:"from_id"`
	ToID      uint      `gorm:"index;not null" json:"to_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthorizedUser хранит авторизованных пользователей бота.
type AuthorizedUser struct {
	UserID int64 `gorm:"primaryKey" json:"user_id"`
}
