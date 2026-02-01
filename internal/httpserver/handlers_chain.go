package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type chainSetRequest struct {
	Plugins []string `json:"plugins"`
}

// POST /api/chains/{chain}/set
// Body: {"plugins":["NoiseGate_2","Delay",...]}
func (s *Server) handleChainSet(w http.ResponseWriter, r *http.Request) {
	chain := strings.TrimSpace(chi.URLParam(r, "chain"))
	if chain == "" {
		http.Error(w, "missing chain", http.StatusBadRequest)
		return
	}

	var req chainSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	clean := make([]string, 0, len(req.Plugins))
	for _, p := range req.Plugins {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		clean = append(clean, t)
	}

	if err := s.sb.SetChain(chain, clean); err != nil {
		http.Error(w, "setchain error: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"chain":   chain,
		"plugins": clean,
	})
}

// POST /api/plugins/{plugin}/release
func (s *Server) handlePluginRelease(w http.ResponseWriter, r *http.Request) {
	plugin := strings.TrimSpace(chi.URLParam(r, "plugin"))
	if plugin == "" {
		http.Error(w, "missing plugin", http.StatusBadRequest)
		return
	}

	if err := s.sb.ReleasePlugin(plugin); err != nil {
		http.Error(w, "releaseplugin error: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"plugin": plugin,
	})
}
