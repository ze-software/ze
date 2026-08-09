// Design: docs/architecture/ospf/ospf-1-types.md -- LSAKey extraction, equality, and ordering

package types

import "testing"

func testLSAHeader() []byte {
	return []byte{
		0x00, 0x01,
		byte(OptionE), byte(LSTypeRouter),
		1, 2, 3, 4,
		5, 6, 7, 8,
		0x80, 0x00, 0x00, 0x01,
		0x12, 0x34,
		0x00, 0x18,
	}
}

// VALIDATES: AC-4 - lsaKeyFromHeader extracts only type, Link State ID, and Advertising Router.
// PREVENTS: LSDB keys that include version fields and grow on every LSA refresh.
func TestLSAKeyFromLSAHeader(t *testing.T) {
	key, err := lsaKeyFromHeader(testLSAHeader())
	if err != nil {
		t.Fatalf("lsaKeyFromHeader returned error: %v", err)
	}
	if key.Type != LSTypeRouter {
		t.Fatalf("key type = %d, want %d", key.Type, LSTypeRouter)
	}
	if got := key.LinkStateID.String(); got != "1.2.3.4" {
		t.Fatalf("key LinkStateID = %q, want 1.2.3.4", got)
	}
	if got := key.AdvertisingRouter.String(); got != "5.6.7.8" {
		t.Fatalf("key AdvertisingRouter = %q, want 5.6.7.8", got)
	}
	m := map[LSAKey]string{key: "present"}
	if m[key] != "present" {
		t.Fatalf("LSAKey did not behave as a map key")
	}
}

// VALIDATES: AC-4 - sequence, age, checksum, and length are excluded from LSAKey equality.
// PREVENTS: a refreshed LSA being stored as a distinct LSDB entry.
func TestLSAKeyEqualityExcludesVersion(t *testing.T) {
	first := testLSAHeader()
	second := testLSAHeader()
	second[0] = 0x0e
	second[1] = 0x10
	second[12] = 0x7f
	second[13] = 0xff
	second[14] = 0xff
	second[15] = 0xff
	second[16] = 0xaa
	second[17] = 0xbb
	second[18] = 0x00
	second[19] = 0x28

	key1, err := lsaKeyFromHeader(first)
	if err != nil {
		t.Fatalf("first key returned error: %v", err)
	}
	key2, err := lsaKeyFromHeader(second)
	if err != nil {
		t.Fatalf("second key returned error: %v", err)
	}
	if key1 != key2 {
		t.Fatalf("LSAKey includes version fields: %v != %v", key1, key2)
	}
}

// VALIDATES: AC-13 - LSAKey has a total order consistent with equality.
// PREVENTS: nondeterministic LSDB listing order in CLI output.
func TestLSAKeyOrder(t *testing.T) {
	base, err := lsaKeyFromHeader(testLSAHeader())
	if err != nil {
		t.Fatalf("base key returned error: %v", err)
	}
	higherType := base
	higherType.Type = LSTypeNetwork
	if !base.Less(higherType) || base.Compare(higherType) >= 0 {
		t.Fatalf("type order mismatch: base=%v higher=%v", base, higherType)
	}
	higherID := base
	higherID.LinkStateID = LinkStateID{1, 2, 3, 5}
	if !base.Less(higherID) || base.Compare(higherID) >= 0 {
		t.Fatalf("link-state-id order mismatch: base=%v higher=%v", base, higherID)
	}
	higherAdv := base
	higherAdv.AdvertisingRouter = RouterID{5, 6, 7, 9}
	if !base.Less(higherAdv) || base.Compare(higherAdv) >= 0 {
		t.Fatalf("advertising-router order mismatch: base=%v higher=%v", base, higherAdv)
	}
	same := base
	if base.Less(same) || base.Compare(same) != 0 {
		t.Fatalf("equal key was ordered before itself")
	}
}

// VALIDATES: AC-2 - lsaKeyFromHeader rejects non-20-byte headers before indexing.
// PREVENTS: malformed LSAs causing out-of-range panics.
func TestLSAKeyFromLSAHeaderRejectsWrongLength(t *testing.T) {
	for _, input := range [][]byte{testLSAHeader()[:19], append(testLSAHeader(), 0)} {
		if _, err := lsaKeyFromHeader(input); err == nil {
			t.Fatalf("lsaKeyFromHeader length %d succeeded, want error", len(input))
		}
	}
}
