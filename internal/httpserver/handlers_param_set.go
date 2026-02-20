package httpserver

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type paramSetRequest struct {
	Plugin string      `json:"plugin"`
	Param  string      `json:"param"`
	Value  interface{} `json:"value"`
}

func (s *Server) handleParamSet(w http.ResponseWriter, r *http.Request) {
	var req paramSetRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.Plugin = strings.TrimSpace(req.Plugin)
	req.Param = strings.TrimSpace(req.Param)

	if req.Plugin == "" || req.Param == "" {
		http.Error(w, "missing fields: plugin and param are required", http.StatusBadRequest)
		return
	}

	val, err := toStompboxValue(req.Value)
	if err != nil {
		http.Error(w, "invalid value: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Apply
	if err := s.sb.SetParam(req.Plugin, req.Param, val); err != nil {
		http.Error(w, "setparam error: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Return JSON ack
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"plugin": req.Plugin,
		"param":  req.Param,
		"value":  val, // final token sent to Stompbox
	})
}

// JSON numbers come as float64; we convert to a Stompbox token.
// - bool -> 0/1
// - number -> decimal (no scientific notation)
// - string -> quoted if it contains whitespace (Stompbox supports std::quoted)
func toStompboxValue(v interface{}) (string, error) {
	switch t := v.(type) {
	case bool:
		if t {
			return "1", nil
		}
		return "0", nil

	case json.Number:
		// Prefer integer if it parses cleanly
		if i64, err := t.Int64(); err == nil {
			return strconv.FormatInt(i64, 10), nil
		}
		f64, err := t.Float64()
		if err != nil {
			return "", fmt.Errorf("invalid number")
		}
		if math.IsNaN(f64) || math.IsInf(f64, 0) {
			return "", fmt.Errorf("invalid number (nan/inf)")
		}
		return strconv.FormatFloat(f64, 'f', -1, 64), nil

	case float64:
		// Fallback (in case some caller bypasses UseNumber)
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return "", fmt.Errorf("invalid number (nan/inf)")
		}
		return strconv.FormatFloat(t, 'f', -1, 64), nil

	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", fmt.Errorf("empty string")
		}
		// Allow numeric strings (optional: accept comma decimal)
		ns := strings.ReplaceAll(s, ",", ".")
		if f, err := strconv.ParseFloat(ns, 64); err == nil {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return "", fmt.Errorf("invalid number (nan/inf)")
			}
			return strconv.FormatFloat(f, 'f', -1, 64), nil
		}
		// Quote if it has spaces/tabs/etc. (safe for enum stems / IR names)
		if strings.IndexFunc(s, unicode.IsSpace) >= 0 {
			return strconv.Quote(s), nil
		}
		return s, nil

	default:
		return "", fmt.Errorf("unsupported type %T (use number, bool or string)", v)
	}
}
