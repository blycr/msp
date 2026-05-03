package storage

import "context"

type Store struct {
	db *SQLite
}

func NewStore(sq *SQLite) *Store {
	return &Store{db: sq}
}

func (s *Store) GetProgress(ctx context.Context, mediaID string) (float64, error) {
	if s.db == nil || s.db.DB() == nil || mediaID == "" {
		return 0, nil
	}
	return s.db.GetProgress(ctx, mediaID)
}

func (s *Store) SetProgress(ctx context.Context, mediaID string, t float64) error {
	if s.db == nil || s.db.DB() == nil || mediaID == "" {
		return nil
	}
	return s.db.SetProgress(ctx, mediaID, t)
}

func (s *Store) GetAllPrefs(ctx context.Context) (map[string]string, error) {
	if s.db == nil || s.db.DB() == nil {
		return map[string]string{}, nil
	}
	return s.db.GetAllPrefs(ctx)
}

func (s *Store) SetPrefs(ctx context.Context, prefs map[string]string) error {
	if s.db == nil || s.db.DB() == nil || len(prefs) == 0 {
		return nil
	}
	return s.db.SetPrefs(ctx, prefs)
}