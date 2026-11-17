// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_bridge_link.go -- closeBackendAfterOwnedDevices, the stop under test
// Related: register.go -- the ordered stop that calls it

package iface

import (
	"maps"
	"testing"
)

// TestStopPathDeletesOwnedDevices proves the interface engine's ordered stop
// deletes every marked device of an owned kind (macvlan, bridge, veth) before
// it closes the backend, keeps every unmarked device and every marked device
// of another kind, and forgets the link records whose devices it deleted.
// Method: seed the scripted backend with each case, run the stop's own helper,
// and read what is left.
func TestStopPathDeletesOwnedDevices(t *testing.T) {
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	marked := map[string]string{
		ownedTestPrefix + "-net": TypeBridge,
		ownedTestPrefix + "-1-1": TypeVeth,
		"zv4-2-7":                zeTypeMacvlan,
	}
	kept := map[string]fakeIface{
		"br-operator": {name: "br-operator", linkType: TypeBridge, index: 31},
		"veth-op":     {name: "veth-op", linkType: TypeVeth, index: 32},
		"zx-marked":   {name: "zx-marked", linkType: TypeDummy, alias: ownedDeviceAliasPrefix + "someone", index: 33},
	}
	index := 40
	for name, kind := range marked {
		index++
		fb.ifaces[name] = fakeIface{name: name, linkType: kind, alias: ownedTestAlias, index: index}
	}
	maps.Copy(fb.ifaces, kept)
	deviceOwnerMu.Lock()
	ownedLinkOwners[ownedTestOwner] = map[string][]ownedLink{"live": {{hostEnd: ownedTestPrefix + "-1-1", bridge: ownedTestPrefix + "-net"}}}
	deviceOwnerMu.Unlock()

	ownedErr, closeErr := closeBackendAfterOwnedDevices()
	if ownedErr != nil {
		t.Fatalf("owned device deletion: %v", ownedErr)
	}
	if closeErr != nil {
		t.Fatalf("backend close: %v", closeErr)
	}
	for name, kind := range marked {
		if _, ok := fb.ifaces[name]; ok {
			t.Errorf("the stop kept the marked %s %s", kind, name)
		}
	}
	for name := range kept {
		if _, ok := fb.ifaces[name]; !ok {
			t.Errorf("the stop deleted %s, which is not an owned device", name)
		}
	}
	if GetBackend() != nil {
		t.Errorf("the stop left the backend loaded")
	}
	deviceOwnerMu.Lock()
	records := len(ownedLinkOwners)
	deviceOwnerMu.Unlock()
	if records != 0 {
		t.Errorf("the stop kept %d owners' link records for deleted devices", records)
	}
}
