package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	domain "nac/internal/domain/radiusauth"
)

func main() {
	req := domain.AuthorizeRequest{}
	flag.StringVar(&req.MACAddress, "mac", envFirst("CALLING_STATION_ID", "USER_NAME"), "")
	flag.StringVar(&req.Hostname, "hostname", os.Getenv("HOSTNAME"), "")
	flag.StringVar(&req.VendorClass, "vendor-class", os.Getenv("VENDOR_CLASS"), "")
	flag.StringVar(&req.NASIPAddress, "nas-ip", os.Getenv("NAS_IP_ADDRESS"), "")
	flag.StringVar(&req.NASIdentifier, "nas-identifier", os.Getenv("NAS_IDENTIFIER"), "")
	flag.StringVar(&req.NASPort, "nas-port", os.Getenv("NAS_PORT"), "")
	flag.StringVar(&req.NASPortID, "nas-port-id", os.Getenv("NAS_PORT_ID"), "")
	flag.StringVar(&req.CalledStationID, "called-station-id", os.Getenv("CALLED_STATION_ID"), "")
	flag.StringVar(&req.CallingStationID, "calling-station-id", os.Getenv("CALLING_STATION_ID"), "")
	flag.Parse()

	endpoint := os.Getenv("NAC_RADIUS_API_URL")
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "http://127.0.0.1:8080/api/v1/radius/authorize"
	}

	body, err := json.Marshal(req)
	if err != nil {
		fail(err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		fail(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		fail(fmt.Errorf("radius api returned status %d", resp.StatusCode))
	}

	var decision domain.AuthorizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		fail(err)
	}

	writePairs(decision)

	if strings.EqualFold(decision.Decision, "reject") {
		os.Exit(2)
	}
}

func writePairs(decision domain.AuthorizeResponse) {
	replyKeys := make([]string, 0, len(decision.ReplyAttributes))
	for key := range decision.ReplyAttributes {
		replyKeys = append(replyKeys, key)
	}
	sort.Strings(replyKeys)
	for _, key := range replyKeys {
		fmt.Printf("%s := %q\n", key, decision.ReplyAttributes[key])
	}

	controlKeys := make([]string, 0, len(decision.ControlAttributes))
	for key := range decision.ControlAttributes {
		controlKeys = append(controlKeys, key)
	}
	sort.Strings(controlKeys)
	for _, key := range controlKeys {
		fmt.Fprintf(os.Stderr, "%s := %q\n", key, decision.ControlAttributes[key])
	}
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
