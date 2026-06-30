# Phase 4 Checklist

## Hedef

- [ ] Topoloji link modelini veritabaninda kalici hale getirmek
- [ ] Manual topology link API'sini acmak
- [ ] Access-port secimini topology-aware hale getirmek

## Topoloji Veri Modeli

- [x] `topology_links` migration dosyasi eklendi
- [x] `topology` repository arayuzu eklendi
- [x] PostgreSQL topology repository implementasyonu eklendi
- [x] Topology service eklendi
- [x] `POST /api/v1/topology-links` endpoint'i eklendi
- [x] `GET /api/v1/topology-links` endpoint'i eklendi
- [x] `POST /api/v1/topology-links/discover` endpoint'i eklendi
- [x] `POST /api/v1/switches/identity` endpoint'i eklendi

## Topology-Aware Korelasyon

- [x] Korelasyon servisine topology link checker baglandi
- [x] Bilinen uplink portlari scoring'de cezalandiriliyor
- [x] LLDP tabanli otomatik topology discovery iskeleti eklendi
- [x] `system_name` ve `aliases` ile switch identity eslestirme katmani eklendi
- [x] Sunucuda ilk topology link kaydi olusturuldu
- [x] LLDP discovery sunucuda canli test edildi
- [x] Alias cakismasi icin ambiguity ve self-link korumasi eklendi
- [ ] Topology-aware secim canli test edildi

## Faz 4 Kapanis Kriterleri

- [x] Topoloji motoru icin ilk persistence ve API iskeleti kodlandi
- [x] Topology link API sunucuda test edildi
- [ ] Korelasyon secimi topology verisi ile dogrulandi
