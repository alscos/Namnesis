package httpserver

import (
	"errors"
	"encoding/json"
	"net/http"
	"time"
	"strings"
	"github.com/go-chi/chi/v5"
	"regexp"

	"namnesis-ui-gateway/internal/stompbox"
	
)

func (s *Server) handleDumpConfigRaw(w http.ResponseWriter, r *http.Request) {
	out, err := s.sb.DumpConfig()
	if err != nil {
		http.Error(w, "dumpconfig error: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (s *Server) handleProgramRaw(w http.ResponseWriter, r *http.Request) {
	out, err := s.sb.DumpProgram()
	if err != nil {
		http.Error(w, "program error: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (s *Server) handlePresetsRaw(w http.ResponseWriter, r *http.Request) {
	out, err := s.sb.ListPresets()
	if err != nil {
		http.Error(w, "presets error: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}
func (s *Server) handleDumpConfigPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if s.tpl == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}

	if err := s.tpl.ExecuteTemplate(w, "dumpconfig.html", nil); err != nil {
		http.Error(w, "template render error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
func (s *Server) handleConfigParsedDebug(w http.ResponseWriter, r *http.Request) {
	raw, err := s.sb.DumpConfig()
	if err != nil {
		http.Error(w, "dumpconfig error: "+err.Error(), http.StatusBadGateway)
		return
	}

	parsed, err := stompbox.ParseDumpConfig(raw)
	if err != nil {
		http.Error(w, "parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(parsed)
}

func (s *Server) handleProgramParsedDebug(w http.ResponseWriter, r *http.Request) {
	raw, err := s.sb.DumpProgram()
	if err != nil {
		http.Error(w, "program error: "+err.Error(), http.StatusBadGateway)
		return
	}

	parsed, err := stompbox.ParseDumpProgram(raw)
	if err != nil {
		http.Error(w, "parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(parsed)
}

type pluginEnabledReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handlePluginEnabled(w http.ResponseWriter, r *http.Request) {
	plugin := chi.URLParam(r, "plugin")
	if plugin == "" {
		http.Error(w, "missing plugin name", http.StatusBadRequest)
		return
	}

	var req pluginEnabledReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	val := "0"
	if req.Enabled {
		val = "1"
	}

	// Use your existing Stompbox client abstraction (same style as handleSetFileParam)
	if err := s.sb.SetParam(plugin, "Enabled", val); err != nil {
		http.Error(w, "setparam error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Return JSON in the same style as your other handlers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"plugin":  plugin,
		"enabled": req.Enabled,
	})
}


type stateResponse struct {
	Meta struct {
		Now string `json:"now"`
	} `json:"meta"`

	DumpConfig struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"dumpConfig"`

	Program struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"program"`

	Presets struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"presets"`
}

type presetLoadRequest struct {
    Name string `json:"name"`
}
type setFileParamRequest struct {
	Plugin string `json:"plugin"`
	Param  string `json:"param"`
	Value  string `json:"value"`
}


func (s *Server) handleSetFileParam(w http.ResponseWriter, r *http.Request) {
	var req setFileParamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.Plugin == "" || req.Param == "" || req.Value == "" {
		http.Error(w, "missing fields: plugin, param, value are required", http.StatusBadRequest)
		return
	}

	// 1) Validate against DumpConfig-parsed (authoritative config metadata)
	raw, err := s.sb.DumpConfig()
	if err != nil {
		http.Error(w, "dumpconfig error: "+err.Error(), http.StatusBadGateway)
		return
	}
	cfg, err := stompbox.ParseDumpConfig(raw)
	if err != nil {
		http.Error(w, "parse dumpconfig error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// IMPORTANT: DumpConfig is keyed by *base plugin type* (e.g. "ConvoReverb"),
	// while runtime params can reference instances (e.g. "ConvoReverb_2").
	pluginInstance := req.Plugin
	base := regexp.MustCompile(`_\d+$`).ReplaceAllString(pluginInstance, "")

	p, ok := cfg.Plugins[base]
	if !ok {
		http.Error(w, "unknown plugin: "+pluginInstance, http.StatusBadRequest)
		return
	}

	paramDef, ok := p.Params[req.Param]
	if !ok {
		http.Error(w, "unknown param for plugin: "+pluginInstance+"."+req.Param, http.StatusBadRequest)
		return
	}
	if paramDef.Type != "File" {
		http.Error(w, "param is not a File type: "+pluginInstance+"."+req.Param, http.StatusBadRequest)
		return
	}

	// If we have a file tree for this param, ensure the value is valid.
	// If ParseDumpConfig didn't build a tree for this param, we allow setting anyway.
	if p.FileTrees != nil {
		if ft, ok := p.FileTrees[req.Param]; ok && ft != nil {
			if !fileTreeContains(ft, req.Value) {
				http.Error(w, "value not present in file tree: "+req.Value, http.StatusBadRequest)
				return
			}
		}
	}

	// 2) Apply to running Stompbox (apply to the *instance*, not the base type)
	if err := s.sb.SetParam(pluginInstance, req.Param, req.Value); err != nil {
		http.Error(w, "setparam error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// 3) Return OK
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"plugin": req.Plugin,
		"param":  req.Param,
		"value":  req.Value,
	})
}

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


func (s *Server) handlePresetLoad(w http.ResponseWriter, r *http.Request) {
    var req presetLoadRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad json", http.StatusBadRequest)
        return
    }
    if req.Name == "" || req.Name == "---" {
        http.Error(w, "missing preset name", http.StatusBadRequest)
        return
    }

    // This method should send the TCP command: LoadPreset <name>
    if err := s.sb.LoadPreset(req.Name); err != nil {
        http.Error(w, "load preset error: "+err.Error(), http.StatusBadGateway)
        return
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(map[string]any{
        "ok":   true,
        "name": req.Name,
    })
}
type savePresetRequest struct {
    Name string `json:"name"`
}

func (s *Server) handlePresetSave(w http.ResponseWriter, r *http.Request) {
    var req savePresetRequest
    _ = json.NewDecoder(r.Body).Decode(&req) // allow empty body

    name := strings.TrimSpace(req.Name)

    // If no name provided, save "current preset" (from DumpProgram parse)
    if name == "" {
        raw, err := s.sb.DumpProgram()
        if err != nil {
            http.Error(w, "DumpProgram failed: "+err.Error(), http.StatusBadGateway)
            return
        }

        parsed, err := stompbox.ParseDumpProgram(raw)
        if err != nil {
            http.Error(w, "ParseDumpProgram failed: "+err.Error(), http.StatusInternalServerError)
            return
        }

        name = strings.TrimSpace(parsed.ActivePreset)
        if name == "" {
            http.Error(w, "cannot save: ActivePreset is empty", http.StatusBadRequest)
            return
        }
    }

    if err := s.sb.SavePreset(name); err != nil {
        http.Error(w, "SavePreset failed: "+err.Error(), http.StatusBadGateway)
        return
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    _ = json.NewEncoder(w).Encode(map[string]any{
        "ok":     true,
        "preset": name,
    })
}


type presetNameRequest struct {
	Name string `json:"name"`
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

func (s *Server) handlePresetSaveAs(w http.ResponseWriter, r *http.Request) {
	var req presetNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if err := validatePresetName(name); err != nil {
		http.Error(w, "invalid preset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.sb.SavePreset(name); err != nil {
		http.Error(w, "SavePreset failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"preset": name,
	})
}

func (s *Server) handlePresetDelete(w http.ResponseWriter, r *http.Request) {
	var req presetNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if err := validatePresetName(name); err != nil {
		http.Error(w, "invalid preset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.sb.DeletePreset(name); err != nil {
		http.Error(w, "DeletePreset failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"preset": name,
	})
}


type loadPresetRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleLoadPreset(w http.ResponseWriter, r *http.Request) {
	var req loadPresetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "invalid json body, expected {\"name\":\"...\"}", http.StatusBadRequest)
		return
	}

	if err := s.sb.LoadPreset(req.Name); err != nil {
		http.Error(w, "loadpreset error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Return OK; UI will refresh via /api/state
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleUIPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if s.tpl == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}

	if err := s.tpl.ExecuteTemplate(w, "ui.html", nil); err != nil {
		http.Error(w, "template render error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}



func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	var resp stateResponse
	resp.Meta.Now = time.Now().Format(time.RFC3339)

	// Dump Config
	t0 := time.Now()
	out, err := s.sb.DumpConfig()
	resp.DumpConfig.Duration = time.Since(t0).String()
	if err != nil {
		resp.DumpConfig.Error = err.Error()
	} else {
		resp.DumpConfig.Raw = out
	}

	// Dump Program
	t1 := time.Now()
	out, err = s.sb.DumpProgram()
	resp.Program.Duration = time.Since(t1).String()
	if err != nil {
		resp.Program.Error = err.Error()
	} else {
		resp.Program.Raw = out
	}

	// List Presets
	t2 := time.Now()
	out, err = s.sb.ListPresets()
	resp.Presets.Duration = time.Since(t2).String()
	if err != nil {
		resp.Presets.Error = err.Error()
	} else {
		resp.Presets.Raw = out
	}

	
	// Decide HTTP status
	status := http.StatusOK
	allFailed := resp.DumpConfig.Error != "" &&
		resp.Program.Error != "" &&
		resp.Presets.Error != ""

	if allFailed {
		status = http.StatusBadGateway
	}

	// Write headers + status ONCE
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// Encode response body
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)

}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	if s.sys == nil {
		http.Error(w, "sysinfo collector not initialized", http.StatusInternalServerError)
		return
	}

	// Use request context (respects client disconnect + server timeouts)
	ctx := r.Context()
	snap := s.sys.Snapshot(ctx)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snap)
}
