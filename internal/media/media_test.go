package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"msp/internal/config"
	"msp/internal/domain"
)

func TestBuildMediaResponseEmpty(t *testing.T) {
	ctx := context.Background()
	shares := []domain.Share{{Label: "Empty", Path: "/nonexistent_path_12345"}}
	resp, err := BuildMediaResponse(ctx, shares, config.BlacklistConfig{}, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Videos) != 0 {
		t.Errorf("expected 0 videos, got %d", len(resp.Videos))
	}
	if len(resp.Audios) != 0 {
		t.Errorf("expected 0 audios, got %d", len(resp.Audios))
	}
	if len(resp.Shares) != 1 {
		t.Errorf("expected 1 share, got %d", len(resp.Shares))
	}
}

func TestBuildMediaResponseWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	videoDir := filepath.Join(tmpDir, "videos")
	audioDir := filepath.Join(tmpDir, "audios")
	if err := os.MkdirAll(videoDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(audioDir, 0750); err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(filepath.Join(videoDir, "test.mp4"), []byte("fake"), 0600)
	_ = os.WriteFile(filepath.Join(audioDir, "song.mp3"), []byte("fake"), 0600)

	ctx := context.Background()
	shares := []domain.Share{
		{Label: "Videos", Path: videoDir},
		{Label: "Audio", Path: audioDir},
	}
	resp, err := BuildMediaResponse(ctx, shares, config.BlacklistConfig{}, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Videos) != 1 {
		t.Errorf("expected 1 video, got %d", len(resp.Videos))
	}
	if len(resp.Audios) != 1 {
		t.Errorf("expected 1 audio, got %d", len(resp.Audios))
	}
	if len(resp.Shares) != 2 {
		t.Errorf("expected 2 shares, got %d", len(resp.Shares))
	}
}

func TestBuildMediaResponseMaxItems(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "a.mp4"), []byte("a"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDir, "b.mp4"), []byte("b"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDir, "c.mp4"), []byte("c"), 0600)

	ctx := context.Background()
	shares := []domain.Share{{Label: "V", Path: tmpDir}}
	resp, err := BuildMediaResponse(ctx, shares, config.BlacklistConfig{}, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Videos) > 2 {
		t.Errorf("expected at most 2 videos with maxItems=2, got %d", len(resp.Videos))
	}
}

func TestBuildMediaResponseSorted(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "zeta.mp4"), []byte("z"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDir, "alpha.mp4"), []byte("a"), 0600)

	ctx := context.Background()
	shares := []domain.Share{{Label: "V", Path: tmpDir}}
	resp, err := BuildMediaResponse(ctx, shares, config.BlacklistConfig{}, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(resp.Videos))
	}
	if resp.Videos[0].Name > resp.Videos[1].Name {
		t.Errorf("videos should be sorted by name, got %s before %s", resp.Videos[0].Name, resp.Videos[1].Name)
	}
}
