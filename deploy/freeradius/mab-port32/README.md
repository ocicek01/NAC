# MAB field test on port 32

These snippets are restricted to source 10.6.8.19, NAS-Port 32 and MAC
30:9C:23:9B:97:AA. The authorize path only accepts the matching MAC identity
after the NAC helper returns success. An API failure or a reject cannot fall
through into the legacy authorize path. Accounting goes to the existing NAC
accounting API and identifies the session as MAC authentication.

Before testing, save the RADIUS default site and switch running configuration.
Install the snippets at the beginning of the default authorize/accounting
sections. Do not change `clients.conf` or other switch ports. Validate with
`freeradius -XC` and schedule automatic rollback before enabling:

```
aaa port-access mac-based 32
```

Observe the actual RADIUS exchange and `show port-access clients 32 detail`.
The NAC decision for this unknown endpoint is quarantine VLAN 333. The test
may temporarily interrupt the endpoint's connectivity. Remove MAC-based
authentication after the test and verify that the endpoint returns to VLAN106:

```
no aaa port-access mac-based 32
```

No Windows supplicant configuration is used. MAC identity is device identity;
Sophos Captive Portal continues to authenticate the human user.

[HPE 2530 MAC authentication command reference](https://arubanetworking.hpe.com/techdocs/AOS-S/16.10/ASG/YAYB/content/common%20files/cnf-swi-sup-rad-ass-acl.htm)
