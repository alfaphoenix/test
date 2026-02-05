package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// NotesStore управляет хранением заметок в PostgreSQL.
type NotesStore struct {
	db *sql.DB
}

// NewNotesStore создает подключение к базе данных и инициализирует схему.
func NewNotesStore(databaseURL string) (*NotesStore, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	store := &NotesStore{db: db}
	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// Close закрывает соединение с базой данных.
func (s *NotesStore) Close() error {
	return s.db.Close()
}

// initSchema создает необходимые таблицы и индексы.
func (s *NotesStore) initSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS notes (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			text TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_notes_user ON notes(user_id);

		CREATE TABLE IF NOT EXISTS note_links (
			user_id BIGINT NOT NULL,
			from_id BIGINT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
			to_id BIGINT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, from_id, to_id)
		);
		CREATE INDEX IF NOT EXISTS idx_note_links_user ON note_links(user_id);

		CREATE TABLE IF NOT EXISTS authorized_users (
			user_id BIGINT PRIMARY KEY
		);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// AddNote сохраняет новую заметку пользователя.
func (s *NotesStore) AddNote(ctx context.Context, userID int64, text string) (Note, error) {
	query := `
		INSERT INTO notes (user_id, text, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	var note Note
	note.UserID = userID
	note.Text = text
	note.CreatedAt = time.Now().UTC()

	row := s.db.QueryRowContext(ctx, query, userID, text, note.CreatedAt)
	if err := row.Scan(&note.ID, &note.CreatedAt); err != nil {
		return Note{}, err
	}

	return note, nil
}

// ListNotes возвращает заметки пользователя.
func (s *NotesStore) ListNotes(ctx context.Context, userID int64) ([]Note, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, text, created_at
		FROM notes
		WHERE user_id = $1
		ORDER BY created_at, id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		if err := rows.Scan(&note.ID, &note.UserID, &note.Text, &note.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notes, nil
}

// DeleteNote удаляет заметку пользователя по идентификатору.
func (s *NotesStore) DeleteNote(ctx context.Context, userID int64, id int) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM notes
		WHERE user_id = $1 AND id = $2
	`, userID, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ClearNotes удаляет все заметки пользователя.
func (s *NotesStore) ClearNotes(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM notes
		WHERE user_id = $1
	`, userID)
	return err
}

// AddLink создает связь между заметками пользователя.
func (s *NotesStore) AddLink(ctx context.Context, userID int64, fromID, toID int) error {
	if fromID == toID {
		return fmt.Errorf("from and to are одинаковые")
	}

	exists, err := s.notesExist(ctx, userID, fromID, toID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("заметки не найдены")
	}

	query := `
		INSERT INTO note_links (user_id, from_id, to_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, userID, fromID, toID)
	return err
}

// ListLinks возвращает список связей заметок пользователя.
func (s *NotesStore) ListLinks(ctx context.Context, userID int64) ([]NoteLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_id, to_id
		FROM note_links
		WHERE user_id = $1
		ORDER BY from_id, to_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []NoteLink
	for rows.Next() {
		var link NoteLink
		if err := rows.Scan(&link.FromID, &link.ToID); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

// ListLinksForNote возвращает связи для конкретной заметки пользователя.
func (s *NotesStore) ListLinksForNote(ctx context.Context, userID int64, fromID int) ([]NoteLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT from_id, to_id
		FROM note_links
		WHERE user_id = $1 AND from_id = $2
		ORDER BY from_id, to_id
	`, userID, fromID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []NoteLink
	for rows.Next() {
		var link NoteLink
		if err := rows.Scan(&link.FromID, &link.ToID); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

// AuthorizeUser сохраняет идентификатор пользователя как авторизованный.
func (s *NotesStore) AuthorizeUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO authorized_users (user_id)
		VALUES ($1)
		ON CONFLICT DO NOTHING
	`, userID)
	return err
}

// IsUserAuthorized проверяет, авторизован ли пользователь.
func (s *NotesStore) IsUserAuthorized(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	row := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM authorized_users WHERE user_id = $1)
	`, userID)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// notesExist проверяет, что обе заметки принадлежат пользователю.
func (s *NotesStore) notesExist(ctx context.Context, userID int64, fromID, toID int) (bool, error) {
	var count int
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notes
		WHERE user_id = $1 AND id IN ($2, $3)
	`, userID, fromID, toID)
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count == 2, nil
}
