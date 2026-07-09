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
}

type AppConfig struct {
	Name string
	Env  string
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       string
}

type LogConfig struct {
	Level string
}

type DHCPCollectorConfig struct {
	Enabled     bool
	Interface   string
	Promiscuous bool
	SnapshotLen int32
}

type SNMPTrapConfig struct {
	Enabled           bool
	BindHost          string
	Port              int
	ForwardEnabled    bool
	ForwardURL        string
	ForwardToken      string
	ForwardTimeoutSec int
}

type SNMPConfig struct {
	Port               int
	TimeoutMS          int
	Retries            int
	WalkPath           string
	ARPExternalEnabled bool
}

type RadiusConfig struct {
	GuestVLAN        string
	RegistrationVLAN string
	QuarantineVLAN   string
	CoAPort          int
	CoAClientPath    string
}

type IdentityConfig struct {
	LDAPHost           string
	LDAPBindDN         string
	LDAPBindPassword   string
	LDAPBaseDN         string
	LDAPDeviceBaseDN   string
	LDAPStaffGID       string
	LDAPStudentGID     string
	LDAPFacultyGID     string
	LDAPWhitelistUIDs  []string
	LDAPWhitelistMails []string
	StaffTargetVLAN    int
	StudentTargetVLAN  int
	FacultyTargetVLAN  int
	LDAPVerifyURL      string
	StaffVerifyURL     string
	StudentVerifyURL   string
	HTTPTimeoutSeconds int
}

type FeatureConfig struct {
	Option82CorrelationEnabled bool
	AutoEnforcementExecution   bool
}

type PostEnforcementConfig struct {
	IPLearningEnabled    bool
	IPLearningWaitSec    int
	IPRecheckSec         int
	PortBounceEnabled    bool
	PortBounceDelaySec   int
	MaxMACCountForBounce int
}

func Load() (Config, error) {
	cfg := Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "nac"),
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "127.0.0.1"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			Database: getEnv("POSTGRES_DB", ""),
			User:     getEnv("POSTGRES_USER", ""),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnv("REDIS_DB", "0"),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		DHCP: DHCPCollectorConfig{
			Enabled:     getEnvAsBool("DHCP_COLLECTOR_ENABLED", false),
			Interface:   getEnv("DHCP_INTERFACE", "eth0"),
			Promiscuous: getEnvAsBool("DHCP_PROMISCUOUS", true),
			SnapshotLen: getEnvAsInt32("DHCP_SNAPSHOT_LEN", 1600),
		},
		SNMPTrap: SNMPTrapConfig{
			Enabled:           getEnvAsBool("SNMP_TRAP_ENABLED", false),
			BindHost:          getEnv("SNMP_TRAP_BIND_HOST", "0.0.0.0"),
			Port:              getEnvAsInt("SNMP_TRAP_PORT", 9162),
			ForwardEnabled:    getEnvAsBool("SNMP_TRAP_FORWARD_ENABLED", false),
			ForwardURL:        getEnv("SNMP_TRAP_FORWARD_URL", ""),
			ForwardToken:      getEnv("SNMP_TRAP_FORWARD_TOKEN", ""),
			ForwardTimeoutSec: getEnvAsInt("SNMP_TRAP_FORWARD_TIMEOUT_SECONDS", 5),
		},
		SNMP: SNMPConfig{
			Port:               getEnvAsInt("SNMP_PORT", 161),
			TimeoutMS:          getEnvAsInt("SNMP_TIMEOUT_MS", 2000),
			Retries:            getEnvAsInt("SNMP_RETRIES", 1),
			WalkPath:           getEnv("SNMP_WALK_PATH", "snmpwalk"),
			ARPExternalEnabled: getEnvAsBool("SNMP_ARP_EXTERNAL_ENABLED", true),
		},
		Radius: RadiusConfig{
			GuestVLAN:        getEnv("RADIUS_GUEST_VLAN", ""),
			RegistrationVLAN: getEnv("REGISTRATION_VLAN", getEnv("RADIUS_GUEST_VLAN", "")),
			QuarantineVLAN:   getEnv("RADIUS_QUARANTINE_VLAN", ""),
			CoAPort:          getEnvAsInt("RADIUS_COA_PORT", 3799),
			CoAClientPath:    getEnv("RADIUS_COA_CLIENT_PATH", "radclient"),
		},
		Identity: IdentityConfig{
			LDAPHost:           getEnv("LDAP_HOST", ""),
			LDAPBindDN:         getEnv("LDAP_BIND_DN", ""),
			LDAPBindPassword:   getEnv("LDAP_BIND_PASSWORD", ""),
			LDAPBaseDN:         getEnv("LDAP_BASE_DN", ""),
			LDAPDeviceBaseDN:   getEnv("LDAP_DEVICE_BASE_DN", ""),
			LDAPStaffGID:       getEnv("LDAP_STAFF_GID", "501"),
			LDAPStudentGID:     getEnv("LDAP_STUDENT_GID", "500"),
			LDAPFacultyGID:     getEnv("LDAP_FACULTY_GID", "504"),
			LDAPWhitelistUIDs:  getEnvAsCSV("LDAP_WHITELIST_UIDS"),
			LDAPWhitelistMails: getEnvAsCSV("LDAP_WHITELIST_MAILS"),
			StaffTargetVLAN:    getEnvAsInt("IDENTITY_STAFF_TARGET_VLAN", 0),
			StudentTargetVLAN:  getEnvAsInt("IDENTITY_STUDENT_TARGET_VLAN", 0),
			FacultyTargetVLAN:  getEnvAsInt("IDENTITY_FACULTY_TARGET_VLAN", 0),
			LDAPVerifyURL:      getEnv("IDENTITY_LDAP_VERIFY_URL", ""),
			StaffVerifyURL:     getEnv("IDENTITY_STAFF_VERIFY_URL", ""),
			StudentVerifyURL:   getEnv("IDENTITY_STUDENT_VERIFY_URL", ""),
			HTTPTimeoutSeconds: getEnvAsInt("IDENTITY_HTTP_TIMEOUT_SECONDS", 5),
		},
		Feature: FeatureConfig{
			Option82CorrelationEnabled: getEnvAsBool("FEATURE_OPTION82_CORRELATION_ENABLED", false),
			AutoEnforcementExecution:   getEnvAsBool("FEATURE_AUTO_ENFORCEMENT_EXECUTION", false),
		},
		PostEnforcement: PostEnforcementConfig{
			IPLearningEnabled:    getEnvAsBool("POST_ENFORCEMENT_IP_LEARNING_ENABLED", false),
			IPLearningWaitSec:    getEnvAsInt("POST_ENFORCEMENT_IP_LEARNING_WAIT_SECONDS", 30),
			IPRecheckSec:         getEnvAsInt("POST_ENFORCEMENT_IP_RECHECK_SECONDS", 10),
			PortBounceEnabled:    getEnvAsBool("POST_ENFORCEMENT_PORT_BOUNCE_ENABLED", false),
			PortBounceDelaySec:   getEnvAsInt("POST_ENFORCEMENT_PORT_BOUNCE_DELAY_SECONDS", 3),
			MaxMACCountForBounce: getEnvAsInt("POST_ENFORCEMENT_MAX_MAC_COUNT_FOR_BOUNCE", 1),
		},
	}

	if cfg.Postgres.Database == "" || cfg.Postgres.User == "" {
		return Config{}, fmt.Errorf("postgres configuration is incomplete")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvAsBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return fallback
	}
}

func getEnvAsInt32(key string, fallback int32) int32 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}

	return int32(parsed)
}

func getEnvAsInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvAsCSV(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}

	if len(items) == 0 {
		return nil
	}

	return items
}
