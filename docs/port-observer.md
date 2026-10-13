# Port observer

`cmd/port-observer` is an independent read-only SNMP collector. It does not start
the API, trap listener, authentication, policy evaluation or enforcement workers.
The old `reader/management/snmp_manager.py` MAC set comparison and 15-second
cadence are the behavioral reference. No per-switch subprocesses or log-tail bus
are used.

## Collection and persistence

The fixed Go worker pool scans configured switches with nonempty SNMP read
communities. Switches have independent due times and round-robin queue admission,
so one slow device cannot delay or starve the rest of the inventory. For large
inventories, tune pool size based on switch capacity. Each scan has a deadline.
Currently SNMP v2c is supported; other versions produce a visible polling error.
Each request uses the switch timeout (capped at 2 seconds when above 3 seconds)
and at most one retry. Bulk walks fall back to GETNEXT walks on failure.

The observer reads IF-MIB type/admin/oper state, Q-BRIDGE PVIDs (mapped through
bridge ports), Cisco access VLANs, and merged BRIDGE/Q-BRIDGE FDB entries. Only
physical Ethernet ifTypes are stored. VLAN here means the
PVID/access VLAN, not the full trunk allowed VLAN set. Unreadable optional tables
preserve the previous VLAN/MAC values and set `switches.last_error`. A successful
empty FDB clears MACs. A missing/invalid status row skips only that port and is
reported in switch health; a failed complete status table commits no snapshot.

An in-process per-switch guard prevents overlapping scans. SNMP I/O happens
without a PostgreSQL connection; a short transaction then locks and compares the
affected rows. A trap/refresh committed during collection causes that entire
port snapshot to be skipped, including VLAN/MAC and delta generation; the next
poll reconciles it. Missing interfaces are not deleted.

Only changed admin/oper/VLAN/canonical MAC sets write `switch_ports`,
`port_events` and `audit_logs`, in one transaction. First observations create one
baseline event. An unchanged poll updates `last_discovered_at` without emitting
an event. Event metadata
contains previous/current state, so MAC addition and removal are recoverable.
Failed commits leave the previous snapshot intact for retry. Existing port
topology, protection and enforcement attributes are not changed. MAC learned and
removed deltas enter a bounded database outbox. They are delivered with stable
event IDs to the existing API inventory/enrichment/policy chain; retries are
idempotent and rate limited by a delivery goroutine independent from switch
scheduling. Transient failures retry at most five times; permanent HTTP 4xx
responses are dead-lettered. The API claims each event in a
`processing/completed/failed` inbox before inventory side effects. A removed MAC
detaches a device only while MAC, switch, port and observation freshness match.

## Panel and SSE

The Go API exposes `/api/v1/port-events/feed?switch_id=<uuid>&after=<cursor>`.
Omitting `after` establishes the current cursor. Results are ordered and limited
to 250 events per request. The cursor uses the existing immutable `created_at/id`
tuple, avoiding a large-table serial-column rewrite. This feed only includes
observer snapshot events and remains database-backed across
worker/API restarts. No event retention job is installed by this change.

With `NAC_PORT_OBSERVER_ENABLED=true` in the panel environment, the existing SSE
endpoint maps the Go switch UUID and ifIndex to panel switch/port IDs. Each
connection establishes its cursor before the browser requests a snapshot, then
sends batches of changed ports. `Last-Event-ID` resumes delivery. On reconnect a
fresh snapshot also reconciles gaps (including events removed by external
retention). SSE requests close after 25 seconds and reconnect after 3 seconds.
Configure the reverse proxy with response buffering disabled and a timeout above
30 seconds; allocate PHP workers for the expected simultaneous viewers.

The visible page fetches the complete stored snapshot every 15 seconds with one
in-flight request even while SSE is connected. Port clicks no longer trigger SNMP refreshes.
MAC removal and lower MAC counts replace stale UI values. Explicit discovery and
administrative actions remain available. Initial inventory discovery is still
needed to create the panel's local port records; unknown remote ports are not
guessed by port-number suffix.

The observer rollout flag defaults to false for compatibility. Enabling it also
disables Laravel's scheduled port-status polling and full discovery, preventing
duplicate collectors and destructive periodic snapshot replacement. Stop any
separately installed legacy poll/discovery timers too. Explicit full discovery
can still replace port inventory; run it during maintenance, then allow the
observer to reconcile. Trap ingest behavior is unchanged; the observer feed
reflects the following poll's stored state.

## Deployment

1. Apply Go migrations through 47 using the existing migration procedure.
   The runner wraps migrations in a transaction, so migration 47 deliberately
   avoids indexing the potentially large existing `port_events` table. After
   deploy, create the optional cursor index as a separate operational command:
   `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_port_events_observer_cursor ON port_events (switch_id, created_at, id) WHERE event_source = 'port-observer' AND event_type = 'port_snapshot_changed';`
2. Build on the deployment platform: `go build -o bin/nac-port-observer ./cmd/port-observer`.
   Rebuild/restart the API for the feed endpoint and corrected port Scan mapping.
3. Create a system `nac` user/group if absent; grant it read access to `/opt/nac`
   and its environment file (which contains SNMP/database secrets). It requires
   outbound UDP/161 and PostgreSQL connectivity, but no root privileges.
4. Install `deploy/systemd/nac-port-observer.service` in `/etc/systemd/system/`.
   Run `systemctl daemon-reload` and `systemctl enable --now nac-port-observer`.
5. Set `NAC_PORT_OBSERVER_ENABLED=true` in `panel-app/.env`, refresh Laravel's
   configuration cache and restart its scheduler. Ensure `NAC_API_URL` targets the
   updated API. The observer uses Go's existing `POSTGRES_*` environment values.

Worker environment defaults (Go reads process environment; systemd loads `.env`):

| Variable | Default |
| --- | --- |
| `PORT_OBSERVER_WORKERS` | `4` (1–64) |
| `PORT_OBSERVER_INTERVAL_SECONDS` | `15` |
| `PORT_OBSERVER_TIMEOUT_SECONDS` | `30` |
| `PORT_OBSERVER_HEALTH_ADDR` | `127.0.0.1:9091` |
| `PORT_OBSERVER_API_URL` | `http://127.0.0.1:8080` |

`GET http://127.0.0.1:9091/healthz` returns heartbeat, last completed cycle, active
workers and cumulative success/failure/skipped/change counters. It returns 503
before the first heartbeat, on stale heartbeat or inventory-read failure.
Individual device failures are visible in counters and `switches.last_error` /
`last_polled_at`; they do not mark all switches unhealthy. Check journal output
with `journalctl -u nac-port-observer` and alert if cycle time grows unexpectedly.

Rollback: stop/disable the observer and set the panel rollout flag to false,
then refresh Laravel config/scheduler. Migration 47 is additive and can remain.
