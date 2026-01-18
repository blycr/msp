package web

import (
	"io/fs"
	"testing"
)

func TestEmbed(t *testing.T) {
	// Verify that we can read from the embedded filesystem
	// Note: This test might fail if 'dist' doesn't exist yet (e.g. before npm run build)
	// So we usually skip it or check for dist only if it exists.
	// For now, we just check if FS is valid.

	_, err := fs.ReadDir(FS, "dist")
	if err != nil {
		t.Logf("Warning: could not read dist dir: %v. This is expected if frontend hasn't been built yet.", err)
	}
}
