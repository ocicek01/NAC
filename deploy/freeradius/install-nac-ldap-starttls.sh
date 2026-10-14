#!/usr/bin/env bash
set -euo pipefail

if [[ $(id -u) -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi

set -a
source /opt/nac/.env
set +a

: "${LDAP_HOST:?LDAP_HOST is required}"
: "${LDAP_BIND_DN:?LDAP_BIND_DN is required}"
: "${LDAP_BIND_PASSWORD:?LDAP_BIND_PASSWORD is required}"
: "${LDAP_BASE_DN:?LDAP_BASE_DN is required}"
: "${LDAP_CA_FILE:?LDAP_CA_FILE is required}"

if [[ $LDAP_HOST == *://* || $LDAP_HOST == *[[:space:]]* ]]; then
  echo "LDAP_HOST must be a hostname or IP without a scheme" >&2
  exit 1
fi
for value in "$LDAP_BIND_DN" "$LDAP_BIND_PASSWORD" "$LDAP_BASE_DN" "$LDAP_CA_FILE"; do
  if [[ $value == *'"'* || $value == *$'\n'* || $value == *$'\r'* ]]; then
    echo "LDAP configuration contains unsupported quote or newline" >&2
    exit 1
  fi
done
test -r "$LDAP_CA_FILE"
export LDAP_RADIUS_CA_FILE=/etc/freeradius/3.0/certs/nac/nac-root-ca.crt
test -r "$LDAP_RADIUS_CA_FILE"
cmp -s "$LDAP_CA_FILE" "$LDAP_RADIUS_CA_FILE"

target=/etc/freeradius/3.0/mods-available/nac_ldap
enabled=/etc/freeradius/3.0/mods-enabled/nac_ldap
template=/opt/nac/deploy/freeradius/mods-available/nac_ldap
tmp=$(mktemp /etc/freeradius/3.0/mods-available/.nac_ldap.XXXXXX)
trap 'rm -f "$tmp"' EXIT

envsubst '${LDAP_HOST} ${LDAP_BIND_DN} ${LDAP_BIND_PASSWORD} ${LDAP_BASE_DN} ${LDAP_RADIUS_CA_FILE}' < "$template" > "$tmp"
if grep -q '\${LDAP_' "$tmp"; then
  echo "unrendered LDAP template placeholder" >&2
  exit 1
fi
chown root:freerad "$tmp"
chmod 0640 "$tmp"
had_target=false
if [[ -e $target ]]; then
  had_target=true
  cp -p "$target" "$target.before-starttls"
fi
mv "$tmp" "$target"
created_link=false
if [[ ! -e $enabled && ! -L $enabled ]]; then
  ln -s "$target" "$enabled"
  created_link=true
fi
if ! freeradius -XC > /root/nac-ldap-freeradius-xc.log 2>&1; then
  if $created_link; then
    rm -f "$enabled"
  fi
  if $had_target; then
    cp -p "$target.before-starttls" "$target"
  else
    rm -f "$target"
  fi
  echo "FreeRADIUS validation failed; previous module state restored. Inspect /root/nac-ldap-freeradius-xc.log" >&2
  exit 1
fi
echo "FreeRADIUS LDAP StartTLS module validated; active site unchanged"
