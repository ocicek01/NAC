# Phase 3 Status

## Tamamlananlar

- `switches` tablosu SNMP alanlari ile genisletildi
- Switch inventory icin repository ve service katmani eklendi
- `POST /api/v1/switches` ve `GET /api/v1/switches` endpoint'leri eklendi
- `POST /api/v1/switches/bulk` endpoint'i eklendi
- `gosnmp` tabanli read-only SNMP client eklendi
- BRIDGE-MIB ve IF-MIB uzerinden `MAC -> bridgePort -> ifIndex -> interface` akisi eklendi
- `POST /api/v1/mac-lookups` endpoint'i eklendi
- `mac_observations` tablosu ve repository katmani eklendi
- DHCP event sonrasinda arka planda otomatik SNMP lookup yapan korelasyon hook'u eklendi
- `GET /api/v1/mac-observations` endpoint'i eklendi
- Switch inventory sunucuda canli test edildi
- Core switch ve access switch'ler envantere eklendi
- Canli SNMP lookup ile ayni MAC'in birden fazla switchte gorundugu dogrulandi
- Ilk access-port secici eklendi ve core yerine `sw-10-6-8-11 / GigabitEthernet0/0/25` secildi
- `DHCP event -> MAC -> best access match -> mac_observation` akisi canli ortamda dogrulandi

## Sonraki Adimlar

- Topoloji modelini eklemek
- Heuristik access-port secimini topology-aware hale getirmek
- Device inventory ile observation katmanini birlestirmek
