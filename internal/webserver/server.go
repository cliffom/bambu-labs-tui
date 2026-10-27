// Package webserver serves allowlisted snapshots and local notice decisions on IPv4 loopback.
// It never owns farm state, cloud credentials, or a monitoring connection.
package webserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed assets/*
var assets embed.FS

type (
	Status struct{ URL, Error string }
	Server struct {
		commands chan Command
		token    string
		ctx      context.Context
		mu       sync.Mutex
		server   *http.Server
		listener net.Listener
		done     <-chan struct{}
		snapshot atomic.Value
		status   atomic.Value
		closed   bool
		port     int
	}
)

func New(ctx context.Context) *Server {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	s := &Server{ctx: ctx, commands: make(chan Command, commandQueueCapacity), token: hex.EncodeToString(token[:])}
	s.snapshot.Store([]byte(`{"printers":[],"connected":false,"summary":{},"demo":false}`))
	s.status.Store(Status{})
	context.AfterFunc(ctx, s.Close)
	return s
}

func (s *Server) Status() Status { return s.status.Load().(Status) }

// Publish takes its own copy; HTTP handlers read only immutable encoded bytes.
func (s *Server) Publish(data []byte) { s.snapshot.Store(bytes.Clone(data)) }

// SetEnabled is called by one serialized host command. Network lifecycle work is
// outside the reducer; context cancellation prevents a late command reopening it.
func (s *Server) SetEnabled(enabled bool, port int) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !enabled {
		s.closeLocked()
		return s.Status()
	}
	if s.closed || s.ctx.Err() != nil {
		return Status{Error: "Web server session has ended"}
	}
	if s.server != nil && s.port == port {
		return s.Status()
	}
	s.closeLocked()
	if port < 1 || port > 65535 {
		status := Status{Error: "Web server port must be between 1 and 65535"}
		s.status.Store(status)
		return status
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		message := fmt.Sprintf("Could not start web server on port %d: %v", port, err)
		if errors.Is(err, syscall.EADDRINUSE) {
			message = fmt.Sprintf("Port %d is already in use; web server not started", port)
		}
		status := Status{Error: message}
		s.status.Store(status)
		return status
	}
	host := listener.Addr().String()
	sub, _ := fs.Sub(assets, "assets")
	files := http.FileServer(http.FS(sub))
	handler := s.handler(host, files)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	s.server = server
	s.port = port
	done := make(chan struct{})
	s.listener, s.done = listener, done
	s.status.Store(Status{URL: "http://" + host})
	go func() {
		err := server.Serve(listener)
		close(done)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.mu.Lock()
			if s.server == server {
				s.closeLocked()
				s.status.Store(Status{Error: "Localhost web server stopped unexpectedly"})
			}
			s.mu.Unlock()
		}
	}()
	return s.Status()
}

func (s *Server) handler(host string, files http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		// Exact Host and Origin checks prevent DNS rebinding and cross-origin reads.
		if r.Host != host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/api/dismiss" {
			s.command(w, r, host)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Read-only endpoint", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/api/session":
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]string{"token": s.token})
			}
		case "/api/snapshot":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == http.MethodGet {
				_, _ = w.Write(s.snapshot.Load().([]byte))
			}
		case "/", "/app.js", "/style.css", "/icon.png":
			files.ServeHTTP(w, r)
		default:
			if isPrinterImage(r.URL.Path) {
				// These immutable, content-versioned illustrations contain no telemetry.
				w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
				files.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
		}
	})
}

func (s *Server) closeLocked() {
	if s.server != nil {
		_ = s.listener.Close()
		_ = s.server.Close()
		<-s.done
		s.listener, s.done = nil, nil
		s.server = nil
		s.port = 0
	}
	s.status.Store(Status{})
}

// Close immediately stops listening and closes active browser connections.
// It is safe alongside cancellation, late commands, and repeated cleanup.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.closeLocked()
}
