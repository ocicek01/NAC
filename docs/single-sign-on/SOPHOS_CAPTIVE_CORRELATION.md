# Sophos Captive Portal Correlation

This component consumes Sophos Firewall Authentication syslog events and maps the
current captive-portal user to an existing NAC device. It does not authenticate a
user, expose a NAC login page, or replace the Sophos captive portal.

## Data boundary

- Only Captive Portal/Web Client events are accepted. RADIUS SSO and WiFi SSO
  events are excluded to avoid duplicating managed 802.1X identity.
- `registered_owner` and all LDAP ownership fields are never changed.
- No password, credential, or complete raw syslog message is stored. The event
  table contains selected metadata and a SHA-256 fingerprint only.
- A login updates `devices.sophos_*` only when the IP maps to one fresh MAC, or
  when Sophos supplies a valid MAC and there is no conflicting fresh binding.
- A logout clears current identity only when username, IP, and correlated MAC
  still match.

## Configuration

The listener is disabled by default.

```dotenv
SOPHOS_SYSLOG_ENABLED=false
SOPHOS_SYSLOG_BIND_HOST=0.0.0.0
SOPHOS_SYSLOG_PORT=5514
SOPHOS_SYSLOG_ALLOWED_SOURCES=10.6.0.1
SOPHOS_SYSLOG_ALLOWED_CLIENTS=Captive Portal,Web Client
SOPHOS_SYSLOG_BINDING_FRESHNESS_SECONDS=600
SOPHOS_SYSLOG_PROCESSING_TIMEOUT_SECONDS=3
```

When enabled, `SOPHOS_SYSLOG_ALLOWED_SOURCES` is mandatory and accepts IPs or
CIDRs. Configure Sophos to send Firewall Authentication logs to the NAC host's
UDP port. Restrict that port at the host firewall to the same source addresses.

## Visibility

- `GET /api/v1/sophos-auth/sessions?limit=50&offset=0`
- `GET /api/v1/sophos-auth/events?limit=50&offset=0`
- `GET /api/v1/sophos-auth/health`

Expected audit actions include `sophos_auth_login_correlated`,
`sophos_auth_logout`, `sophos_auth_uncorrelated`, `sophos_auth_ambiguous`,
`sophos_auth_rejected_source`, and `sophos_auth_parse_failed`.
