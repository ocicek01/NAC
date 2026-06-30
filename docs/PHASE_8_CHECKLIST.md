# Phase 8 Checklist

## Hedef

- [x] Enforcement icin dry-run karar modeli olusturmak
- [x] Policy sonucu sonrasi onerilen aksiyonu kaydetmek
- [x] Panelde enforcement dry-run ekranini canli dogrulamak

## Dry-Run Omurgasi

- [x] `enforcement_decisions` domain modeli eklendi
- [x] PostgreSQL repository eklendi
- [x] Dry-run enforcement service eklendi
- [x] Device inventory upsert sonrasi enforcement kaydi baglandi
- [x] `GET /api/v1/enforcement-decisions` endpoint'i eklendi
- [x] Approval / queue / retry omurgasi eklendi
- [ ] Sunucuda migration uygulandi
- [ ] Dry-run ve queue kararlar canli veride dogrulandi

## Panel Entegrasyonu

- [x] `/enforcement` sayfasi eklendi
- [x] Dry-run kararlar panelde goruldu
- [x] Approval / retry aksiyonlari panelde gosterildi

## Teknik Borclar

- [ ] Gercek switch port enforcement henuz yok
- [ ] Queue worker / scheduled processor henuz yok
- [ ] Approval yetkilendirmesi / RBAC henuz yok
- [ ] Dry-run / live mode toggle henuz yok
