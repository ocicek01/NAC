# FreeRADIUS deployment templates

These files target FreeRADIUS 3.x on Debian/Ubuntu. They are templates: replace
every `@@NAME@@` value before enabling them. Never commit rendered secrets.
`nac_ldap` is rendered separately by `install-nac-ldap-starttls.sh` using the
LDAP variables in `/opt/nac/.env`; it requires a verified CA and StartTLS.
`sites-available/nac-ldap-test` is an optional loopback-only PAP test server on
port 18121 (18120 is used by the default inner tunnel). It does not replace the live site, has no MAB or EAP path, and
rejects requests without a password. Enable it only for isolated LDAP tests.

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

For the NAC API's LDAP user and device lookups, set `LDAP_HOST` to an
`ldap://host:389` address for mandatory StartTLS or `ldaps://host:636` for
direct TLS. Set `LDAP_CA_FILE` to the trusted PEM CA certificate. The host must
match the server certificate's DNS or IP subject alternative name. LDAP binds
fail closed when TLS negotiation or certificate verification fails.
