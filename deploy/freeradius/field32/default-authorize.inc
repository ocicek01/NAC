if (&Packet-Src-IP-Address == 10.6.8.19 && &NAS-Port == 32) {
	if (!&EAP-Message) {
		reject
	}
	nac_field_eap {
		ok = return
		updated = return
		handled = return
	}
	reject
}
