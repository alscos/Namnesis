package httpserver

import (
	"net/http"

	"github.com/alscos/Namnesis/internal/stompbox"
)

func (s *Server) handleDumpConfigRaw(w http.ResponseWriter, r *http.Request) {
	var (
		out string
		err error
	)
	if r.URL.Query().Get("fresh") == "1" || s.state == nil {
		out, err = s.sb.DumpConfig()
	} else {
		out, err = s.state.ConfigRaw()
	}
	if err != nil {
		http.Error(w, "dumpconfig error: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (s *Server) handleProgramRaw(w http.ResponseWriter, r *http.Request) {
	var (
		out string
		err error
	)
	if r.URL.Query().Get("fresh") == "1" || s.state == nil {
		out, err = s.sb.DumpProgram()
	} else {
		out, err = s.state.ProgramRaw()
	}
	if err != nil {
		http.Error(w, "program error: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (s *Server) handleConfigParsedDebug(w http.ResponseWriter, r *http.Request) {
	raw, err := s.cachedConfigRaw(r)
	if err != nil {
		http.Error(w, "dumpconfig error: "+err.Error(), http.StatusBadGateway)
		return
	}

	parsed, err := stompbox.ParseDumpConfig(raw)
	if err != nil {
		http.Error(w, "parse dumpconfig error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, parsed)
}

func (s *Server) handleProgramParsedDebug(w http.ResponseWriter, r *http.Request) {
	raw, err := s.cachedProgramRaw(r)
	if err != nil {
		http.Error(w, "program error: "+err.Error(), http.StatusBadGateway)
		return
	}

	parsed, err := stompbox.ParseDumpProgram(raw)
	if err != nil {
		http.Error(w, "parse program error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, parsed)
}

func (s *Server) cachedConfigRaw(r *http.Request) (string, error) {
	if r.URL.Query().Get("fresh") == "1" || s.state == nil {
		return s.sb.DumpConfig()
	}
	return s.state.ConfigRaw()
}

func (s *Server) cachedProgramRaw(r *http.Request) (string, error) {
	if r.URL.Query().Get("fresh") == "1" || s.state == nil {
		return s.sb.DumpProgram()
	}
	return s.state.ProgramRaw()
}
