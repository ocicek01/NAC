#!/usr/bin/env bash
set -euo pipefail

if [[ $(id -u) -ne 0 || $# -ne 2 ]]; then
  echo "usage: sudo bash $0 <test-username> <root-only-password-file>" >&2
  exit 2
fi

username=$1
password_file=$2
if [[ ! $username =~ ^[A-Za-z0-9._@-]+$ ]]; then
  echo "invalid test username" >&2
  exit 2
fi
if [[ ! -f $password_file || $(stat -c %u "$password_file") -ne 0 || $(stat -c %a "$password_file") != 600 ]]; then
  echo "password file must be a root-owned regular file with mode 600" >&2
  exit 2
fi
password=$(<"$password_file")
if [[ -z $password || $password == *'"'* || $password == *'\'* || $password == *$'\n'* || $password == *$'\r'* ]]; then
  echo "test password contains unsupported characters" >&2
  exit 2
fi

site=/etc/freeradius/3.0/sites-enabled/nac-ldap-test
if [[ -e $site || -L $site ]]; then
  echo "test site is already enabled; refusing to alter it" >&2
  exit 2
fi
if ss -lun | grep -q '127.0.0.1:18121'; then
  echo "loopback test port is occupied" >&2
  exit 2
fi

secret_file=$(mktemp)
request_file=$(mktemp)
chmod 0600 "$secret_file" "$request_file"
cleanup() {
  rm -f "$secret_file" "$request_file"
  rm -f "$site"
  systemctl restart freeradius
}
trap cleanup EXIT

awk '
  /^client localhost[[:space:]]*\{/ { in_client=1; next }
  in_client && /^\}/ { exit }
  in_client && /^[[:space:]]*secret[[:space:]]*=/ {
    sub(/^[^=]*=[[:space:]]*/, "")
    sub(/[[:space:]]*#.*/, "")
    gsub(/"/, "")
    sub(/^[[:space:]]+/, "")
    sub(/[[:space:]]+$/, "")
    print
    exit
  }
' /etc/freeradius/3.0/clients.conf > "$secret_file"
test -s "$secret_file"

ln -s /etc/freeradius/3.0/sites-available/nac-ldap-test "$site"
freeradius -XC > /root/nac-ldap-test-xc.log 2>&1
systemctl restart freeradius
systemctl is-active --quiet freeradius

check_reply() {
  local label=$1 expected=$2 user=$3 pass=${4-} reply
  printf 'User-Name = "%s"\n' "$user" > "$request_file"
  if [[ $# -eq 4 ]]; then
    printf 'User-Password = "%s"\n' "$pass" >> "$request_file"
  fi
  reply=$(radclient -S "$secret_file" -f "$request_file" -t 3 -r 1 127.0.0.1:18121 auth 2>&1) || true
  if [[ $reply != *"$expected"* ]]; then
    echo "$label: unexpected RADIUS reply" >&2
    return 1
  fi
  echo "$label: $expected"
}

check_reply correct_password Access-Accept "$username" "$password"
check_reply wrong_password Access-Reject "$username" "${password}incorrect"
check_reply missing_password Access-Reject "$username"
check_reply nonexistent_user Access-Reject "nac-no-such-user-20261014" "$password"
