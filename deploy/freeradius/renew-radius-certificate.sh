#!/bin/sh
set -eu

lineage=${RENEWED_LINEAGE:-/etc/letsencrypt/live/radius.hakkari.edu.tr}
if [ "$lineage" != /etc/letsencrypt/live/radius.hakkari.edu.tr ]; then
    exit 0
fi

destination=/etc/freeradius/3.0/certs/letsencrypt
test -s "$lineage/fullchain.pem"
test -s "$lineage/privkey.pem"
openssl x509 -in "$lineage/fullchain.pem" -noout -checkend 86400 >/dev/null
certificate_key=$(openssl x509 -in "$lineage/fullchain.pem" -noout -pubkey | openssl pkey -pubin -outform DER | openssl dgst -sha256)
private_key=$(openssl pkey -in "$lineage/privkey.pem" -pubout -outform DER | openssl dgst -sha256)
test "$certificate_key" = "$private_key"

install -d -o root -g freerad -m 0750 "$destination"
install -o root -g freerad -m 0640 "$lineage/privkey.pem" "$destination/privkey.pem.new"
install -o root -g freerad -m 0644 "$lineage/fullchain.pem" "$destination/fullchain.pem.new"
mv -f "$destination/privkey.pem.new" "$destination/privkey.pem"
mv -f "$destination/fullchain.pem.new" "$destination/fullchain.pem"

freeradius -XC >/dev/null 2>&1
systemctl restart freeradius.service
systemctl is-active --quiet freeradius.service
