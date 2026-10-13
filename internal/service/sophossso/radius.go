package sophossso

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	domain "nac/internal/domain/sophossso"
)

const (
	accountingRequestCode  = 4
	accountingResponseCode = 5
)

type UDPClient interface {
	Exchange(address string, packet []byte, timeout time.Duration) ([]byte, error)
}

type netUDPClient struct{}

func (netUDPClient) Exchange(address string, packet []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("udp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(packet); err != nil {
		return nil, err
	}
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		return nil, err
	}
	return response[:n], nil
}

func buildAccountingPacket(item domain.Item, nasIdentifier, secret string) ([]byte, byte, [16]byte, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, 0, [16]byte{}, errors.New("RADIUS shared secret is empty")
	}
	ip := net.ParseIP(item.IPAddress).To4()
	if ip == nil {
		return nil, 0, [16]byte{}, errors.New("invalid framed IP address")
	}
	status := uint32(0)
	switch item.AcctStatusType {
	case "Start":
		status = 1
	case "Stop":
		status = 2
	case "Interim-Update":
		status = 3
	default:
		return nil, 0, [16]byte{}, fmt.Errorf("unsupported accounting status %q", item.AcctStatusType)
	}
	identifierBytes := []byte{0}
	if _, err := rand.Read(identifierBytes); err != nil {
		return nil, 0, [16]byte{}, err
	}
	id := identifierBytes[0]
	attrs := make([]byte, 0, 128)
	attrs = appendStringAttribute(attrs, 1, item.Username)
	attrs = append(attrs, 8, 6)
	attrs = append(attrs, ip...)
	attrs = appendStringAttribute(attrs, 31, item.MACAddress)
	attrs = appendUint32Attribute(attrs, 40, status)
	attrs = appendUint32Attribute(attrs, 41, uint32(max(item.AcctDelayTime, 0)))
	attrs = appendStringAttribute(attrs, 44, item.AcctSessionID)
	attrs = appendUint32Attribute(attrs, 45, 1)
	attrs = appendStringAttribute(attrs, 32, nasIdentifier)
	attrs = appendUint32Attribute(attrs, 55, uint32(item.EventTimestamp.Unix()))
	length := 20 + len(attrs)
	if length > 4096 {
		return nil, 0, [16]byte{}, errors.New("RADIUS packet is too large")
	}
	packet := make([]byte, length)
	packet[0], packet[1] = accountingRequestCode, id
	binary.BigEndian.PutUint16(packet[2:4], uint16(length))
	copy(packet[20:], attrs)
	hashInput := append(append([]byte{}, packet...), []byte(secret)...)
	auth := md5.Sum(hashInput)
	copy(packet[4:20], auth[:])
	return packet, id, auth, nil
}

func validateAccountingResponse(response []byte, identifier byte, requestAuthenticator [16]byte, secret string) error {
	if len(response) < 20 {
		return errors.New("RADIUS response is shorter than header")
	}
	length := int(binary.BigEndian.Uint16(response[2:4]))
	if length < 20 || length > len(response) {
		return errors.New("invalid RADIUS response length")
	}
	if response[0] != accountingResponseCode {
		return fmt.Errorf("unexpected RADIUS response code %d", response[0])
	}
	if response[1] != identifier {
		return errors.New("RADIUS response identifier mismatch")
	}
	input := make([]byte, 0, length+len(secret))
	input = append(input, response[:4]...)
	input = append(input, requestAuthenticator[:]...)
	input = append(input, response[20:length]...)
	input = append(input, []byte(secret)...)
	expected := md5.Sum(input)
	if subtle.ConstantTimeCompare(response[4:20], expected[:]) != 1 {
		return errors.New("RADIUS response authenticator mismatch")
	}
	return nil
}

func appendStringAttribute(dst []byte, typ byte, value string) []byte {
	value = strings.TrimSpace(value)
	if value == "" {
		return dst
	}
	data := []byte(value)
	if len(data) > 253 {
		data = data[:253]
	}
	return append(append(dst, typ, byte(len(data)+2)), data...)
}

func appendUint32Attribute(dst []byte, typ byte, value uint32) []byte {
	start := len(dst)
	dst = append(dst, typ, 6, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(dst[start+2:start+6], value)
	return dst
}
