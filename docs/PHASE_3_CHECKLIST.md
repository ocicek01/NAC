# Phase 3 Checklist

## Hedef

- [x] Switch inventory yapisini SNMP read-only korelasyon icin hazir hale getirmek
- [x] SNMP ile `MAC -> switch/port` lookup akisini kurmak
- [x] Faz 3 icin ilk read-only API katmanini acmak

## Switch Inventory

- [x] `switches` tablosu SNMP alanlari ile genisletildi
- [x] `switchasset` repository arayuzu eklendi
- [x] PostgreSQL switch repository implementasyonu eklendi
- [x] Switch create/list service eklendi
- [x] `POST /api/v1/switches` endpoint'i eklendi
- [x] `GET /api/v1/switches` endpoint'i eklendi
- [x] `POST /api/v1/switches/bulk` endpoint'i eklendi

## SNMP Korelasyon

- [x] Read-only SNMP client eklendi
- [x] `dot1dTpFdbPort` tabanli MAC lookup akisi eklendi
- [x] `bridgePort -> ifIndex` cozumlemesi eklendi
- [x] `ifName` ve `ifDescr` sorgulari eklendi
- [x] `POST /api/v1/mac-lookups` endpoint'i eklendi
- [x] Sunucuda en az bir switch envantere eklendi
- [x] Canli SNMP lookup testi yapildi
- [x] Bir MAC icin switch/port sonucu dogrulandi
- [x] `mac_observations` tablosu tasarlandi
- [x] DHCP ingest sonrasi otomatik MAC korelasyon hook'u eklendi
- [x] `GET /api/v1/mac-observations` endpoint'i eklendi
- [x] Otomatik korelasyon ile ilk observation kaydi olustu

## Faz 3 Kapanis Kriterleri

- [x] Read-only SNMP korelasyon iskeleti kodlandi
- [x] Switch inventory API sunucuda test edildi
- [x] SNMP lookup API sunucuda test edildi
- [x] `DHCP event -> MAC -> switch/port` akisi ilk kez dogrulandi
