# NAC

Bu repo NAC platformunun Go backend ve Laravel panel uygulamasini icerir.

## Bilesenler

- Go NAC API
- Migration araci
- NAC helper komutlari
- Laravel panel (`panel-app`)
- SQL migration dosyalari
- Faz bazli teknik dokumanlar

## Klasor Yapisi

- `cmd/api`: Go API giris noktasi
- `cmd/migrate`: migration araci
- `internal`: backend domain, repository ve servis katmanlari
- `migrations`: SQL migration dosyalari
- `panel-app`: Laravel panel uygulamasi
- `deploy`: local/deploy altyapisi
- `docs`: faz bazli notlar ve kurulum dokumanlari

## Baslangic

1. `env.example` dosyasini `.env` olarak kopyalayin
2. PostgreSQL ve Redis servislerini hazirlayin
3. Migrationlari calistirin
4. Go API'yi derleyip baslatin
5. Gerekirse `panel-app` icin ayri `.env` tanimlayin

## Notlar

- DHCP, RADIUS, SNMP trap, enforcement ve discovery akislarinin kodu bu repo icindedir.
- Panel tarafinda switch detay, endpoint, NAC aksiyonlari ve VLAN tasima ekranlari bulunur.

## Ortam Degiskenleri

Go API icin trap forwarding ayarlari:

- SNMP_TRAP_ENABLED
- SNMP_TRAP_BIND_HOST
- SNMP_TRAP_PORT
- SNMP_TRAP_FORWARD_ENABLED
- SNMP_TRAP_FORWARD_URL
- SNMP_TRAP_FORWARD_TOKEN
- SNMP_TRAP_FORWARD_TIMEOUT_SECONDS


