package flowspecfirewall

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
)

// lowerOnlyCanonicalBackend mirrors the one contract of the shipped backends
// that this spec is about.
//
// nft's lowerProtoMatch resolves a MatchProtocol through firewall.ProtocolNumber
// and returns an error for anything else, and nft's Apply returns that error
// from applyTable BEFORE its single conn.Flush. So a term nothing can lower does
// not fail alone: nothing at all reaches the kernel, for any owner. flushed is
// written only on the success path, which is what reproduces that.
type lowerOnlyCanonicalBackend struct {
	flushed []firewall.Table
}

func (b *lowerOnlyCanonicalBackend) Apply(desired []firewall.Table) error {
	for _, tbl := range desired {
		for _, chain := range tbl.Chains {
			for _, term := range chain.Terms {
				for _, m := range term.Matches {
					pm, ok := m.(firewall.MatchProtocol)
					if !ok {
						continue
					}
					if _, known := firewall.ProtocolNumber(pm.Protocol); !known {
						return fmt.Errorf("unknown protocol %q", pm.Protocol)
					}
				}
			}
		}
	}
	b.flushed = append([]firewall.Table{}, desired...)
	return nil
}

func (b *lowerOnlyCanonicalBackend) ListTables() ([]firewall.Table, error) { return b.flushed, nil }

func (b *lowerOnlyCanonicalBackend) GetCounters(string) ([]firewall.ChainCounters, error) {
	return nil, nil
}

func (b *lowerOnlyCanonicalBackend) Close() error { return nil }

// otherOwnerTable stands for any firewall owner that is not flowspec: the
// firewall engine, copp, policy routes, ddos-local. They all reach the kernel
// through the same ApplyAll and the same single flush.
func otherOwnerTable() firewall.Table {
	return firewall.Table{
		Name:   "ze_local_filter",
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     "input",
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms: []firewall.Term{{
				Name:    "allow-ssh",
				Matches: []firewall.Match{firewall.MatchProtocol{Protocol: "tcp"}},
				Actions: []firewall.Action{firewall.Accept{}},
			}},
		}},
	}
}

func flowSpecAddEvent(peer, protocol string) string {
	return daemonAddJSON(peer, "rate-limit:0", `{
		"destination-ipv4": [["10.1.0.0/24"]],
		"protocol": [["=`+protocol+`"]]
	}`)
}

// TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers is the security case.
//
// VALIDATES: a FlowSpec route ze cannot translate never reaches the backend, so
// a later reconcile by any other owner still lands in the kernel, and the
// routes ze CAN enforce are still enforced.
// PREVENTS: a peer freezing this router's firewall reconciliation by announcing
// one legal FlowSpec route. The translator used to render an unnamed protocol
// number as decimal digits; no backend resolves digits, and the resulting error
// returns from Apply before its single Flush, so from that moment the kernel
// kept its previous ruleset for the firewall engine, copp, policy routes and
// ddos-local alike.
//
// The order below is what makes the test discriminate. The untranslatable route
// arrives FIRST and the other owner reconciles AFTER it: asserting on a flush
// that happened before the bad route arrived would pass with the defect in
// place, because the recorder would still be holding that earlier success.
func TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers(t *testing.T) {
	const backendName = "flowspec-blast-radius-test"

	be := &lowerOnlyCanonicalBackend{}
	require.NoError(t, firewall.RegisterBackend(backendName, func() (firewall.Backend, error) {
		return be, nil
	}))
	require.NoError(t, firewall.LoadBackend(backendName))
	t.Cleanup(func() {
		_ = firewall.RegisterTables("flowspec", nil)
		_ = firewall.RegisterTables("local-firewall", nil)
		_ = firewall.CloseBackend()
	})

	b := testBridge()
	// 253 is legal on the wire (RFC 3692 experimentation) and has no canonical
	// name, so ze cannot enforce it.
	require.NoError(t, b.handleEvent(flowSpecAddEvent("10.0.0.2", "253")))
	// 132 is SCTP: legal on the wire, and ze can enforce it.
	require.NoError(t, b.handleEvent(flowSpecAddEvent("10.0.0.1", "132")))

	// Only now does another owner reconcile. With an unenforceable FlowSpec term
	// registered, this is the call that never reaches the kernel.
	_ = firewall.RegisterTables("local-firewall", []firewall.Table{otherOwnerTable()})
	require.NoError(t, firewall.ApplyAll(), "one untranslatable FlowSpec route must not fail every owner's reconcile")

	names := make(map[string]bool, len(be.flushed))
	protocols := make([]string, 0, 4)
	for _, tbl := range be.flushed {
		names[tbl.Name] = true
		for _, chain := range tbl.Chains {
			for _, term := range chain.Terms {
				for _, m := range term.Matches {
					if pm, ok := m.(firewall.MatchProtocol); ok {
						protocols = append(protocols, pm.Protocol)
					}
				}
			}
		}
	}
	assert.True(t, names["ze_local_filter"], "every other owner's ruleset must still reach the kernel")
	assert.True(t, names["ze_flowspec"], "the FlowSpec routes ze can enforce must still reach the kernel")
	assert.Contains(t, protocols, "sctp", "the enforceable FlowSpec route must be installed")
	assert.NotContains(t, protocols, "253", "no MatchProtocol may ever carry digits")
}
