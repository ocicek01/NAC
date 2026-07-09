package identitysource

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	"nac/internal/config"
)

type LDAPResolver struct {
	host           string
	bindDN         string
	bindPassword   string
	baseDN         string
	staffGID       string
	studentGID     string
	facultyGID     string
	whitelistUIDs  map[string]struct{}
	whitelistMails map[string]struct{}
	staffVLAN      int
	studentVLAN    int
	facultyVLAN    int
	timeout        time.Duration
}

func NewLDAPResolver(cfg config.IdentityConfig) *LDAPResolver {
	if strings.TrimSpace(cfg.LDAPHost) == "" || strings.TrimSpace(cfg.LDAPBindDN) == "" || strings.TrimSpace(cfg.LDAPBaseDN) == "" {
		return nil
	}
	timeout := time.Duration(cfg.HTTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LDAPResolver{
		host:           strings.TrimSpace(cfg.LDAPHost),
		bindDN:         strings.TrimSpace(cfg.LDAPBindDN),
		bindPassword:   cfg.LDAPBindPassword,
		baseDN:         strings.TrimSpace(cfg.LDAPBaseDN),
		staffGID:       strings.TrimSpace(cfg.LDAPStaffGID),
		studentGID:     strings.TrimSpace(cfg.LDAPStudentGID),
		facultyGID:     strings.TrimSpace(cfg.LDAPFacultyGID),
		whitelistUIDs:  buildWhitelistSet(cfg.LDAPWhitelistUIDs),
		whitelistMails: buildWhitelistSet(cfg.LDAPWhitelistMails),
		staffVLAN:      cfg.StaffTargetVLAN,
		studentVLAN:    cfg.StudentTargetVLAN,
		facultyVLAN:    cfg.FacultyTargetVLAN,
		timeout:        timeout,
	}
}

func (r *LDAPResolver) Resolve(ctx context.Context, identifier, password string) (*Result, error) {
	_ = ctx
	if strings.TrimSpace(identifier) == "" || strings.TrimSpace(password) == "" {
		return nil, nil
	}

	conn, err := ldap.DialURL(r.dialURL(), ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}), ldap.DialWithDialer(&net.Dialer{Timeout: r.timeout}))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetTimeout(r.timeout)
	if err := conn.Bind(r.bindDN, r.bindPassword); err != nil {
		return nil, err
	}

	searchReq := ldap.NewSearchRequest(
		r.baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,
		int(r.timeout.Seconds()),
		false,
		fmt.Sprintf("(|(uid=%s)(cn=%s)(mail=%s))", ldap.EscapeFilter(identifier), ldap.EscapeFilter(identifier), ldap.EscapeFilter(identifier)),
		[]string{"dn", "uid", "cn", "mail", "gidNumber"},
		nil,
	)
	searchResp, err := conn.Search(searchReq)
	if err != nil {
		return nil, err
	}
	if len(searchResp.Entries) == 0 {
		return nil, nil
	}

	entry := searchResp.Entries[0]
	uid := strings.TrimSpace(entry.GetAttributeValue("uid"))
	mail := strings.TrimSpace(entry.GetAttributeValue("mail"))
	if !r.allowedByWhitelist(uid, mail) {
		return nil, nil
	}

	userDN := entry.DN
	if strings.TrimSpace(userDN) == "" {
		return nil, nil
	}

	userConn, err := ldap.DialURL(r.dialURL(), ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true}), ldap.DialWithDialer(&net.Dialer{Timeout: r.timeout}))
	if err != nil {
		return nil, err
	}
	defer userConn.Close()
	userConn.SetTimeout(r.timeout)
	if err := userConn.Bind(userDN, password); err != nil {
		return nil, nil
	}

	gid := strings.TrimSpace(entry.GetAttributeValue("gidNumber"))
	identityType, targetVLAN := r.mapGID(gid)
	if identityType == "" {
		return nil, nil
	}

	attrs := map[string]any{
		"gid_number": gid,
		"mail":       mail,
		"user_dn":    userDN,
	}

	return &Result{
		Matched:      true,
		Source:       "ldap",
		IdentityType: identityType,
		ExternalID:   uid,
		Username:     uid,
		FullName:     strings.TrimSpace(entry.GetAttributeValue("cn")),
		TargetVLAN:   targetVLAN,
		Attributes:   attrs,
	}, nil
}

func (r *LDAPResolver) mapGID(gid string) (string, int) {
	switch gid {
	case r.staffGID:
		return "personel", r.staffVLAN
	case r.facultyGID:
		return "personel", r.facultyVLAN
	case r.studentGID:
		return "ogrenci", r.studentVLAN
	default:
		return "", 0
	}
}

func (r *LDAPResolver) dialURL() string {
	host := strings.TrimSpace(r.host)
	if strings.HasPrefix(host, "ldap://") || strings.HasPrefix(host, "ldaps://") {
		return host
	}
	return "ldap://" + host
}

func (r *LDAPResolver) allowedByWhitelist(uid, mail string) bool {
	if len(r.whitelistUIDs) == 0 && len(r.whitelistMails) == 0 {
		return true
	}

	if _, ok := r.whitelistUIDs[normalizeWhitelistValue(uid)]; ok {
		return true
	}
	if _, ok := r.whitelistMails[normalizeWhitelistValue(mail)]; ok {
		return true
	}

	return false
}

func buildWhitelistSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeWhitelistValue(value)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}

	if len(set) == 0 {
		return nil
	}

	return set
}

func normalizeWhitelistValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
