package httpserver

import (
	"encoding/json"
	"fmt"
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		"value":  req.Value,
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

	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil

	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", fmt.Errorf("empty string")
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
