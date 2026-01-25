package httpserver

import (
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"namnesis-ui-gateway/internal/config"
	"namnesis-ui-gateway/internal/stompbox"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type RouterDeps struct {
	Config config.Config
	SB     *stompbox.Client
}

type Server struct {
	cfg config.Config
	sb  *stompbox.Client
	tpl *template.Template
}

func NewRouter(deps RouterDeps) (http.Handler, error) {
	s := &Server{
		cfg: deps.Config,
		sb:  deps.SB,
	}

	tplPath := filepath.Join("web", "templates", "*.html")
	tpl, err := template.ParseGlob(tplPath)
	if err != nil {
	return nil, err
	}
	s.tpl = tpl


	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(3 * time.Second))

	if len(s.cfg.AllowedSubnets) > 0 {
		allow, err := newCIDRAllowlist(s.cfg.AllowedSubnets)
		if err != nil {
			return nil, err
		}
		r.Use(allow.middleware)
	}

	fs := http.FileServer(http.Dir(filepath.Join("web", "static")))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dumpconfig", http.StatusFound)
	})

	// Raw API endpoints (plain text)
	r.Get("/api/dumpconfig", s.handleDumpConfigRaw)
	r.Get("/api/program", s.handleProgramRaw)
	r.Get("/api/debug/program-parsed", s.handleProgramParsedDebug)
	r.Get("/api/presets", s.handlePresetsRaw)
	r.Get("/api/state", s.handleState)
	r.Get("/ui", s.handleUIPage)
	r.Post("/api/preset/load", s.handlePresetLoad)


	// HTML page
	r.Get("/dumpconfig", s.handleDumpConfigPage)

	return r, nil
}
