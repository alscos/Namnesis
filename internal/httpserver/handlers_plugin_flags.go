package httpserver

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/alscos/Namnesis/internal/stompbox"
)

type pluginEnabledReq struct {
	Enabled bool `json:"enabled"`
}
type setFileParamRequest struct {
	Plugin string `json:"plugin"`
	Param  string `json:"param"`
	Value  string `json:"value"`
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
