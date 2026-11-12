// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_firewall_linux.go -- the analyzer these proofs judge with
// Related: espform_firewall_kernel_linux.go -- the reader that dumps the kernel ruleset back
//
// These proofs judge the rules a stock Ze producer puts in the kernel, not rules
// written by hand: the producer builds its tables, the nft firewall backend lowers
// and commits them, and the analyzer reads them back through espFormKernelReader,
// exactly as the fast path's gate does. Each runs in a user and network namespace
// of its own (userns.Enter), which holds CAP_NET_ADMIN, so it needs no privilege
// and touches no host ruleset.

//go:build linux

package dataplane

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/gtsm"
	"github.com/ze-software/ze/internal/core/ddosevent"
	ddoslocal "github.com/ze-software/ze/internal/plugins/ddos/local"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft" // registers the nft firewall backend
	"github.com/ze-software/ze/internal/test/userns"
)

// TestESPFormFirewallStockGTSMKeepsTheFastPathOn proves that stock GTSM, for an
// IPv4 and an IPv6 peer, leaves the ESP form fast path on. Goal: every rule
// gtsm.FilterTables yields reads as one bare IPv4 ESP never matches, so the
// listen-port drop and the related-ICMP drops keep no Child SA off the fast path.
// Method: the gtsm package builds its table, the nft backend lowers and commits it
// in the test's own namespace, and the kernel's copy is read back and judged rule
// by rule (espFormJudgeRule must answer excluded for each) and as a whole
// ruleset (espFormFirewallBlocks must answer not blocked). The rule count is
// checked against the producer's term count, so an empty or missing chain cannot
// pass.
//
// MUTATION: put MatchSourceAddress back ahead of MatchProtocol in listenPortTerm
// (internal/component/gtsm/gtsm.go): the IPv4 peer's listen rule then opens with
// `meta nfproto ipv4`, is judged unsafe at its drop rather than excluded, and the
// ruleset reads blocked at that rule.
func TestESPFormFirewallStockGTSMKeepsTheFastPathOn(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	tables := gtsm.FilterTables([]gtsm.Peer{
		{Addr: netip.MustParseAddr("192.0.2.1"), Port: 179, HopLimit: 255, Floor: 255},
		{Addr: netip.MustParseAddr("2001:db8::1"), Port: 179, HopLimit: 255, Floor: 255},
	}, nil)
	if len(tables) != 1 || len(tables[0].Chains) != 1 {
		t.Fatalf("gtsm.FilterTables built %d tables, want one table with one chain: %+v", len(tables), tables)
	}
	espFWApplyStock(t, tables)

	reader, closeReader, err := openESPFormNFTReader()
	if err != nil {
		t.Fatalf("open the nftables reader: %v", err)
	}
	t.Cleanup(func() {
		if err := closeReader(); err != nil {
			t.Errorf("close the nftables reader: %v", err)
		}
	})

	want := tables[0]
	rules := espFWStockRules(t, reader, nftables.TableFamilyINet, want.Name, want.Chains[0].Name)
	if len(rules) != len(want.Chains[0].Terms) {
		t.Fatalf("the kernel holds %d rules in %s %s, want one for each of the producer's %d terms",
			len(rules), want.Name, want.Chains[0].Name, len(want.Chains[0].Terms))
	}
	for _, r := range rules {
		got := espFormJudgeRule(nftables.TableFamilyINet, r.Exprs)
		if got.kind != espFormRuleExcluded {
			t.Errorf("rule handle %d %q is judged %d (%s), want excluded: its first two expressions must compare the protocol with one other than ESP",
				r.Handle, espFormRuleComment(r.UserData), got.kind, got.reason)
		}
	}

	blocked, site, err := espFormFirewallBlocks(reader)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: %v", err)
	}
	if blocked {
		t.Fatalf("stock gtsm keeps the ESP form fast path off at %s", site)
	}
}

// TestESPFormFirewallStockGTSMRangesKeepTheFastPathOn proves that stock GTSM for
// dynamic-group ranges, IPv4 and IPv6, leaves the ESP form fast path on, with a
// narrower group that asked for no GTSM inside each wider GTSM range. Goal: each
// range's listen-port drop opens with its protocol and is excluded, as a peer's is;
// each claim (claimTerm) opens with its source address and accepts, so the IPv4
// claim, whose source lowers to `meta nfproto ipv4` first, which IPv4 ESP matches,
// passes on harmless selectors and its accept, and the IPv6 claim, which opens with
// `meta nfproto ipv6`, is excluded. Method: gtsm.FilterTables builds the table from
// the ranges, the nft backend lowers and commits it in the test's own namespace, and
// the kernel's copy is read back. Each kernel rule is paired with the producer term
// whose name it carries, in order, and judged against the kind that term's name owes
// (espFWStockGTSMKind: a listen-port drop or the IPv6 claim excluded, the IPv4 claim
// passing), never the kind the term's own matches imply; the ruleset as a whole must
// read not blocked. The rule count must match the term count and both kinds must be
// seen, so an empty or missing chain cannot pass.
//
// MUTATION: put MatchSourceAddress back ahead of MatchProtocol in listenPortTerm
// (internal/component/gtsm/gtsm.go): the IPv4 range's listen rule
// dangerous-10-0-0-0-8-listen then opens with `meta nfproto ipv4`, and the test
// reports that rule judged unsafe (4) where its name owes an exclusion (1), then
// fails with the ruleset blocked at that rule's drop.
func TestESPFormFirewallStockGTSMRangesKeepTheFastPathOn(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	tables := gtsm.FilterTables(nil, []gtsm.Range{
		{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Port: 179, Floor: 255},
		{Prefix: netip.MustParsePrefix("10.1.0.0/16"), Port: 179},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), Port: 179, Floor: 255},
		{Prefix: netip.MustParsePrefix("2001:db8:1::/48"), Port: 179},
	})
	if len(tables) != 1 || len(tables[0].Chains) != 1 {
		t.Fatalf("gtsm.FilterTables built %d tables, want one table with one chain: %+v", len(tables), tables)
	}
	reader := espFWStockLowered(t, tables[0])

	want := tables[0].Chains[0]
	rules := espFWStockRules(t, reader, nftables.TableFamilyINet, tables[0].Name, want.Name)
	if len(rules) != len(want.Terms) {
		t.Fatalf("the kernel holds %d rules in %s %s, want one for each of the producer's %d terms",
			len(rules), tables[0].Name, want.Name, len(want.Terms))
	}
	seen := make(map[espFormRuleKind]int)
	for i, r := range rules {
		term := want.Terms[i]
		if name := string(r.UserData); name != term.Name {
			t.Fatalf("rule %d (handle %d) carries the name %q, want the producer's term %q in order", i, r.Handle, name, term.Name)
		}
		wantKind := espFWStockGTSMKind(t, term.Name)
		got := espFormJudgeRule(nftables.TableFamilyINet, r.Exprs)
		if got.kind != wantKind {
			t.Errorf("rule %q is judged %d (%s), want %d: a claim with an IPv4 source passes, every other GTSM rule is excluded",
				term.Name, got.kind, got.reason, wantKind)
		}
		seen[got.kind]++
	}
	if seen[espFormRuleExcluded] == 0 {
		t.Errorf("no rule was judged excluded: the ranges' listen drops are missing")
	}
	if seen[espFormRulePasses] == 0 {
		t.Errorf("no rule was judged to pass: the IPv4 claim is missing")
	}

	blocked, site, err := espFormFirewallBlocks(reader)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: %v", err)
	}
	if blocked {
		t.Fatalf("stock gtsm for dynamic-group ranges keeps the ESP form fast path off at %s", site)
	}
}

// Name prefixes of the GTSM terms (internal/component/gtsm/gtsm.go): a range's
// listen-port drop is named by listenTermName, a claim by claimTermName.
const (
	espFWStockGTSMListen = "dangerous-"
	espFWStockGTSMClaim  = "claimed-"
	// espFWStockGTSMClaim4 opens the name of the claim for this test's one IPv4 claim
	// source, 10.1.0.0/16, whose dots the name spells as hyphens.
	espFWStockGTSMClaim4 = espFWStockGTSMClaim + "10-1-0-0-"
)

// espFWStockGTSMKind answers the judgment the GTSM term named name owes, from its name
// and the test's own ranges, never from the producer's matches, so a producer that
// reorders a term's matches meets an expectation it did not write. A listen-port drop
// owes an exclusion: it must open with its protocol. The IPv4 claim owes a pass: its
// source lowers to `meta nfproto ipv4` first, which IPv4 ESP matches, and its
// selectors and accept are harmless. The IPv6 claim opens with `meta nfproto ipv6` and
// owes an exclusion. A name of neither kind fails the test.
func espFWStockGTSMKind(t *testing.T, name string) espFormRuleKind {
	t.Helper()
	switch {
	case strings.HasPrefix(name, espFWStockGTSMListen):
		return espFormRuleExcluded
	case strings.HasPrefix(name, espFWStockGTSMClaim4):
		return espFormRulePasses
	case strings.HasPrefix(name, espFWStockGTSMClaim):
		return espFormRuleExcluded
	}
	t.Fatalf("the GTSM term %q is neither a listen-port drop (%s) nor a claim (%s)", name, espFWStockGTSMListen, espFWStockGTSMClaim)
	return espFormRuleUnspecified
}

// espFWApplyStock lowers and commits a producer's tables through the nft firewall
// backend, in the calling test's namespace, as the firewall registry's ApplyAll
// would.
func espFWApplyStock(t *testing.T, tables []firewall.Table) {
	t.Helper()
	if err := firewall.LoadBackend("nft"); err != nil {
		t.Fatalf("load the nft firewall backend: %v", err)
	}
	t.Cleanup(func() {
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("close the nft firewall backend: %v", err)
		}
	})
	backend := firewall.GetBackend()
	if backend == nil {
		t.Fatal("the nft firewall backend loaded, but no backend is active")
	}
	if err := backend.Apply(tables); err != nil {
		t.Fatalf("apply the producer's tables through the nft backend: %v", err)
	}
}

// espFWStockRules reads back, through the analyzer's own reader, the rules of one
// base chain the kernel holds.
func espFWStockRules(t *testing.T, r espFormNFTReader, family nftables.TableFamily, table, chain string) []*nftables.Rule {
	t.Helper()
	chains, err := r.ListChains()
	if err != nil {
		t.Fatalf("list the nftables chains: %v", err)
	}
	for _, c := range chains {
		if c.Table == nil || c.Table.Family != family || c.Table.Name != table || c.Name != chain {
			continue
		}
		rules, err := r.GetRules(c.Table, c)
		if err != nil {
			t.Fatalf("dump the rules of %s %s: %v", table, chain, err)
		}
		return rules
	}
	t.Fatalf("the kernel holds no chain %s in table %s", chain, table)
	return nil
}

// espFWStockVictim is the victim the DDoS proofs mitigate for.
var espFWStockVictim = netip.MustParsePrefix("192.0.2.1/32")

// TestESPFormFirewallStockDDoSNamedProtocolKeepsTheFastPathOn proves that an armed
// on-host DDoS mitigation for a vector whose protocol has a name leaves the ESP form
// fast path on. Goal: the drop ddos-local installs reads as one bare IPv4 ESP never
// matches. Method: local.MitigationTable builds the table applyMitigation installs
// for a UDP flood on the input hook, the nft backend lowers and commits it in the
// test's own namespace, and the kernel's copy is read back and judged rule by rule
// (excluded) and as a whole ruleset (not blocked), with the rule count checked
// against the producer's term count.
//
// MUTATION: put MatchDestinationAddress ahead of MatchProtocol in buildDropTerm
// (internal/plugins/ddos/local/match.go): the rule then opens with the victim
// address load, is judged unsafe at its drop, and the ruleset reads blocked.
func TestESPFormFirewallStockDDoSNamedProtocolKeepsTheFastPathOn(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	table := ddoslocal.MitigationTable(ddosevent.VectorTuple{DstPrefix: espFWStockVictim, Proto: 17, DstPort: 53}, firewall.HookInput)
	reader := espFWStockLowered(t, table)

	rules := espFWStockRules(t, reader, nftables.TableFamilyIPv4, table.Name, table.Chains[0].Name)
	if len(rules) != len(table.Chains[0].Terms) {
		t.Fatalf("the kernel holds %d rules in %s %s, want one for each of the producer's %d terms",
			len(rules), table.Name, table.Chains[0].Name, len(table.Chains[0].Terms))
	}
	for _, r := range rules {
		got := espFormJudgeRule(nftables.TableFamilyIPv4, r.Exprs)
		if got.kind != espFormRuleExcluded {
			t.Errorf("rule handle %d %q is judged %d (%s), want excluded: its first two expressions must compare the protocol with one other than ESP",
				r.Handle, espFormRuleComment(r.UserData), got.kind, got.reason)
		}
	}
	blocked, site, err := espFormFirewallBlocks(reader)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: %v", err)
	}
	if blocked {
		t.Fatalf("a stock ddos mitigation for udp keeps the ESP form fast path off at %s", site)
	}
}

// TestESPFormFirewallStockDDoSUnnamedProtocolClosesTheFastPath proves the permitted
// survivor: a mitigation for a vector whose protocol has no canonical name carries
// no protocol match, so it can drop ESP sent to the victim, and the fast path stays
// off at that drop. Method: as the named-protocol proof, for protocol 253; the walk
// must read blocked, at the ddos-local chain on the input hook, for a drop.
//
// MUTATION: in espFormJudgeVerdict, treat VerdictDrop like accept/break/continue: the
// rule passes, the policy is accept, the walk reads not blocked.
func TestESPFormFirewallStockDDoSUnnamedProtocolClosesTheFastPath(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	table := ddoslocal.MitigationTable(ddosevent.VectorTuple{DstPrefix: espFWStockVictim, Proto: 253, DstPort: 53}, firewall.HookInput)
	reader := espFWStockLowered(t, table)

	if rules := espFWStockRules(t, reader, nftables.TableFamilyIPv4, table.Name, table.Chains[0].Name); len(rules) != 1 {
		t.Fatalf("the kernel holds %d rules in %s %s, want the one drop", len(rules), table.Name, table.Chains[0].Name)
	}
	blocked, site, err := espFormFirewallBlocks(reader)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: %v", err)
	}
	if !blocked {
		t.Fatal("a ddos mitigation with no protocol match left the ESP form fast path on, though it can drop ESP to the victim")
	}
	want := espFormFirewallSite{Family: "ip", Table: table.Name, Chain: table.Chains[0].Name, Hook: "input", Reason: espFormReasonDrop}
	got := site
	got.Rule = ""
	if got != want || !strings.HasPrefix(site.Rule, "handle ") {
		t.Fatalf("site = %+v, want %+v at a rule handle", site, want)
	}
}

// TestESPFormFirewallStockDDoSESPVectorClosesTheFastPath proves that an on-host DDoS
// mitigation against ESP itself keeps the ESP form fast path off, as the guide's
// "Both ESP wire forms" says. Goal: the drop ddos-local installs for an ESP flood
// opens with a protocol load compared with 50, which every bare IPv4 ESP packet
// matches, so the rule is judged unsafe at its drop and the walk reads blocked at the
// ddos-local chain. Method: as the named-protocol proof, for protocol 50 with no
// port; the kernel's one rule must open with that protocol compare, be judged unsafe
// for a drop, and the whole ruleset must read blocked at that chain on the input hook.
//
// MUTATION: in espFormExcludesESP, exclude on a compare equal to 50 too (answer true
// for every one-octet equality compare of the load, ESP's value included): the rule is
// then excluded and the ruleset reads not blocked.
func TestESPFormFirewallStockDDoSESPVectorClosesTheFastPath(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	table := ddoslocal.MitigationTable(ddosevent.VectorTuple{DstPrefix: espFWStockVictim, Proto: 50}, firewall.HookInput)
	reader := espFWStockLowered(t, table)

	rules := espFWStockRules(t, reader, nftables.TableFamilyIPv4, table.Name, table.Chains[0].Name)
	if len(rules) != 1 {
		t.Fatalf("the kernel holds %d rules in %s %s, want the one drop", len(rules), table.Name, table.Chains[0].Name)
	}
	exprs := rules[0].Exprs
	if len(exprs) < 2 {
		t.Fatalf("the drop holds %d expressions, want a protocol load and its compare first", len(exprs))
	}
	load, register := espFormFirstLoad(nftables.TableFamilyIPv4, exprs[0])
	compare, compared := exprs[1].(*expr.Cmp)
	if load != espFormLoadProtocol || !compared {
		t.Fatalf("the drop opens with %#v, %#v, want a protocol load and its compare", exprs[0], exprs[1])
	}
	if compare.Op != expr.CmpOpEq || compare.Register != register || !bytes.Equal(compare.Data, espFormProtoESP) {
		t.Fatalf("the drop's first compare is %#v, want equality of the loaded protocol with ESP (50)", compare)
	}
	if got := espFormJudgeRule(nftables.TableFamilyIPv4, exprs); got.kind != espFormRuleUnsafe || got.reason != espFormReasonDrop {
		t.Errorf("the drop is judged %d (%s), want unsafe for %s", got.kind, got.reason, espFormReasonDrop)
	}

	blocked, site, err := espFormFirewallBlocks(reader)
	if err != nil {
		t.Fatalf("espFormFirewallBlocks: %v", err)
	}
	if !blocked {
		t.Fatal("a ddos mitigation against ESP left the ESP form fast path on, though it drops ESP to the victim")
	}
	want := espFormFirewallSite{Family: "ip", Table: table.Name, Chain: table.Chains[0].Name, Hook: "input", Reason: espFormReasonDrop}
	got := site
	got.Rule = ""
	if got != want || !strings.HasPrefix(site.Rule, "handle ") {
		t.Fatalf("site = %+v, want %+v at a rule handle", site, want)
	}
}

// espFWStockLowered commits one producer table through the nft backend and opens
// the analyzer's reader on the namespace that now holds it.
func espFWStockLowered(t *testing.T, table firewall.Table) espFormNFTReader {
	t.Helper()
	espFWApplyStock(t, []firewall.Table{table})
	reader, closeReader, err := openESPFormNFTReader()
	if err != nil {
		t.Fatalf("open the nftables reader: %v", err)
	}
	t.Cleanup(func() {
		if err := closeReader(); err != nil {
			t.Errorf("close the nftables reader: %v", err)
		}
	})
	return reader
}
