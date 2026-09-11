package nlrisplit

// keyEVPN preserves route type and RD while excluding route attributes from
// the prefix identity (RFC 7432 Sections 7.1-7.4, RFC 9136 Section 3.1).
// Announcements and withdrawals have the same route-specific encoding.
func keyEVPN(raw, scratch []byte, _ bool) ([]byte, error) {
	if len(raw) < 2 || int(raw[1]) != len(raw)-2 {
		return nil, errPrefixKey
	}
	body := raw[2:]
	switch raw[0] {
	case 1:
		// Ethernet A-D: RD, ESI, and Ethernet Tag, but not the label.
		if len(body) != 25 || len(scratch) < 23 {
			return nil, errPrefixKey
		}
		scratch[0] = raw[0]
		copy(scratch[1:23], body[:22])
		return scratch[:23], nil
	case 2:
		// MAC/IP Advertisement: the MAC is 48 bits; the IP is absent,
		// IPv4, or IPv6. One label is required and a second is optional.
		if len(body) < 33 || body[22] != 48 {
			return nil, errPrefixKey
		}
		ipBits := int(body[29])
		if ipBits != 0 && ipBits != 32 && ipBits != 128 {
			return nil, errPrefixKey
		}
		end := 30 + ipBits/8
		if len(body) != end+3 && len(body) != end+6 {
			return nil, errPrefixKey
		}
		keyLen := 9 + end - 18
		if len(scratch) < keyLen {
			return nil, errPrefixKey
		}
		scratch[0] = raw[0]
		copy(scratch[1:9], body[:8])
		copy(scratch[9:keyLen], body[18:end])
		return scratch[:keyLen], nil
	case 3, 4:
		// All payload fields identify these routes. Validate the fixed
		// fields and originating-router address before removing framing.
		ipOffset := 12
		if raw[0] == 4 {
			ipOffset = 18
		}
		if len(body) <= ipOffset {
			return nil, errPrefixKey
		}
		ipBits := int(body[ipOffset])
		if (ipBits != 32 && ipBits != 128) || len(body) != ipOffset+1+ipBits/8 {
			return nil, errPrefixKey
		}
		if len(scratch) < 1+len(body) {
			return nil, errPrefixKey
		}
		scratch[0] = raw[0]
		copy(scratch[1:1+len(body)], body)
		return scratch[:1+len(body)], nil
	case 5:
		// IP Prefix: exclude ESI, gateway, and label. Retain the full
		// address width so IPv4 and IPv6 prefixes remain distinct even
		// when their prefix lengths and significant bytes match.
		addressLen := 4
		if len(body) == 58 {
			addressLen = 16
		} else if len(body) != 34 {
			return nil, errPrefixKey
		}
		prefixBits := int(body[22])
		keyLen := 14 + addressLen
		if prefixBits > addressLen*8 || len(scratch) < keyLen {
			return nil, errPrefixKey
		}
		scratch[0] = raw[0]
		copy(scratch[1:9], body[:8])
		copy(scratch[9:14], body[18:23])
		prefixBytes := (prefixBits + 7) / 8
		copy(scratch[14:14+prefixBytes], body[23:23+prefixBytes])
		if bits := prefixBits % 8; bits != 0 {
			scratch[13+prefixBytes] &= byte(0xff << (8 - bits))
		}
		clear(scratch[14+prefixBytes : keyLen])
		return scratch[:keyLen], nil
	default:
		// Unknown route types retain their opaque identity after framing
		// validation; no route-specific attributes can be inferred.
		return raw, nil
	}
}
