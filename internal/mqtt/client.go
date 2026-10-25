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
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout}, Config: &tls.Config{MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, errors.New("cannot connect to Bambu MQTT")
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	id := make([]byte, clientIDRandomBytes)
	if _, err := rand.Read(id); err != nil {
		_ = conn.Close()
		return nil, err
	}
	c, err := handshake(conn, clientIDPrefix+hex.EncodeToString(id), username, password)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func handshake(conn net.Conn, id, user, password string) (*Client, error) {
	for _, s := range []string{id, user, password} {
		if len(s) > maxFieldBytes {
			return nil, errors.New("MQTT credential exceeds protocol limit")
		}
	}
	c := &Client{conn: conn}
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	defer func() { _ = conn.SetDeadline(time.Time{}) }()
	body := append(field(protocolName), protocolLevel, connectFlags, 0, keepAliveSeconds)
	body = append(body, field(id)...)
	body = append(body, field(user)...)
	body = append(body, field(password)...)
	if err := c.write(packetConnect, body); err != nil {
		return nil, err
	}
	kind, data, err := readPacket(conn)
	if err != nil {
		return nil, err
	}
	if kind != packetConnAck || len(data) != connAckBytes {
		return nil, errors.New("invalid MQTT connection response")
	}
	if data[connAckCodeOffset] == connBadCredentials || data[connAckCodeOffset] == connUnauthorized {
		return nil, ErrAuth
	}
	if data[connAckCodeOffset] != connAccepted {
		return nil, fmt.Errorf("MQTT connection rejected (%d)", data[connAckCodeOffset])
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
	body := []byte{byte(c.next >> bitsPerByte), byte(c.next)}
	for _, id := range ids {
		if !validSerial(id) {
			return errors.New("invalid printer identity")
		}
		body = append(body, field(topicPrefix+id+reportTopicSuffix)...)
		body = append(body, qosAtMostOnce)
	}
	return c.writeLocked(packetSubscribe, body)
}

// Refresh sends only fixed status-snapshot and firmware-version queries.
func (c *Client) Refresh(ids []string) error {
	for _, id := range ids {
		if !validSerial(id) {
			return errors.New("invalid printer identity")
		}
		body := append(field(topicPrefix+id+requestTopicSuffix), []byte(statusRequest)...)
		if err := c.write(packetPublish, body); err != nil {
			return err
		}
		body = append(field(topicPrefix+id+requestTopicSuffix), []byte(versionRequest)...)
		if err := c.write(packetPublish, body); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Ping() error { return c.write(packetPingRequest, nil) }

func (c *Client) Close() error { return c.conn.Close() }

// Read returns the next printer report and acknowledges inbound QoS 1 packets.
func (c *Client) Read() (string, []byte, error) {
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		kind, data, err := readPacket(c.conn)
		if err != nil {
			return "", nil, err
		}
		switch kind & packetTypeMask {
		case packetPublish:
			if len(data) < uint16Bytes {
				return "", nil, errors.New("short MQTT publication")
			}
			n := int(binary.BigEndian.Uint16(data))
			if n == 0 || n+uint16Bytes > len(data) {
				return "", nil, errors.New("invalid MQTT topic")
			}
			topic := string(data[uint16Bytes : n+uint16Bytes])
			data = data[n+uint16Bytes:]
			qos := (kind >> qosShift) & qosMask
			if qos > qosAtLeastOnce {
				return "", nil, errors.New("unsupported inbound MQTT QoS")
			}
			if qos == qosAtLeastOnce {
				if len(data) < uint16Bytes {
					return "", nil, errors.New("short MQTT packet ID")
				}
				if err := c.write(packetPubAck, data[:uint16Bytes]); err != nil {
					return "", nil, err
				}
				data = data[uint16Bytes:]
			}
			id, ok := reportPrinterID(topic)
			if !ok {
				continue
			}
			return id, data, nil
		case packetSubAck:
			if len(data) < uint16Bytes+1 {
				return "", nil, errors.New("short MQTT subscription response")
			}
			for _, code := range data[uint16Bytes:] {
				if code > qosExactlyOnce {
					return "", nil, errors.New("MQTT subscription rejected")
				}
			}
		case packetPingResponse:
		default:
			return "", nil, errors.New("unexpected MQTT packet")
		}
	}
}

func field(s string) []byte {
	return append([]byte{byte(len(s) >> bitsPerByte), byte(len(s))}, []byte(s)...)
}

func (c *Client) write(kind byte, body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeLocked(kind, body)
}

func (c *Client) writeLocked(kind byte, body []byte) error {
	if len(body) > maxPacketBytes {
		return errors.New("MQTT packet too large")
	}
	packet := []byte{kind}
	for n := len(body); ; {
		b := byte(n % remainingLengthBase)
		n /= remainingLengthBase
		if n > 0 {
			b |= remainingLengthContinuation
		}
		packet = append(packet, b)
		if n == 0 {
			break
		}
	}
	packet = append(packet, body...)
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
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
	for i := 0; i < maxRemainingLengthBytes; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, nil, err
		}
		n += int(b[0]&remainingLengthMask) * m
		if n > maxPacketBytes {
			return 0, nil, errors.New("MQTT packet too large")
		}
		if b[0]&remainingLengthContinuation == 0 {
			data := make([]byte, n)
			_, err := io.ReadFull(r, data)
			return kind, data, err
		}
		m *= remainingLengthBase
	}
	return 0, nil, errors.New("malformed MQTT length")
}
