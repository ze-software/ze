package network

import (
	"strings"
	"testing"
)

// TestBridgeNameIsRefusedNotTruncated: an 11-character name gives a 15-byte
// bridge name, and a 12-character name is refused rather than shortened.
func TestBridgeNameIsRefusedNotTruncated(t *testing.T) {
	bridge, err := bridgeName("abcdefghijk")
	if err != nil {
		t.Fatalf("an 11-character name was refused: %v", err)
	}
	if bridge != "zcn-abcdefghijk" {
		t.Fatalf("bridge %q, want zcn-abcdefghijk", bridge)
	}
	if _, err := bridgeName("abcdefghijkl"); err == nil {
		t.Fatal("a 12-character name was accepted")
	}
	for _, name := range []string{"", "-lan", "lan-", "Lan", "l_an"} {
		if _, err := bridgeName(name); err == nil {
			t.Errorf("name %q was accepted", name)
		}
	}
}

// TestHardwareAddressIsStableAndUnicast: the derived address is the same for
// the same two names, differs for another instance, and is a locally
// administered unicast address.
func TestHardwareAddressIsStableAndUnicast(t *testing.T) {
	first := derivedHardwareAddress("lan", "web")
	if again := derivedHardwareAddress("lan", "web"); again.String() != first.String() {
		t.Fatalf("the address changed between two derivations: %s then %s", first, again)
	}
	if other := derivedHardwareAddress("lan", "api"); other.String() == first.String() {
		t.Fatalf("two instances derived the same address %s", first)
	}
	if split := derivedHardwareAddress("la", "nweb"); split.String() == first.String() {
		t.Fatalf("moving a character between the names kept the address %s", first)
	}
	if first[0]&0x02 == 0 {
		t.Fatalf("%s is not locally administered", first)
	}
	if first[0]&0x01 != 0 {
		t.Fatalf("%s is a multicast address", first)
	}
	if len(first) != 6 {
		t.Fatalf("%s is not six octets", first)
	}
	if containerLinkName(1) != "eth1" {
		t.Fatalf("the second link is named %q, want eth1", containerLinkName(1))
	}
	if !strings.HasPrefix(bridgePrefix, devicePrefix) {
		t.Fatalf("the bridge prefix %q lies outside the granted prefix %q", bridgePrefix, devicePrefix)
	}
}
