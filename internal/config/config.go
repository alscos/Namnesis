package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr     string
	StompHost      string
	StompPort      int
	DialTimeout    time.Duration
	ReadTimeout    time.Duration
	MaxBytes       int64
	EndMarker      string
	DumpCommand    string
	AllowedSubnets []string
}

func LoadFromEnv() Config {
	return Config{
		ListenAddr:  env("LISTEN_ADDR", "0.0.0.0:3000"),
		StompHost:   env("STOMPBOX_HOST", "127.0.0.1"),
		StompPort:   envInt("STOMPBOX_PORT", 0),
		DialTimeout: envDuration("DIAL_TIMEOUT", 1*time.Second),
		ReadTimeout: envDuration("READ_TIMEOUT", 5*time.Second),
		MaxBytes:    int64(envInt("MAX_BYTES", 2_000_000)),
		EndMarker:   env("END_MARKER", "EndConfig"),
		DumpCommand: env("DUMP_COMMAND", "Dump Config"),
		AllowedSubnets: splitCSV(env("ALLOWED_SUBNETS", "")),
	}
}

func env(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
