package storage

import (
	"context"

	"msp/internal/domain"
)

type Store struct {
	db *SQLite
}

func NewStore(sq *SQLite) *Store {
	return &Store{db: sq}
}

// unavailable reports whether the underlying database is missing.
func (s *Store) unavailable() bool {
	return s.db == nil || s.db.DB() == nil
}

func (s *Store) GetProgress(ctx context.Context, mediaID string) (float64, error) {
	if s.unavailable() {
		return 0, ErrUnavailable
	}
	if mediaID == "" {
		return 0, nil
	}
	return s.db.GetProgress(ctx, mediaID)
}

func (s *Store) SetProgress(ctx context.Context, mediaID string, t float64) error {
	if s.unavailable() {
		return ErrUnavailable
	}
	if mediaID == "" {
		return nil
	}
	return s.db.SetProgress(ctx, mediaID, t)
}

func (s *Store) ListRecentProgress(ctx context.Context, limit int) ([]domain.PlaybackProgress, error) {
	if s.unavailable() {
		return nil, ErrUnavailable
	}
	return s.db.ListRecentProgress(ctx, limit)
}

func (s *Store) GetAllPrefs(ctx context.Context) (map[string]string, error) {
	if s.unavailable() {
		return nil, ErrUnavailable
	}
	return s.db.GetAllPrefs(ctx)
}

func (s *Store) SetPrefs(ctx context.Context, prefs map[string]string) error {
	if s.unavailable() {
		return ErrUnavailable
	}
	if len(prefs) == 0 {
		return nil
	}
	return s.db.SetPrefs(ctx, prefs)
}

func (s *Store) ListFavorites(ctx context.Context) ([]domain.Favorite, error) {
	if s.unavailable() {
		return nil, ErrUnavailable
	}
	return s.db.ListFavorites(ctx)
}

func (s *Store) AddFavorite(ctx context.Context, mediaID string) error {
	if s.unavailable() {
		return ErrUnavailable
	}
	if mediaID == "" {
		return nil
	}
	return s.db.AddFavorite(ctx, mediaID)
}

func (s *Store) RemoveFavorite(ctx context.Context, mediaID string) error {
	if s.unavailable() {
		return ErrUnavailable
	}
	if mediaID == "" {
		return nil
	}
	return s.db.RemoveFavorite(ctx, mediaID)
}

func (s *Store) IsFavorite(ctx context.Context, mediaID string) (bool, error) {
	if s.unavailable() {
		return false, ErrUnavailable
	}
	if mediaID == "" {
		return false, nil
	}
	return s.db.IsFavorite(ctx, mediaID)
}
