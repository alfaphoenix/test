package usecase

import (
	"context"
	"errors"

	"test/internal/domain"
)

var ErrUnauthorized = errors.New("unauthorized")

type NoteService struct {
	repo        domain.Repository
	botLogin    string
	botPassword string
}

func NewNoteService(repo domain.Repository, botLogin, botPassword string) *NoteService {
	return &NoteService{repo: repo, botLogin: botLogin, botPassword: botPassword}
}

func (s *NoteService) Close() error { return s.repo.Close() }

func (s *NoteService) Login(ctx context.Context, userID int64, login, password string) (bool, error) {
	if login != s.botLogin || password != s.botPassword {
		return false, nil
	}
	if err := s.repo.AuthorizeUser(ctx, userID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *NoteService) EnsureAuthorized(ctx context.Context, userID int64) error {
	ok, err := s.repo.IsUserAuthorized(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnauthorized
	}
	return nil
}

func (s *NoteService) AddNote(ctx context.Context, userID int64, title, text string) (domain.Note, error) {
	return s.repo.AddNote(ctx, userID, title, text)
}
func (s *NoteService) ListNotes(ctx context.Context, userID int64) ([]domain.Note, error) {
	return s.repo.ListNotes(ctx, userID)
}
func (s *NoteService) GetNoteByTitle(ctx context.Context, userID int64, title string) (domain.Note, bool, error) {
	return s.repo.GetNoteByTitle(ctx, userID, title)
}
func (s *NoteService) DeleteNote(ctx context.Context, userID int64, id int) (bool, error) {
	return s.repo.DeleteNote(ctx, userID, id)
}
func (s *NoteService) ClearNotes(ctx context.Context, userID int64) error {
	return s.repo.ClearNotes(ctx, userID)
}
func (s *NoteService) AddLink(ctx context.Context, userID int64, fromID, toID int) (domain.NoteLink, error) {
	return s.repo.AddLink(ctx, userID, fromID, toID)
}
func (s *NoteService) UpdateLink(ctx context.Context, userID int64, linkID uint, toID uint) (bool, error) {
	return s.repo.UpdateLink(ctx, userID, linkID, toID)
}
func (s *NoteService) DeleteLink(ctx context.Context, userID int64, linkID uint) (bool, error) {
	return s.repo.DeleteLink(ctx, userID, linkID)
}
func (s *NoteService) ListLinks(ctx context.Context, userID int64) ([]domain.NoteLink, error) {
	return s.repo.ListLinks(ctx, userID)
}
func (s *NoteService) ListLinksForNote(ctx context.Context, userID int64, fromID int) ([]domain.NoteLink, error) {
	return s.repo.ListLinksForNote(ctx, userID, fromID)
}
