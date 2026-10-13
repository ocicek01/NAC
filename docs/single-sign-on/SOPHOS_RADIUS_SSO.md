# Sophos Firewall RADIUS SSO Relay

This component forwards established human 802.1X accounting identities to Sophos Firewall. It does not authenticate users, replace Sophos Captive Portal, or handle passwords.

## Data flow

1. FreeRADIUS sends Start, Interim-Update, and Stop to the NAC accounting API.
2. NAC updates its accounting/session state and inserts every eligible event into the durable PostgreSQL outbox before marking the source accounting event complete.
3. If the outbox insert fails, NAC marks source processing failed and returns an accounting error so FreeRADIUS can retry. Duplicate retries are safe because both the source event and outbox source key are idempotent.
4. The asynchronous worker sends RFC 2866 Accounting-Request packets to Sophos UDP/1813.
5. An item is successful only after the Accounting-Response identifier and authenticator are verified.

MAB identities, blank usernames, MAC-shaped usernames, and missing/invalid IP addresses are skipped and audited. An IP-less Start can therefore be followed by an IP-bearing Interim-Update that creates the Sophos mapping. Password attributes are never accepted, stored, logged, or sent.

## Configuration

The relay is disabled by default.

```dotenv
SOPHOS_RADIUS_SSO_ENABLED=false
SOPHOS_RADIUS_SSO_HOST=10.0.0.1
SOPHOS_RADIUS_SSO_PORT=1813
SOPHOS_RADIUS_SSO_SECRET=replace-with-a-long-random-secret
SOPHOS_RADIUS_SSO_NAS_IDENTIFIER=nac-university
SOPHOS_RADIUS_SSO_TIMEOUT_SECONDS=3
SOPHOS_RADIUS_SSO_WORKER_INTERVAL_SECONDS=2
SOPHOS_RADIUS_SSO_WORKER_BATCH_SIZE=20
SOPHOS_RADIUS_SSO_MAX_RETRIES=5
SOPHOS_RADIUS_SSO_RETRY_BASE_SECONDS=5
SOPHOS_RADIUS_SSO_CLAIM_LEASE_SECONDS=60
```

Configure the NAC server IPv4 as the allowed RADIUS client on Sophos and use the same shared secret. Restrict UDP/1813 so only NAC can reach the Sophos accounting listener. Do not commit the secret to Git.

## Operations

Apply migration `000049_create_sophos_radius_sso_outbox.sql`, configure the environment, and restart `nac-api.service`.

Read-only visibility:

```text
GET /api/v1/sophos-sso/outbox?limit=50&offset=0
GET /api/v1/sophos-sso/health
```

Outbox states are `pending`, `processing`, `retry`, `sent`, and `dead_letter`. A processing lease protects against duplicate workers and permits recovery after a process crash. Completion and retry updates are fenced by the exact claimed lease token, so a stale worker cannot overwrite a reclaimed item. Retries use bounded exponential backoff and stop after exactly the configured maximum attempt count. Operators must investigate dead-letter items instead of enabling an infinite retry loop.

Audit actions include `sophos_sso_queued`, `sophos_sso_sent`, `sophos_sso_retry_scheduled`, `sophos_sso_dead_letter`, `sophos_sso_skipped`, and `sophos_sso_queue_failed`.
