package httpserver

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/alscos/Namnesis/internal/config"
	"github.com/alscos/Namnesis/internal/controlstate"
	"github.com/alscos/Namnesis/internal/stompbox"
	"github.com/alscos/Namnesis/internal/sysinfo"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type RouterDeps struct {
	Config config.Config
	SB     *stompbox.Client
	State  *controlstate.Manager
}

type Server struct {
	cfg   config.Config
	sb    *stompbox.Client
	tpl   *template.Template
	sys   *sysinfo.Collector
	state *controlstate.Manager
}

func NewRouter(deps RouterDeps) (http.Handler, error) {
	s := &Server{
		cfg:   deps.Config,
		sb:    deps.SB,
		sys:   sysinfo.NewCollector(),
		state: deps.State,
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

	if len(s.cfg.AllowedSubnets) > 0 {
		allow, err := newCIDRAllowlist(s.cfg.AllowedSubnets)
		if err != nil {
			return nil, err
		}
		r.Use(allow.middleware)
	}

	fs := http.FileServer(http.Dir(filepath.Join("web", "static")))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("web", "static", "img", "logo_s.png"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui", http.StatusFound)
	})

	// Raw API endpoints (plain text)
	r.Get("/api/dumpconfig", s.handleDumpConfigRaw)
	r.Get("/api/program", s.handleProgramRaw)
	r.Get("/api/debug/program-parsed", s.handleProgramParsedDebug)
	r.Get("/api/presets", s.handlePresetsRaw)
	r.Get("/api/state", s.handleState)
	r.Get("/api/events", s.handleEvents)
	r.Post("/api/state/refresh", s.handleStateRefresh)
	r.Get("/api/system", s.handleSystem)
	r.Get("/ui", s.handleUIPage)
	r.Get("/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.tpl.ExecuteTemplate(w, "live.html", nil); err != nil {
			http.Error(w, "render live UI", http.StatusInternalServerError)
		}
	})
	r.Get("/api/preset/current", s.handlePresetCurrent)
	r.Get("/api/preset/human", s.handlePresetHuman)
	r.Post("/api/preset/load", s.handlePresetLoad)
	r.Post("/api/preset/save-as", s.handlePresetSaveAs)
	r.Post("/api/preset/delete", s.handlePresetDelete)
	r.Get("/api/debug/config-parsed", s.handleConfigParsedDebug)
	r.Post("/api/param/file", s.handleSetFileParam)
	r.Post("/api/preset/save", s.handlePresetSave)
	r.Post("/api/plugins/{plugin}/enabled", s.handlePluginEnabled)
	r.Post("/api/param/set", s.handleParamSet)
	r.Post("/api/chains/{chain}/set", s.handleChainSet)
	r.Post("/api/plugins/{plugin}/release", s.handlePluginRelease)

	// HTML page
	r.Get("/dumpconfig", s.handleDumpConfigPage)

	return r, nil
}
