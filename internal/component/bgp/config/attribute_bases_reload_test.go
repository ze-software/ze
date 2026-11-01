package bgpconfig

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// basesReloadConfig is a file-configured daemon whose one peer imports through
// the plugin filter ext:inc-lp, with bgp defaults attribute local-preference
// set to base. Every reload in the test below changes base and nothing else.
func basesReloadConfig(base string) string {
	return strings.ReplaceAll(`bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	defaults { attribute { local-preference BASE; } }
	peer source {
		connection {
			remote { ip 192.0.2.1; }
			local { ip 192.168.1.1; }
		}
		session { asn { remote 65000; } }
		filter {
			import { filters [ ext:inc-lp ]; final accept; }
		}
	}
}`, "BASE", base)
}

// incrementLocalPref is the plugin filter ext:inc-lp: it answers
// "local-preference increment 50" for every route, as bgp-filter-modify does
// for policy { modify INC-LP { increment { local-preference 50; } } }.
type incrementLocalPref struct{}

func (incrementLocalPref) FilterDeclaration(string, string) (policychain.ExternalDeclaration, bool) {
	return policychain.ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject}, true
}

func (incrementLocalPref) CallFilter(string, *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
		Field: "local-preference", Action: rpc.FilterSetActionIncrement, Value: json.RawMessage("50"),
	}}}, nil
}

// runningLocalPref runs the running peer's import chain over an UPDATE that
// carries no LOCAL_PREF and answers the LOCAL_PREF the chain writes.
func runningLocalPref(t *testing.T, r *reactor.Reactor) uint32 {
	t.Helper()
	peers := r.Peers()
	if len(peers) != 1 {
		t.Fatalf("running peers = %d, want 1", len(peers))
	}
	chain, ok := peers[0].ImportPolicy().Library().(*policychain.Chain)
	if !ok {
		t.Fatalf("the running import policy carries library %T, want *policychain.Chain", peers[0].ImportPolicy().Library())
	}
	view := new(policyview.View)
	view.Reset(updateWithoutLocalPref(t), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Caller: incrementLocalPref{}, Direction: "import", Peer: "192.0.2.1", PeerAS: 65000})
	defer sets.SetRoute(policychain.ExternalRoute{})
	outcome := chain.Run(view, sets)
	if outcome.Action != policychain.OutcomeAccept || outcome.Failed {
		t.Fatalf("import chain outcome = %+v, want accept", outcome)
	}
	var acc filterapi.ModAccumulator
	if err := chain.Apply(view, sets, &acc); err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, op := range acc.Ops() {
		if op.Code == uint8(attribute.AttrLocalPref) && op.Action == filterapi.AttrModSet && len(op.Buf) == 4 {
			return binary.BigEndian.Uint32(op.Buf)
		}
	}
	t.Fatalf("the import chain wrote no LOCAL_PREF: ops = %+v", acc.Ops())
	return 0
}

// updateWithoutLocalPref builds an UPDATE for 203.0.113.0/24 carrying ORIGIN
// IGP only, so the increment starts from the configured base (RFC 4271
// Section 5.1.5 leaves an absent LOCAL_PREF to local policy).
func updateWithoutLocalPref(t *testing.T) *wireu.WireUpdate {
	t.Helper()
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	payload = append(payload, attrs...)
	payload = append(payload, 24, 203, 0, 113)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65000},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

// reconcileFrom writes text over the daemon's config file and runs the BGP
// plugin's apply (register.go OnConfigApply: ReconcilePeersWithJournal), which
// re-reads the file through PeersFromConfigTree, parseAttributeBases and
// compileLibraryChains, and swaps the result into the running peer.
func reconcileFrom(t *testing.T, r *reactor.Reactor, path, text string, j registry.ConfigJournal) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadConfig(text, path, nil)
	if err != nil {
		t.Fatalf("parse the reload config: %v", err)
	}
	bgpTree, isMap := loaded.Tree.ToPluginMap()["bgp"].(map[string]any)
	if !isMap {
		t.Fatal("the reload config has no bgp block")
	}
	if err := r.ReconcilePeersWithJournal(bgpTree, j); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

// TestAttributeBaseReloadReachesTheRunningChain proves a reload that changes
// only bgp defaults attribute local-preference changes the LOCAL_PREF the
// running peer's import chain writes, both ways, and that a rollback of that
// reload restores the base it replaced. It is the reload half of
// filter_modify's TestAttributeDefaultsReloadTakesEffect, which moved here
// with the bases: the engine resolves an increment against the chain's base
// (policychain resolveNumeric), and the chain is compiled at peer load.
//
// Method: boot a file-configured reactor at base 100 and run the peer's import
// chain over a route with no LOCAL_PREF, then reload at 80 and at 100 through
// the BGP plugin's journaled apply, reading 150, 130, 150; then roll the last
// reload back and read 130. The two bases are 20 apart so neither result can
// be mistaken for the other, and the reload back to 100 catches a base that
// latched on its first change.
//
// VALIDATES: the base reaches the running chain on reload, and a rollback
// reinstalls the replaced chain with its own base.
// PREVENTS: a reload whose only change is the base answered as "no change"
// (peerSettingsEqual), or a chain compiled with the boot base forever.
func TestAttributeBaseReloadReachesTheRunningChain(t *testing.T) {
	declareRuntime(t, "ext", []rpc.FilterDecl{{
		Name: "inc-lp", Direction: rpc.FilterBoth, Reads: []string{"origin"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject,
	}})
	clearPending(t)

	path := filepath.Join(t.TempDir(), "router.conf")
	store := newReloadFileStore(t, path)
	boot := basesReloadConfig("100")
	if err := os.WriteFile(path, []byte(boot), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadConfig(boot, path, nil)
	if err != nil {
		t.Fatalf("parse the boot config: %v", err)
	}
	r, err := CreateReactor(loaded, path, store, true)
	if err != nil {
		t.Fatalf("create reactor: %v", err)
	}
	if got := runningLocalPref(t, r); got != 150 {
		t.Fatalf("boot: LOCAL_PREF = %d, want 150 (base 100 plus 50)", got)
	}

	first := sdk.NewJournal()
	reconcileFrom(t, r, path, basesReloadConfig("80"), first)
	first.Discard()
	if got := runningLocalPref(t, r); got != 130 {
		t.Fatalf("after the reload to 80: LOCAL_PREF = %d, want 130", got)
	}

	second := sdk.NewJournal()
	reconcileFrom(t, r, path, basesReloadConfig("100"), second)
	if got := runningLocalPref(t, r); got != 150 {
		t.Fatalf("after the reload back to 100: LOCAL_PREF = %d, want 150", got)
	}

	if errs := second.Rollback(); len(errs) != 0 {
		t.Fatalf("rollback: %v", errs)
	}
	if got := runningLocalPref(t, r); got != 130 {
		t.Fatalf("after the rollback: LOCAL_PREF = %d, want 130 (the base 80 the rolled-back reload replaced)", got)
	}
}
