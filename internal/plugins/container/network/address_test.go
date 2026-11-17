package network

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// fakeEngine records every request the network package makes and answers
// from an in-memory state store. failLink names a container link name whose
// creation is refused.
type fakeEngine struct {
	mu       sync.Mutex
	state    map[string][]byte
	bridges  [][]rpc.OwnedBridge
	sections []string
	created  []rpc.OwnedLinkCreateInput
	removed  []string
	failLink string
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{state: map[string][]byte{}}
}

func (f *fakeEngine) OwnedBridgesReplace(_ context.Context, input *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bridges = append(f.bridges, input.Bridges)
	return &rpc.OwnedBridgesReplaceOutput{}, nil
}

func (f *fakeEngine) OwnedLinkCreate(_ context.Context, input *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if input.Name == f.failLink {
		return nil, errors.New("refused by the test")
	}
	f.created = append(f.created, *input)
	return &rpc.OwnedLinkCreateOutput{HostEnd: "zc-1-0"}, nil
}

func (f *fakeEngine) OwnedLinkRemove(_ context.Context, input *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, input.Key)
	return &rpc.OwnedLinkRemoveOutput{}, nil
}

func (f *fakeEngine) FirewallTablesReplace(_ context.Context, input *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sections = append(f.sections, string(input.Section))
	return &rpc.FirewallTablesReplaceOutput{}, nil
}

func (f *fakeEngine) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, found := f.state[key]
	return value, found, nil
}

func (f *fakeEngine) StatePut(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[key] = append([]byte(nil), data...)
	return nil
}

// sections wraps a container section body with an interface and a firewall
// section, as the plugin receives them.
func sections(container string) []sdk.ConfigSection {
	return []sdk.ConfigSection{
		{Root: containerRoot, Data: `{"container":` + container + `}`},
		{Root: interfaceRoot, Data: `{"interface":{}}`},
		{Root: firewallRoot, Data: `{"firewall":{}}`},
	}
}

// mustParse parses a container body that the rules accept.
func mustParse(t *testing.T, container string) Config {
	t.Helper()
	cfg, err := parseSections(sections(container))
	if err != nil {
		t.Fatalf("parse refused a valid config: %v", err)
	}
	return cfg
}

// applyAndPlan applies cfg through a fresh manager on engine and answers the
// links planned for instance.
func applyAndPlan(t *testing.T, engine *fakeEngine, cfg Config, instance string) []linkPlan {
	t.Helper()
	m := newManager()
	m.receive(engine)
	if err := m.apply(context.Background(), cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return m.plans[instance]
}

// TestGatewayIsNetworkAddressPlusOne: with no gateway set, each family's
// gateway is the network address plus one, and a set gateway is kept.
func TestGatewayIsNetworkAddressPlusOne(t *testing.T) {
	cfg := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24","fd00:88::/64"]},"dmz":{"prefix":["10.89.0.0/24"],"gateway":["10.89.0.254"]}}}`)
	lan, _ := cfg.network("lan")
	want := []netip.Addr{netip.MustParseAddr("10.88.0.1"), netip.MustParseAddr("fd00:88::1")}
	if !slices.Equal(lan.Gateways, want) {
		t.Fatalf("lan gateways %v, want %v", lan.Gateways, want)
	}
	dmz, _ := cfg.network("dmz")
	if dmz.Gateways[0] != netip.MustParseAddr("10.89.0.254") {
		t.Fatalf("dmz gateway %v, want the set 10.89.0.254", dmz.Gateways[0])
	}
}

// TestAssignLowestFreeAddress: the assignment skips the gateway, the address
// an operator set on another attachment, and an address a still running
// container that left the network holds; IPv6 is assigned the same way.
func TestAssignLowestFreeAddress(t *testing.T) {
	network := Network{
		Name:     "lan",
		Prefixes: []netip.Prefix{netip.MustParsePrefix("10.88.0.0/24"), netip.MustParsePrefix("fd00::/64")},
		Gateways: []netip.Addr{netip.MustParseAddr("10.88.0.1"), netip.MustParseAddr("fd00::1")},
	}
	members := []attached{
		{instance: "a"},
		{instance: "b", set: []netip.Addr{netip.MustParseAddr("10.88.0.2")}},
		{instance: "c"},
	}
	held := []netip.Addr{netip.MustParseAddr("10.88.0.3")}
	record, err := assign(network, members, addressRecord{}, held)
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	cases := map[string][]netip.Addr{
		"a": {netip.MustParseAddr("10.88.0.4"), netip.MustParseAddr("fd00::2")},
		"b": {netip.MustParseAddr("10.88.0.2"), netip.MustParseAddr("fd00::3")},
		"c": {netip.MustParseAddr("10.88.0.5"), netip.MustParseAddr("fd00::4")},
	}
	for instance, want := range cases {
		if got := record.Instances[instance].Addresses; !slices.Equal(got, want) {
			t.Errorf("instance %s holds %v, want %v", instance, got, want)
		}
	}
	slots := []uint16{record.Instances["a"].Slot, record.Instances["b"].Slot, record.Instances["c"].Slot}
	if !slices.Equal(slots, []uint16{0, 1, 2}) {
		t.Errorf("slots %v, want 0 1 2", slots)
	}
}

// TestAssignmentIsKeptAcrossRestart: an apply stores the record under the
// network's key, and a later apply, with a newcomer sorted before the stored
// instance, keeps the stored address and slot rather than giving them to the
// newcomer.
func TestAssignmentIsKeptAcrossRestart(t *testing.T) {
	engine := newFakeEngine()
	first := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"web":{"network":[{"name":"lan"}]}}}`)
	plan := applyAndPlan(t, engine, first, "web")
	if plan[0].addresses[0] != netip.MustParsePrefix("10.88.0.2/24") {
		t.Fatalf("web got %v, want 10.88.0.2/24", plan[0].addresses)
	}
	stored := engine.state[zefs.KeyContainerNetworkAddress.Key("lan")]
	var record addressRecord
	if err := json.Unmarshal(stored, &record); err != nil {
		t.Fatalf("the stored record does not read: %v (%q)", err, stored)
	}
	if record.Instances["web"].Addresses[0] != netip.MustParseAddr("10.88.0.2") {
		t.Fatalf("the stored record holds %v", record)
	}

	second := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"api":{"network":[{"name":"lan"}]},"web":{"network":[{"name":"lan"}]}}}`)
	if again := applyAndPlan(t, engine, second, "web"); again[0].addresses[0] != netip.MustParsePrefix("10.88.0.2/24") {
		t.Fatalf("web moved to %v after the restart", again[0].addresses)
	}
	if again := applyAndPlan(t, engine, second, "web"); again[0].slot != 0 {
		t.Fatalf("web's slot moved to %d", again[0].slot)
	}
	if newcomer := applyAndPlan(t, engine, second, "api"); newcomer[0].addresses[0] != netip.MustParsePrefix("10.88.0.3/24") {
		t.Fatalf("api got %v, want 10.88.0.3/24", newcomer[0].addresses)
	}
}

// TestAssignmentReleasedWhenAttachmentLeaves: an instance that left the
// network is dropped from the record, and its address goes to the next
// instance that needs one.
func TestAssignmentReleasedWhenAttachmentLeaves(t *testing.T) {
	engine := newFakeEngine()
	both := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"a":{"network":[{"name":"lan"}]},"b":{"network":[{"name":"lan"}]}}}`)
	applyAndPlan(t, engine, both, "a")
	onlyB := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"b":{"network":[{"name":"lan"}]}}}`)
	applyAndPlan(t, engine, onlyB, "b")
	var record addressRecord
	if err := json.Unmarshal(engine.state[zefs.KeyContainerNetworkAddress.Key("lan")], &record); err != nil {
		t.Fatalf("the stored record does not read: %v", err)
	}
	if _, kept := record.Instances["a"]; kept {
		t.Fatalf("the record still holds a after its attachment left: %v", record)
	}
	newcomer := mustParse(t, `{"network":{"lan":{"prefix":["10.88.0.0/24"]}},"instance":{"b":{"network":[{"name":"lan"}]},"c":{"network":[{"name":"lan"}]}}}`)
	if plan := applyAndPlan(t, engine, newcomer, "c"); plan[0].addresses[0] != netip.MustParsePrefix("10.88.0.2/24") {
		t.Fatalf("c got %v, want the released 10.88.0.2/24", plan[0].addresses)
	}
}
