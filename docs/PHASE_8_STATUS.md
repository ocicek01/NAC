# Phase 8 Status

## Tamamlananlar

- Dry-run enforcement karar modeli eklendi
- Policy sonucundan `decision_action` turetiliyor
- `enforcement_decisions` tablosuna kayit akisi baglandi
- Read-only enforcement API eklendi
- Panelde `/enforcement` ekran omurgasi eklendi
- Enforcement selector iskeleti eklendi: `radius-vlan -> coa -> ssh -> snmp-write` fallback zinciri capability bazli seciliyor
- Canli dogrulama yapildi:
  - `LAB3` icin `active -> allow`
  - dry-run kaydi panelde goruldu
- Approval / queue / retry alanlari ve aksiyonlari eklendi

## Sonraki Adimlar

- `000013_create_enforcement_decisions.sql` ve `000014_alter_enforcement_decisions_for_queue.sql` migration'larini sunucuda uygulamak
- `000022_add_switch_enforcement_capabilities.sql` ve `000023_add_method_selection_to_enforcement_decisions.sql` migration'larini sunucuda uygulamak
- `blocked / guest / unknown` policy aksiyonlarini canli test etmek
- Approval ve retry gecislerini panelden dogrulamak
- Sonraki iterasyonda queue worker ve gercek enforcement akisini planlamak

## Acik Test Borclari

- `blocked` policy yolunun canli ortamda urettigi `awaiting-approval -> approve/reject` akisi henuz dogrulanmadi
- `guest` policy yolunun canli ortamda urettigi `awaiting-approval` akisi henuz dogrulanmadi
- `unknown` policy yolunun canli ortamda urettigi `queued / retry` akisi henuz dogrulanmadi
