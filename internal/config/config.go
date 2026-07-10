package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App             AppConfig
	Postgres        PostgresConfig
	Redis           RedisConfig
	Log             LogConfig
	DHCP            DHCPCollectorConfig
	SNMPTrap        SNMPTrapConfig
	SNMP            SNMPConfig
	Radius          RadiusConfig
	Identity        IdentityConfig
	Feature         FeatureConfig
	PostEnforcement PostEnforcementConfig
	Policy          PolicyConfig
	Enforcement     EnforcementConfig
}

type AppConfig struct{ Name, Env, Port string }
type PostgresConfig struct{ Host, Port, Database, User, Password, SSLMode string }
type RedisConfig struct{ Addr, Password, DB string }
type LogConfig struct{ Level string }
type DHCPCollectorConfig struct {
	Enabled     bool
	Interface   string
	Promiscuous bool
	SnapshotLen int32
}
type SNMPTrapConfig struct {
	Enabled                  bool
	BindHost                 string
	Port                     int
	ForwardEnabled           bool
	ForwardURL, ForwardToken string
	ForwardTimeoutSec        int
}
type SNMPConfig struct {
	Port, TimeoutMS, Retries int
	WalkPath                 string
	ARPExternalEnabled       bool
}
type RadiusConfig struct {
	GuestVLAN, RegistrationVLAN, QuarantineVLAN string
	CoAPort                                     int
	CoAClientPath                               string
}
type IdentityConfig struct {
	LDAPHost, LDAPBindDN, LDAPBindPassword, LDAPBaseDN, LDAPDeviceBaseDN, LDAPStaffGID, LDAPStudentGID, LDAPFacultyGID string
	LDAPWhitelistUIDs, LDAPWhitelistMails                                                                              []string
	StaffTargetVLAN, StudentTargetVLAN, FacultyTargetVLAN                                                              int
	LDAPVerifyURL, StaffVerifyURL, StudentVerifyURL                                                                    string
	HTTPTimeoutSeconds                                                                                                 int
}
type FeatureConfig struct{ Option82CorrelationEnabled, AutoEnforcementExecution bool }
type PostEnforcementConfig struct {
	IPLearningEnabled                        bool
	IPLearningWaitSec, IPRecheckSec          int
	PortBounceEnabled                        bool
	PortBounceDelaySec, MaxMACCountForBounce int
}
type EnforcementConfig struct {
	Mode                  string
	AllowedActions        []string
	AllowedSwitches       []string
	AllowedPorts          []string
	AllowedVLANs          []int
	AllowedDeviceIDs      []string
	AllowedMACs           []string
	MaxRetries            int
	AutoRollback          bool
	WorkerBatchSize       int
	RetryBackoffSeconds   int
	RequestTimeoutSeconds int
	AdapterPriority       []string
	MockAdapterEnabled    bool
	DefaultRestrictedVLAN int
	DefaultQuarantineVLAN int
}
type PolicyConfig struct {
	EnforcementEnabled, DefaultDryRun                                                                                                                                                                                                                                                                                                 bool
	ThresholdAllow, ThresholdMonitor, ThresholdRestricted, ThresholdRegistration                                                                                                                                                                                                                                                      int
	TrustBaseScore, TrustLDAPRegistryMatch, TrustRegisteredOwner, TrustKnownDeviceType, TrustDepartmentPresent, TrustDefaultVLANPresent, TrustStableAttachment, TrustLDAPNotFound, TrustUnknownDeviceType, TrustRapidPortMovement, TrustPreviousQuarantine, TrustIPMACAnomaly, TrustPortProfileMismatch, TrustRepeatedEnrichmentError int
}

func Load() (Config, error) {
	cfg := Config{
		App:             AppConfig{Name: getEnv("APP_NAME", "nac"), Env: getEnv("APP_ENV", "development"), Port: getEnv("APP_PORT", "8080")},
		Postgres:        PostgresConfig{Host: getEnv("POSTGRES_HOST", "127.0.0.1"), Port: getEnv("POSTGRES_PORT", "5432"), Database: getEnv("POSTGRES_DB", ""), User: getEnv("POSTGRES_USER", ""), Password: getEnv("POSTGRES_PASSWORD", ""), SSLMode: getEnv("POSTGRES_SSLMODE", "disable")},
		Redis:           RedisConfig{Addr: getEnv("REDIS_ADDR", "127.0.0.1:6379"), Password: getEnv("REDIS_PASSWORD", ""), DB: getEnv("REDIS_DB", "0")},
		Log:             LogConfig{Level: getEnv("LOG_LEVEL", "info")},
		DHCP:            DHCPCollectorConfig{Enabled: getEnvAsBool("DHCP_COLLECTOR_ENABLED", false), Interface: getEnv("DHCP_INTERFACE", "eth0"), Promiscuous: getEnvAsBool("DHCP_PROMISCUOUS", true), SnapshotLen: getEnvAsInt32("DHCP_SNAPSHOT_LEN", 1600)},
		SNMPTrap:        SNMPTrapConfig{Enabled: getEnvAsBool("SNMP_TRAP_ENABLED", false), BindHost: getEnv("SNMP_TRAP_BIND_HOST", "0.0.0.0"), Port: getEnvAsInt("SNMP_TRAP_PORT", 9162), ForwardEnabled: getEnvAsBool("SNMP_TRAP_FORWARD_ENABLED", false), ForwardURL: getEnv("SNMP_TRAP_FORWARD_URL", ""), ForwardToken: getEnv("SNMP_TRAP_FORWARD_TOKEN", ""), ForwardTimeoutSec: getEnvAsInt("SNMP_TRAP_FORWARD_TIMEOUT_SECONDS", 5)},
		SNMP:            SNMPConfig{Port: getEnvAsInt("SNMP_PORT", 161), TimeoutMS: getEnvAsInt("SNMP_TIMEOUT_MS", 2000), Retries: getEnvAsInt("SNMP_RETRIES", 1), WalkPath: getEnv("SNMP_WALK_PATH", "snmpwalk"), ARPExternalEnabled: getEnvAsBool("SNMP_ARP_EXTERNAL_ENABLED", true)},
		Radius:          RadiusConfig{GuestVLAN: getEnv("RADIUS_GUEST_VLAN", ""), RegistrationVLAN: getEnv("REGISTRATION_VLAN", getEnv("RADIUS_GUEST_VLAN", "")), QuarantineVLAN: getEnv("RADIUS_QUARANTINE_VLAN", ""), CoAPort: getEnvAsInt("RADIUS_COA_PORT", 3799), CoAClientPath: getEnv("RADIUS_COA_CLIENT_PATH", "radclient")},
		Identity:        IdentityConfig{LDAPHost: getEnv("LDAP_HOST", ""), LDAPBindDN: getEnv("LDAP_BIND_DN", ""), LDAPBindPassword: getEnv("LDAP_BIND_PASSWORD", ""), LDAPBaseDN: getEnv("LDAP_BASE_DN", ""), LDAPDeviceBaseDN: getEnv("LDAP_DEVICE_BASE_DN", ""), LDAPStaffGID: getEnv("LDAP_STAFF_GID", "501"), LDAPStudentGID: getEnv("LDAP_STUDENT_GID", "500"), LDAPFacultyGID: getEnv("LDAP_FACULTY_GID", "504"), LDAPWhitelistUIDs: getEnvAsCSV("LDAP_WHITELIST_UIDS"), LDAPWhitelistMails: getEnvAsCSV("LDAP_WHITELIST_MAILS"), StaffTargetVLAN: getEnvAsInt("IDENTITY_STAFF_TARGET_VLAN", 0), StudentTargetVLAN: getEnvAsInt("IDENTITY_STUDENT_TARGET_VLAN", 0), FacultyTargetVLAN: getEnvAsInt("IDENTITY_FACULTY_TARGET_VLAN", 0), LDAPVerifyURL: getEnv("IDENTITY_LDAP_VERIFY_URL", ""), StaffVerifyURL: getEnv("IDENTITY_STAFF_VERIFY_URL", ""), StudentVerifyURL: getEnv("IDENTITY_STUDENT_VERIFY_URL", ""), HTTPTimeoutSeconds: getEnvAsInt("IDENTITY_HTTP_TIMEOUT_SECONDS", 5)},
		Feature:         FeatureConfig{Option82CorrelationEnabled: getEnvAsBool("FEATURE_OPTION82_CORRELATION_ENABLED", false), AutoEnforcementExecution: getEnvAsBool("FEATURE_AUTO_ENFORCEMENT_EXECUTION", false)},
		PostEnforcement: PostEnforcementConfig{IPLearningEnabled: getEnvAsBool("POST_ENFORCEMENT_IP_LEARNING_ENABLED", false), IPLearningWaitSec: getEnvAsInt("POST_ENFORCEMENT_IP_LEARNING_WAIT_SECONDS", 30), IPRecheckSec: getEnvAsInt("POST_ENFORCEMENT_IP_RECHECK_SECONDS", 10), PortBounceEnabled: getEnvAsBool("POST_ENFORCEMENT_PORT_BOUNCE_ENABLED", false), PortBounceDelaySec: getEnvAsInt("POST_ENFORCEMENT_PORT_BOUNCE_DELAY_SECONDS", 3), MaxMACCountForBounce: getEnvAsInt("POST_ENFORCEMENT_MAX_MAC_COUNT_FOR_BOUNCE", 1)},
		Policy:          PolicyConfig{EnforcementEnabled: getEnvAsBool("POLICY_ENFORCEMENT_ENABLED", false), DefaultDryRun: getEnvAsBool("POLICY_DEFAULT_DRY_RUN", true), ThresholdAllow: getEnvAsInt("POLICY_THRESHOLD_ALLOW", 80), ThresholdMonitor: getEnvAsInt("POLICY_THRESHOLD_MONITOR", 60), ThresholdRestricted: getEnvAsInt("POLICY_THRESHOLD_RESTRICTED", 40), ThresholdRegistration: getEnvAsInt("POLICY_THRESHOLD_REGISTRATION", 20), TrustBaseScore: getEnvAsInt("POLICY_TRUST_BASE_SCORE", 50), TrustLDAPRegistryMatch: getEnvAsInt("POLICY_TRUST_LDAP_REGISTRY_MATCH", 20), TrustRegisteredOwner: getEnvAsInt("POLICY_TRUST_REGISTERED_OWNER", 10), TrustKnownDeviceType: getEnvAsInt("POLICY_TRUST_KNOWN_DEVICE_TYPE", 10), TrustDepartmentPresent: getEnvAsInt("POLICY_TRUST_DEPARTMENT_PRESENT", 5), TrustDefaultVLANPresent: getEnvAsInt("POLICY_TRUST_DEFAULT_VLAN_PRESENT", 5), TrustStableAttachment: getEnvAsInt("POLICY_TRUST_STABLE_ATTACHMENT", 10), TrustLDAPNotFound: getEnvAsInt("POLICY_TRUST_LDAP_NOT_FOUND", -15), TrustUnknownDeviceType: getEnvAsInt("POLICY_TRUST_UNKNOWN_DEVICE_TYPE", -10), TrustRapidPortMovement: getEnvAsInt("POLICY_TRUST_RAPID_PORT_MOVEMENT", -20), TrustPreviousQuarantine: getEnvAsInt("POLICY_TRUST_PREVIOUS_QUARANTINE", -20), TrustIPMACAnomaly: getEnvAsInt("POLICY_TRUST_IP_MAC_ANOMALY", -25), TrustPortProfileMismatch: getEnvAsInt("POLICY_TRUST_PORT_PROFILE_MISMATCH", -15), TrustRepeatedEnrichmentError: getEnvAsInt("POLICY_TRUST_REPEATED_ENRICHMENT_ERROR", -10)},
		Enforcement:     EnforcementConfig{Mode: getEnv("ENFORCEMENT_MODE", "dry_run"), AllowedActions: getEnvAsCSV("ENFORCEMENT_ALLOWED_ACTIONS"), AllowedSwitches: getEnvAsCSV("ENFORCEMENT_ALLOWED_SWITCHES"), AllowedPorts: getEnvAsCSV("ENFORCEMENT_ALLOWED_PORTS"), AllowedVLANs: getEnvAsIntCSV("ENFORCEMENT_ALLOWED_VLANS"), AllowedDeviceIDs: getEnvAsCSV("ENFORCEMENT_ALLOWED_DEVICE_IDS"), AllowedMACs: getEnvAsCSV("ENFORCEMENT_ALLOWED_MACS"), MaxRetries: getEnvAsInt("ENFORCEMENT_MAX_RETRIES", 3), AutoRollback: getEnvAsBool("ENFORCEMENT_AUTO_ROLLBACK", false), WorkerBatchSize: getEnvAsInt("ENFORCEMENT_WORKER_BATCH_SIZE", 20), RetryBackoffSeconds: getEnvAsInt("ENFORCEMENT_RETRY_BACKOFF_SECONDS", 30), RequestTimeoutSeconds: getEnvAsInt("ENFORCEMENT_REQUEST_TIMEOUT_SECONDS", 15), AdapterPriority: getEnvAsCSV("ENFORCEMENT_ADAPTER_PRIORITY"), MockAdapterEnabled: getEnvAsBool("ENFORCEMENT_MOCK_ADAPTER_ENABLED", false), DefaultRestrictedVLAN: getEnvAsInt("ENFORCEMENT_DEFAULT_RESTRICTED_VLAN", 0), DefaultQuarantineVLAN: getEnvAsInt("ENFORCEMENT_DEFAULT_QUARANTINE_VLAN", 0)},
	}
	if cfg.Postgres.Database == "" || cfg.Postgres.User == "" {
		return Config{}, fmt.Errorf("postgres configuration is incomplete")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func getEnvAsBool(key string, fallback bool) bool {
	switch os.Getenv(key) {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return fallback
	}
}
func getEnvAsInt32(key string, fallback int32) int32 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int32(parsed)
		}
	}
	return fallback
}
func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
func getEnvAsCSV(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getEnvAsIntCSV(key string) []int {
	items := getEnvAsCSV(key)
	if len(items) == 0 {
		return nil
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		parsed, err := strconv.Atoi(item)
		if err != nil || parsed <= 0 {
			continue
		}
		out = append(out, parsed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
