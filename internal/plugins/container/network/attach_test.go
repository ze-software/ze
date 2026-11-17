//go:build linux

package network

import (
	"context"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestNetworkParticipantIsRegistered: the package's init registered its
// participant and its config sibling with the container plugin. The runtime
// keeps both registries private, so the test registers a second entry under
// the same name and expects the duplicate refusal, which only an existing
// registration produces.
func TestNetworkParticipantIsRegistered(t *testing.T) {
	expectDuplicate := func(what string, register func()) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatalf("the network %s is not registered: a second registration was accepted", what)
			}
			text, isText := recovered.(string)
			if !isText {
				t.Fatalf("the second %s registration panicked with %v, want a message", what, recovered)
			}
			if !strings.Contains(text, "twice") {
				t.Fatalf("the second %s registration panicked for another reason: %v", what, recovered)
			}
		}()
		register()
	}
	expectDuplicate("participant", func() { container.RegisterParticipant(&participant{manager: newManager()}) })
	expectDuplicate("config sibling", func() {
		container.RegisterConfigSibling(participantName, func([]sdk.ConfigSection) (Config, error) { return Config{}, nil },
			func(context.Context, Config) error { return nil })
	})
}

// TestParticipantSkipsUnattachedInstances: an instance with no network entry
// gets no request at the created call or the ended call.
func TestParticipantSkipsUnattachedInstances(t *testing.T) {
	engine := newFakeEngine()
	m := newManager()
	m.receive(engine)
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"web":{"network":[{"name":"lan"}]},"lone":{}}}`)
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	p := &participant{manager: m}
	if err := p.Created(context.Background(), container.CreatedCall{Instance: "lone", InitProcessID: os.Getpid()}); err != nil {
		t.Fatalf("created: %v", err)
	}
	p.Ended(context.Background(), "lone")
	if len(engine.created) != 0 || len(engine.removed) != 0 {
		t.Fatalf("an unattached instance sent requests: created %v, removed %v", engine.created, engine.removed)
	}
}

// TestParticipantCreatesLinksInOrder: at the created call the participant
// first removes what a previous start left, then asks for one link per
// attachment in the order written, with the namespace inode of the init
// process; at the ended call it removes them.
func TestParticipantCreatesLinksInOrder(t *testing.T) {
	engine := newFakeEngine()
	m := newManager()
	m.receive(engine)
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"],"mtu":"1400"},"dmz":{"prefix":["10.89.0.0/24"]}},`+
		`"instance":{"web":{"network":[{"name":"lan"},{"name":"dmz","address":["10.89.0.9"],"mac-address":"02:AA:00:00:00:01"}]}}}`)
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	inode, err := namespaceInode(os.Getpid())
	if err != nil {
		t.Fatalf("read this process's namespace: %v", err)
	}
	p := &participant{manager: m}
	if err := p.Created(context.Background(), container.CreatedCall{Instance: "web", InitProcessID: os.Getpid()}); err != nil {
		t.Fatalf("created: %v", err)
	}
	if !slices.Equal(engine.removed, []string{"web"}) {
		t.Fatalf("removals before the create %v, want web once", engine.removed)
	}
	if len(engine.created) != 2 {
		t.Fatalf("created %d links, want 2", len(engine.created))
	}
	lan, dmz := engine.created[0], engine.created[1]
	if lan.Name != "eth0" || lan.Bridge != "zcn-lan" || dmz.Name != "eth1" || dmz.Bridge != "zcn-dmz" {
		t.Fatalf("links %+v then %+v, want eth0 on zcn-lan then eth1 on zcn-dmz", lan, dmz)
	}
	wantLan := []netip.Prefix{netip.MustParsePrefix("10.88.0.2/24"), netip.MustParsePrefix("fd00:88::2/64")}
	if !slices.Equal(lan.Addresses, wantLan) || lan.MTU != 1400 {
		t.Fatalf("eth0 addresses %v mtu %d, want %v and 1400", lan.Addresses, lan.MTU, wantLan)
	}
	if !slices.Equal(lan.Gateways, []netip.Addr{netip.MustParseAddr("10.88.0.1"), netip.MustParseAddr("fd00:88::1")}) {
		t.Fatalf("eth0 gateways %v", lan.Gateways)
	}
	if lan.HardwareAddress != derivedHardwareAddress("lan", "web").String() {
		t.Fatalf("eth0 hardware address %q, want the derived one", lan.HardwareAddress)
	}
	if dmz.HardwareAddress != "02:aa:00:00:00:01" || dmz.Addresses[0] != netip.MustParsePrefix("10.89.0.9/24") {
		t.Fatalf("eth1 %+v, want the set address and hardware address", dmz)
	}
	for _, link := range engine.created {
		if link.PID != os.Getpid() || link.NamespaceInode != inode || link.Key != "web" {
			t.Fatalf("link %+v does not name the init process, its namespace inode %d and the instance", link, inode)
		}
	}
	p.Ended(context.Background(), "web")
	if !slices.Equal(engine.removed, []string{"web", "web"}) {
		t.Fatalf("removals after the end %v, want web twice", engine.removed)
	}
}

// TestParticipantRemovesWhatItMadeOnFailure: a link the interface component
// refuses fails the start, the links already made are removed, and the
// ended call that follows has nothing to remove.
func TestParticipantRemovesWhatItMadeOnFailure(t *testing.T) {
	engine := newFakeEngine()
	engine.failLink = "eth1"
	m := newManager()
	m.receive(engine)
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]},"dmz":{"prefix":["10.89.0.0/24"]}},"instance":{"web":{"network":[{"name":"lan"},{"name":"dmz"}]}}}`)
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	p := &participant{manager: m}
	err := p.Created(context.Background(), container.CreatedCall{Instance: "web", InitProcessID: os.Getpid()})
	if err == nil {
		t.Fatal("the start succeeded with a refused link")
	}
	if !strings.Contains(err.Error(), "network dmz") {
		t.Fatalf("the failure %q does not name the network", err)
	}
	if !slices.Equal(engine.removed, []string{"web", "web"}) {
		t.Fatalf("removals %v, want the one before the create and the one after the failure", engine.removed)
	}
	p.Ended(context.Background(), "web")
	if len(engine.removed) != 2 {
		t.Fatalf("the ended call of a failed start sent a removal: %v", engine.removed)
	}
}

// TestApplySendsBridgesAndTables: the apply sends one bridge per network with
// its mtu and gateway addresses, and the tables; a later config with no
// network withdraws both with empty sets, and a config that never had a
// network sends nothing.
func TestApplySendsBridgesAndTables(t *testing.T) {
	engine := newFakeEngine()
	m := newManager()
	m.receive(engine)
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"],"mtu":"9000"}}}`)
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(engine.bridges) != 1 || len(engine.bridges[0]) != 1 {
		t.Fatalf("bridge sets %v, want one set of one", engine.bridges)
	}
	bridge := engine.bridges[0][0]
	if bridge.Name != "zcn-lan" || bridge.MTU != 9000 || !slices.Equal(bridge.Addresses, []netip.Prefix{netip.MustParsePrefix("10.88.0.1/24")}) {
		t.Fatalf("bridge %+v", bridge)
	}
	if len(engine.sections) != 1 || !strings.Contains(engine.sections[0], `"container"`) {
		t.Fatalf("table sections %v", engine.sections)
	}
	if err := m.apply(context.Background(), Config{}); err != nil {
		t.Fatalf("apply of the empty config: %v", err)
	}
	if len(engine.bridges) != 2 || len(engine.bridges[1]) != 0 || engine.sections[1] != `{"firewall":{"table":{}}}` {
		t.Fatalf("the withdrawal sent bridges %v and tables %v", engine.bridges, engine.sections)
	}
	if err := m.apply(context.Background(), Config{}); err != nil {
		t.Fatalf("second apply of the empty config: %v", err)
	}
	if len(engine.bridges) != 2 || len(engine.sections) != 2 {
		t.Fatalf("an empty config sent requests again: bridges %v, tables %v", engine.bridges, engine.sections)
	}
}
