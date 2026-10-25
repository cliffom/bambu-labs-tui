package mqtt

import (
	"strings"
	"time"
)

// MQTT 3.1.1 wire values and local resource/deadline policies.
// Separate deadlines express independent operations, even at equal durations.
const (
	dialTimeout                 = 10 * time.Second
	handshakeTimeout            = 10 * time.Second
	writeTimeout                = 10 * time.Second
	readTimeout                 = 90 * time.Second
	clientIDRandomBytes         = 8
	clientIDPrefix              = "bambu-tui-"
	protocolName                = "MQTT"
	protocolLevel               = 4
	connectFlags                = 0xc2 // Username, password, and clean session.
	keepAliveSeconds            = 60
	maxFieldBytes               = 1<<16 - 1
	maxPacketBytes              = 2 * 1024 * 1024
	packetTypeMask              = 0xf0
	packetConnect               = 0x10
	packetConnAck               = 0x20
	packetPublish               = 0x30
	packetPubAck                = 0x40
	packetSubscribe             = 0x82 // SUBSCRIBE requires the low flags 0x02.
	packetSubAck                = 0x90
	packetPingRequest           = 0xc0
	packetPingResponse          = 0xd0
	connAckBytes                = 2
	connAckCodeOffset           = 1
	connAccepted                = 0
	connBadCredentials          = 4
	connUnauthorized            = 5
	uint16Bytes                 = 2
	bitsPerByte                 = 8
	qosShift                    = 1
	qosMask                     = 3
	qosAtMostOnce               = 0
	qosAtLeastOnce              = 1
	qosExactlyOnce              = 2
	remainingLengthBase         = 128
	remainingLengthMask         = 127
	remainingLengthContinuation = 128
	maxRemainingLengthBytes     = 4
	topicPrefix                 = "device/"
	reportTopicSuffix           = "/report"
	requestTopicSuffix          = "/request"
	statusRequest               = `{"pushing":{"sequence_id":"0","command":"pushall"}}`
	versionRequest              = `{"info":{"sequence_id":"0","command":"get_version"}}`
)

// reportPrinterID accepts exactly device/<id>/report, matching the wire topic.
func reportPrinterID(topic string) (string, bool) {
	path, ok := strings.CutPrefix(topic, topicPrefix)
	if !ok {
		return "", false
	}
	id, suffix, found := strings.Cut(path, "/")
	return id, found && "/"+suffix == reportTopicSuffix
}
