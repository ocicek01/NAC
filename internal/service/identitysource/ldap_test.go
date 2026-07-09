package identitysource

import (
	"testing"

	"nac/internal/config"
)

func TestNewLDAPResolverBuildsWhitelistSets(t *testing.T) {
	resolver := NewLDAPResolver(config.IdentityConfig{
		LDAPHost:           "ldap.example.local:389",
		LDAPBindDN:         "cn=admin,dc=example,dc=local",
		LDAPBaseDN:         "dc=example,dc=local",
		LDAPWhitelistUIDs:  []string{"  Alice ", "bob"},
		LDAPWhitelistMails: []string{" Alice@example.local ", "bob@example.local"},
	})
	if resolver == nil {
		t.Fatal("expected resolver to be created")
	}

	if _, ok := resolver.whitelistUIDs["alice"]; !ok {
		t.Fatal("expected alice uid in whitelist")
	}
	if _, ok := resolver.whitelistUIDs["bob"]; !ok {
		t.Fatal("expected bob uid in whitelist")
	}
	if _, ok := resolver.whitelistMails["alice@example.local"]; !ok {
		t.Fatal("expected alice mail in whitelist")
	}
}

func TestLDAPResolverAllowedByWhitelist(t *testing.T) {
	resolver := &LDAPResolver{
		whitelistUIDs:  buildWhitelistSet([]string{"allowed.user"}),
		whitelistMails: buildWhitelistSet([]string{"allowed@example.local"}),
	}

	if !resolver.allowedByWhitelist("allowed.user", "other@example.local") {
		t.Fatal("expected uid whitelist match to pass")
	}
	if !resolver.allowedByWhitelist("other.user", "allowed@example.local") {
		t.Fatal("expected mail whitelist match to pass")
	}
	if resolver.allowedByWhitelist("other.user", "other@example.local") {
		t.Fatal("expected non-whitelisted user to be rejected")
	}
}

func TestLDAPResolverAllowedByWhitelistWhenDisabled(t *testing.T) {
	resolver := &LDAPResolver{}

	if !resolver.allowedByWhitelist("any.user", "any@example.local") {
		t.Fatal("expected whitelist-disabled resolver to allow ldap match")
	}
}
