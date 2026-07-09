package identitysource

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"

	"nac/internal/config"
	"nac/internal/normalize"
)

type LDAPDeviceRecord struct {
	CommonName      string
	IPAddress       string
	MACAddress      string
	OwnerDN         string
	OwnerName       string
	OwnerUsername   string
	OwnerRole       string
	LocationDN      string
	DeviceType      string
	VLANID          int
	VLANName        string
	AssetTag        string
	DeviceStatus    string
	OwnershipType   string
	DefaultVLANID   int
	DefaultVLANName string
	Vendor          string
	Model           string
	Department      string
	PolicyName      string
	Description     string
}

type ldapDeviceCacheEntry struct {
	record    *LDAPDeviceRecord
	expiresAt time.Time
}

type LDAPDeviceResolver struct {
	host         string
	bindDN       string
	bindPassword string
	baseDN       string
	timeout      time.Duration
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	cache        map[string]ldapDeviceCacheEntry
}

func NewLDAPDeviceResolver(cfg config.IdentityConfig) *LDAPDeviceResolver {
	if strings.TrimSpace(cfg.LDAPHost) == "" || strings.TrimSpace(cfg.LDAPBindDN) == "" {
		return nil
	}

	baseDN := strings.TrimSpace(cfg.LDAPDeviceBaseDN)
	if baseDN == "" && strings.TrimSpace(cfg.LDAPBaseDN) != "" {
		baseDN = "ou=NetworkDevices," + strings.TrimSpace(cfg.LDAPBaseDN)
	}
	if baseDN == "" {
		return nil
	}

	timeout := time.Duration(cfg.HTTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &LDAPDeviceResolver{
		host:         strings.TrimSpace(cfg.LDAPHost),
		bindDN:       strings.TrimSpace(cfg.LDAPBindDN),
		bindPassword: cfg.LDAPBindPassword,
		baseDN:       baseDN,
		timeout:      timeout,
		cacheTTL:     5 * time.Minute,
		cache:        map[string]ldapDeviceCacheEntry{},
	}
}

func (r *LDAPDeviceResolver) LookupByMAC(ctx context.Context, macAddress string) (*LDAPDeviceRecord, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	normalizedMAC := normalize.MACAddress(macAddress)
	if normalizedMAC == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if record, ok := r.getCached(normalizedMAC); ok {
		return record, nil
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
		fmt.Sprintf("(macAddress=%s)", ldap.EscapeFilter(normalizedMAC)),
		[]string{
			"cn",
			"ipHostNumber",
			"macAddress",
			"ownerDn",
			"ownerName",
			"ownerUsername",
			"ownerUid",
			"ownerRole",
			"locationDn",
			"deviceType",
			"vlanId",
			"vlanName",
			"assetTag",
			"deviceStatus",
			"ownershipType",
			"defaultVlanId",
			"defaultVlanName",
			"vendor",
			"model",
			"department",
			"policyName",
			"description",
		},
		nil,
	)

	searchResp, err := conn.Search(searchReq)
	if err != nil {
		return nil, err
	}
	if len(searchResp.Entries) == 0 {
		r.setCached(normalizedMAC, nil)
		return nil, nil
	}

	entry := searchResp.Entries[0]
	ownerDN := strings.TrimSpace(entry.GetAttributeValue("ownerDn"))
	ownerUsername := firstNonEmpty(
		strings.TrimSpace(entry.GetAttributeValue("ownerUsername")),
		strings.TrimSpace(entry.GetAttributeValue("ownerUid")),
		ldapDNAttribute(ownerDN, "uid"),
	)
	record := &LDAPDeviceRecord{
		CommonName:      strings.TrimSpace(entry.GetAttributeValue("cn")),
		IPAddress:       strings.TrimSpace(entry.GetAttributeValue("ipHostNumber")),
		MACAddress:      normalize.MACAddress(entry.GetAttributeValue("macAddress")),
		OwnerDN:         ownerDN,
		OwnerName:       firstNonEmpty(strings.TrimSpace(entry.GetAttributeValue("ownerName")), ldapDNAttribute(ownerDN, "cn")),
		OwnerUsername:   ownerUsername,
		OwnerRole:       firstNonEmpty(strings.TrimSpace(entry.GetAttributeValue("ownerRole")), ldapDNAttribute(ownerDN, "title")),
		LocationDN:      strings.TrimSpace(entry.GetAttributeValue("locationDn")),
		DeviceType:      strings.TrimSpace(entry.GetAttributeValue("deviceType")),
		VLANID:          parseLDAPInt(entry.GetAttributeValue("vlanId")),
		VLANName:        strings.TrimSpace(entry.GetAttributeValue("vlanName")),
		AssetTag:        strings.TrimSpace(entry.GetAttributeValue("assetTag")),
		DeviceStatus:    strings.TrimSpace(entry.GetAttributeValue("deviceStatus")),
		OwnershipType:   strings.TrimSpace(entry.GetAttributeValue("ownershipType")),
		DefaultVLANID:   parseLDAPInt(entry.GetAttributeValue("defaultVlanId")),
		DefaultVLANName: strings.TrimSpace(entry.GetAttributeValue("defaultVlanName")),
		Vendor:          strings.TrimSpace(entry.GetAttributeValue("vendor")),
		Model:           strings.TrimSpace(entry.GetAttributeValue("model")),
		Department:      strings.TrimSpace(entry.GetAttributeValue("department")),
		PolicyName:      strings.TrimSpace(entry.GetAttributeValue("policyName")),
		Description:     strings.TrimSpace(entry.GetAttributeValue("description")),
	}
	r.setCached(normalizedMAC, record)
	return cloneLDAPDeviceRecord(record), nil
}

func (r *LDAPDeviceResolver) dialURL() string {
	host := strings.TrimSpace(r.host)
	if strings.HasPrefix(host, "ldap://") || strings.HasPrefix(host, "ldaps://") {
		return host
	}
	return "ldap://" + host
}

func parseLDAPInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func (r *LDAPDeviceResolver) getCached(macAddress string) (*LDAPDeviceRecord, bool) {
	if r == nil || r.cacheTTL <= 0 {
		return nil, false
	}
	r.cacheMu.RLock()
	entry, ok := r.cache[macAddress]
	r.cacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().UTC().After(entry.expiresAt) {
		r.cacheMu.Lock()
		delete(r.cache, macAddress)
		r.cacheMu.Unlock()
		return nil, false
	}
	return cloneLDAPDeviceRecord(entry.record), true
}

func (r *LDAPDeviceResolver) setCached(macAddress string, record *LDAPDeviceRecord) {
	if r == nil || r.cacheTTL <= 0 {
		return
	}
	r.cacheMu.Lock()
	r.cache[macAddress] = ldapDeviceCacheEntry{record: cloneLDAPDeviceRecord(record), expiresAt: time.Now().UTC().Add(r.cacheTTL)}
	r.cacheMu.Unlock()
}

func cloneLDAPDeviceRecord(record *LDAPDeviceRecord) *LDAPDeviceRecord {
	if record == nil {
		return nil
	}
	clone := *record
	return &clone
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ldapDNAttribute(value, attribute string) string {
	value = strings.TrimSpace(value)
	attribute = strings.ToLower(strings.TrimSpace(attribute))
	if value == "" || attribute == "" {
		return ""
	}
	parsed, err := ldap.ParseDN(value)
	if err != nil {
		return ""
	}
	for _, rdn := range parsed.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(strings.TrimSpace(attr.Type), attribute) {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	return ""
}
