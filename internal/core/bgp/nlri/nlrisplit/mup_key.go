package nlrisplit

// keyMUP retains the architecture/type discriminator but not the native length
// octet. The route keys are specified in draft-ietf-bess-mup-safi Sections
// 3.1.1 through 3.1.4; session-route TLVs are explicitly outside the key.
func keyMUP(raw, scratch []byte, _ bool) ([]byte, error) {
	if len(raw) < 4 || int(raw[3]) != len(raw)-4 {
		return nil, errPrefixKey
	}
	if raw[0] != 1 || raw[1] != 0 || raw[2] < 1 || raw[2] > 4 {
		return raw, nil
	}

	body := raw[4:]
	keyLen := 0
	prefixBits := 0
	switch raw[2] {
	case 1, 3: // ISD and T1ST: RD, prefix length, prefix.
		if len(body) < 9 || body[8] > 128 {
			return nil, errPrefixKey
		}
		prefixBits = int(body[8])
		keyLen = 9 + (prefixBits+7)/8
		if len(body) < keyLen || (raw[2] == 1 && len(body) != keyLen) {
			return nil, errPrefixKey
		}
		// Do not parse T1ST's non-key tail: existing writeT1STData also emits
		// older forms without an absent source-address length octet.
	case 2: // DSD: RD and the complete IPv4 or IPv6 address.
		if len(body) != 12 && len(body) != 24 {
			return nil, errPrefixKey
		}
		keyLen = len(body)
	case 4: // T2ST: RD, endpoint and architecture-specific identifier.
		if len(body) < 9 || body[8] > 160 {
			return nil, errPrefixKey
		}
		// Endpoint Length covers both the address and the variable-width
		// identifier, not the optional TLVs. Keep its bit length to distinguish
		// aggregation widths. writeTEIDWithBits emits partial-width identifiers
		// as integers, so their low bits are identity, not CIDR padding.
		keyLen = 9 + (int(body[8])+7)/8
		if len(body) < keyLen {
			return nil, errPrefixKey
		}
	}
	if len(scratch) < 3+keyLen {
		return nil, errPrefixKey
	}
	key := scratch[:3+keyLen]
	copy(key, raw[:3])
	copy(key[3:], body[:keyLen])
	if prefixBits%8 != 0 {
		key[len(key)-1] &= byte(0xff << (8 - prefixBits%8))
	}
	return key, nil
}
