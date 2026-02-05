package main

import "time"

// Note описывает заметку пользователя.
type Note struct {
	ID        int
	UserID    int64
	Text      string
	CreatedAt time.Time
}

// NoteLink описывает связь между двумя заметками одного пользователя.
type NoteLink struct {
	FromID int
	ToID   int
}
