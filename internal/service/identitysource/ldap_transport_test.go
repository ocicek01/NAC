package identitysource

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestDialLDAPRejectsInvalidHost(t *testing.T) {
	for _, host := range []string{"", "https://ldap.example.org", "ldap://"} {
		if _, err := dialLDAP(host, "", time.Second); err == nil {
			t.Fatalf("expected invalid host %q to fail", host)
		}
	}
}

func TestDialLDAPFailsWithoutStartTLS(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	conn, err := dialLDAP(listener.Addr().String(), "", time.Second)
	if conn != nil {
		conn.Close()
		t.Fatal("unexpected plaintext LDAP connection")
	}
	if err == nil || !strings.Contains(err.Error(), "StartTLS failed") {
		t.Fatalf("expected StartTLS failure, got %v", err)
	}
}

func TestDialLDAPFailsWithoutCAFile(t *testing.T) {
	_, err := dialLDAP("ldap://127.0.0.1:389", "missing-ca.pem", time.Second)
	if err == nil || !strings.Contains(err.Error(), "read LDAP CA file") {
		t.Fatalf("expected CA file error, got %v", err)
	}
}
