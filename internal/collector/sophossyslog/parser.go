package sophossyslog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	domain "nac/internal/domain/sophosauth"
)

var errNotAuthentication = errors.New("not a Sophos Firewall Authentication record")

func Parse(payload []byte, receivedAt time.Time) (domain.ParsedEvent, error) {
	line := strings.TrimSpace(string(payload))
	fields, err := parseKeyValues(line)
	if err != nil {
		return domain.ParsedEvent{}, err
	}
	if !isAuthenticationRecord(fields) {
		return domain.ParsedEvent{}, errNotAuthentication
	}
	messageID := first(fields, "message_id", "log_id", "messageid", "logid")
	status := first(fields, "status", "auth_status")
	message := first(fields, "message", "msg")
	eventType := classify(messageID, status, message)
	eventAt := parseEventTime(fields, receivedAt)
	sum := sha256.Sum256([]byte(line))
	return domain.ParsedEvent{
		Fingerprint:   hex.EncodeToString(sum[:]),
		EventType:     eventType,
		MessageID:     messageID,
		Username:      first(fields, "user_name", "user", "username"),
		UserGroup:     first(fields, "user_group", "usergroupname", "usergroup"),
		ClientUsed:    first(fields, "client_used", "auth_client", "client"),
		AuthMechanism: first(fields, "auth_mechanism", "authentication_mechanism", "mechanism"),
		AuthStatus:    status,
		SourceIP:      first(fields, "src_ip", "sourceip", "source_ip"),
		SourceMAC:     first(fields, "src_mac", "sourcemac", "source_mac"),
		Message:       message,
		EventAt:       eventAt,
		Metadata: map[string]any{
			"log_component": first(fields, "log_component", "component", "module", "log_type"),
		},
	}, nil
}

func parseKeyValues(line string) (map[string]string, error) {
	out := make(map[string]string)
	for i := 0; i < len(line); {
		for i < len(line) && unicode.IsSpace(rune(line[i])) {
			i++
		}
		start := i
		for i < len(line) && isKeyChar(line[i]) {
			i++
		}
		if start == i || i >= len(line) || line[i] != '=' {
			for i < len(line) && !unicode.IsSpace(rune(line[i])) {
				i++
			}
			continue
		}
		key := strings.ToLower(line[start:i])
		i++
		var value strings.Builder
		if i < len(line) && line[i] == '"' {
			i++
			closed := false
			for i < len(line) {
				if line[i] == '\\' && i+1 < len(line) {
					i++
					value.WriteByte(line[i])
					i++
					continue
				}
				if line[i] == '"' {
					i++
					closed = true
					break
				}
				value.WriteByte(line[i])
				i++
			}
			if !closed {
				return nil, errors.New("unterminated quoted syslog value")
			}
		} else {
			for i < len(line) && !unicode.IsSpace(rune(line[i])) {
				value.WriteByte(line[i])
				i++
			}
		}
		out[key] = strings.TrimSpace(value.String())
	}
	return out, nil
}

func isKeyChar(value byte) bool {
	return value == '_' || value == '-' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func isAuthenticationRecord(fields map[string]string) bool {
	for _, key := range []string{"log_component", "component", "module", "log_type", "category", "subsystem", "log_subtype"} {
		value := normalize(fields[key])
		if value == "authentication" || strings.Contains(value, "firewallauthentication") {
			return true
		}
	}
	return false
}

func classify(messageID, status, message string) string {
	id := digits(messageID)
	state := normalize(status + " " + message)
	if strings.HasSuffix(id, "17703") || strings.Contains(state, "logout") || strings.Contains(state, "loggedout") {
		return "logout"
	}
	if strings.HasSuffix(id, "17701") && !isFailure(state) {
		return "login"
	}
	if strings.Contains(state, "login") && (strings.Contains(state, "success") || strings.Contains(state, "authenticated")) && !isFailure(state) {
		return "login"
	}
	return "ignored"
}

func isFailure(value string) bool {
	return strings.Contains(value, "fail") || strings.Contains(value, "denied") || strings.Contains(value, "invalid") || strings.Contains(value, "reject")
}

func parseEventTime(fields map[string]string, fallback time.Time) time.Time {
	for _, key := range []string{"eventtime", "timestamp", "event_timestamp"} {
		value := strings.TrimSpace(fields[key])
		if value == "" {
			continue
		}
		if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
			if unix > 1_000_000_000_000 {
				unix /= 1000
			}
			return time.Unix(unix, 0).UTC()
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return parsed.UTC()
		}
	}
	date, clock := strings.TrimSpace(fields["date"]), strings.TrimSpace(fields["time"])
	if date != "" && clock != "" {
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04:05Z07:00"} {
			if parsed, err := time.ParseInLocation(layout, date+" "+clock, fallback.Location()); err == nil {
				return parsed.UTC()
			}
		}
	}
	return fallback.UTC()
}

func first(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[key]); value != "" {
			return value
		}
	}
	return ""
}

func normalize(value string) string {
	var out strings.Builder
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			out.WriteRune(char)
		}
	}
	return out.String()
}

func digits(value string) string {
	var out strings.Builder
	for _, char := range value {
		if unicode.IsDigit(char) {
			out.WriteRune(char)
		}
	}
	return out.String()
}
