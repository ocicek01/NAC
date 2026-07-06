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
