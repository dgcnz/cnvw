package app

import (
	"os"
	"path/filepath"
)

// FindVault walks up from the canvas file looking for the marker directory an
// Obsidian vault keeps at its root. File nodes store paths relative to that
// root, so without it a canvas kept in a subfolder cannot resolve them.
func FindVault(canvasPath string) string {
	dir, err := filepath.Abs(filepath.Dir(canvasPath))
	if err != nil {
		return ""
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, ".obsidian")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ResolveFile turns a file node's path into something on disk. Vault-relative
// wins over canvas-relative because that is what the path in the document
// actually means; the canvas-relative attempt is a fallback for canvases kept
// outside a vault.
func ResolveFile(rel, vault, canvasPath string) (string, bool) {
	if rel == "" {
		return "", false
	}
	if filepath.IsAbs(rel) {
		return rel, exists(rel)
	}
	var tries []string
	if vault != "" {
		tries = append(tries, filepath.Join(vault, rel))
	}
	tries = append(tries, filepath.Join(filepath.Dir(canvasPath), rel))
	for _, p := range tries {
		if exists(p) {
			return p, true
		}
	}
	// Report the most likely path even when it is missing, so the focus pane
	// can say where it looked.
	return tries[0], false
}

func exists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// isBinary reports whether the content looks like something that should not be
// printed to a terminal.
func isBinary(b []byte) bool {
	if len(b) > 8000 {
		b = b[:8000]
	}
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}
