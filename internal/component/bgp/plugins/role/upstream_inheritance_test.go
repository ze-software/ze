// Design: docs/guide/bgp-role.md -- The Upstream Check a Role Inserts.
package role

import (
	"encoding/binary"
	"encoding/json"
	"slices"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// upstreamInheritanceCaller connects the compiled member to the real role
// evaluator and its declaration. It holds no mutable state and is safe for
// concurrent use; CallFilter borrows the route only until it returns.
type upstreamInheritanceCaller struct{}

func (upstreamInheritanceCaller) FilterDeclaration(plugin, filter string) (policychain.ExternalDeclaration, bool) {
	if plugin != rolePluginName {
		return policychain.ExternalDeclaration{}, false
	}
	for _, decl := range upstreamFilterDecls() {
		if decl.Name == filter {
			return policychain.ExternalDeclaration{Reads: decl.Reads, Sets: decl.Sets, OnError: decl.OnError}, true
		}
	}
	return policychain.ExternalDeclaration{}, false
}

func (upstreamInheritanceCaller) CallFilter(_ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return evaluateRoleFilter(route)
}

// TestUpstreamInheritanceUsesConfiguredRole proves a peer's transit-asns or
// default strict leaf cannot hide its group's provider relationship. It derives
// the set through config parsing, compares the actual role resolver, and runs a
// compiled upstream member over real AS_PATH bytes, with a clean-route control.
func TestUpstreamInheritanceUsesConfiguredRole(t *testing.T) {
	t.Cleanup(func() { upstreamASNs.Store(nil) })
	const attachment = "bgp import"
	reg, err := policy.NewRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(attachment, reg,
		[]policychain.Member{{Name: upstreamFilterName, Plugin: rolePluginName}},
		policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	cases := []struct {
		name      string
		groupRole map[string]any
		peerRole  map[string]any
		wantRole  string
		wantSet   upstreamSet
	}{
		{"transit-only peer inherits", map[string]any{"import": "customer"}, map[string]any{"transit-asns": "far"}, roleCustomer, upstreamSet{64500, 64700, 64701}},
		{"explicit import overrides", map[string]any{"import": "customer"}, map[string]any{"import": "peer", "transit-asns": "far"}, rolePeer, upstreamSet{64700, 64701}},
		{"no peer container inherits", map[string]any{"import": "customer"}, nil, roleCustomer, upstreamSet{64500}},
		{"default strict alone inherits", map[string]any{"import": "customer"}, map[string]any{"strict": "false"}, roleCustomer, upstreamSet{64500}},
		{"no role condition", nil, map[string]any{"strict": "false"}, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
				"session":    map[string]any{"asn": map[string]any{"remote": "64500"}},
			}
			if tc.peerRole != nil {
				peer["role"] = tc.peerRole
			}
			group := map[string]any{"peer": map[string]any{"provider": peer}}
			if tc.groupRole != nil {
				group["role"] = tc.groupRole
			}
			bgp := map[string]any{"group": map[string]any{"uplinks": group}}
			body, err := json.Marshal(map[string]any{"bgp": bgp})
			if err != nil {
				t.Fatalf("marshal config: %v", err)
			}
			cfg, present, err := parseRoleConfig([]sdk.ConfigSection{
				{Root: configRootBGP, Data: string(body)},
				{Root: "policy", Data: `{"policy":{"asn-set":{"far":{"asn":["64700","64701"]}}}}`},
			})
			if err != nil {
				t.Fatalf("parse config: %v", err)
			}
			if !present {
				t.Fatal("config was not parsed")
			}
			var actualRole string
			if configured := cfg.peers["192.0.2.1"]; configured != nil {
				actualRole = configured.role
			}
			if actualRole != tc.wantRole {
				t.Fatalf("configured role = %q, want %q", actualRole, tc.wantRole)
			}
			if !slices.Equal(cfg.upstream, tc.wantSet) {
				t.Errorf("derived upstream set = %v, want %v", cfg.upstream, tc.wantSet)
			}
			installUpstreamSet(cfg.upstream)
			for _, route := range []struct {
				name string
				path []uint32
				want policychain.OutcomeAction
			}{
				{"clean", []uint32{64600, 65000}, policychain.OutcomeAccept},
				{"through provider", []uint32{64600, 64500, 65000}, upstreamInheritanceProviderVerdict(tc.wantRole)},
			} {
				attrs := []byte{0x40, 2, byte(2 + 4*len(route.path)), 2, byte(len(route.path))}
				for _, number := range route.path {
					attrs = binary.BigEndian.AppendUint32(attrs, number)
				}
				id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
					&capability.PeerIdentity{LocalASN: 64496, PeerASN: 64600},
					&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
				if err != nil {
					t.Fatalf("register encoding context: %v", err)
				}
				var view policyview.View
				view.Reset(wireu.NewWireUpdate(buildTestPayload(attrs, []byte{24, 203, 0, 113}), id), policyview.Session{})
				var sets policychain.RunSets
				sets.SetRoute(policychain.ExternalRoute{Caller: upstreamInheritanceCaller{}, Direction: "import", Peer: "192.0.2.2", PeerAS: 64600})
				out := chain.Run(&view, &sets)
				view.Clear()
				sets.SetRoute(policychain.ExternalRoute{})
				if out.Action != route.want {
					t.Errorf("%s: compiled upstream action = %v, want %v", route.name, out.Action, route.want)
				}
			}
		})
	}
}

func upstreamInheritanceProviderVerdict(role string) policychain.OutcomeAction {
	if role == roleCustomer {
		return policychain.OutcomeReject
	}
	return policychain.OutcomeAccept
}
