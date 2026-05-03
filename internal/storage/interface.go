package storage

import "context"

type ProgressStore interface {
	GetProgress(ctx context.Context, mediaID string) (float64, error)
	SetProgress(ctx context.Context, mediaID string, t float64) error
}

type PrefsStore interface {
	GetAllPrefs(ctx context.Context) (map[string]string, error)
	SetPrefs(ctx context.Context, prefs map[string]string) error
}