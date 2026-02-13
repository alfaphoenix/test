package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"test/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type noteModel struct {
	ID        uint              `gorm:"primaryKey"`
	UserID    int64             `gorm:"index;not null"`
	Title     string            `gorm:"type:varchar(255);not null"`
	Text      string            `gorm:"type:text;not null"`
	Status    domain.NoteStatus `gorm:"type:varchar(16);not null;default:'active';index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type noteLinkModel struct {
	ID        uint `gorm:"primaryKey"`
	UserID    int64
	FromID    uint
	ToID      uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

type authorizedUserModel struct {
	UserID int64 `gorm:"primaryKey"`
}

type Store struct{ db *gorm.DB }

func NewStore(databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.WithContext(context.Background()).AutoMigrate(&noteModel{}, &noteLinkModel{}, &authorizedUserModel{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func toDomainNote(m noteModel) domain.Note {
	return domain.Note{ID: m.ID, UserID: m.UserID, Title: m.Title, Text: m.Text, Status: m.Status, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func toDomainLink(m noteLinkModel) domain.NoteLink {
	return domain.NoteLink{ID: m.ID, UserID: m.UserID, FromID: m.FromID, ToID: m.ToID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}

func (s *Store) AddNote(ctx context.Context, userID int64, title, text string) (domain.Note, error) {
	m := noteModel{UserID: userID, Title: title, Text: text, Status: domain.NoteStatusActive}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return domain.Note{}, err
	}
	return toDomainNote(m), nil
}
func (s *Store) ListNotes(ctx context.Context, userID int64) ([]domain.Note, error) {
	var models []noteModel
	err := s.db.WithContext(ctx).Where("user_id = ? AND status = ?", userID, domain.NoteStatusActive).Order("created_at asc, id asc").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.Note, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainNote(m))
	}
	return out, nil
}
func (s *Store) GetNoteByTitle(ctx context.Context, userID int64, title string) (domain.Note, bool, error) {
	var m noteModel
	err := s.db.WithContext(ctx).Where("user_id = ? AND status = ? AND LOWER(title) = LOWER(?)", userID, domain.NoteStatusActive, title).Order("created_at asc, id asc").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Note{}, false, nil
		}
		return domain.Note{}, false, err
	}
	return toDomainNote(m), true, nil
}
func (s *Store) DeleteNote(ctx context.Context, userID int64, id int) (bool, error) {
	res := s.db.WithContext(ctx).Model(&noteModel{}).Where("user_id = ? AND id = ? AND status = ?", userID, id, domain.NoteStatusActive).Update("status", domain.NoteStatusDeleted)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
func (s *Store) ClearNotes(ctx context.Context, userID int64) error {
	return s.db.WithContext(ctx).Model(&noteModel{}).Where("user_id = ? AND status = ?", userID, domain.NoteStatusActive).Update("status", domain.NoteStatusDeleted).Error
}
func (s *Store) AddLink(ctx context.Context, userID int64, fromID, toID int) (domain.NoteLink, error) {
	if fromID == toID {
		return domain.NoteLink{}, fmt.Errorf("from_id and to_id must be different")
	}
	ok, err := s.notesExist(ctx, userID, uint(fromID), uint(toID))
	if err != nil {
		return domain.NoteLink{}, err
	}
	if !ok {
		return domain.NoteLink{}, fmt.Errorf("notes not found or deleted")
	}
	m := noteLinkModel{UserID: userID, FromID: uint(fromID), ToID: uint(toID)}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return domain.NoteLink{}, err
	}
	return toDomainLink(m), nil
}
func (s *Store) UpdateLink(ctx context.Context, userID int64, linkID uint, toID uint) (bool, error) {
	var existing noteLinkModel
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", linkID, userID).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	ok, err := s.notesExist(ctx, userID, existing.FromID, toID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("notes not found or deleted")
	}
	if err := s.db.WithContext(ctx).Model(&noteLinkModel{}).Where("id = ? AND user_id = ?", linkID, userID).Update("to_id", toID).Error; err != nil {
		return false, err
	}
	return true, nil
}
func (s *Store) DeleteLink(ctx context.Context, userID int64, linkID uint) (bool, error) {
	res := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", linkID, userID).Delete(&noteLinkModel{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
func (s *Store) ListLinks(ctx context.Context, userID int64) ([]domain.NoteLink, error) {
	var models []noteLinkModel
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("from_id asc, to_id asc").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.NoteLink, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainLink(m))
	}
	return out, nil
}
func (s *Store) ListLinksForNote(ctx context.Context, userID int64, fromID int) ([]domain.NoteLink, error) {
	var models []noteLinkModel
	err := s.db.WithContext(ctx).Where("user_id = ? AND from_id = ?", userID, fromID).Order("to_id asc").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.NoteLink, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainLink(m))
	}
	return out, nil
}
func (s *Store) AuthorizeUser(ctx context.Context, userID int64) error {
	au := authorizedUserModel{UserID: userID}
	return s.db.WithContext(ctx).FirstOrCreate(&au, authorizedUserModel{UserID: userID}).Error
}
func (s *Store) IsUserAuthorized(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&authorizedUserModel{}).Where("user_id = ?", userID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func (s *Store) notesExist(ctx context.Context, userID int64, fromID, toID uint) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&noteModel{}).Where("user_id = ? AND status = ? AND id IN ?", userID, domain.NoteStatusActive, []uint{fromID, toID}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count == 2, nil
}
