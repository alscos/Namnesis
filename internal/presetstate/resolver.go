package presetstate

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Resolver recovers file-backed parameters that Stompbox may omit from
// Dump Program (notably NAM.Model in the A2 fast path) by reading the active
// preset file. It never rewrites the authoritative dump; callers expose the
// recovered values separately as resolved metadata.
type Resolver struct {
	dirs []string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	path    string
	modTime time.Time
	size    int64
	params  map[string]map[string]string
}

func New(dirs []string) *Resolver {
	clean := make([]string, 0, len(dirs))
	seen := make(map[string]struct{}, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		clean = append(clean, dir)
	}
	return &Resolver{dirs: clean, cache: make(map[string]cacheEntry)}
}

// Resolve returns only values missing from the current Dump Program. The map
// is keyed as plugin -> parameter -> value.
func (r *Resolver) Resolve(programRaw string) map[string]map[string]string {
	preset, current := parseProgram(programRaw)
	if preset == "" || !safePresetName(preset) {
		return nil
	}

	presetParams := r.readPreset(preset)
	if len(presetParams) == 0 {
		return nil
	}

	resolved := make(map[string]map[string]string)
	for plugin, params := range presetParams {
		for param, value := range params {
			if value == "" || (param != "Model" && param != "Impulse") {
				continue
			}
			if current[plugin] != nil && strings.TrimSpace(current[plugin][param]) != "" {
				continue
			}
			if resolved[plugin] == nil {
				resolved[plugin] = make(map[string]string)
			}
			resolved[plugin][param] = value
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func (r *Resolver) readPreset(name string) map[string]map[string]string {
	for _, dir := range r.dirs {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		r.mu.Lock()
		cached, ok := r.cache[name]
		if ok && cached.path == path && cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
			out := cloneParams(cached.params)
			r.mu.Unlock()
			return out
		}
		r.mu.Unlock()

		params, err := parsePresetFile(path)
		if err != nil {
			continue
		}
		r.mu.Lock()
		r.cache[name] = cacheEntry{
			path:    path,
			modTime: info.ModTime(),
			size:    info.Size(),
			params:  cloneParams(params),
		}
		r.mu.Unlock()
		return params
	}
	return nil
}

func parsePresetFile(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	params := make(map[string]map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		plugin, param, value, ok := parseSetParam(scanner.Text())
		if !ok || (param != "Model" && param != "Impulse") || value == "" {
			continue
		}
		if params[plugin] == nil {
			params[plugin] = make(map[string]string)
		}
		params[plugin][param] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return params, nil
}

func parseProgram(raw string) (string, map[string]map[string]string) {
	var preset string
	params := make(map[string]map[string]string)
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SetPreset ") {
			preset = strings.TrimSpace(strings.TrimPrefix(line, "SetPreset "))
			continue
		}
		plugin, param, value, ok := parseSetParam(line)
		if !ok {
			continue
		}
		if params[plugin] == nil {
			params[plugin] = make(map[string]string)
		}
		params[plugin][param] = value
	}
	return preset, params
}

func parseSetParam(line string) (plugin, param, value string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "SetParam ") {
		return "", "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "SetParam "))
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return "", "", "", false
	}
	plugin, param = parts[0], parts[1]
	prefix := plugin + " " + param
	value = strings.TrimSpace(strings.TrimPrefix(rest, prefix))
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, "\"")
		}
	}
	return plugin, param, value, true
}

func safePresetName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return filepath.Base(name) == name && !strings.ContainsAny(name, `/\\`)
}

func cloneParams(in map[string]map[string]string) map[string]map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(in))
	for plugin, params := range in {
		out[plugin] = make(map[string]string, len(params))
		for param, value := range params {
			out[plugin][param] = value
		}
	}
	return out
}
