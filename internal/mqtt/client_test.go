package mqtt

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBrokerMonitoringExchange(t *testing.T) {
	local, broker := net.Pipe()
	defer func() { _ = local.Close() }()
	defer func() { _ = broker.Close() }()
	finished := make(chan error, 1)
	go func() {
		kind, body, err := readPacket(broker)
		if err != nil {
			finished <- err
			return
		}
		if kind != 0x10 || !bytes.Contains(body, field("fixture-user")) || !bytes.Contains(body, field("fixture-token")) {
			finished <- errors.New("invalid CONNECT")
			return
		}
		if _, err = broker.Write([]byte{0x20, 2, 0, 0}); err != nil {
			finished <- err
			return
		}
		kind, body, err = readPacket(broker)
		if err != nil {
			finished <- err
			return
		}
		if kind != 0x82 || !bytes.Contains(body, []byte("device/TEST/report")) {
			finished <- errors.New("invalid subscription")
			return
		}
		packetID := append([]byte(nil), body[:2]...)
		kind, body, err = readPacket(broker)
		if err != nil {
			finished <- err
			return
		}
		if kind != 0x30 || !bytes.Contains(body, []byte("device/TEST/request")) || !bytes.Contains(body, []byte(`"command":"pushall"`)) {
			finished <- errors.New("invalid monitoring request")
			return
		}
		kind, body, err = readPacket(broker)
		if err != nil {
			finished <- err
			return
		}
		if kind != 0x30 || !bytes.Equal(body, append(field("device/TEST/request"), []byte(`{"info":{"sequence_id":"0","command":"get_version"}}`)...)) {
			finished <- errors.New("invalid read-only firmware query")
			return
		}
		if _, err = broker.Write(append([]byte{0x90, 3}, append(packetID, 0)...)); err != nil {
			finished <- err
			return
		}
		payload := append(field("device/TEST/report"), 0, 7)
		payload = append(payload, []byte(`{"print":{"mc_percent":42}}`)...)
		if _, err = broker.Write(append([]byte{0x32, byte(len(payload))}, payload...)); err != nil {
			finished <- err
			return
		}
		kind, body, err = readPacket(broker)
		if err != nil {
			finished <- err
			return
		}
		if kind != 0x40 || !bytes.Equal(body, []byte{0, 7}) {
			finished <- errors.New("missing QoS 1 acknowledgement")
			return
		}
		finished <- nil
	}()
	c, err := handshake(local, "fixture-client", "fixture-user", "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Subscribe([]string{"TEST"}); err != nil {
		t.Fatal(err)
	}
	if err = c.Refresh([]string{"TEST"}); err != nil {
		t.Fatal(err)
	}
	id, payload, err := c.Read()
	if err != nil || id != "TEST" || !bytes.Contains(payload, []byte(`42`)) {
		t.Fatalf("report: %s %s %v", id, payload, err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestAuthRejection(t *testing.T) {
	local, broker := net.Pipe()
	defer func() { _ = local.Close() }()
	go func() {
		defer func() { _ = broker.Close() }()
		_, _, _ = readPacket(broker)
		_, _ = broker.Write([]byte{0x20, 2, 0, 5})
	}()
	_, err := handshake(local, "fixture", "user", "password")
	if !errors.Is(err, ErrAuth) {
		t.Fatal(err)
	}
}

func TestRejectedSubscription(t *testing.T) {
	local, broker := net.Pipe()
	defer func() { _ = local.Close() }()
	go func() { defer func() { _ = broker.Close() }(); _, _ = broker.Write([]byte{0x90, 3, 0, 1, 0x80}) }()
	c := &Client{conn: local}
	if _, _, err := c.Read(); err == nil || !strings.Contains(err.Error(), "subscription") {
		t.Fatal(err)
	}
}

func TestTopicIsolation(t *testing.T) {
	local, broker := net.Pipe()
	defer func() { _ = local.Close() }()
	c := &Client{conn: local}
	for _, id := range []string{"", "#", "TEST/request", "+", "test\x00"} {
		if c.Subscribe([]string{id}) == nil || c.Refresh([]string{id}) == nil {
			t.Fatalf("accepted topic injection %q", id)
		}
	}
	_ = broker.Close()
}

func TestPacketBoundsAndTruncation(t *testing.T) {
	for _, b := range [][]byte{{}, {0x30}, {0x30, 0xff, 0xff, 0xff, 0x7f}, {0x30, 0x80, 0x80, 0x80, 0x80}, {0x30, 5, 1}} {
		if _, _, err := readPacket(bytes.NewReader(b)); err == nil {
			t.Fatalf("accepted malformed packet %x", b)
		}
	}
	kind, b, err := readPacket(bytes.NewReader([]byte{0xd0, 0}))
	if err != nil || kind != 0xd0 || len(b) != 0 {
		t.Fatal(kind, b, err)
	}
}

func FuzzReadPacket(f *testing.F) {
	f.Add([]byte{0xd0, 0})
	f.Add([]byte{0x30, 1, 1})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		_, _, err := readPacket(bytes.NewReader(b))
		_ = errors.Is(err, io.EOF)
	})
}

// fragmentConn forces writeLocked's partial-write loop to run so concurrent
// keepalives, requests, subscriptions, and QoS acknowledgements can interleave
// only between complete packets.
type fragmentConn struct{ net.Conn }

func (c fragmentConn) Write(body []byte) (int, error) {
	if len(body) > 3 {
		body = body[:3]
	}
	return c.Conn.Write(body)
}

func TestConcurrentRequestsAndAcknowledgementsPreservePacketBoundaries(t *testing.T) {
	local, broker := net.Pipe()
	defer func() { _ = local.Close(); _ = broker.Close() }()
	c := &Client{conn: fragmentConn{local}}
	brokerDone := make(chan error, 1)
	go func() {
		counts := map[byte]int{}
		for range 5 {
			kind, body, err := readPacket(broker)
			if err != nil {
				brokerDone <- err
				return
			}
			counts[kind]++
			switch kind {
			case 0x82:
				if !bytes.Contains(body, field("device/TEST/report")) {
					brokerDone <- errors.New("corrupt subscription")
					return
				}
			case 0x30:
				if !bytes.HasPrefix(body, field("device/TEST/request")) {
					brokerDone <- errors.New("corrupt refresh")
					return
				}
			case 0xc0:
				if len(body) != 0 {
					brokerDone <- errors.New("corrupt keepalive")
					return
				}
			case 0x40:
				if !bytes.Equal(body, []byte{0, 7}) {
					brokerDone <- errors.New("corrupt acknowledgement")
					return
				}
			default:
				brokerDone <- errors.New("interleaved packet")
				return
			}
		}
		if counts[0x82] != 1 || counts[0x30] != 2 || counts[0xc0] != 1 || counts[0x40] != 1 {
			brokerDone <- errors.New("missing packet")
			return
		}
		brokerDone <- nil
	}()
	// Sending and receiving are full duplex, as on a real broker connection.
	sent := make(chan error, 1)
	go func() {
		body := append(field("device/TEST/report"), 0, 7)
		body = append(body, []byte(`{"print":{"mc_percent":42}}`)...)
		_, err := broker.Write(append([]byte{0x32, byte(len(body))}, body...))
		sent <- err
	}()
	writes := make(chan error, 4)
	go func() { writes <- c.Subscribe([]string{"TEST"}) }()
	go func() { writes <- c.Refresh([]string{"TEST"}) }()
	go func() { writes <- c.Ping() }()
	go func() {
		id, body, err := c.Read()
		if err == nil && (id != "TEST" || !bytes.Contains(body, []byte("42"))) {
			err = errors.New("corrupt report")
		}
		writes <- err
	}()
	for range 4 {
		select {
		case err := <-writes:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent packet operation blocked")
		}
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if err := <-brokerDone; err != nil {
		t.Fatal(err)
	}
}
