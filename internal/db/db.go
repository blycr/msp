package db

import (
	"database/sql"
	"encoding/json"
	"msp/internal/types"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(dbPath string) error {
	if dbPath == "" {
		dbPath = "msp.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	return createTables()
}

func createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS media_items (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		name TEXT,
		ext TEXT,
		kind TEXT,
		share_label TEXT,
		size INTEGER,
		mod_time INTEGER,
		subtitles TEXT, -- JSON array
		audio_cover TEXT,
		audio_lyrics TEXT,
		scan_id INTEGER,
		share_root TEXT,
		UNIQUE(path)
	);
	CREATE INDEX IF NOT EXISTS idx_kind ON media_items(kind);
	CREATE INDEX IF NOT EXISTS idx_share_label ON media_items(share_label);
	CREATE INDEX IF NOT EXISTS idx_scan_kind ON media_items(scan_id, kind);
	CREATE INDEX IF NOT EXISTS idx_scan_share_label ON media_items(scan_id, share_label);

	CREATE TABLE IF NOT EXISTS media_scans (
		cache_key TEXT PRIMARY KEY,
		scan_id INTEGER NOT NULL,
		built_at INTEGER NOT NULL,
		complete INTEGER NOT NULL
	);
	`
	_, err := DB.Exec(query)
	if err != nil {
		return err
	}
	if err := ensureMediaItemsColumn("scan_id"); err != nil {
		return err
	}
	if err := ensureMediaItemsColumn("share_root"); err != nil {
		return err
	}
	return nil
}

type ScanMeta struct {
	ScanID   int64
	BuiltAt  int64
	Complete bool
}

func GetScanMeta(cacheKey string) (ScanMeta, bool, error) {
	if DB == nil || strings.TrimSpace(cacheKey) == "" {
		return ScanMeta{}, false, nil
	}
	var scanID int64
	var builtAt int64
	var complete int
	err := DB.QueryRow(`SELECT scan_id, built_at, complete FROM media_scans WHERE cache_key = ?`, cacheKey).Scan(&scanID, &builtAt, &complete)
	if err == sql.ErrNoRows {
		return ScanMeta{}, false, nil
	}
	if err != nil {
		return ScanMeta{}, false, err
	}
	return ScanMeta{ScanID: scanID, BuiltAt: builtAt, Complete: complete != 0}, true, nil
}

func SetScanMeta(tx *sql.Tx, cacheKey string, meta ScanMeta) error {
	if DB == nil || tx == nil || strings.TrimSpace(cacheKey) == "" || meta.ScanID <= 0 || meta.BuiltAt <= 0 {
		return nil
	}
	complete := 0
	if meta.Complete {
		complete = 1
	}
	_, err := tx.Exec(
		`INSERT INTO media_scans (cache_key, scan_id, built_at, complete)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(cache_key) DO UPDATE SET
		   scan_id = excluded.scan_id,
		   built_at = excluded.built_at,
		   complete = excluded.complete`,
		cacheKey, meta.ScanID, meta.BuiltAt, complete,
	)
	return err
}

func PrepareUpsertMediaItem(tx *sql.Tx) (*sql.Stmt, error) {
	if DB == nil || tx == nil {
		return nil, nil
	}
	return tx.Prepare(`
		INSERT INTO media_items (
			id, path, name, ext, kind, share_label,
			size, mod_time, subtitles, audio_cover, audio_lyrics,
			scan_id, share_root
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			id = excluded.id,
			name = excluded.name,
			ext = excluded.ext,
			kind = excluded.kind,
			share_label = excluded.share_label,
			size = excluded.size,
			mod_time = excluded.mod_time,
			subtitles = excluded.subtitles,
			audio_cover = excluded.audio_cover,
			audio_lyrics = excluded.audio_lyrics,
			scan_id = excluded.scan_id,
			share_root = excluded.share_root
	`)
}

func DeleteStaleByScan(tx *sql.Tx, scanID int64, shareRoots []string) error {
	if DB == nil || tx == nil || scanID <= 0 || len(shareRoots) == 0 {
		return nil
	}
	ph := make([]string, 0, len(shareRoots))
	args := make([]any, 0, 1+len(shareRoots))
	args = append(args, scanID)
	for range shareRoots {
		ph = append(ph, "?")
	}
	for _, r := range shareRoots {
		args = append(args, r)
	}
	q := `DELETE FROM media_items WHERE scan_id != ? AND share_root IN (` + strings.Join(ph, ",") + `)`
	_, err := tx.Exec(q, args...)
	return err
}

func DeleteByShareRootsNotIn(tx *sql.Tx, shareRoots []string) error {
	if DB == nil || tx == nil {
		return nil
	}
	if len(shareRoots) == 0 {
		_, err := tx.Exec(`DELETE FROM media_items`)
		return err
	}
	ph := make([]string, 0, len(shareRoots))
	args := make([]any, 0, len(shareRoots))
	for _, r := range shareRoots {
		ph = append(ph, "?")
		args = append(args, r)
	}
	q := `DELETE FROM media_items WHERE share_root NOT IN (` + strings.Join(ph, ",") + `)`
	_, err := tx.Exec(q, args...)
	return err
}

func QueryMediaItems(scanID int64, kind string) ([]types.MediaItem, error) {
	if DB == nil || scanID <= 0 || strings.TrimSpace(kind) == "" {
		return nil, nil
	}
	rows, err := DB.Query(
		`SELECT id, name, ext, kind, share_label, size, mod_time, subtitles, audio_cover, audio_lyrics
		 FROM media_items
		 WHERE scan_id = ? AND kind = ?
		 ORDER BY share_label, lower(name)`,
		scanID, kind,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MediaItem, 0, 128)
	for rows.Next() {
		var it types.MediaItem
		var subs string
		var cover string
		var lyrics string
		if err := rows.Scan(&it.ID, &it.Name, &it.Ext, &it.Kind, &it.ShareLabel, &it.Size, &it.ModTime, &subs, &cover, &lyrics); err != nil {
			return nil, err
		}
		if strings.TrimSpace(subs) != "" {
			var v []types.Subtitle
			_ = json.Unmarshal([]byte(subs), &v)
			it.Subtitles = v
		}
		if strings.TrimSpace(cover) != "" {
			it.CoverID = cover
		}
		if strings.TrimSpace(lyrics) != "" {
			it.LyricsID = lyrics
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func CountMediaItems(scanID int64, kind string) (int, error) {
	if DB == nil || scanID <= 0 || strings.TrimSpace(kind) == "" {
		return 0, nil
	}
	n := 0
	err := DB.QueryRow(`SELECT COUNT(1) FROM media_items WHERE scan_id = ? AND kind = ?`, scanID, kind).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func ensureMediaItemsColumn(name string) error {
	if DB == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	rows, err := DB.Query(`PRAGMA table_info(media_items)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var n string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &n, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(n, name) {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found {
		return nil
	}
	switch strings.ToLower(name) {
	case "scan_id":
		_, err = DB.Exec(`ALTER TABLE media_items ADD COLUMN scan_id INTEGER`)
		return err
	case "share_root":
		_, err = DB.Exec(`ALTER TABLE media_items ADD COLUMN share_root TEXT`)
		return err
	default:
		return nil
	}
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
