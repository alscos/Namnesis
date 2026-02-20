package httpserver

import (
	"encoding/json"
	"net/http"
)

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
