# Phase 1 Status

## Tamamlananlar

- Yeni urun klasoru `nac` olusturuldu
- Go backend iskeleti kuruldu
- Config ve logger yapisi eklendi
- PostgreSQL baglantisi eklendi
- Healthcheck endpoint eklendi
- Ilk domain modelleri eklendi
- Ilk migration dosyalari eklendi
- Docker compose ile PostgreSQL ve Redis tanimlandi
- Go kurulup uygulama derlendi
- `.env` olusturulup uygulama calistirildi
- `/healthz` endpoint'i dogrulandi
- Ilk migration seti `nac` veritabanina uygulandi

## Sonraki Adimlar

- Faz 2 DHCP collector tasarimina gecmek
- Event modeli ile collector akisini netlestirmek
- `dhcp_events` tablosuna ilk gercek veri akisini yazmak
