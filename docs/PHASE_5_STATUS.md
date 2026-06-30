# Phase 5 Status

## Tamamlananlar

- `devices` modeli current-state alanlariyla genisletildi
- Device repository/service katmani eklendi
- `mac_observations` olustuktan sonra device inventory upsert akisi eklendi
- `GET /api/v1/devices` endpoint'i eklendi
- Device inventory Faz 6 Laravel paneli icin backend tabanina baglandi
- Sunucuda migration uygulandi
- `GET /api/v1/devices` ile canli current-state dogrulamasi yapildi
- Canli cihaz icin `current_switch_name=sw-10-6-8-11` ve `current_interface_name=GigabitEthernet0/0/25` goruldu

## Teknik Borclar

- `last_seen_at` degisiminin birden fazla DHCP dongusu boyunca izlendiginden emin olunmali
- `hostname` ve `vendor_class` alanlarinin dolu cihazlarda merge davranisi daha genis test edilmeli
- `status` alani su an `active/observed/unresolved` seviyesinde; Faz 7 policy/classification ile derinlestirilmeli
- `current_port_id` ileride port envanteri/topoloji ile normalize edilmeli
