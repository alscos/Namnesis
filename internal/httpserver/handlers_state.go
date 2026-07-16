package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type stateRefreshRequest struct {
	Scope string `json:"scope"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if s.state == nil {
		http.Error(w, "state manager unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.state.Snapshot())
}

func (s *Server) handleStateRefresh(w http.ResponseWriter, r *http.Request) {
	if s.state == nil {
		http.Error(w, "state manager unavailable", http.StatusServiceUnavailable)
		return
	}

	var req stateRefreshRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "all"
	}

	var err error
	switch scope {
	case "program":
		err = s.state.RefreshProgramNow("manual")
	case "presets":
		err = s.state.RefreshPresetsNow("manual")
	case "config":
		err = s.state.RefreshConfigNow("manual")
	case "all":
		err = s.state.RefreshAllNow("manual")
	default:
		http.Error(w, "invalid scope; use program, presets, config or all", http.StatusBadRequest)
		return
	}

	status := http.StatusOK
	if err != nil {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, s.state.Snapshot())
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.state == nil {
		http.Error(w, "state manager unavailable", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := writeSSE(w, "snapshot", s.state.Snapshot()); err != nil {
		return
	}
	flusher.Flush()

	events, cancel := s.state.Subscribe()
	defer cancel()

	heartbeatEvery := s.cfg.SSEHeartbeatInterval
	if heartbeatEvery <= 0 {
		heartbeatEvery = 15 * time.Second
	}
	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := writeSSE(w, event.Type, event); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, eventName string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}
