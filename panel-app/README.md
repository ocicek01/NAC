# NAC Panel

This application serves the NAC panel frontend and the Laravel-owned NAC APIs.

## Apache proxy notes

The deployment uses Apache in front of both:

- this Laravel app on `/opt/nac/panel-app/public`
- a legacy backend on `http://127.0.0.1:8080/api`

For switch live status, SSE, rediscovery and trap ingest endpoints, Apache must
keep selected `/api` routes on Laravel and only proxy the remaining legacy API
traffic to `:8080`.

Use the example vhost in [deploy/apache/nac.conf](deploy/apache/nac.conf).

Critical exclusions:

- `/api/events/ports`
- `/api/switches`
- `/api/switch-ports`
- `/api/discovery-jobs`
- `/api/traps/snmp`

## Verification

After updating Apache:

```bash
sudo apache2ctl configtest
sudo systemctl reload apache2

curl -i http://127.0.0.1/api/switches/13/ports/status
curl -N http://127.0.0.1/api/events/ports
curl -i -X POST http://127.0.0.1/api/traps/snmp
```

Do not test these Laravel endpoints against `127.0.0.1:8080`; that target is
the legacy backend and will return `404 page not found` for the new NAC routes.

## UDP trap listener

The project now includes a native UDP SNMP trap listener command for direct
switch trap reception without going through the HTTP trap endpoint first.

Default listener settings:

- `NAC_TRAP_LISTENER_ENABLED=true`
- `NAC_TRAP_LISTENER_HOST=0.0.0.0`
- `NAC_TRAP_LISTENER_PORT=162`
- `NAC_TRAP_LISTENER_BUFFER_BYTES=65535`
- `NAC_TRAP_VALIDATE_COMMUNITY=false`

Run it manually:

```bash
cd /opt/nac/panel-app
php artisan nac:listen-snmp-traps --port=162
```

For a single test packet workflow:

```bash
cd /opt/nac/panel-app
php artisan nac:listen-snmp-traps --port=162 --max-packets=1
```

The listener currently supports SNMP v1/v2c trap packets and feeds the decoded
interface data into the existing `SnmpTrapIngestService` and `PortStatusUpdater`
flow with `source=snmp_trap`.

