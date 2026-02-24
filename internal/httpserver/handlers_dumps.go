package httpserver

import (
	"encoding/json"
	"namnesis-ui-gateway/internal/stompbox"
	"net/http"
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
