package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"namnesis-ui-gateway/internal/stompbox"
)

type presetCurrentResponse struct {
	CurrentPreset string `json:"currentPreset"`
	Error         string `json:"error,omitempty"`
}
type presetLoadRequest struct {
	Name string `json:"name"`
}
type savePresetRequest struct {
	Name string `json:"name"`
}
type presetNameRequest struct {
	Name string `json:"name"`
}
type loadPresetRequest struct {
	Name string `json:"name"`
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
func (s *Server) handlePresetCurrent(w http.ResponseWriter, r *http.Request) {
	out, err := s.sb.DumpProgram()
	if err != nil {
		writeJSON(w, http.StatusOK, presetCurrentResponse{CurrentPreset: "", Error: err.Error()})
		return
	}

	preset := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SetPreset ") {
			preset = strings.TrimSpace(strings.TrimPrefix(line, "SetPreset "))
			break
		}
	}

	writeJSON(w, http.StatusOK, presetCurrentResponse{CurrentPreset: preset})
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
