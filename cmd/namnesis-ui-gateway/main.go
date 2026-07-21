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
	"github.com/alscos/Namnesis/internal/controlstate"
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

	state := controlstate.New(
		sb,
		cfg.ProgramPollInterval,
		cfg.ConfigRefreshInterval,
		cfg.PresetRefreshInterval,
	)

	// The HTTP API is cache-first. Populate the first snapshot before accepting traffic,
	// but keep the service available even when Stompbox is temporarily offline.
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 20*time.Second)
	if err := state.Bootstrap(bootstrapCtx); err != nil {
		log.Printf("state bootstrap completed with errors: %v", err)
	}
	bootstrapCancel()

	r, err := httpserver.NewRouter(httpserver.RouterDeps{
		Config: cfg,
		SB:     sb,
		State:  state,
	})
	if err != nil {
		log.Fatalf("router init: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go state.Run(ctx)

	// The optional OLED bridge reads the shared cached program snapshot.
	// It never opens a separate Stompbox polling connection.
	var o *oled.OLEDSerial
	if cfg.OLEDEnabled {
		o = oled.NewOLEDSerial(cfg.OLEDDevice, cfg.OLEDBaud)
		go o.Start(ctx, state.ProgramRaw, cfg.OLEDInterval)
		log.Printf(
			"oled bridge enabled on %s at %d baud (interval %s)",
			cfg.OLEDDevice,
			cfg.OLEDBaud,
			cfg.OLEDInterval,
		)
	} else {
		log.Printf("oled bridge disabled")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		log.Printf("shutdown requested; stopping...")
		cancel()

		ctxTO, cancelTO := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelTO()
		_ = srv.Shutdown(ctxTO)
		if o != nil {
			o.Close()
		}
	}()

	log.Printf(
		"namnesis-ui-gateway listening on %s (stompbox %s:%d, program poll %s)",
		cfg.ListenAddr,
		cfg.StompHost,
		cfg.StompPort,
		cfg.ProgramPollInterval,
	)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
