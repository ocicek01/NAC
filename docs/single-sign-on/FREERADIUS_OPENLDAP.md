# Single-login FreeRADIUS and OpenLDAP design

## Authentication path

Managed endpoints use `802.1X -> EAP-TTLS/PAP -> FreeRADIUS -> LDAPS bind`.
The PAP credential exists only inside the server-authenticated TLS tunnel and is
consumed by the LDAP module. NAC receives username, MAC, switch and port data,
but never receives or stores the password.

Clients must validate the RADIUS server certificate against the institution CA.
FreeRADIUS must validate the LDAP server certificate with `require_cert = demand`.
Do not use IP-address LDAPS unless that IP is present in the certificate SAN.

EAP-TLS is the preferred future managed-device method because it replaces user
passwords with device/user certificates. PEAP-MSCHAPv2 is not a safe default for
this deployment: it normally requires an NT-compatible password hash, which a
standard OpenLDAP password store may not expose.

## MAB path

MAB is device authorization, not user authentication. A MAC-formatted identity
is detected before LDAP, sent to `nac_mab_authorize`, and restricted by NAC
policy. It must not create a human user session or be relayed as a Sophos user.

## Accounting and later Sophos relay

Start, Interim-Update and Stop records are posted to NAC with event occurrence
time and accounting delay. NAC's durable session record is the source for a
later Sophos RADIUS SSO relay. This phase intentionally contains no Sophos code.

## Failure policy

- LDAP certificate, bind or timeout failure rejects managed-user access.
- NAC authorize timeout, malformed response or missing decision rejects access.
- MAB does not fall through to LDAP.
- Accounting delivery failure is visible as a FreeRADIUS module failure and the
  local `detail` record remains available for replay.
- No configuration contains a universal authentication-success handler.
