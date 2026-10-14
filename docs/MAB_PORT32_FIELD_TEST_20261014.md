# MAB live field test - 14 October 2026

Scope: HP J9775A 2530-48G, 10.6.8.19, physical port/NAS-Port 32,
endpoint 30:9C:23:9B:97:AA. Windows 11 required no supplicant profile or agent.
Times below are Europe/Istanbul.

## Verified result

- At 14:24:49 the switch sent an actual RADIUS Access-Request for port32.
- FreeRADIUS returned Access-Accept with Tunnel-Type VLAN,
  Tunnel-Medium-Type 802, and Tunnel-Private-Group-ID 333.
- Switch output showed `Authentication Type: mac-based`,
  `Client Status: authenticated`, `Untagged VLAN: 333`.
- The switch MAC table showed 309c23-9b97aa on VLAN333.
- The NAC authorize event was processed with decision accept and VLAN333.
- Accounting Start and the Stop generated during rollback were both processed;
  the session type was mab and the final session status was stopped.

Evidence identifiers:

| Evidence | Identifier |
| --- | --- |
| Authorize | 2b94a055-a8cf-4ccb-86e4-0d90bb219268 |
| Accounting Start | 52b3a82b-e777-8702-b1a4-674faf0919fe |
| Accounting Stop, 14:25:51 | 375fc1e4-a804-88fd-8931-86cad4d508e7 |
| NAC session | f16b5afd-f83b-41b6-ada6-10eaa1389931 |
| Switch accounting session | 080E00000003 |

Server evidence is root-only under `/root/nac-mab32`: `proof.pcap`,
`database-success.txt`, `database-final.txt`, and switch status/rollback
transcripts. The original switch configuration is saved separately at
`/root/nac-switch-10.6.8.19-before-8021x.txt`.

## Corrections found during testing

The HP sends Calling-Station-Id as 30-9c-23-9b-97-aa. The field allowlist now
accepts that form and normalizes it to the canonical MAC before the helper.

FreeRADIUS Event-Timestamp string expansion did not match the API's RFC3339
or Unix-seconds contract. The field accounting path now passes integer Unix
seconds when the attribute exists and an empty value when it does not.

Device repository Upsert returned a stale 59-column projection while the shared
scanner required 82 destinations. Reusing `deviceSelectColumns` fixed the
inventory error. Commit 9f66c3a was deployed. `go test ./...` passed; a real
PostgreSQL integration test passed insert, conflict update, readback and empty
management-IP handling in 0.07s, then removed its temporary device.
GET devices with limit=1 returned HTTP200 in approximately 14ms after deployment.
Failed events from the first diagnostic attempts remain as historical evidence.

## Final state and limits

MAC authentication on port32 was removed, the RADIUS default site was restored,
and the automatic rollback timer was stopped after manual cleanup. Switch
changes were not saved to startup-config. Other ports and clients.conf were
unchanged. The endpoint returned to its original untagged VLAN106.

This proves MAB authorization, actual switch VLAN enforcement, and accounting
lifecycle. It does not prove DHCP connectivity on VLAN333, Sophos login or
internet username mapping. The canonical MAB identity is present in
radius_sessions.session_type; the legacy devices.authentication_method/status
fields were empty during the test and should not be used as its evidence.
The field snippets contain a single MAC allowlist and are test material, not
a general production rollout or a new panel control.
