package main

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NotesStore управляет хранением заметок в PostgreSQL через GORM.
type NotesStore struct {
	db *gorm.DB
}

// NewNotesStore создает подключение к базе данных и выполняет миграции.
func NewNotesStore(databaseURL string) (*NotesStore, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.WithContext(context.Background()).AutoMigrate(&Note{}, &NoteLink{}, &AuthorizedUser{}); err != nil {
		return nil, err
	}

	return &NotesStore{db: db}, nil
}

// Close закрывает соединение с базой данных.
func (s *NotesStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// AddNote сохраняет новую активную заметку пользователя.
func (s *NotesStore) AddNote(ctx context.Context, userID int64, text string) (Note, error) {
	note := Note{UserID: userID, Text: text, Status: NoteStatusActive}
	if err := s.db.WithContext(ctx).Create(&note).Error; err != nil {
		return Note{}, err
	}
	return note, nil
}

// ListNotes возвращает только активные заметки пользователя.
func (s *NotesStore) ListNotes(ctx context.Context, userID int64) ([]Note, error) {
	var notes []Note
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, NoteStatusActive).
		Order("created_at asc, id asc").
		Find(&notes).Error
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// DeleteNote не удаляет запись физически, а меняет статус на deleted.
func (s *NotesStore) DeleteNote(ctx context.Context, userID int64, id int) (bool, error) {
	result := s.db.WithContext(ctx).
		Model(&Note{}).
		Where("user_id = ? AND id = ? AND status = ?", userID, id, NoteStatusActive).
		Update("status", NoteStatusDeleted)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// ClearNotes помечает все активные заметки пользователя как удаленные.
func (s *NotesStore) ClearNotes(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).
		Model(&Note{}).
		Where("user_id = ? AND status = ?", userID, NoteStatusActive).
		Update("status", NoteStatusDeleted).Error
}

// AddLink создает связь между активными заметками пользователя.
func (s *NotesStore) AddLink(ctx context.Context, userID int64, fromID, toID int) (NoteLink, error) {
	if fromID == toID {
		return NoteLink{}, fmt.Errorf("from_id and to_id must be different")
	}

	exists, err := s.notesExist(ctx, userID, uint(fromID), uint(toID))
	if err != nil {
		return NoteLink{}, err
	}
	if !exists {
		return NoteLink{}, fmt.Errorf("notes not found or deleted")
	}

	link := NoteLink{UserID: userID, FromID: uint(fromID), ToID: uint(toID)}
	if err := s.db.WithContext(ctx).Create(&link).Error; err != nil {
		return NoteLink{}, err
	}
	return link, nil
}

// UpdateLink изменяет целевую заметку у связи.
func (s *NotesStore) UpdateLink(ctx context.Context, userID int64, linkID uint, toID uint) (bool, error) {
	var existing NoteLink
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", linkID, userID).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	exists, err := s.notesExist(ctx, userID, existing.FromID, toID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("notes not found or deleted")
	}

	if err := s.db.WithContext(ctx).Model(&NoteLink{}).
		Where("id = ? AND user_id = ?", linkID, userID).
		Update("to_id", toID).Error; err != nil {
		return false, err
	}

	return true, nil
}

// DeleteLink удаляет связь между заметками.
func (s *NotesStore) DeleteLink(ctx context.Context, userID int64, linkID uint) (bool, error) {
	res := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", linkID, userID).Delete(&NoteLink{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ListLinks возвращает список связей заметок пользователя.
func (s *NotesStore) ListLinks(ctx context.Context, userID int64) ([]NoteLink, error) {
	var links []NoteLink
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("from_id asc, to_id asc").
		Find(&links).Error
	if err != nil {
		return nil, err
	}
	return links, nil
}

// ListLinksForNote возвращает связи для конкретной заметки пользователя.
func (s *NotesStore) ListLinksForNote(ctx context.Context, userID int64, fromID int) ([]NoteLink, error) {
	var links []NoteLink
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND from_id = ?", userID, fromID).
		Order("to_id asc").
		Find(&links).Error
	if err != nil {
		return nil, err
	}
	return links, nil
}

// AuthorizeUser сохраняет идентификатор пользователя как авторизованный.
func (s *NotesStore) AuthorizeUser(ctx context.Context, userID int64) error {
	au := AuthorizedUser{UserID: userID}
	return s.db.WithContext(ctx).FirstOrCreate(&au, AuthorizedUser{UserID: userID}).Error
}

// IsUserAuthorized проверяет, авторизован ли пользователь.
func (s *NotesStore) IsUserAuthorized(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&AuthorizedUser{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// notesExist проверяет, что обе заметки активны и принадлежат пользователю.
func (s *NotesStore) notesExist(ctx context.Context, userID int64, fromID, toID uint) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&Note{}).
		Where("user_id = ? AND status = ? AND id IN ?", userID, NoteStatusActive, []uint{fromID, toID}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count == 2, nil
}
