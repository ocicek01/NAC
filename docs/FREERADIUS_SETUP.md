# FreeRADIUS + OpenLDAP secure setup

Bu kurulum managed cihazlarda tek gorunur giris icin FreeRADIUS 3.x,
EAP-TTLS/PAP ve OpenLDAP kullanir. Sophos captive portal BYOD/guest fallback
olarak kalir; managed 802.1X oturumunda ikinci web login uretilmez.

## Guvenlik sinirlari

- Kullanici parolasi yalnizca istemci ile FreeRADIUS arasindaki TLS tunelinde ve
  FreeRADIUS ile LDAPS bind islemi arasinda bulunur.
- NAC API parola almaz, saklamaz veya loglamaz.
- LDAP sunucu sertifikasi kurum CA'si ile zorunlu dogrulanir.
- Istemci profili RADIUS sunucu sertifikasini ve beklenen sunucu adini zorunlu
  dogrulamalidir. Kullaniciya sertifika uyarisini gecme secenegi verilmemelidir.
- NAC HTTP callout sadece `127.0.0.1`/`localhost` hedefini kabul eden helper ile
  yapilir. Bearer token yalnizca HTTP header'inda tasinir.
- API hatasi, timeout veya gecersiz karar erisim icin fail-closed sonuc verir.

## Akislar

### Managed kullanici

1. Switch 802.1X baslatir.
2. FreeRADIUS EAP-TTLS sunucu sertifikasini sunar.
3. Ic tunelde PAP credential LDAPS bind ile gercekten dogrulanir.
4. Basarili LDAP dogrulamasindan sonra helper, parolasiz baglami NAC authorize
   endpointine yollar.
5. NAC VLAN/policy reply attribute'larini dondurur.
6. Start/Interim-Update/Stop accounting kayitlari NAC'a gider.

EAP-TLS sertifika altyapisi hazir oldugunda parola kullanmayan tercih edilen
gelecek secenegidir. OpenLDAP'ta NT hash bulunmadigi durumda PEAP-MSCHAPv2
uygulanabilir bir varsayim degildir ve bu kurulumda ana yontem olarak kullanilmaz.

### MAB

MAC formatindaki identity LDAP'a gonderilmez. `nac_mab_authorize` yalnizca cihaz
politikasini sorar. MAB sonucu insan kullanici kimligi veya Sophos username'i
olarak yorumlanmaz.

## Dosyalar ve kurulum

Drop-in sablonlar [deploy/freeradius/README.md](../deploy/freeradius/README.md)
altindadir. Sablonlardaki tum `@@...@@` degerleri deployment sirasinda
secret store veya root-only dosyadan uretilmelidir.

Helper build:

```bash
cd /opt/nac
go build -o /usr/local/libexec/nac-radius-helper ./cmd/nac-radius-helper
chown root:root /usr/local/libexec/nac-radius-helper
chmod 0755 /usr/local/libexec/nac-radius-helper
```

Ortam dosyasi:

```bash
install -d -m 0750 -o root -g freerad /etc/nac
install -m 0640 -o root -g freerad deploy/freeradius/radius-helper.env.example /etc/nac/radius-helper.env
```

`NAC_RADIUS_API_TOKEN` uzun ve rastgele olmalidir. Degisken bos birakildiginda
eski kurulumlar icin endpointler tokensiz calismaya devam eder; bu yalnizca
gecis uyumlulugudur. Production'da ayni token hem `nac-api.service` hem
FreeRADIUS environment dosyasinda mutlaka tanimli olmalidir.

Sablonlari render ettikten sonra:

```bash
ln -s ../mods-available/nac_ldap /etc/freeradius/3.0/mods-enabled/nac_ldap
ln -s ../mods-available/nac_eap /etc/freeradius/3.0/mods-enabled/nac_eap
ln -s ../mods-available/nac_helper /etc/freeradius/3.0/mods-enabled/nac_helper
ln -s ../sites-available/nac /etc/freeradius/3.0/sites-enabled/nac
ln -s ../sites-available/nac-inner-tunnel /etc/freeradius/3.0/sites-enabled/nac-inner-tunnel
freeradius -XC
```

## Accounting kontrati

Helper su alanlari `/api/v1/radius/accounting` endpointine iletir:

- `username`, `mac_address`, `nas_ip_address`, `nas_identifier`
- `nas_port`, `nas_port_id`, `nas_port_type`
- `acct_status_type`, `acct_session_id`, `framed_ip_address`
- `session_time`, `terminate_cause`
- `event_timestamp`, `acct_delay_time`

Accounting kaydi daha sonraki Sophos RADIUS SSO relay icin kaynak olacaktir.
Bu faz Sophos'a veri gondermez.

## Dogrulama

```bash
go test ./cmd/nac-radius-helper
go build ./cmd/nac-radius-helper
freeradius -XC
```

Saha testinde sirayla MAB bilinmeyen cihaz, gecersiz LDAP parolasi, gecerli
TTLS/PAP kullanicisi, LDAP kapali senaryo ve Start/Interim/Stop accounting
paketleri kontrol edilmelidir. LDAP veya NAC kapaliyken managed kullaniciya
Access-Accept donmemelidir.
