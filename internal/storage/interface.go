package storage

import (
	"context"

	"msp/internal/domain"
)

type ProgressStore interface {
	GetProgress(ctx context.Context, mediaID string) (float64, error)
	SetProgress(ctx context.Context, mediaID string, t float64) error
	ListRecentProgress(ctx context.Context, limit int) ([]domain.PlaybackProgress, error)
}

type PrefsStore interface {
	GetAllPrefs(ctx context.Context) (map[string]string, error)
	SetPrefs(ctx context.Context, prefs map[string]string) error
}

type FavoriteStore interface {
	ListFavorites(ctx context.Context) ([]domain.Favorite, error)
	AddFavorite(ctx context.Context, mediaID string) error
	RemoveFavorite(ctx context.Context, mediaID string) error
	IsFavorite(ctx context.Context, mediaID string) (bool, error)
}