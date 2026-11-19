// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA
// Related: icmp_error_return.go -- the producer these units judge
// RFC naming: untagged -- builder and ordering units; the packet proof is TestRFC4301ForwardedICMPErrorsMapUnderDiscard.

package engine

import (
	"errors"
	"math"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// icmpReturnDP records every install and removal by full selector, in one log shared
// with the recording kernel, so a test can judge the order of the two.
type icmpReturnDP struct {
	*forwardDefaultTestDP
	held       map[spdSelector]dataplane.SPParams
	log        *[]string
	installErr error
}

func newICMPReturnDP(log *[]string, xfrm bool) *icmpReturnDP {
	return &icmpReturnDP{
		forwardDefaultTestDP: &forwardDefaultTestDP{spdDP: newSPDDP(), enabled: xfrm},
		held:                 make(map[spdSelector]dataplane.SPParams),
		log:                  log,
	}
}

func (d *icmpReturnDP) InstallPolicy(p dataplane.SPParams) error {
	if d.installErr != nil {
		return d.installErr
	}
	d.held[selectorOf(&p)] = p
	*d.log = append(*d.log, "install "+p.Dst.String())
	return nil
}

func (d *icmpReturnDP) RemovePolicyParams(p dataplane.SPParams) error {
	delete(d.held, selectorOf(&p))
	*d.log = append(*d.log, "remove "+p.Dst.String())
	return nil
}

type icmpReturnKernel struct {
	log    *[]string
	tables [][]firewall.Table
	err    error
}

func (k *icmpReturnKernel) publish(tables []firewall.Table) error {
	*k.log = append(*k.log, "publish")
	k.tables = append(k.tables, tables)
	return k.err
}

func icmpReturnConfig(t *testing.T) *ipsec.IPsecConfig {
	t.Helper()
	selector := func(local, remote string) []ipsec.TrafficSelectorPolicy {
		return []ipsec.TrafficSelectorPolicy{{LocalPrefix: icmpReturnNet(t, local), RemotePrefix: icmpReturnNet(t, remote)}}
	}
	return &ipsec.IPsecConfig{
		Unmatched: dataplane.SPActionDiscard,
		Peers: map[string]ipsec.SiteToSitePeer{
			"mapped":   {TrafficSelectors: selector("10.2.0.0/24", "10.1.0.0/24")},
			"v6":       {TrafficSelectors: selector("2001:db8:2::/64", "2001:db8:1::/64")},
			"no-check": {TrafficSelectors: selector("10.2.0.0/24", "10.9.0.0/24"), SkipICMPErrorCheck: true},
			"vti":      {TrafficSelectors: selector("10.2.0.0/24", "10.8.0.0/24"), VTIBind: "vti0"},
		},
	}
}

// VALIDATES: under `unmatched discard` the mapping covers the remote selectors of the
// peers whose Child SAs carry XFRM_POLICY_ICMP, and nothing under `unmatched bypass`.
// PREVENTS: an entry towards a peer whose out policy the kernel cannot map into (check
// off, or a vti if_id), which would pass the error to a table that then drops it, or an
// entry under a bypass catch-all, which already carries the error.
func TestICMPErrorReturnDestinationsFollowTheMappingPeers(t *testing.T) {
	cfg := icmpReturnConfig(t)
	got := icmpErrorReturnDestinations(cfg)
	want := []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24"), netip.MustParsePrefix("2001:db8:1::/64")}
	if !slices.Equal(got, want) {
		t.Fatalf("destinations = %v, want %v", got, want)
	}
	cfg.Unmatched = dataplane.SPActionBypass
	if got := icmpErrorReturnDestinations(cfg); got != nil {
		t.Fatalf("under unmatched bypass the destinations are %v, want none", got)
	}
}

// VALIDATES: one out ALLOW per forward-mapped error type (IPv4 3 and 11; IPv6 1, 2
// and 3), from any source, ranked beneath every operator order and Child SA priority
// and above the catch-all. An operator entry holding one entry's selector keeps it.
// PREVENTS: an entry that outranks an SA or entry carrying the error, or one for a
// type xfrm_out_fwd_icmp does not map, which would leave in clear.
func TestICMPErrorReturnEntriesRankJustAboveTheCatchAll(t *testing.T) {
	destinations := icmpErrorReturnDestinations(icmpReturnConfig(t))
	policies := icmpErrorReturnPolicies(destinations, nil)
	var v4, v6 []uint16
	for i := range policies {
		p := &policies[i]
		if p.Dir != dataplane.SADirOut || p.Action != dataplane.SPActionBypass || p.Role != dataplane.PolicyRoleICMPErrorReturn {
			t.Fatalf("entry %d is dir %d action %d role %v, want an out bypass of role icmp-error-return", i, p.Dir, p.Action, p.Role)
		}
		if p.Priority <= math.MaxInt32 || p.Priority >= dataplane.PriorityUnmatched {
			t.Fatalf("entry %d ranks %d, want beneath every operator order and above the catch-all", i, p.Priority)
		}
		if ones, _ := p.Src.Mask.Size(); ones != 0 || !p.DstPort.IsAny() || p.SrcPort.Mask != 0xffff {
			t.Fatalf("entry %d selects source %v, type %+v, code %+v; want any source, one exact type, any code", i, p.Src, p.SrcPort, p.DstPort)
		}
		if p.UpperProto == dataplane.UpperProtoICMP {
			v4 = append(v4, p.SrcPort.Port)
		} else {
			v6 = append(v6, p.SrcPort.Port)
		}
	}
	if !slices.Equal(v4, []uint16{3, 11}) || !slices.Equal(v6, []uint16{1, 2, 3}) {
		t.Fatalf("types = v4 %v v6 %v, want v4 [3 11] v6 [1 2 3]", v4, v6)
	}
	operator := map[string]ipsec.SPDPolicy{"same": {
		Name: "same", Action: dataplane.SPActionDiscard, Order: 500, Direction: ipsec.SPDDirOut,
		Protocol: dataplane.UpperProtoICMP, LocalPrefix: icmpReturnNet(t, "0.0.0.0/0"), LocalPort: ipsec.AnyPort(),
		RemotePrefix: icmpReturnNet(t, "10.1.0.0/24"), RemotePort: ipsec.AnyPort(),
		ICMP: ipsec.ICMPSelector{HasType: true, Type: 3},
	}}
	kept := icmpErrorReturnPolicies(destinations, operator)
	if len(kept) != len(policies)-1 {
		t.Fatalf("beside an operator entry on one entry's selector %d entries are built, want %d", len(kept), len(policies)-1)
	}
}

// VALIDATES: each chain accepts an error whose dst is an XFRM bundle and then drops the
// same error, in forward and in output, for every entry.
// PREVENTS: a table that drops a mapped error, or one that lets an unmapped one leave.
func TestICMPErrorReturnTableDropsOnlyWhatLeavesUnprotected(t *testing.T) {
	policies := icmpErrorReturnPolicies([]netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")}, nil)
	tables := icmpErrorReturnTables(policies)
	if len(tables) != 1 || tables[0].Family != firewall.FamilyIP || len(tables[0].Chains) != 2 {
		t.Fatalf("tables = %+v, want one ip table with two chains", tables)
	}
	for _, chain := range tables[0].Chains {
		if chain.Hook != firewall.HookForward && chain.Hook != firewall.HookOutput {
			t.Fatalf("chain %q hooks %v, want forward or output", chain.Name, chain.Hook)
		}
		if len(chain.Terms) != 2*len(policies) {
			t.Fatalf("chain %q holds %d terms, want %d", chain.Name, len(chain.Terms), 2*len(policies))
		}
		for i := 0; i < len(chain.Terms); i += 2 {
			accept, drop := chain.Terms[i], chain.Terms[i+1]
			if _, ok := accept.Actions[0].(firewall.Accept); !ok || !slices.Contains(accept.Matches, firewall.Match(firewall.MatchIPsecTransform{})) {
				t.Fatalf("chain %q term %q is not the transformed accept", chain.Name, accept.Name)
			}
			if _, ok := drop.Actions[0].(firewall.Drop); !ok || slices.Contains(drop.Matches, firewall.Match(firewall.MatchIPsecTransform{})) {
				t.Fatalf("chain %q term %q is not the untransformed drop", chain.Name, drop.Name)
			}
		}
	}
}

// VALIDATES: the table is published before any entry is installed, a refused table
// leaves no entry and is reported, and a release removes every entry before the table
// is withdrawn.
// PREVENTS: an instant where an entry passes an unmapped error with no drop behind it.
func TestICMPErrorReturnPublishesBeforeInstallAndWithdrawsAfterRemove(t *testing.T) {
	var log []string
	dp := newICMPReturnDP(&log, true)
	kern := &icmpReturnKernel{log: &log}
	mapping := newICMPErrorReturns(kern)
	cfg := icmpReturnConfig(t)
	mapping.apply(dp, cfg, slogutil.DiscardLogger())
	if len(log) == 0 || log[0] != "publish" || len(dp.held) != 5 {
		t.Fatalf("apply did %q holding %d entries, want the publish first and 5 entries", log, len(dp.held))
	}
	log = nil
	mapping.release(dp, slogutil.DiscardLogger())
	if len(dp.held) != 0 || log[len(log)-1] != "publish" || kern.tables[len(kern.tables)-1] != nil {
		t.Fatalf("release did %q holding %d entries, want every removal then the table withdrawn", log, len(dp.held))
	}

	kern.err = errors.New("nf_tables refused")
	mapping.apply(dp, cfg, slogutil.DiscardLogger())
	if len(dp.held) != 0 {
		t.Fatalf("a refused table left %d entries installed, want none", len(dp.held))
	}
	if answer := answerICMPErrorReturnOf(mapping); len(answer) != 1 {
		t.Fatalf("a refused table drew %d doctor findings, want 1", len(answer))
	}

	other := newICMPReturnDP(&log, false)
	log = nil
	fresh := newICMPErrorReturns(&icmpReturnKernel{log: &log})
	fresh.apply(other, cfg, slogutil.DiscardLogger())
	if len(other.held) != 0 || len(log) != 0 {
		t.Fatalf("a dataplane without Linux's forwarding path got %q and %d entries, want nothing", log, len(other.held))
	}
}

// answerICMPErrorReturnOf reads one mapping's findings the way answerICMPErrorReturn
// reads the process's.
func answerICMPErrorReturnOf(r *icmpErrorReturns) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.problems)
}

// icmpReturnNet parses a prefix for these units, which build on every platform.
func icmpReturnNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, prefix, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %s: %v", cidr, err)
	}
	return prefix
}
