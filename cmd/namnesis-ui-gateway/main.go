package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"namnesis-ui-gateway/internal/config"
	"namnesis-ui-gateway/internal/httpserver"
	"namnesis-ui-gateway/internal/stompbox"
)

func main() {
	cfg := config.LoadFromEnv()

	if cfg.StompPort == 0 {
		log.Fatalf("STOMPBOX_PORT is not set (or 0). Fix /etc/namnesis-ui-gateway.env")
	}

	sb := stompbox.New(fmt.Sprintf("%s:%d", cfg.StompHost, cfg.StompPort))
	sb.DialTimeout = cfg.DialTimeout
	sb.ReadTimeout = cfg.ReadTimeout
	sb.MaxBytes = int(cfg.MaxBytes)

	r, err := httpserver.NewRouter(httpserver.RouterDeps{
		Config: cfg,
		SB:     sb,
	})
	if err != nil {
		log.Fatalf("router init: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
	}

	log.Printf("namnesis-ui-gateway listening on %s (stompbox %s:%d)\n",
		cfg.ListenAddr, cfg.StompHost, cfg.StompPort)

	log.Fatal(srv.ListenAndServe())
}
