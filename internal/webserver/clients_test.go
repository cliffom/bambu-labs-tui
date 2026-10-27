package webserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type clientTestConn struct {
	net.Conn
	address net.Addr
}

func (c *clientTestConn) RemoteAddr() net.Addr { return c.address }

func clientTestRequest(clients *clientConnections, port int, agent string) net.Conn {
	conn := &clientTestConn{address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}}
	clients.state(conn, http.StateNew)
	request := httptest.NewRequest("GET", "http://127.0.0.1/", nil).WithContext(clients.context(context.Background(), conn))
	request.Header.Set("User-Agent", agent)
	request.Header.Set("X-Forwarded-For", "203.0.113.99")
	clients.request(request)
	return conn
}

func TestClientLedgerGroupsBrowserSocketsAndIgnoresEmptySockets(t *testing.T) {
	var clients clientConnections
	empty := &clientTestConn{address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}}
	clients.state(empty, http.StateNew)
	if got := clients.snapshot(); got.Active != 0 || len(got.Events) != 0 {
		t.Fatal("unrequested socket logged", got)
	}
	agent := "Synthetic\x1b[31m browser\x1b[0m\r\n\t\u202e" + strings.Repeat("X", 500)
	first := clientTestRequest(&clients, 4445, agent)
	second := clientTestRequest(&clients, 4446, agent)
	connected := clients.snapshot()
	if connected.Active != 1 || len(connected.Events) != 1 {
		t.Fatal("browser socket noise", connected)
	}
	event := connected.Events[0]
	if event.IP != "127.0.0.1" || len(event.UserAgent) > clientUserAgentLimit || strings.ContainsAny(event.UserAgent, "\x1b\r\n\t\u202e") || !strings.HasPrefix(event.UserAgent, "Synthetic browser") {
		t.Fatal(event)
	}
	request := httptest.NewRequest("GET", "http://127.0.0.1/", nil).WithContext(clients.context(context.Background(), first))
	request.Header.Set("User-Agent", "replacement")
	clients.request(request)
	clients.state(first, http.StateActive)
	clients.state(first, http.StateIdle)
	clients.state(first, http.StateClosed)
	clients.state(empty, http.StateClosed)
	if got := clients.snapshot(); got.Active != 1 || len(got.Events) != 1 {
		t.Fatal("extra connection/disconnection", got)
	}
	clients.state(second, http.StateClosed)
	if got := clients.snapshot(); got.Active != 1 || len(got.Events) != 1 {
		t.Fatal("no disconnect grace", got)
	}
	clients.expire(time.Now().Add(clientDisconnectGrace))
	closed := clients.snapshot()
	if closed.Active != 0 || len(closed.Events) != 2 || closed.Events[1].Connected || closed.Events[1].Client != event.Client || closed.Events[1].UserAgent != event.UserAgent {
		t.Fatal(closed)
	}
	connected.Events[0].UserAgent = "mutated"
	if clients.snapshot().Events[0].UserAgent == "mutated" {
		t.Fatal("snapshot aliases live ledger")
	}
}

func TestClientLedgerReplacementGraceAndListenerStop(t *testing.T) {
	var clients clientConnections
	first := clientTestRequest(&clients, 4444, "Browser A")
	clients.state(first, http.StateClosed)
	second := clientTestRequest(&clients, 4445, "Browser A")
	clients.expire(time.Now().Add(clientDisconnectGrace))
	if got := clients.snapshot(); got.Active != 1 || len(got.Events) != 1 {
		t.Fatal("socket replacement created a new client", got)
	}
	other := clientTestRequest(&clients, 4446, "Browser B")
	missing := clientTestRequest(&clients, 4447, "")
	if got := clients.snapshot(); got.Active != 3 || got.Events[2].UserAgent != "Unknown (not supplied)" {
		t.Fatal(got)
	}
	clients.stop()
	clients.state(second, http.StateClosed)
	clients.state(other, http.StateClosed)
	clients.state(missing, http.StateClosed)
	clients.stop()
	if got := clients.snapshot(); got.Active != 0 || len(got.Events) != 6 {
		t.Fatal("shutdown duplicate disconnect", got)
	}
}

func TestClientLedgerBoundsAndConcurrentRequests(t *testing.T) {
	var clients clientConnections
	var group sync.WaitGroup
	for i := range clientTrackingLimit + 12 {
		conn := &clientTestConn{address: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: i + 1}}
		clients.state(conn, http.StateNew)
		if i >= clientTrackingLimit {
			clients.state(conn, http.StateClosed)
			continue
		}
		group.Go(func() {
			request := httptest.NewRequest("GET", "http://127.0.0.1/", nil).WithContext(clients.context(context.Background(), conn))
			request.Header.Set("User-Agent", fmt.Sprintf("Synthetic client %d", i))
			clients.request(request)
			_ = clients.snapshot()
		})
	}
	group.Wait()
	snapshot := clients.snapshot()
	if snapshot.Active != clientTrackingLimit || snapshot.Omitted != 12 || len(snapshot.Events) != clientHistoryLimit {
		t.Fatal(snapshot.Active, snapshot.Omitted, len(snapshot.Events))
	}
	for conn := range clients.connections {
		clients.state(conn, http.StateClosed)
	}
	clients.expire(time.Now().Add(clientDisconnectGrace))
	if got := clients.snapshot(); got.Active != 0 || len(got.Events) != clientHistoryLimit {
		t.Fatal(got.Active, len(got.Events))
	}
}

func TestClientLedgerRealHTTPAndShutdown(t *testing.T) {
	server := New(t.Context())
	defer server.Close()
	status := server.SetEnabled(true, availablePort(t))
	if status.Error != "" {
		t.Fatal(status)
	}
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	for range 2 {
		request, err := http.NewRequest(http.MethodGet, status.URL+"/api/snapshot", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("User-Agent", "Synthetic ledger integration")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "Synthetic ledger") {
			t.Fatal("client ledger leaked into browser snapshot")
		}
	}
	snapshot := server.Clients()
	if snapshot.Active != 1 || len(snapshot.Events) != 1 || snapshot.Events[0].UserAgent != "Synthetic ledger integration" {
		t.Fatalf("keepalive: %+v", snapshot)
	}
	server.SetEnabled(false, 0)
	snapshot = server.Clients()
	if snapshot.Active != 0 || len(snapshot.Events) != 2 || snapshot.Events[1].Connected || snapshot.Events[1].IP != "127.0.0.1" {
		t.Fatal(snapshot)
	}
}
