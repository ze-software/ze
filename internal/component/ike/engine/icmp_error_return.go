// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ICMP error messages carried by a Child SA
// Related: unmatched.go -- the out catch-all these entries rank above
// Related: icmp_overlap.go -- refusal A, which keeps every bypass that would carry these errors out of the configuration
// Related: register_tables.go -- the reservation of this producer's nft table name
// Related: doctor_runtime.go -- engineDoctorChecks, where ipsec-icmp-error-return is declared
// RFC: rfc/short/rfc4301.md -- Section 6.2, ICMP error processing

package engine

import (
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A forwarded ICMP error under `unmatched discard` is mapped into the return SA only
// after the out lookup passed it untransformed. __xfrm_route_forward
// (net/xfrm/xfrm_policy.c, Linux 7.3) calls xfrm_out_fwd_icmp only when xfrm_lookup
// answered a dst with no transform (`if (dst && !dst->xfrm)`), and the out catch-all
// BLOCK answers -EPERM first (xfrm_lookup_with_ifid, `num_xfrms < 0`). So Ze installs,
// one place above the catch-all, a template-free ALLOW entry for each ICMP error type
// the forward mapping handles, towards each remote selector of a peer whose Child SAs
// carry XFRM_POLICY_ICMP. The error then reaches xfrm_out_fwd_icmp, whose
// XFRM_LOOKUP_ICMP lookup of the reversed embedded header swaps in the Child SA's
// bundle when its out policy matches.
//
// An error the mapping does not take leaves the lookup untransformed, and the catch-all
// meant it to be discarded. The nft table ze_ipsec_icmp_error_return drops it: one
// term per entry, in the forward chain (ip_forward reaches NF_INET_FORWARD after
// xfrm4_route_forward, net/ipv4/ip_forward.c) and in the output chain, because the
// local route lookup (icmp_route_lookup, net/ipv4/icmp.c; icmpv6_route_lookup,
// net/ipv6/icmp.c) sends an error whose reverse lookup fails on the plain route it
// found first, and the entry makes that route a pass rather than -EPERM. The term
// accepts a packet whose dst is an XFRM bundle (nft `rt ipsec exists`, nft_rt_get_eval
// NFT_RT_XFRM stores `!!dst->xfrm`) and drops the rest.
//
// No operator bypass is dropped by the table. Refusal A (refuseICMPErrorBypass) refuses
// every out or both bypass that would carry these error types towards such a peer's
// remote selectors, and `unmatched bypass` beside such a peer, so the entries and the
// terms cover only destinations where the SPD's answer was discard. An operator entry
// installing the very selector of an entry keeps its selector: the database holds one
// entry per selector, so no entry is built there (icmpErrorReturnPolicies).
//
// The table is published BEFORE the entries are installed and withdrawn AFTER they are
// removed, so no instant holds an entry without its drop. A table that cannot be
// published leaves no entry installed: the catch-all then discards the error, which
// loses the mapping and leaks nothing, and the doctor check ipsec-icmp-error-return
// reports it.

const (
	// icmpErrorReturnOwner is the firewall registry owner of the drop table.
	icmpErrorReturnOwner = "ike-icmp-error-return"
	// icmpErrorReturnTableName is the drop table; the ze_ prefix is the ownership mark
	// the firewall registry requires (firewall.RegisterTables).
	icmpErrorReturnTableName = "ze_ipsec_icmp_error_return"
)

// icmpForwardMappedTypesV4 and icmpForwardMappedTypesV6 are the ICMP error types
// xfrm_out_fwd_icmp maps: icmp_err_packet (net/xfrm/xfrm_policy.c, Linux 7.3) answers
// true for ICMP_DEST_UNREACH (3) and ICMP_TIME_EXCEEDED (11) in IPv4, and for
// ICMPV6_DEST_UNREACH (1), ICMPV6_PKT_TOOBIG (2) and ICMPV6_TIME_EXCEED (3) in IPv6.
// A forwarded error of any other type gets no entry, so the catch-all still discards
// it: an entry for a type the kernel does not map would pass it in clear.
var (
	icmpForwardMappedTypesV4 = []uint8{3, 11}
	icmpForwardMappedTypesV6 = []uint8{1, 2, 3}
)

// icmpErrorReturnDestinations answers the remote selectors towards which an ICMP error
// must be mapped rather than discarded, sorted and without repeats: those of every peer
// whose icmp-error-check is true and that has no `vti bind`, when the catch-all is
// `unmatched discard`. Under `unmatched bypass` the catch-all carries the error itself,
// and refusal A refuses that beside such a peer. A `vti bind` peer's out policy carries
// its interface's if_id, and xfrm_out_fwd_icmp looks up at if_id 0 (xfrm_lookup), so no
// error can be mapped into it. The answer is nil when nothing is to be mapped.
func icmpErrorReturnDestinations(cfg *ipsec.IPsecConfig) []netip.Prefix {
	if cfg == nil {
		return nil
	}
	if cfg.Unmatched != dataplane.SPActionDiscard {
		return nil
	}
	var out []netip.Prefix
	for _, name := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[name]
		if peer.SkipICMPErrorCheck {
			continue
		}
		if peer.VTIBind != "" {
			continue
		}
		for _, span := range peerRemoteSpans(peer.TrafficSelectors) {
			if !slices.Contains(out, span) {
				out = append(out, span)
			}
		}
	}
	slices.SortFunc(out, comparePrefix)
	return out
}

// comparePrefix orders prefixes by address, then by length, so a plan is stable.
func comparePrefix(a, b netip.Prefix) int {
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}
	return a.Bits() - b.Bits()
}

// icmpErrorReturnPolicies builds the out ALLOW entries for the destinations: one per
// destination and per forward-mapped error type of its family, from any source, at
// dataplane.PriorityICMPErrorReturn. An entry whose selector an operator entry of the
// configuration installs is left out, because the database holds one entry per
// selector and the operator's entry would replace it, or it the operator's.
func icmpErrorReturnPolicies(destinations []netip.Prefix, operator map[string]ipsec.SPDPolicy) []dataplane.SPParams {
	taken := make([]dataplane.SPParams, 0, 2*len(operator))
	for _, name := range slices.Sorted(maps.Keys(operator)) {
		taken = append(taken, spdPolicyParams(operator[name])...)
	}
	out := make([]dataplane.SPParams, 0, 3*len(destinations))
	for _, dest := range destinations {
		proto, types := dataplane.UpperProtoICMP, icmpForwardMappedTypesV4
		source := anyNetFor(net.IPv4zero)
		if dest.Addr().Is6() {
			proto, types = dataplane.UpperProtoICMPv6, icmpForwardMappedTypesV6
			source = anyNetFor(net.IPv6zero)
		}
		for _, typ := range types {
			// RFC 4301 Section 6.2: "If no SA exists that would carry the outbound ICMP
			// message in question, and if no SPD entry would allow carriage of this
			// outbound ICMP error message, then an IPsec implementation MUST map the
			// message to the SA that would carry the return traffic associated with the
			// packet that triggered the ICMP error message." The entry ranks below every
			// Child SA policy and operator entry, so an SA or entry that carries the
			// error still does; only the catch-all's discard is replaced by the mapping.
			p := dataplane.SPParams{
				Src:        source,
				Dst:        &net.IPNet{IP: net.IP(dest.Addr().AsSlice()), Mask: net.CIDRMask(dest.Bits(), dest.Addr().BitLen())},
				Dir:        dataplane.SADirOut,
				Action:     dataplane.SPActionBypass,
				Role:       dataplane.PolicyRoleICMPErrorReturn,
				Priority:   dataplane.PriorityICMPErrorReturn,
				UpperProto: proto,
				SrcPort:    dataplane.ExactPortMatch(uint16(typ)),
				DstPort:    dataplane.AnyPortMatch(),
			}
			if spdPoliciesShareASelector([]dataplane.SPParams{p}, taken) {
				continue
			}
			out = append(out, p)
		}
	}
	return out
}

// icmpErrorReturnTables builds the drop table for the entries: an ip and an ip6 table
// of that name, each with a forward and an output base chain that accept by policy, so
// they decide only what their terms name. For each entry the chain accepts the error
// when its dst is an XFRM bundle (it was mapped, or a Child SA carries it) and drops it
// otherwise. The answer is nil when there is no entry, which withdraws the table.
func icmpErrorReturnTables(policies []dataplane.SPParams) []firewall.Table {
	var v4, v6 []firewall.Term
	for i := range policies {
		p := &policies[i]
		dest, ok := netipPrefix(p.Dst)
		if !ok {
			continue
		}
		typ := uint8(p.SrcPort.Port)
		n := strconv.Itoa(i)
		var icmp firewall.Match = firewall.MatchICMPType{Type: typ}
		if dest.Addr().Is6() {
			icmp = firewall.MatchICMPv6Type{Type: typ}
		}
		target := firewall.MatchDestinationAddress{Prefix: dest}
		terms := []firewall.Term{{
			Name:    "icmp-error-" + n + "-mapped",
			Matches: []firewall.Match{target, icmp, firewall.MatchIPsecTransform{}},
			Actions: []firewall.Action{firewall.Accept{}},
		}, {
			Name:    "icmp-error-" + n + "-unmapped",
			Matches: []firewall.Match{target, icmp},
			Actions: []firewall.Action{firewall.Drop{}},
		}}
		if dest.Addr().Is6() {
			v6 = append(v6, terms...)
			continue
		}
		v4 = append(v4, terms...)
	}
	var tables []firewall.Table
	if len(v4) > 0 {
		tables = append(tables, icmpErrorReturnTable(firewall.FamilyIP, v4))
	}
	if len(v6) > 0 {
		tables = append(tables, icmpErrorReturnTable(firewall.FamilyIP6, v6))
	}
	return tables
}

// icmpErrorReturnTable is one family's drop table, the same terms in the forward and
// the output chain.
func icmpErrorReturnTable(family firewall.TableFamily, terms []firewall.Term) firewall.Table {
	chain := func(name string, hook firewall.ChainHook) firewall.Chain {
		return firewall.Chain{
			Name:     name,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     hook,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms:    slices.Clone(terms),
		}
	}
	return firewall.Table{
		Name:   icmpErrorReturnTableName,
		Family: family,
		Chains: []firewall.Chain{chain("forward", firewall.HookForward), chain("output", firewall.HookOutput)},
	}
}

// icmpErrorReturnKernel publishes the drop table. A test stands a recorder in its
// place; production never replaces kernelICMPErrorReturn.
type icmpErrorReturnKernel interface {
	publish(tables []firewall.Table) error
}

type kernelICMPErrorReturn struct{}

// publish registers the table and reconciles the firewall. The registry is
// process-local, which holds: the IKE engine is a component and never runs forked.
func (kernelICMPErrorReturn) publish(tables []firewall.Table) error {
	if err := firewall.RegisterTables(icmpErrorReturnOwner, tables); err != nil {
		return err
	}
	return firewall.ApplyAll()
}

// icmpErrorReturns is the installed mapping: the entries the dataplane holds and the
// problems the last apply met. The apply path (under applyMu) and the doctor check
// both reach it, so mu guards every field. Safe for concurrent use.
type icmpErrorReturns struct {
	mu   sync.Mutex
	kern icmpErrorReturnKernel
	// installed are the entries the dataplane took, in install order.
	installed []dataplane.SPParams
	// published says the drop table the last successful publish left covers installed.
	published bool
	// problems are the findings of the last apply or release, in the order met.
	problems []string
}

//nolint:gochecknoglobals // One mapping per process: the SPD and the nft table are per namespace.
var icmpReturns = newICMPErrorReturns(kernelICMPErrorReturn{})

func newICMPErrorReturns(kern icmpErrorReturnKernel) *icmpErrorReturns {
	return &icmpErrorReturns{kern: kern}
}

// icmpErrorReturnFix is the command that retries every step of the mapping: a reload
// runs the apply again.
const icmpErrorReturnFix = "ze signal reload"

// apply makes the dataplane hold the entries cfg needs and nothing else, the drop
// table first. It runs after the catch-all is installed (applyConfig). It refuses
// nothing: every failure leaves the error discarded by the catch-all, which is safe,
// and is logged and reported by the doctor check.
//
// A dataplane that does not run Linux's forwarding path gets no entry: the XFRM
// backend is the one that declares the namespace forward default
// (forwardDefaultInstaller), and it is the one whose forwarded packets meet
// __xfrm_route_forward. An entry on another backend would carry the error in clear
// with no table to drop it.
func (r *icmpErrorReturns) apply(dp dataplane.Dataplane, cfg *ipsec.IPsecConfig, log *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.problems = nil
	var want []dataplane.SPParams
	if _, xfrm := forwardDefaultInstaller(dp); xfrm && dp != nil {
		// RFC 4301 Section 6.2
		want = icmpErrorReturnPolicies(icmpErrorReturnDestinations(cfg), cfg.Policies)
	}
	if len(want) == 0 {
		r.releaseLocked(dp, log)
		return
	}
	// The registry holds the table from here whatever the reconcile answers, so a
	// later release withdraws it.
	r.published = true
	if err := r.kern.publish(icmpErrorReturnTables(want)); err != nil {
		r.problem(log, "Ze could not publish the nft table "+icmpErrorReturnTableName+
			", so no forwarded ICMP error is mapped into its return Child SA and the SPD catch-all discards it", err)
		r.removeLocked(dp, nil, log)
		return
	}
	held := make([]dataplane.SPParams, 0, len(want))
	for i := range want {
		if err := dp.InstallPolicy(want[i]); err != nil {
			r.problem(log, "Ze could not install the ICMP error return entry towards "+want[i].Dst.String()+
				" for ICMP type "+strconv.Itoa(int(want[i].SrcPort.Port))+
				", so a forwarded error of that type is discarded rather than mapped into its return Child SA", err)
			continue
		}
		held = append(held, want[i])
	}
	r.removeLocked(dp, held, log)
}

// release removes every entry, then withdraws the drop table. It runs before the
// catch-all is removed (releaseSPD).
func (r *icmpErrorReturns) release(dp dataplane.Dataplane, log *slog.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.problems = nil
	r.releaseLocked(dp, log)
}

// releaseLocked removes the entries, and withdraws the table only once none is left:
// an entry still installed keeps the drop that makes it safe.
func (r *icmpErrorReturns) releaseLocked(dp dataplane.Dataplane, log *slog.Logger) {
	if !r.removeLocked(dp, nil, log) {
		return
	}
	if !r.published {
		return
	}
	if err := r.kern.publish(nil); err != nil {
		r.problem(log, "Ze could not withdraw the nft table "+icmpErrorReturnTableName+
			", so it still drops forwarded and local ICMP errors towards the remote selectors it names when they leave unprotected", err)
		return
	}
	r.published = false
}

// removeLocked removes every installed entry keep does not hold, and answers whether
// every one it tried is gone. One that stays is kept in installed, so the next apply
// or release tries it again and the table that makes it safe stays.
func (r *icmpErrorReturns) removeLocked(dp dataplane.Dataplane, keep []dataplane.SPParams, log *slog.Logger) bool {
	gone := true
	var stay []dataplane.SPParams
	for i := range r.installed {
		p := r.installed[i]
		if spdPoliciesShareASelector([]dataplane.SPParams{p}, keep) {
			continue
		}
		if dp == nil {
			stay = append(stay, p)
			gone = false
			continue
		}
		if err := dp.RemovePolicyParams(p); !policyGone(err) {
			r.problem(log, "Ze could not remove the ICMP error return entry towards "+p.Dst.String()+
				" for ICMP type "+strconv.Itoa(int(p.SrcPort.Port))+", so the drop table that keeps it safe stays", err)
			stay = append(stay, p)
			gone = false
		}
	}
	stay = append(stay, keep...)
	r.installed = stay
	return gone
}

// problem logs one failure at Error under the check's code and records it for the
// doctor check.
func (r *icmpErrorReturns) problem(log *slog.Logger, message string, err error) {
	log.Error("ike: "+message+"; the daemon needs CAP_NET_ADMIN and nf_tables, and ze signal reload retries it",
		"code", diagnostic.CodeDoctorIPsecICMPErrorReturn, "error", err)
	r.problems = append(r.problems, message+". The kernel answered: "+err.Error()+".")
}

// icmpErrorReturnCheckName names the check in the engine's Stage 1 registration and
// in the doctor callback's dispatch.
const icmpErrorReturnCheckName = "ipsec-icmp-error-return"

// icmpErrorReturnCheckDecl declares the check at the runtime phase: the record lives
// only in the engine's process.
var icmpErrorReturnCheckDecl = rpc.DoctorCheckDecl{
	Name:  icmpErrorReturnCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 747,
	Codes: []string{diagnostic.CodeDoctorIPsecICMPErrorReturn},
}

// answerICMPErrorReturn answers ipsec-icmp-error-return: one error per problem the
// last apply or release of the mapping met, in the order met.
func answerICMPErrorReturn(_ *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	icmpReturns.mu.Lock()
	defer icmpReturns.mu.Unlock()
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(icmpReturns.problems))
	for _, message := range icmpReturns.problems {
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecICMPErrorReturn,
			Severity: severityError,
			Message:  message,
			Fix:      icmpErrorReturnFix,
		})
	}
	return out
}
