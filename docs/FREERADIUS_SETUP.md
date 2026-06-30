# FreeRADIUS Setup

Bu dokuman NAC ile FreeRADIUS arasindaki calisan entegrasyonun mevcut durumunu ozetler.

## Tamamlanan omurga

- `POST /api/v1/radius/authorize` endpointi aktif
- `POST /api/v1/radius/accounting` endpointi aktif
- `cmd/nac-radius-helper` authorize kararlarini FreeRADIUS `exec` modulu icin reply pair formatinda uretiyor
- `cmd/nac-radius-clients` NAC envanterindeki switchlerden `clients.conf` snippet uretiyor
- `radius_events` audit tablosu authorize ve accounting olaylarini sakliyor
- Per-switch `radius_secret` alani `switches` tablosuna eklendi
- Cozulen switch/port bilgisi `mac_observations` ve `devices` envanterine `radius / authoritative` olarak yaziliyor

## NAC endpointleri

### Authorize

`POST /api/v1/radius/authorize`

Ornek istek:

```json
{
  "username": "00-11-22-33-44-55",
  "mac_address": "00:11:22:33:44:55",
  "hostname": "PC-LAB-01",
  "vendor_class": "MSFT 5.0",
  "nas_ip_address": "10.6.8.4",
  "nas_identifier": "sw-10-6-8-4",
  "nas_port": "38",
  "nas_port_id": "GigabitEthernet1/0/38",
  "called_station_id": "",
  "calling_station_id": "00-11-22-33-44-55"
}
```

Ornek yanit:

```json
{
  "decision": "accept",
  "policy_action": "unknown",
  "policy_reason": "RADIUS MAB fallback for missing hostname",
  "reply_message": "Quarantine access granted",
  "vlan_id": "999",
  "reply_attributes": {
    "Reply-Message": "Quarantine access granted",
    "Tunnel-Type": "VLAN",
    "Tunnel-Medium-Type": "IEEE-802",
    "Tunnel-Private-Group-Id": "999"
  },
  "control_attributes": {}
}
```

### Accounting

`POST /api/v1/radius/accounting`

Ornek istek:

```json
{
  "username": "00-11-22-33-44-55",
  "mac_address": "00:11:22:33:44:55",
  "nas_ip_address": "10.6.8.4",
  "nas_identifier": "sw-10-6-8-4",
  "nas_port": "38",
  "nas_port_id": "GigabitEthernet1/0/38",
  "nas_port_type": "Ethernet",
  "called_station_id": "",
  "calling_station_id": "00-11-22-33-44-55",
  "acct_status_type": "Start",
  "acct_session_id": "abc123",
  "framed_ip_address": "10.10.10.25",
  "session_time": "0",
  "terminate_cause": ""
}
```

Ornek yanit:

```json
{
  "status": "ok"
}
```

## Policy mapping

- `blocked` -> `reject`
- `guest` -> `accept + guest VLAN`
- `unknown` -> `accept + quarantine VLAN` varsa, yoksa `reject`
- `observed` -> `accept`
- `active` -> `accept`

Not:

- MAB akisinda hostname cogu zaman bos geldigi icin `Hostname Missing -> observed` sonucu authorize servisinde `unknown`a dusurulur.
- Bu sayede quarantine VLAN yolu bos hostname yuzunden bypass edilmez.

## Ortam degiskenleri

NAC API sunucusunda:

- `RADIUS_GUEST_VLAN`
- `RADIUS_QUARANTINE_VLAN`

## Helper binary

`cmd/nac-radius-helper`

Bu helper NAC API endpointine istek atar ve reply attribute satirlari uretir.

Ornek:

```bash
export NAC_RADIUS_API_URL="http://127.0.0.1:8080/api/v1/radius/authorize"
./nac-radius-helper \
  --mac "00:11:22:33:44:55" \
  --hostname "PC-LAB-01" \
  --nas-ip "10.6.8.4" \
  --nas-identifier "sw-10-6-8-4" \
  --nas-port "38" \
  --nas-port-id "GigabitEthernet1/0/38" \
  --calling-station-id "00-11-22-33-44-55"
```

Ornek cikti:

```text
Reply-Message := "Access granted"
```

## Client generator

`cmd/nac-radius-clients`

Bu arac `switches` tablosundaki switchlerden FreeRADIUS `client {}` bloklari uretir.

Kurallar:

- `switches.radius_secret` doluysa o secret kullanilir
- bos ise `--secret` ile verilen fallback kullanilir
- `--require-switch-secret` verilirse fallback'e izin verilmez

Mevcut switch kayitlari icin secret guncelleme endpointi:

`POST /api/v1/switches/radius-secret`

```json
{
  "id": "switch-uuid",
  "radius_secret": "super-secret-value"
}
```

Ornek:

```bash
set -a
source .env
set +a
./bin/nac-radius-clients --secret 'testing123' --output /etc/freeradius/3.0/clients.d/nac-switches.conf
```

`clients.conf` icine include:

```conf
$INCLUDE clients.d/nac-switches.conf
```

## FreeRADIUS `exec` modu

Bu repoda ilk asamada `exec` tabanli entegrasyon calistirildi.

- `mods-available/nac-helper`
- `mods-enabled/nac-helper`
- `sites-enabled/default` icinde `authorize { ... nac_helper ... }`
- `authenticate { Auth-Type Accept { ok } }`

## Canli dogrulanan senaryolar

- `LAB-PC-01` benzeri known hostname -> `Access-Accept`
- Bos hostname MAB cihazi -> `Access-Accept + VLAN 999`
- Gecici blocked test -> `Access-Reject`
- `radius_events` tablosuna authorize audit kaydi yazildi

## Accounting audit

`radius_events` artik iki tur olay tutar:

- `event_type = authorize`
- `event_type = accounting`

Accounting icin saklanan ek alanlar:

- `username`
- `nas_port_type`
- `acct_status_type`
- `acct_session_id`
- `framed_ip_address`
- `session_time`
- `terminate_cause`

## Sonraki isler

- Per-switch secret rotasyonu ve panel/API yonetimi
- FreeRADIUS `rlm_rest` varyantini eklemek
- CoA ve tam accounting/session korelasyonunu baglamak
