package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	domain "nac/internal/domain/radiusauth"
)

const (
	defaultAuthorizeURL  = "http://127.0.0.1:8080/api/v1/radius/authorize"
	defaultAccountingURL = "http://127.0.0.1:8080/api/v1/radius/accounting"
	defaultTimeout       = 3 * time.Second
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, nil))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer, client *http.Client) int {
	fs := flag.NewFlagSet("nac-radius-helper", flag.ContinueOnError)
	fs.SetOutput(stderr)

	mode := fs.String("mode", "authorize", "authorize or accounting")
	username := fs.String("username", envFirst(getenv, "USER_NAME"), "")
	macAddress := fs.String("mac", envFirst(getenv, "CALLING_STATION_ID", "USER_NAME"), "")
	hostname := fs.String("hostname", getenv("HOSTNAME"), "")
	vendorClass := fs.String("vendor-class", getenv("VENDOR_CLASS"), "")
	nasIP := fs.String("nas-ip", getenv("NAS_IP_ADDRESS"), "")
	nasIdentifier := fs.String("nas-identifier", getenv("NAS_IDENTIFIER"), "")
	nasPort := fs.String("nas-port", getenv("NAS_PORT"), "")
	nasPortID := fs.String("nas-port-id", getenv("NAS_PORT_ID"), "")
	nasPortType := fs.String("nas-port-type", getenv("NAS_PORT_TYPE"), "")
	calledStationID := fs.String("called-station-id", getenv("CALLED_STATION_ID"), "")
	callingStationID := fs.String("calling-station-id", getenv("CALLING_STATION_ID"), "")
	acctStatusType := fs.String("acct-status-type", getenv("ACCT_STATUS_TYPE"), "")
	acctSessionID := fs.String("acct-session-id", getenv("ACCT_SESSION_ID"), "")
	framedIP := fs.String("framed-ip-address", getenv("FRAMED_IP_ADDRESS"), "")
	sessionTime := fs.String("session-time", getenv("ACCT_SESSION_TIME"), "")
	terminateCause := fs.String("terminate-cause", getenv("ACCT_TERMINATE_CAUSE"), "")
	eventTimestamp := fs.String("event-timestamp", getenv("EVENT_TIMESTAMP"), "")
	acctDelayTime := fs.String("acct-delay-time", getenv("ACCT_DELAY_TIME"), "")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	var payload any
	var endpoint string
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "authorize":
		endpoint = envDefault(getenv, "NAC_RADIUS_AUTHORIZE_URL", defaultAuthorizeURL)
		payload = domain.AuthorizeRequest{
			Username: *username, MACAddress: *macAddress, Hostname: *hostname,
			VendorClass: *vendorClass, NASIPAddress: *nasIP, NASIdentifier: *nasIdentifier,
			NASPort: *nasPort, NASPortID: *nasPortID, CalledStationID: *calledStationID,
			CallingStationID: *callingStationID,
		}
	case "accounting":
		endpoint = envDefault(getenv, "NAC_RADIUS_ACCOUNTING_URL", defaultAccountingURL)
		delay, err := parseDelay(*acctDelayTime)
		if err != nil {
			writeError(stderr, err)
			return 1
		}
		payload = domain.AccountingRequest{
			Username: *username, MACAddress: *macAddress, Hostname: *hostname,
			VendorClass: *vendorClass, NASIPAddress: *nasIP, NASIdentifier: *nasIdentifier,
			NASPort: *nasPort, NASPortID: *nasPortID, NASPortType: *nasPortType,
			CalledStationID: *calledStationID, CallingStationID: *callingStationID,
			AcctStatusType: *acctStatusType, AcctSessionID: *acctSessionID,
			FramedIPAddress: *framedIP, SessionTime: *sessionTime,
			TerminateCause: *terminateCause, EventTimestamp: *eventTimestamp, AcctDelayTime: delay,
		}
	default:
		writeError(stderr, errors.New("unsupported mode"))
		return 1
	}

	if err := validateLoopbackURL(endpoint); err != nil {
		writeError(stderr, err)
		return 1
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeError(stderr, err)
		return 1
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		writeError(stderr, err)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(getenv("NAC_RADIUS_API_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if client == nil {
		client = &http.Client{Timeout: helperTimeout(getenv)}
	}
	resp, err := client.Do(req)
	if err != nil {
		writeError(stderr, errors.New("NAC RADIUS API is unavailable"))
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		writeError(stderr, fmt.Errorf("NAC RADIUS API returned status %d", resp.StatusCode))
		return 1
	}

	if strings.EqualFold(*mode, "accounting") {
		var result domain.AccountingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !strings.EqualFold(result.Status, "ok") {
			writeError(stderr, errors.New("invalid NAC accounting response"))
			return 1
		}
		return 0
	}

	var decision domain.AuthorizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		writeError(stderr, errors.New("invalid NAC authorize response"))
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(decision.Decision)) {
	case "accept":
		writePairs(stdout, stderr, decision)
		return 0
	case "reject":
		writePairs(stdout, stderr, decision)
		return 2
	default:
		writeError(stderr, errors.New("NAC authorize response has no valid decision"))
		return 1
	}
}

func writePairs(stdout, stderr io.Writer, decision domain.AuthorizeResponse) {
	replyKeys := make([]string, 0, len(decision.ReplyAttributes))
	for key := range decision.ReplyAttributes {
		replyKeys = append(replyKeys, key)
	}
	sort.Strings(replyKeys)
	for _, key := range replyKeys {
		fmt.Fprintf(stdout, "%s := %q\n", key, decision.ReplyAttributes[key])
	}

	controlKeys := make([]string, 0, len(decision.ControlAttributes))
	for key := range decision.ControlAttributes {
		controlKeys = append(controlKeys, key)
	}
	sort.Strings(controlKeys)
	for _, key := range controlKeys {
		fmt.Fprintf(stderr, "%s := %q\n", key, decision.ControlAttributes[key])
	}
}

func validateLoopbackURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Port() == "" {
		return errors.New("NAC RADIUS API URL must be an explicit loopback HTTP URL")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("NAC RADIUS API URL must use a loopback host")
}

func helperTimeout(getenv func(string) string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(getenv("NAC_RADIUS_HELPER_TIMEOUT_SECONDS")))
	if err != nil || seconds < 1 || seconds > 10 {
		return defaultTimeout
	}
	return time.Duration(seconds) * time.Second
}

func parseDelay(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	delay, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || delay < 0 {
		return 0, errors.New("acct-delay-time must be a non-negative integer")
	}
	return delay, nil
}

func envFirst(getenv func(string) string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envDefault(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func writeError(stderr io.Writer, err error) {
	fmt.Fprintln(stderr, err.Error())
}
