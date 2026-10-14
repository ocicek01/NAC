# Port 32 field test

Scope: HP 2530 at 10.6.8.19, NAS-Port 32, endpoint MAC
30:9C:23:9B:97:AA. The default authorize snippet sends only that source/port
to EAP-TTLS; the existing RADIUS flow remains available to other requests.
The inner server uses `nac_field_ldap`, rendered for a temporary LDAP OU.
No production LDAP user password is included in these files.

Preflight on 127.0.0.1:18123 must pass certificate-verified TTLS/PAP with the
correct password and reject the wrong password. `freeradius -XC` alone does
not prove a successful TLS or LDAP exchange.

Run `windows/Prepare-Windows.ps1` as administrator on the matching Windows
test NIC. The script exports the original LAN profile and service state to
`%ProgramData%\NAC-Field32-Backup`. Use `-Rollback` to restore them. The profile
checks `radius.hakkari.edu.tr` and ISRG Root X1 and refuses certificate errors.
Credentials are entered through Windows and are not part of the profile.

For this session, the server keeps temporary credentials in the root-only
`/root/nac-field32/credentials.txt`. A transient systemd timer invokes
`/root/nac-field32/cleanup.sh` after 60 minutes. That cleanup removes the
temporary account, restores the RADIUS default site, and rolls back switch
authentication if it was activated. Verify that the timer is active before
changing the switch. Switch changes must not be saved to startup-config.

Protocol references:
- [Microsoft TTLS schema](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpwl/7fda6c4b-0347-466c-926f-0e7e45a0aa7a)
- [HPE 2530 authenticator port configuration](https://arubanetworking.hpe.com/techdocs/AOS-S/16.11/ASG/YAYB/content/common%20files/ena-sel-por-as-aut-ena-def-por-bas-aut.htm)
