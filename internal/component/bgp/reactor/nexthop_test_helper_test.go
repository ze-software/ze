// Design: docs/architecture/core-design.md -- Policy Filter Chain, test helpers for export next-hop policies
// Related: peer_settings.go -- ExportPolicyWithNextHop
// Related: export_nexthop_chain_test.go -- exportNextHopPolicy, answerTransport

package reactor

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ExportPolicyWithNextHop returns a static export filterapi.Policy containing
// the single built-in next-hop step ref (policychain.BuiltinNextHopAuto,
// BuiltinNextHopSelf, or BuiltinNextHopUnchanged) compiled into a library
// chain, while preserving base's configured final disposition (or unconfigured
// state for RFC 8212). Panics if ref is not a valid built-in next-hop name.
func ExportPolicyWithNextHop(base filterapi.Policy, ref string) filterapi.Policy {
	if _, ok := policychain.BuiltinNextHopMode(ref); !ok {
		panic("BUG: reactor: ExportPolicyWithNextHop requires a built-in next-hop ref, got " + ref)
	}
	refs := []routepolicy.Ref{{Name: ref}}
	members := []policychain.Member{{Name: ref}}
	if base.Configured() {
		chain, err := policychain.Compile("bgp export", nil, members, base.Final(), policychain.Bases{})
		if err != nil {
			panic("BUG: reactor: ExportPolicyWithNextHop compile failed: " + err.Error())
		}
		p, err := filterapi.NewPolicy(refs, base.Final())
		if err != nil {
			panic("BUG: reactor: ExportPolicyWithNextHop policy failed: " + err.Error())
		}
		return p.WithLibrary(chain)
	}
	chain, err := policychain.CompileUnconfigured("bgp export", nil, members, policychain.Bases{})
	if err != nil {
		panic("BUG: reactor: ExportPolicyWithNextHop unconfigured compile failed: " + err.Error())
	}
	return filterapi.InternalPolicy(refs).WithLibrary(chain)
}

// setExportNextHopSelf configures p with built-in next-hop:self, preserving
// any configured final (or setting final accept when unconfigured if accept is true).
func setExportNextHopSelf(base filterapi.Policy) filterapi.Policy {
	return ExportPolicyWithNextHop(base, policychain.BuiltinNextHopSelf)
}

// setExportNextHopUnchanged configures p with built-in next-hop:unchanged,
// preserving any configured final.
func setExportNextHopUnchanged(base filterapi.Policy) filterapi.Policy {
	return ExportPolicyWithNextHop(base, policychain.BuiltinNextHopUnchanged)
}

// setExportNextHopAuto configures p with built-in next-hop:auto, preserving
// any configured final.
func setExportNextHopAuto(base filterapi.Policy) filterapi.Policy {
	return ExportPolicyWithNextHop(base, policychain.BuiltinNextHopAuto)
}

// attachExplicitNextHop wires an export policy setting explicit next-hop addr
// on peer through r's in-memory answerTransport, preserving peer's existing
// export final (defaulting to FinalAccept if unconfigured).
func attachExplicitNextHop(t testing.TB, r *Reactor, peer *Peer, addr netip.Addr) {
	t.Helper()
	rawValue, err := json.Marshal(addr.String())
	if err != nil {
		t.Fatalf("marshal next-hop %s: %v", addr, err)
	}
	if r != nil {
		if len(r.orderedEgressSteps) == 0 {
			r.orderedEgressSteps = buildOrderedEgressSteps()
		}
		r.filterTransportSeam = &answerTransport{
			reads: []string{"prefix"},
			sets:  []string{exportNHField},
			answer: func(_ rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{
					Action: rpc.FilterModify,
					Sets: []rpc.FilterSet{{
						Field:  exportNHField,
						Action: rpc.FilterSetActionSet,
						Value:  rawValue,
					}},
				}
			},
		}
	}
	final := peer.settings.ExportPolicy.Final()
	if !peer.settings.ExportPolicy.Configured() {
		final = routepolicy.FinalAccept
	}
	members := []policychain.Member{
		{Name: exportNHFilter, Plugin: answerPlugin},
		{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
	}
	chain, err := policychain.Compile(exportNHAttachment, nil, members, final, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile explicit next-hop chain: %v", err)
	}
	refs := []routepolicy.Ref{
		{Name: answerPlugin + ":" + exportNHFilter},
		{Name: policychain.BuiltinNextHopAuto},
	}
	peer.settings.ExportPolicy = mustPolicy(t, refs, final).WithLibrary(chain)
	peer.fwdFacts.Store(peer.buildForwardFacts())
}
