# Phase 5 Checklist

## Hedef

- [x] Device inventory modelini current-state mantigina getirmek
- [x] DHCP + observation akisini `devices` tablosuna baglamak
- [x] Cihazin son switch/port bilgisini API uzerinden okunur yapmak

## Device Inventory

- [x] `devices` domain modeli current-state alanlariyla genisletildi
- [x] Device repository arayuzu eklendi
- [x] PostgreSQL device repository implementasyonu eklendi
- [x] Device service eklendi
- [x] Korelasyon sonrasi `devices` upsert akisi eklendi
- [x] `GET /api/v1/devices` endpoint'i eklendi
- [x] Sunucuda migration uygulandi
- [x] Sunucuda device inventory akisi canli test edildi

## Faz 5 Kapanis Kriterleri

- [x] Device inventory current-state omurgasi kodlandi
- [x] Cihazin ilk/son gorulme ve son switch/port durumu sunucuda dogrulandi
- [x] Device inventory verisi Laravel paneli icin hazir hale geldi

## Teknik Borclar

- [ ] `last_seen_at` alaninin ayni MAC tekrarlari boyunca ilerlemesi daha uzun canli test ile dogrulanmali
- [ ] `hostname` ve `vendor_class` merge davranisi hostname dolu cihazlarla ek dogrulanmali
- [ ] `status` alani su an basit kurallarla uretiliyor; policy/classification katmaninda zenginlestirilmeli
- [ ] `current_port_id` su an bos; ileride ayrik port envanteri ile baglanmali
