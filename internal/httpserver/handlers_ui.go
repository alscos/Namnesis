package httpserver

import (
	"net/http"
)

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
