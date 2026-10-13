# FreeRADIUS deployment templates

These files target FreeRADIUS 3.x on Debian/Ubuntu. They are templates: replace
every `@@NAME@@` value before enabling them. Never commit rendered secrets.

Install locations:

| Repository file | Destination |
| --- | --- |
| `mods-available/nac_ldap` | `/etc/freeradius/3.0/mods-available/nac_ldap` |
| `mods-available/nac_eap` | `/etc/freeradius/3.0/mods-available/nac_eap` |
| `mods-available/nac_helper` | `/etc/freeradius/3.0/mods-available/nac_helper` |
| `sites-available/nac` | `/etc/freeradius/3.0/sites-available/nac` |
| `sites-available/nac-inner-tunnel` | `/etc/freeradius/3.0/sites-available/nac-inner-tunnel` |
| `radius-helper.env.example` | `/etc/nac/radius-helper.env` |

Create symlinks in `mods-enabled` and `sites-enabled`, then disable conflicting
default sites only after `freeradius -XC` succeeds. The helper binary belongs at
`/usr/local/libexec/nac-radius-helper` and must be owned by root, mode `0755`.

The service account running FreeRADIUS needs read permission for
`/etc/nac/radius-helper.env`, while the file should not be readable by other
users. Load it into the FreeRADIUS systemd unit with an override:

```ini
[Service]
EnvironmentFile=/etc/nac/radius-helper.env
```

Validate before restart:

```bash
freeradius -XC
systemctl restart freeradius
journalctl -u freeradius -n 100 --no-pager
```

`nac-inner-tunnel` implements managed-user authentication using EAP-TTLS/PAP.
`nac_mab_authorize` is a separate authorization-only path and never calls LDAP.
Accounting is sent to NAC after local RADIUS accounting modules run.
