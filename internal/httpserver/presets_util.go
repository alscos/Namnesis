package httpserver

import (
	"errors"
	"strings"

	"namnesis-ui-gateway/internal/stompbox"
)

// helper
func fileTreeContains(ft *stompbox.FileTreeDef, value string) bool {
	v := strings.TrimSpace(value)
	for _, it := range ft.Items {
		if strings.TrimSpace(it) == v {
			return true
		}
	}
	for _, opt := range ft.Options {
		if strings.TrimSpace(opt.Value) == v {
			return true
		}
	}
	return false
}

func validatePresetName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return errors.New("preset name is empty")
	}
	// Avoid path traversal / paths. Presets are treated as filenames by Stompbox.
	if strings.ContainsAny(n, "/\\") || strings.Contains(n, "..") {
		return errors.New("path separators are not allowed")
	}
	// Avoid characters that are problematic in filenames across platforms.
	if strings.ContainsAny(n, ":*?\"<>|") {
		return errors.New("invalid characters in preset name")
	}
	// Avoid NUL + control chars
	for _, r := range n {
		if r == 0 || r < 0x20 {
			return errors.New("control chars not allowed")
		}
	}
	// Avoid trailing dot/space (Windows filename quirk, harmless elsewhere).
	if strings.HasSuffix(n, " ") || strings.HasSuffix(n, ".") {
		return errors.New("preset name cannot end with space or dot")
	}
	if len(n) > 200 {
		return errors.New("too long")
	}
	return nil
}
