package webserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDismissRequestProtectionAndValidation(t *testing.T) {
	s := New(t.Context())
	t.Cleanup(s.Close)
	host := "127.0.0.1:8080"
	handler := s.handler(host, http.NotFoundHandler())
	valid := `{"target":"0123456789abcdef0123456789abcdef","retry":false}`
	for _, test := range []struct {
		name, method, host, origin, token, content, body string
		code                                             int
	}{
		{"method", "GET", host, "http://" + host, s.token, "application/json", valid, 405},
		{"host", "POST", "localhost:8080", "http://" + host, s.token, "application/json", valid, 403},
		{"origin missing", "POST", host, "", s.token, "application/json", valid, 403},
		{"foreign origin", "POST", host, "https://evil.example", s.token, "application/json", valid, 403},
		{"token missing", "POST", host, "http://" + host, "", "application/json", valid, 403},
		{"token wrong", "POST", host, "http://" + host, "wrong", "application/json", valid, 403},
		{"form", "POST", host, "http://" + host, s.token, "text/plain", valid, 415},
		{"unknown field", "POST", host, "http://" + host, s.token, "application/json", `{"target":"0123456789abcdef0123456789abcdef","control":true}`, 400},
		{"oversize", "POST", host, "http://" + host, s.token, "application/json", strings.Repeat(" ", 513) + valid, 400},
		{"trailing", "POST", host, "http://" + host, s.token, "application/json", valid + `{}`, 400},
		{"nonhex id", "POST", host, "http://" + host, s.token, "application/json", `{"target":"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}`, 400},
		{"invalid id", "POST", host, "http://" + host, s.token, "application/json", `{"target":"bad"}`, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, "http://"+host+"/api/dismiss", strings.NewReader(test.body))
			req.Host = test.host
			req.Header.Set("Origin", test.origin)
			req.Header.Set("X-CSRF-Token", test.token)
			req.Header.Set("Content-Type", test.content)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != test.code {
				t.Fatalf("got %d want %d", w.Code, test.code)
			}
			if len(s.commands) != 0 {
				t.Fatal("invalid request reached reducer")
			}
		})
	}
}

func TestDismissRouteWaitsForReducerAndBoundsQueue(t *testing.T) {
	s := New(t.Context())
	t.Cleanup(s.Close)
	host := "127.0.0.1:8080"
	handler := s.handler(host, http.NotFoundHandler())
	request := func() *http.Request {
		r := httptest.NewRequest("POST", "http://"+host+"/api/dismiss", strings.NewReader(`{"target":"0123456789abcdef0123456789abcdef"}`))
		r.Header.Set("Origin", "http://"+host)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", s.token)
		return r
	}
	for _, result := range []string{"applied", "stale"} {
		done := make(chan struct{})
		w := httptest.NewRecorder()
		go func() { handler.ServeHTTP(w, request()); close(done) }()
		c := <-s.Commands()
		select {
		case <-done:
			t.Fatal("reported success before reducer")
		default:
		}
		c.Reply <- result
		<-done
		want := 200
		if result != "applied" {
			want = 409
		}
		if w.Code != want {
			t.Fatal(w.Code)
		}
	}
	for range cap(s.commands) {
		s.commands <- Command{}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, request())
	if w.Code != 503 {
		t.Fatal("unbounded queue", w.Code)
	}
}

func TestShutdownUnblocksPendingBrowserReply(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := New(ctx)
	t.Cleanup(s.Close)
	host := "127.0.0.1:8080"
	req := httptest.NewRequest("POST", "http://"+host+"/api/dismiss", strings.NewReader(`{"target":"0123456789abcdef0123456789abcdef"}`))
	req.Header.Set("Origin", "http://"+host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", s.token)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { s.handler(host, http.NotFoundHandler()).ServeHTTP(w, req); close(done) }()
	c := <-s.Commands()
	cancel()
	<-done
	if w.Code != 503 {
		t.Fatal("pending command survived shutdown", w.Code)
	}
	select {
	case c.Reply <- "Session ended":
	default:
		t.Fatal("late reducer reply blocked")
	}
}
