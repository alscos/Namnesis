package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

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

