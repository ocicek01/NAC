# Phase 6 Checklist

## Hedef

- [x] Laravel panel iskeletini kurmak
- [x] Go API verilerini read-only panelde gostermek
- [x] Faz 7 oncesi temel operasyon ekranlarini hazirlamak

## Panel Iskeleti

- [x] `panel-app` Laravel iskeleti kuruldu
- [x] NAC API istemcisi eklendi
- [x] Dashboard controller eklendi
- [x] Inventory controller eklendi
- [x] `GET /`, `/devices`, `/switches`, `/observations`, `/topology` rota seti eklendi
- [x] Temel Blade layout ve sayfalar eklendi
- [x] `NAC_API_URL` panel konfigurasyonuna eklendi
- [x] Panel yerelde/sunucuda HTTP olarak calistirildi
- [x] Read-only ekranlar canli backend verisi ile dogrulandi

## Faz 6 Kapanis Kriterleri

- [x] Laravel panel omurgasi kodlandi
- [x] Dashboard backend verisi ile calisiyor
- [x] Devices, Switches, Observations, Topology ekranlari canli veride dogrulandi
- [x] Faz 7 policy UI icin panel zemini hazirlandi

## Teknik Borclar

- [ ] Kimlik dogrulama ve RBAC henuz eklenmedi
- [ ] Vite / asset pipeline henuz kullanilmiyor
- [ ] Blade ekranlari su an read-only; CRUD ve filtreleme sonraki iterasyonda eklenmeli
- [ ] `kutuphane-*` alias dagitiminda operasyonel cakisma riski suruyor; topology eslestirme verisi sahada daraltilmali
