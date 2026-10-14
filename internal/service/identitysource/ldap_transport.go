package identitysource

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func dialLDAP(host, caFile string, timeout time.Duration) (*ldap.Conn, error) {
	address := strings.TrimSpace(host)
	if !strings.Contains(address, "://") {
		address = "ldap://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "ldap" && parsed.Scheme != "ldaps") {
		return nil, fmt.Errorf("invalid LDAP host %q", host)
	}

	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system CA pool: %w", err)
	}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read LDAP CA file: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("LDAP CA file contains no certificates")
		}
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12}
	conn, err := ldap.DialURL(address, ldap.DialWithTLSConfig(tlsConfig), ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(timeout)
	if parsed.Scheme == "ldap" {
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("LDAP StartTLS failed: %w", err)
		}
	}
	return conn, nil
}
