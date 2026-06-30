# Phase 2 Checklist

## Hedef

- [ ] DHCP event veri akisini calisir hale getirmek
- [ ] `dhcp_events` tablosuna Go uzerinden veri yazmak
- [ ] Gercek packet capture oncesi event pipeline'i dogrulamak

## Repository ve Service

- [x] `dhcp_events` repository arayuzu eklendi
- [x] PostgreSQL repository implementasyonu eklendi
- [x] DHCP event service eklendi
- [x] Event validation mantigi eklendi
- [x] Sample event insert akisi eklendi

## HTTP Test Akisi

- [x] `POST /api/v1/dhcp-events` endpoint'i eklendi
- [x] `POST /internal/dev/dhcp-events/sample` endpoint'i eklendi
- [x] Sample endpoint sunucuda test edildi
- [x] Manuel JSON event insert testi yapildi
- [x] Verinin `dhcp_events` tablosuna dustugu dogrulandi

## Collector Hazirligi

- [x] Collector package yapisi tasarlandi
- [x] DHCP parser akisi eklendi
- [x] Gercek packet capture kutuphanesi secildi
- [x] Linux capture izinleri dokumante edildi
- [x] Interface bazli dinleme konfigurasyonu eklendi
- [ ] Collector sunucuda aktif edilip test edildi
- [ ] Gercek DHCP DISCOVER kaydi alindi
- [ ] Gercek DHCP REQUEST kaydi alindi

## Faz 2 Kapanis Kriterleri

- [x] Ornek event API uzerinden veritabanina yaziliyor
- [x] Manuel event insert akisi calisiyor
- [x] `dhcp_events` pipeline'i dogrulandi
- [ ] Gercek DHCP collector aktif calisiyor
- [x] Kisa zaman penceresinde ayni `MAC + message_type` icin dedup kurali eklendi
- [x] DHCP `transaction_id (xid)` alani modele ve veritabanina eklendi
- [x] Dedup mantigi `MAC + message_type + transaction_id` oncelikli olacak sekilde guclendirildi
