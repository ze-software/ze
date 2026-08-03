package isis

import "testing"

// VALIDATES: the configUint8/16/32 helpers this file added bound the narrowing
// locally, returning ok=false rather than truncating an out-of-range value.
// PREVENTS: The bare `uintN(v)` they replaced turning a parser-accepted value
// into a different in-range one. The config file parser already rejects
// out-of-range leaves against their YANG type, but relying on a guard three
// layers up fails OPEN for any entry point that skips it
// (ai/rules/evidence.md). OSPF has the identical helpers and test;
// IS-IS got the helpers with no test until this file.
func TestConfigUintRejectsAboveMax(t *testing.T) {
	if _, ok := configUint8("255"); !ok {
		t.Error("configUint8(255) rejected a valid value")
	}
	if _, ok := configUint8("256"); ok {
		t.Error("configUint8(256) accepted an out-of-range value")
	}
	if _, ok := configUint16("65535"); !ok {
		t.Error("configUint16(65535) rejected a valid value")
	}
	if _, ok := configUint16("65536"); ok {
		t.Error("configUint16(65536) accepted an out-of-range value")
	}
	if _, ok := configUint32("4294967295"); !ok {
		t.Error("configUint32(4294967295) rejected a valid value")
	}
	if _, ok := configUint32("4294967296"); ok {
		t.Error("configUint32(4294967296) accepted an out-of-range value")
	}
	// The legacy 64-bit-wide values that used to wrap silently.
	if v, ok := configUint16("18446744073709551615"); ok {
		t.Errorf("configUint16(MaxUint64) accepted, wrapped to %d", v)
	}
}
