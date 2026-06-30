# Phase 1 Checklist

## Repo ve Iskelet

- [x] Yeni urun klasoru `nac` olusturuldu
- [x] Go repo iskeleti kuruldu
- [x] Temel klasor yapisi olusturuldu
- [x] `cmd/api` giris noktasi eklendi
- [x] `internal/app` bootstrap yapisi eklendi
- [x] `internal/config` katmani eklendi
- [x] `internal/logging` katmani eklendi
- [x] `internal/database` katmani eklendi
- [x] `internal/httpserver` katmani eklendi

## Konfigurasyon

- [x] `env.example` olusturuldu
- [x] App config alanlari tanimlandi
- [x] PostgreSQL config alanlari tanimlandi
- [x] Redis config alanlari tanimlandi
- [x] Log level config alani tanimlandi
- [x] `.env` dosyasi olusturuldu

## Altyapi

- [x] PostgreSQL icin docker compose tanimi eklendi
- [x] Redis icin docker compose tanimi eklendi
- [x] Healthcheck endpoint eklendi
- [x] Local servisler ayaga kaldirildi
- [x] Uygulama runtime testi yapildi

## Veritabani

- [x] Migration klasoru olusturuldu
- [x] `devices` migration dosyasi eklendi
- [x] `switches` migration dosyasi eklendi
- [x] `dhcp_events` migration dosyasi eklendi
- [x] Migration araci secildi
- [x] Migration'lar veritabanina uygulandi

## Domain Modelleri

- [x] `device` domain modeli eklendi
- [x] `switchasset` domain modeli eklendi
- [x] `dhcpevent` domain modeli eklendi
- [x] `auditlog` domain modeli eklendi
- [x] `ports` domain modeli eklendi
- [x] `topologies` domain modeli eklendi
- [x] `policies` domain modeli eklendi
- [x] `sessions` domain modeli eklendi

## Teknik Dogrulama

- [x] Go dosyalari formatlandi
- [x] Go mod bagimliliklari indirildi
- [x] Uygulama derleme testi yapildi
- [x] PostgreSQL baglantisi test edildi
- [x] Healthcheck endpoint dogrulandi

## Faz 1 Kapanis Kriterleri

- [x] Uygulama localde aciliyor
- [x] `/healthz` 200 donuyor
- [x] PostgreSQL baglantisi basarili
- [x] Ilk migration seti uygulanmis
- [x] Faz 2 icin temiz backend tabani hazir
