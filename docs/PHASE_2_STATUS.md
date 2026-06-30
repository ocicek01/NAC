# Phase 2 Status

## Tamamlananlar

- `dhcp_events` repository arayuzu eklendi
- PostgreSQL repository implementasyonu eklendi
- DHCP event service eklendi
- Validation ve sample insert mantigi eklendi
- HTTP test endpoint'leri eklendi
- Gopacket/pcap tabanli DHCP collector eklendi
- Collector icin env tabanli interface ve enable konfigurasyonu eklendi
- Sample endpoint sunucuda dogrulandi
- Manuel JSON event insert sunucuda dogrulandi
- `dhcp_events` tablosuna kayit dusumu dogrulandi
- Ayni `MAC + message_type` icin kisa zaman pencereli dedup mantigi eklendi
- Relay-aware dedup mantigi ile `0.0.0.0` kayitlari relay source IP lehine guncellenecek sekilde iyilestirildi
- DHCP `transaction_id (xid)` parse edilip event modeline eklendi
- Dedup mantigi `transaction_id` oncelikli olacak sekilde guclendirildi

## Sonraki Adimlar

- Collector'u sunucuda aktif edip test etmek
- Gercek DHCP capture ile helper address akisini dogrulamak
