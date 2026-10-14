if (&Packet-Src-IP-Address == 10.6.8.19 && &NAS-Port == 32) {
	if (&EAP-Message || !&User-Name || !&Calling-Station-Id) {
		reject
	}
	if (&User-Name !~ /^(309c239b97aa|309c23-9b97aa|30:9c:23:9b:97:aa|30-9c-23-9b-97-aa)$/i) {
		reject
	}
	if (&Calling-Station-Id !~ /^(309c239b97aa|309c23-9b97aa|30:9c:23:9b:97:aa|30-9c-23-9b-97-aa)$/i) {
		reject
	}
	update request {
		User-Name := "30:9C:23:9B:97:AA"
		Calling-Station-Id := "30:9C:23:9B:97:AA"
	}
	nac_mab_field
	if (ok || updated) {
		update control {
			Auth-Type := Accept
		}
		return
	}
	reject
}
