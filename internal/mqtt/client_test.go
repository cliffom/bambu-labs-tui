package mqtt

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

func TestBrokerMonitoringExchange(t *testing.T) {
	local, broker := net.Pipe()
	defer local.Close()
	defer broker.Close()
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
	defer local.Close()
	go func() { defer broker.Close(); _, _, _ = readPacket(broker); _, _ = broker.Write([]byte{0x20, 2, 0, 5}) }()
	_, err := handshake(local, "fixture", "user", "password")
	if !errors.Is(err, ErrAuth) {
		t.Fatal(err)
	}
}
func TestRejectedSubscription(t *testing.T) {
	local, broker := net.Pipe()
	defer local.Close()
	go func() { defer broker.Close(); _, _ = broker.Write([]byte{0x90, 3, 0, 1, 0x80}) }()
	c := &Client{conn: local}
	if _, _, err := c.Read(); err == nil || !strings.Contains(err.Error(), "subscription") {
		t.Fatal(err)
	}
}
func TestTopicIsolation(t *testing.T) {
	local, broker := net.Pipe()
	defer local.Close()
	c := &Client{conn: local}
	for _, id := range []string{"", "#", "TEST/request", "+", "test\x00"} {
		if c.Subscribe([]string{id}) == nil || c.Refresh([]string{id}) == nil {
			t.Fatalf("accepted topic injection %q", id)
		}
	}
	broker.Close()
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
