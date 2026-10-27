package webserver

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
)

const (
	clientHistoryLimit        = 32
	clientTrackingLimit       = 128
	clientUserAgentLimit      = 160
	clientUserAgentInputLimit = 512
	clientDisconnectGrace     = 5 * time.Second
)

type (
	// ClientEvent groups observed HTTP requests by socket IP and first-request
	// user agent. It is not an authenticated identity or an individual browser tab.
	ClientEvent struct {
		Sequence, Client uint64
		At               time.Time
		Connected        bool
		IP, UserAgent    string
	}
	// ClientStatus is an independent, session-only terminal display snapshot.
	// It is never included in browser telemetry or persisted to disk.
	ClientStatus struct {
		Active  int
		Omitted uint64
		Events  []ClientEvent
	}
	clientContextKey struct{}
	clientKey        struct{ ip, agent string }
	clientSocket     struct {
		ip                string
		key               clientKey
		observed, grouped bool
	}
	clientGroup struct {
		event   ClientEvent
		sockets int
		expires time.Time
	}
	clientConnections struct {
		mu                            sync.Mutex
		omitted, sequence, nextClient uint64
		connections                   map[net.Conn]clientSocket
		groups                        map[clientKey]*clientGroup
		events                        []ClientEvent
	}
)

func (c *clientConnections) context(ctx context.Context, conn net.Conn) context.Context {
	return context.WithValue(ctx, clientContextKey{}, conn)
}

func (c *clientConnections) state(conn net.Conn, state http.ConnState) {
	if state != http.StateNew && state != http.StateClosed && state != http.StateHijacked {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if state == http.StateNew {
		if len(c.connections) >= clientTrackingLimit {
			c.omitted++
			return
		}
		if c.connections == nil {
			c.connections = make(map[net.Conn]clientSocket)
		}
		ip, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil || net.ParseIP(ip) == nil {
			ip = "Unknown"
		}
		c.connections[conn] = clientSocket{ip: ip}
		return
	}
	socket, ok := c.connections[conn]
	if !ok {
		return
	}
	delete(c.connections, conn)
	if socket.grouped {
		if group := c.groups[socket.key]; group != nil {
			group.sockets--
			if group.sockets == 0 {
				group.expires = time.Now().Add(clientDisconnectGrace)
			}
		}
	}
}

func (c *clientConnections) append(event ClientEvent) {
	c.sequence++
	event.Sequence = c.sequence
	c.events = append(c.events, event)
	if len(c.events) > clientHistoryLimit {
		copy(c.events, c.events[len(c.events)-clientHistoryLimit:])
		c.events = c.events[:clientHistoryLimit]
	}
}

func clientAgent(value string) string {
	if len(value) > clientUserAgentInputLimit {
		value = value[:clientUserAgentInputLimit]
	}
	value = ansi.Strip(value)
	value = strings.Map(func(r rune) rune {
		if r < ' ' || r > '~' {
			return -1
		}
		return r
	}, value)
	if len(value) > clientUserAgentLimit {
		value = value[:clientUserAgentLimit-3] + "..."
	}
	if value == "" {
		value = "Unknown (not supplied)"
	}
	return value
}

// Capture only the first request's UA, including requests rejected by the
// browser boundary. Forwarded addresses, URLs, cookies and other headers are
// deliberately excluded. Sockets without a request never create ledger rows.
func (c *clientConnections) request(r *http.Request) {
	conn, ok := r.Context().Value(clientContextKey{}).(net.Conn)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	socket, ok := c.connections[conn]
	if !ok || socket.observed {
		return
	}
	now := time.Now()
	c.expire(now)
	socket.observed = true
	socket.key = clientKey{ip: socket.ip, agent: clientAgent(r.UserAgent())}
	group := c.groups[socket.key]
	if group == nil {
		if len(c.groups) >= clientTrackingLimit {
			c.omitted++
			c.connections[conn] = socket
			return
		}
		if c.groups == nil {
			c.groups = make(map[clientKey]*clientGroup)
		}
		c.nextClient++
		group = &clientGroup{event: ClientEvent{Client: c.nextClient, At: now, Connected: true, IP: socket.ip, UserAgent: socket.key.agent}}
		c.groups[socket.key] = group
		c.append(group.event)
	}
	group.sockets++
	group.expires = time.Time{}
	socket.grouped = true
	c.connections[conn] = socket
}

func (c *clientConnections) expire(now time.Time) {
	for key, group := range c.groups {
		if group.sockets == 0 && !group.expires.IsZero() && !now.Before(group.expires) {
			event := group.event
			event.At, event.Connected = group.expires, false
			c.append(event)
			delete(c.groups, key)
		}
	}
}

// Listener shutdown ends all client lifetimes immediately. Later ConnState
// callbacks see no sockets and cannot duplicate a disconnect or affect a restart.
func (c *clientConnections) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, group := range c.groups {
		event := group.event
		event.At, event.Connected = time.Now(), false
		c.append(event)
	}
	clear(c.groups)
	clear(c.connections)
}

func (c *clientConnections) snapshot() ClientStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expire(time.Now())
	return ClientStatus{Active: len(c.groups), Omitted: c.omitted, Events: append([]ClientEvent(nil), c.events...)}
}

func (s *Server) Clients() ClientStatus { return s.clients.snapshot() }
