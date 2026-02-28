package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alscos/Namnesis/internal/oled"
)

func (s *Server) handlePresetHuman(w http.ResponseWriter, r *http.Request) {
	raw, err := s.sb.DumpProgram()
	if err != nil {
		http.Error(w, "program error: "+err.Error(), http.StatusBadGateway)
		return
	}

	num, name, amp, fx := oled.HumanizeFromDumpProgram(raw)

	resp := struct {
		Loaded bool   `json:"loaded"`
		Num    string `json:"num"`
		Name   string `json:"name"`
		Amp    string `json:"amp"`
		FX     string `json:"fx"`
		TS     int64  `json:"ts"`
	}{
		Loaded: (num != "" || name != "" || amp != "" || fx != ""),
		Num:    num,
		Name:   name,
		Amp:    amp,
		FX:     fx,
		TS:     time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}
