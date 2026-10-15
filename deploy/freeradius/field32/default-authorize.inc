if (&Packet-Src-IP-Address == 10.6.8.19 && &NAS-Port == 32) {
	if (!&EAP-Message) {
		reject {
			reject = 1
		}
		return
	}
	# Module action 'return' only leaves the enclosing group. Exit the
	# complete authorize section explicitly so the legacy EAP cannot run.
	nac_field_eap {
		ok = 1
		updated = 1
		handled = 1
		reject = 1
		fail = 1
		invalid = 1
		noop = 1
		notfound = 1
		userlock = 1
	}
	return
}
