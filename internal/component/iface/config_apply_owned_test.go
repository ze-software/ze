// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: owned_bridge_link.go -- the registries and requests under test
// Related: config_apply.go -- reconcileOwnedDevices, the pass under test

package iface

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The owner and prefix these tests grant, a prefix no producer uses.
const (
	ownedTestOwner  = "test-owned-bridge-owner"
	ownedTestPrefix = "zta"
	ownedTestAlias  = ownedDeviceAliasPrefix + ownedTestOwner
)

// useOwnedFakeBackend loads fb as the active backend under a name of its own,
// grants the test owner its prefix, and empties the owned registries. Cleanup
// closes the backend, drops the registration and the grant, and empties the
// registries again, so a second -count pass starts clean.
func useOwnedFakeBackend(t *testing.T, fb *fakeBackend) {
	t.Helper()
	fb.ensureMaps()
	name := "test-owned-" + t.Name()
	if err := CloseBackend(); err != nil {
		t.Fatalf("reset backend: %v", err)
	}
	if err := RegisterBackend(name, func() (Backend, error) { return fb, nil }); err != nil {
		t.Fatalf("register backend: %v", err)
	}
	if err := LoadBackend(name); err != nil {
		t.Fatalf("load backend: %v", err)
	}
	MustGrantOwnedDeviceNames(ownedTestOwner, ownedTestPrefix)
	resetDeviceOwners(t)
	emptyOwnedBridgesAndLinks()
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, name)
		backendsMu.Unlock()
		deviceNameGrants.mu.Lock()
		delete(deviceNameGrants.owners, ownedTestOwner)
		deviceNameGrants.mu.Unlock()
		emptyOwnedBridgesAndLinks()
	})
}

func emptyOwnedBridgesAndLinks() {
	deviceOwnerMu.Lock()
	ownedBridgeOwners = map[string]map[string]OwnedBridgeSpec{}
	ownedLinkOwners = map[string]map[string][]ownedLink{}
	deviceOwnerMu.Unlock()
}

// ownedTestBridgeSet is a set of one bridge with one gateway address.
func ownedTestBridgeSet(name string, mtu uint32) *rpc.OwnedBridgesReplaceInput {
	return &rpc.OwnedBridgesReplaceInput{Bridges: []rpc.OwnedBridge{{
		Name: name, MTU: mtu, Addresses: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")},
	}}}
}

func countCalls(calls []string, call string) int {
	count := 0
	for _, c := range calls {
		if c == call {
			count++
		}
	}
	return count
}

// TestReconcileCreatesOwnedBridgeWithAlias proves the bridge request reaches
// the backend as a marked bridge with the requested MTU and address, that a
// repeat of the same set changes nothing, that a config apply leaves the
// bridge alone, and that an empty set deletes it. Method: a scripted backend
// loaded as the active one, the exported request, then the full reconcile.
func TestReconcileCreatesOwnedBridgeWithAlias(t *testing.T) {
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	const bridge = ownedTestPrefix + "-net"

	output, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(bridge, 1400))
	if err != nil {
		t.Fatalf("ReplaceOwnedBridges: %v", err)
	}
	if !slices.Equal(output.Bridges, []string{bridge}) {
		t.Fatalf("answered bridges %v, want [%s]", output.Bridges, bridge)
	}
	made, ok := fb.ifaces[bridge]
	if !ok {
		t.Fatalf("no bridge %s in the backend", bridge)
	}
	if made.linkType != TypeBridge || made.alias != ownedTestAlias || made.mtu != 1400 {
		t.Fatalf("bridge = type %q alias %q mtu %d, want bridge %q 1400", made.linkType, made.alias, made.mtu, ownedTestAlias)
	}
	if !slices.Equal(fb.addrs[bridge], []string{"192.0.2.1/24"}) {
		t.Fatalf("bridge addresses %v, want [192.0.2.1/24]", fb.addrs[bridge])
	}

	if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(bridge, 1400)); err != nil {
		t.Fatalf("repeat ReplaceOwnedBridges: %v", err)
	}
	if got := countCalls(fb.callOrder, "create-owned-bridge:"+bridge); got != 1 {
		t.Fatalf("the same set created the bridge %d times, want 1", got)
	}
	if got := countCalls(fb.callOrder, "add-address:"+bridge+":192.0.2.1/24"); got != 1 {
		t.Fatalf("the same set added the address %d times, want 1", got)
	}

	if errs, deferred := reconcileOnReady(&ifaceConfig{}, fb); len(errs) != 0 || deferred {
		t.Fatalf("config reconcile = (%v, %v), want (nil, false)", errs, deferred)
	}
	if _, ok := fb.ifaces[bridge]; !ok {
		t.Fatalf("a config apply deleted the owned bridge %s", bridge)
	}

	if _, err := ReplaceOwnedBridges(ownedTestOwner, &rpc.OwnedBridgesReplaceInput{}); err != nil {
		t.Fatalf("empty ReplaceOwnedBridges: %v", err)
	}
	if _, ok := fb.ifaces[bridge]; ok {
		t.Fatalf("an empty set kept the bridge %s", bridge)
	}
}

// TestOrphanScanKeepsForeignBridge proves the orphan scan needs both facts,
// an owned kind and the mark: an unmarked bridge and a marked dummy survive,
// while a marked bridge no owner declares is deleted. It also proves a bridge
// name held by an unmarked bridge fails closed: the request is refused, the
// device is kept and the owner's set is unchanged.
func TestOrphanScanKeepsForeignBridge(t *testing.T) {
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	fb.ifaces["br-operator"] = fakeIface{name: "br-operator", linkType: TypeBridge, index: 11}
	fb.ifaces["zx-marked"] = fakeIface{name: "zx-marked", linkType: TypeDummy, alias: ownedDeviceAliasPrefix + "someone", index: 12}
	fb.ifaces[ownedTestPrefix+"-old"] = fakeIface{name: ownedTestPrefix + "-old", linkType: TypeBridge, alias: ownedTestAlias, index: 13}

	if errs, deferred := reconcileOnReady(&ifaceConfig{}, fb); len(errs) != 0 || deferred {
		t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
	}
	if _, ok := fb.ifaces["br-operator"]; !ok {
		t.Errorf("the orphan scan deleted the operator's unmarked bridge")
	}
	if _, ok := fb.ifaces["zx-marked"]; !ok {
		t.Errorf("the orphan scan deleted a marked device of a kind no registry creates")
	}
	if _, ok := fb.ifaces[ownedTestPrefix+"-old"]; ok {
		t.Errorf("the orphan scan kept a marked bridge no owner declares")
	}

	const held = ownedTestPrefix + "-held"
	fb.ifaces[held] = fakeIface{name: held, linkType: TypeBridge, index: 14}
	if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(held, 1500)); err == nil {
		t.Fatalf("a bridge name held by an unmarked bridge was accepted")
	} else if !strings.Contains(err.Error(), held) {
		t.Fatalf("the refusal does not name %s: %v", held, err)
	}
	if got := fb.ifaces[held]; got.alias != "" || fb.deleted[held] {
		t.Fatalf("the unmarked bridge %s was changed: alias %q deleted %v", held, got.alias, fb.deleted[held])
	}
	deviceOwnerMu.Lock()
	_, kept := ownedBridgeOwners[ownedTestOwner][held]
	deviceOwnerMu.Unlock()
	if kept {
		t.Fatalf("a refused set stayed registered")
	}
}

// TestOrphanScanDeletesLinkWithNoRecord proves a marked veth that no link
// record names is deleted, a recorded one is kept, and a record whose host end
// is gone is forgotten, never re-made. Method: seed the kernel state and the
// records a restart and a namespace exit leave, then run the full reconcile.
func TestOrphanScanDeletesLinkWithNoRecord(t *testing.T) {
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	const bridge = ownedTestPrefix + "-net"
	if _, err := ReplaceOwnedBridges(ownedTestOwner, ownedTestBridgeSet(bridge, 1500)); err != nil {
		t.Fatalf("ReplaceOwnedBridges: %v", err)
	}
	const (
		recorded = ownedTestPrefix + "-1-1"
		orphan   = ownedTestPrefix + "-1-2"
		gone     = ownedTestPrefix + "-1-3"
	)
	fb.ifaces[recorded] = fakeIface{name: recorded, linkType: TypeVeth, alias: ownedTestAlias, index: 21}
	fb.ifaces[orphan] = fakeIface{name: orphan, linkType: TypeVeth, alias: ownedTestAlias, index: 22}
	fb.ifaces["veth-operator"] = fakeIface{name: "veth-operator", linkType: TypeVeth, index: 23}
	deviceOwnerMu.Lock()
	ownedLinkOwners[ownedTestOwner] = map[string][]ownedLink{
		"live": {{hostEnd: recorded, bridge: bridge}},
		"exit": {{hostEnd: gone, bridge: bridge}},
	}
	deviceOwnerMu.Unlock()

	if errs, deferred := reconcileOnReady(&ifaceConfig{}, fb); len(errs) != 0 || deferred {
		t.Fatalf("reconcile = (%v, %v), want (nil, false)", errs, deferred)
	}
	if _, ok := fb.ifaces[orphan]; ok {
		t.Errorf("the orphan scan kept a marked veth no record names")
	}
	if _, ok := fb.ifaces[recorded]; !ok {
		t.Errorf("the orphan scan deleted a recorded link's host end")
	}
	if _, ok := fb.ifaces["veth-operator"]; !ok {
		t.Errorf("the orphan scan deleted an unmarked veth")
	}
	if fb.created[gone] {
		t.Errorf("the reconcile re-made the link %s whose namespace is gone", gone)
	}
	deviceOwnerMu.Lock()
	_, stillExit := ownedLinkOwners[ownedTestOwner]["exit"]
	_, stillLive := ownedLinkOwners[ownedTestOwner]["live"]
	deviceOwnerMu.Unlock()
	if stillExit {
		t.Errorf("the record of a link whose host end is gone was kept")
	}
	if !stillLive {
		t.Errorf("the record of a present link was dropped")
	}
}

// TestConfigRefusesNameInsideOwnedDevicePrefix proves a configured interface
// name inside a prefix a plugin granted itself is refused at parse, naming the
// owner and the prefix, so the config apply never adopts or deletes an owned
// device. A name outside every prefix is the positive control.
func TestConfigRefusesNameInsideOwnedDevicePrefix(t *testing.T) {
	MustGrantOwnedDeviceNames(ownedTestOwner, ownedTestPrefix)
	t.Cleanup(func() {
		deviceNameGrants.mu.Lock()
		delete(deviceNameGrants.owners, ownedTestOwner)
		deviceNameGrants.mu.Unlock()
	})

	_, err := parseIfaceConfig(`{"interface":{"bridge":{"` + ownedTestPrefix + `-net":{}}}}`)
	if err == nil {
		t.Fatalf("a bridge named inside the granted prefix %s was accepted", ownedTestPrefix)
	}
	if !strings.Contains(err.Error(), ownedTestOwner) || !strings.Contains(err.Error(), ownedTestPrefix) {
		t.Fatalf("the refusal does not name the owner and the prefix: %v", err)
	}

	_, err = parseIfaceConfig(`{"interface":{"veth":{"vx0":{"peer":"` + ownedTestPrefix + `-peer"}}}}`)
	if err == nil {
		t.Fatalf("a veth peer named inside the granted prefix %s was accepted", ownedTestPrefix)
	}

	if _, err := parseIfaceConfig(`{"interface":{"bridge":{"br0":{}}}}`); err != nil {
		t.Fatalf("a bridge outside every granted prefix was refused: %v", err)
	}
}
