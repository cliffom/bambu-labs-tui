// Package mqtt implements only the MQTT 3.1.1 features needed for monitoring:
// clean sessions, QoS 0 subscriptions, inbound QoS 0/1 and status-only requests.
package mqtt

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

var ErrAuth = errors.New("MQTT account credentials rejected")

type Client struct {
	conn net.Conn
	mu   sync.Mutex
	next uint16
}

func Dial(ctx context.Context, address, username, password string) (*Client, error) {
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, errors.New("cannot connect to Bambu MQTT")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		_ = conn.Close()
		return nil, err
	}
	c, err := handshake(conn, "bambu-tui-"+hex.EncodeToString(id), username, password)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}
func handshake(conn net.Conn, id, user, password string) (*Client, error) {
	for _, s := range []string{id, user, password} {
		if len(s) > 65535 {
			return nil, errors.New("MQTT credential exceeds protocol limit")
		}
	}
	c := &Client{conn: conn}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = conn.SetDeadline(time.Time{}) }()
	body := append(field("MQTT"), 4, 0xc2, 0, 60)
	body = append(body, field(id)...)
	body = append(body, field(user)...)
	body = append(body, field(password)...)
	if err := c.write(0x10, body); err != nil {
		return nil, err
	}
	kind, data, err := readPacket(conn)
	if err != nil {
		return nil, err
	}
	if kind != 0x20 || len(data) != 2 {
		return nil, errors.New("invalid MQTT connection response")
	}
	if data[1] == 4 || data[1] == 5 {
		return nil, ErrAuth
	}
	if data[1] != 0 {
		return nil, fmt.Errorf("MQTT connection rejected (%d)", data[1])
	}
	return c, nil
}

func validSerial(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
func (c *Client) Subscribe(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	if c.next == 0 {
		c.next++
	}
	body := []byte{byte(c.next >> 8), byte(c.next)}
	for _, id := range ids {
		if !validSerial(id) {
			return errors.New("invalid printer identity")
		}
		body = append(body, field("device/"+id+"/report")...)
		body = append(body, 0)
	}
	return c.writeLocked(0x82, body)
}

// Refresh sends only fixed status-snapshot and firmware-version queries.
func (c *Client) Refresh(ids []string) error {
	for _, id := range ids {
		if !validSerial(id) {
			return errors.New("invalid printer identity")
		}
		body := append(field("device/"+id+"/request"), []byte(`{"pushing":{"sequence_id":"0","command":"pushall"}}`)...)
		if err := c.write(0x30, body); err != nil {
			return err
		}
		body = append(field("device/"+id+"/request"), []byte(`{"info":{"sequence_id":"0","command":"get_version"}}`)...)
		if err := c.write(0x30, body); err != nil {
			return err
		}
	}
	return nil
}
func (c *Client) Ping() error  { return c.write(0xc0, nil) }
func (c *Client) Close() error { return c.conn.Close() }

// Read returns the next printer report and acknowledges inbound QoS 1 packets.
func (c *Client) Read() (string, []byte, error) {
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		kind, data, err := readPacket(c.conn)
		if err != nil {
			return "", nil, err
		}
		switch kind >> 4 {
		case 3:
			if len(data) < 2 {
				return "", nil, errors.New("short MQTT publication")
			}
			n := int(binary.BigEndian.Uint16(data))
			if n == 0 || n+2 > len(data) {
				return "", nil, errors.New("invalid MQTT topic")
			}
			topic := string(data[2 : n+2])
			data = data[n+2:]
			qos := (kind >> 1) & 3
			if qos > 1 {
				return "", nil, errors.New("unsupported inbound MQTT QoS")
			}
			if qos == 1 {
				if len(data) < 2 {
					return "", nil, errors.New("short MQTT packet ID")
				}
				if err := c.write(0x40, data[:2]); err != nil {
					return "", nil, err
				}
				data = data[2:]
			}
			parts := strings.Split(topic, "/")
			if len(parts) != 3 || parts[0] != "device" || parts[2] != "report" {
				continue
			}
			return parts[1], data, nil
		case 9:
			if len(data) < 3 {
				return "", nil, errors.New("short MQTT subscription response")
			}
			for _, code := range data[2:] {
				if code > 2 {
					return "", nil, errors.New("MQTT subscription rejected")
				}
			}
		case 13: // PINGRESP
		default:
			return "", nil, errors.New("unexpected MQTT packet")
		}
	}
}
func field(s string) []byte { return append([]byte{byte(len(s) >> 8), byte(len(s))}, []byte(s)...) }
func (c *Client) write(kind byte, body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeLocked(kind, body)
}
func (c *Client) writeLocked(kind byte, body []byte) error {
	if len(body) > 2*1024*1024 {
		return errors.New("MQTT packet too large")
	}
	packet := []byte{kind}
	for n := len(body); ; {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 128
		}
		packet = append(packet, b)
		if n == 0 {
			break
		}
	}
	packet = append(packet, body...)
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	for len(packet) > 0 {
		n, err := c.conn.Write(packet)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		packet = packet[n:]
	}
	return nil
}
func readPacket(r io.Reader) (byte, []byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, nil, err
	}
	kind := b[0]
	n, m := 0, 1
	for i := 0; i < 4; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		n += int(b[0]&127) * m
		if n > 2*1024*1024 {
			return 0, nil, errors.New("MQTT packet too large")
		}
		if b[0]&128 == 0 {
			data := make([]byte, n)
			_, err := io.ReadFull(r, data)
			return kind, data, err
		}
		m *= 128
	}
	return 0, nil, errors.New("malformed MQTT length")
}
