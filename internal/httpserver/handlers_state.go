package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type stateResponse struct {
	Meta struct {
		Now string `json:"now"`
	} `json:"meta"`

	DumpConfig struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"dumpConfig"`

	Program struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"program"`

	Presets struct {
		Raw      string `json:"raw,omitempty"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	} `json:"presets"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	var resp stateResponse
	resp.Meta.Now = time.Now().Format(time.RFC3339)

	// Dump Config
	t0 := time.Now()
	out, err := s.sb.DumpConfig()
	resp.DumpConfig.Duration = time.Since(t0).String()
	if err != nil {
		resp.DumpConfig.Error = err.Error()
	} else {
		resp.DumpConfig.Raw = out
	}

	// Dump Program
	t1 := time.Now()
	out, err = s.sb.DumpProgram()
	resp.Program.Duration = time.Since(t1).String()
	if err != nil {
		resp.Program.Error = err.Error()
	} else {
		resp.Program.Raw = out
	}

	// List Presets
	t2 := time.Now()
	out, err = s.sb.ListPresets()
	resp.Presets.Duration = time.Since(t2).String()
	if err != nil {
		resp.Presets.Error = err.Error()
	} else {
		resp.Presets.Raw = out
	}

	// Decide HTTP status
	status := http.StatusOK
	allFailed := resp.DumpConfig.Error != "" &&
		resp.Program.Error != "" &&
		resp.Presets.Error != ""

	if allFailed {
		status = http.StatusBadGateway
	}

	// Write headers + status ONCE
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// Encode response body
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)

}
