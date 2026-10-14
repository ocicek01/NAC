if (&Packet-Src-IP-Address == 10.6.8.19 && &NAS-Port == 32) {
	update request {
		Tmp-String-1 := ""
	}
	if (&Event-Timestamp) {
		update request {
			Tmp-String-1 := "%{integer:Event-Timestamp}"
		}
	}
	nac_mab_field_accounting
	if (fail || reject) {
		fail
	}
	return
}
