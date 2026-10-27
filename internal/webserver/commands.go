package webserver

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

const (
	commandTimeout       = 3 * time.Second
	maxCommandBytes      = 512
	commandQueueCapacity = 16
	commandTargetBytes   = 16
)

// Command contains only an opaque occurrence target and a bounded reply channel.
// The session reducer validates identity and owns all mutations and persistence.
type Command struct {
	Target string      `json:"target"`
	Retry  bool        `json:"retry"`
	Reply  chan string `json:"-"`
}

func (s *Server) Commands() <-chan Command { return s.commands }
func (s *Server) Done() <-chan struct{}    { return s.ctx.Done() }

func (s *Server) command(w http.ResponseWriter, r *http.Request, host string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Origin") != "http://"+host || r.Header.Get("X-CSRF-Token") != s.token {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return
	}
	var c Command
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCommandBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil || len(c.Target) != hex.EncodedLen(commandTargetBytes) || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid command", http.StatusBadRequest)
		return
	}
	if _, err := hex.DecodeString(c.Target); err != nil {
		http.Error(w, "Invalid target", http.StatusBadRequest)
		return
	}
	c.Reply = make(chan string, 1)
	select {
	case s.commands <- c:
	default:
		http.Error(w, "Session busy; refresh before retrying", http.StatusServiceUnavailable)
		return
	}
	timer := time.NewTimer(commandTimeout)
	defer timer.Stop()
	select {
	case result := <-c.Reply:
		if result != "applied" {
			http.Error(w, result, http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"applied"}`))
	case <-timer.C:
		http.Error(w, "Outcome unknown; refresh before retrying", http.StatusGatewayTimeout)
	case <-s.ctx.Done():
		http.Error(w, "Session ended", http.StatusServiceUnavailable)
	case <-r.Context().Done():
	}
}
