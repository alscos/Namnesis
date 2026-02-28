package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alscos/Namnesis/internal/config"
	"github.com/alscos/Namnesis/internal/httpserver"
	"github.com/alscos/Namnesis/internal/oled"
	"github.com/alscos/Namnesis/internal/stompbox"
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

	// --- lifecycle context ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- OLED bridge (optional) ---
	// Best: create a udev symlink /dev/ttyNAMNESIS_OLED for stable naming
	o := oled.NewOLEDSerial("/dev/ttyNAMNESIS_OLED", 115200)
	go o.Start(ctx, sb.DumpProgram, 400*time.Millisecond)

	// --- graceful shutdown on SIGINT/SIGTERM ---
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		log.Printf("shutdown requested; stopping...")

		// stop background workers
		cancel()

		// graceful HTTP shutdown
		ctxTO, cancelTO := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelTO()
		_ = srv.Shutdown(ctxTO)

		// close serial explicitly (optional; Start() also closes on ctx.Done())
		o.Close()
	}()

	log.Printf("namnesis-ui-gateway listening on %s (stompbox %s:%d)\n",
		cfg.ListenAddr, cfg.StompHost, cfg.StompPort)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
