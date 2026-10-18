| Test | Reason |
|---|---|
| TestSRPolicyDecodeNLRIHex | Moved and corrected, not weakened. It fed DecodeNLRIHex the body of an IPv4 SR Policy NLRI without its RFC 9830 Section 2.1 Length octet, which is the defect this commit fixes. TestDecodeNLRIHexWireForm/ipv4 (decode_test.go) asserts the same distinguisher 0, color 100 and endpoint 10.0.0.1 from the wire form `60...`, and TestDecodeNLRIHexRefused/"body without length octet" asserts the old body-only input is now refused. |
| TestSRPolicyDecodeNLRIHexIPv6 | Moved and corrected, as above: TestDecodeNLRIHexWireForm/ipv6 asserts distinguisher 42, color 200 and endpoint 2001:db8::1 from the wire form `C0...`. |
| types_test | The ten assertions removed are those of the two tests above, moved to decode_test.go, which adds packed, ADD-PATH, wrong-length and truncated cases and FuzzDecodeSRPolicySection. |
