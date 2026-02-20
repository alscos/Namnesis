package httpserver

import (
	"errors"
	"strings"

	"namnesis-ui-gateway/internal/stompbox"
)

// helper
func fileTreeContains(ft *stompbox.FileTreeDef, value string) bool {
	for _, it := range ft.Items {
		if it == value {
			return true
		}
	}
	for _, opt := range ft.Options { // si Options existe en tu type
		if opt.Value == value {
			return true
		}
	}
	return false
}

func validatePresetName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return errors.New("empty")
	}
	// Avoid path traversal / paths. Presets are treated as filenames by Stompbox.
	if strings.ContainsAny(n, "/\\") || strings.Contains(n, "..") {
		return errors.New("path separators not allowed")
	}
	// Avoid NUL + control chars
	for _, r := range n {
		if r == 0 || r < 0x20 {
			return errors.New("control chars not allowed")
		}
	}
	if len(n) > 200 {
		return errors.New("too long")
	}
	return nil
}
