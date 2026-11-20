// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_policy.go -- spdEntryHasTransit and spdForwardGathers, which project a port-scoped bypass only where the kernel gathers fragments
// Related: register.go -- OnConfigVerify, where a commit holding such an entry is refused
// Related: apply.go -- applyConfig, where the start logs such an entry instead
// Related: doctor_runtime.go -- engineDoctorChecks, where the ipsec-spd-forward-bypass check is declared
// RFC: rfc/short/rfc4301.md -- stateful fragment checking for a port-scoped BYPASS (Section 7.4)

package engine

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// spdForwardBypassCheckName names the check in the engine's Stage 1 registration and in
// the doctor callback's dispatch.
const spdForwardBypassCheckName = "ipsec-spd-forward-bypass" //nolint:gosec // G101: a doctor check name, not a credential

// spdForwardBypassCheckDecl declares the check at the runtime phase: it reads the
// entries the engine installed, the dataplane it loaded and the addresses this host
// holds now, all of which live in the engine's own process (ai/rules/plugins.md).
var spdForwardBypassCheckDecl = rpc.DoctorCheckDecl{
	Name:  spdForwardBypassCheckName,
	Phase: rpc.DoctorPhaseRuntime,
	Order: 747,
	Codes: []string{diagnostic.CodeDoctorIPsecSPDForwardBypass},
}

// spdSideLocal and spdSideRemote name the two sides of a `vpn ipsec policy` entry as the
// operator writes them (`set vpn ipsec policy <name> local ...`), for the fixes a
// refusal prints.
const (
	spdSideLocal  = "local"
	spdSideRemote = "remote"
)

// heldAddressesFn reads the addresses this host holds on its interfaces. Swapped in
// unit tests.
var heldAddressesFn = readHeldAddresses

// readHeldAddresses answers every interface address of this host's network namespace,
// unmapped and without a zone, so it compares with an entry's prefix address.
func readHeldAddresses() (map[netip.Addr]bool, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	held := make(map[netip.Addr]bool, len(addresses))
	for _, a := range addresses {
		// refuse-by-default: guards spdForwardBypassRefusals -- an address this cannot
		// read is not held, so an entry naming it is refused, never exempted.
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		held[addr.Unmap()] = true
	}
	return held, nil
}

// spdBypassHasPort reports whether p is a bypass with a port on either side: the entry
// spdEntryHasTransit projects onto the forward lookup only where the dataplane gathers
// fragments first, and spdForwardBypassRefusals refuses everywhere else.
func spdBypassHasPort(p ipsec.SPDPolicy) bool {
	if p.Action != dataplane.SPActionBypass {
		return false
	}
	if !p.LocalPort.IsAny() {
		return true
	}
	return !p.RemotePort.IsAny()
}

// spdLocalIsHeldAddress reports whether p's local prefix is one address this host
// holds. Such an entry names only traffic to and from Ze: a packet whose destination
// is that address is delivered here rather than forwarded, and one forwarded with it
// as its source is not this host's traffic.
func spdLocalIsHeldAddress(p ipsec.SPDPolicy, held map[netip.Addr]bool) bool {
	if p.LocalPrefix == nil {
		return false
	}
	ones, bits := p.LocalPrefix.Mask.Size()
	if ones != bits {
		return false
	}
	addr, ok := netip.AddrFromSlice(p.LocalPrefix.IP)
	if !ok {
		return false
	}
	return held[addr.Unmap()]
}

// spdForwardBypassRefusals answers one refusal for each operator bypass with a port
// that could match traffic this node forwards, on a dataplane whose forwarding path
// consults a forward lookup that runs before reassembly, in entry name order.
//
// RFC 4301 Section 7.4: "All implementations MUST support stateful fragment checking
// to accommodate BYPASS traffic for which a non-trivial port range is specified."
// Stock Linux checks a forwarded packet in FWD before any reassembly and reads port 0
// from a non-initial fragment (LL-XFRM-FRAGMENT-SELECTOR), so there spdEntryHasTransit
// projects no port-scoped bypass and the entry decides no forwarded traffic: the
// operator wrote a bypass that forwarded traffic on its ports never meets. Thomas
// decided on 2026-11-20 that such an entry is refused at commit, not reported in a
// notice, until the forward path performs the stateful check. Ze's kernel performs it
// (K3, capability bit 3: spdForwardGathers answers true), and there the entry is
// projected and nothing is refused.
//
// Four cases are not refused, because the entry does what it says there:
//   - No dataplane is loaded: nothing is installed.
//   - The dataplane applies its inbound SPD before routing (VPP): every entry it holds
//     decides forwarded traffic already (spdEntryPolicies).
//   - The dataplane's forward check sees whole datagrams (spdForwardGathers): the entry
//     is projected onto it (spdEntryHasTransit).
//   - The local prefix is one address this host holds: the entry names only traffic to
//     and from Ze, which is reassembled before the inbound check (ip_local_deliver) and
//     classified before it is fragmented on output.
//
// A gathering answer the dataplane could not read refuses, with the read failure as the
// refusal's cause: spdEntryPolicies projects no port-scoped bypass on it either.
func spdForwardBypassRefusals(dp dataplane.Dataplane, policies map[string]ipsec.SPDPolicy) []*opererr.Error {
	// refuse-by-default: guards verifyForwardBypass -- with no dataplane loaded no entry
	// is installed, so none can miss forwarded traffic, and the commit is accepted; the
	// start log and the doctor check report nothing either.
	if dp == nil {
		return nil
	}
	if !dp.RequiresForwardPolicy() {
		return nil
	}
	// RFC 4301 Section 7.4
	gathers, gatherErr := spdForwardGathers(dp)
	if gathers {
		return nil
	}
	var refusals []*opererr.Error
	var held map[netip.Addr]bool
	var heldErr error
	heldRead := false
	for _, name := range slices.Sorted(maps.Keys(policies)) {
		p := policies[name]
		if !spdBypassHasPort(p) {
			continue
		}
		if !heldRead {
			held, heldErr = heldAddressesFn()
			heldRead = true
		}
		if spdLocalIsHeldAddress(p, held) {
			continue
		}
		refusals = append(refusals, spdForwardBypassRefusal(p, gatherErr, heldErr))
	}
	return refusals
}

// spdForwardBypassRefusal is the refusal of one port-scoped bypass that forwarded
// traffic can match. gatherErr is why the dataplane could not say whether its forward
// check sees whole datagrams, and heldErr why this host's addresses could not be read;
// either is nil when its read answered.
func spdForwardBypassRefusal(p ipsec.SPDPolicy, gatherErr, heldErr error) *opererr.Error {
	side, other := spdSideLocal, spdSideRemote
	if p.LocalPort.IsAny() {
		side, other = spdSideRemote, spdSideLocal
	}
	problem := "vpn ipsec policy " + p.Name + " is a bypass with a port, and Ze cannot apply it to the traffic it forwards"
	why := "This kernel checks forwarded traffic before it reassembles it and reads port 0 from every fragment " +
		"but the first, and RFC 4301 Section 7.4 requires stateful fragment checking for such a bypass. " +
		"Forwarded traffic on the entry's ports would meet the next entry instead, and under unmatched discard " +
		"it is dropped. Ze's runtime kernel performs the check (its XFRM fragment guard reassembles forwarded " +
		"fragments first, capability bit 3) and accepts the entry. A bypass with both ports any is accepted on " +
		"any kernel, and so is a discard with a port, or a bypass whose local prefix is one address of this " +
		"host (traffic to and from Ze is reassembled before it is checked)"
	if !p.LocalPort.IsAny() && !p.RemotePort.IsAny() {
		why += ". Set " + other + " port any as well"
	}
	if gatherErr != nil {
		why += ". Ze could not read whether this kernel reassembles forwarded fragments first"
	}
	if heldErr != nil {
		why += ". Ze could not read this host's addresses, so it cannot tell whether the local prefix is one of them"
	}
	fix := "set vpn ipsec policy " + p.Name + " " + side + " port any\n" +
		"or, to keep the port, build and boot Ze's runtime kernel:\n" + dataplane.RuntimeKernelBuildFix()
	return opererr.Wrap(errors.Join(gatherErr, heldErr), problem, why, fix)
}

// verifyForwardBypass is the commit half: it answers the first refusal, so the commit
// names the entry, the cause and the any-port form that is accepted.
func verifyForwardBypass(dp dataplane.Dataplane, cfg *ipsec.IPsecConfig) error {
	refusals := spdForwardBypassRefusals(dp, cfg.Policies)
	if len(refusals) == 0 {
		return nil
	}
	return refusals[0]
}

// warnForwardBypassAtStart is the start half of verifyForwardBypass. The configuration
// the daemon starts with is never verified, and a start never refuses the IPsec block
// (owner decision 2026-11-19, "Install, log, doctor"), so the entry is installed for
// traffic to and from Ze, and each one is logged at Error with the refusal's facts and
// fix. ze doctor reports the same entries (answerSPDForwardBypass).
func warnForwardBypassAtStart(dp dataplane.Dataplane, cfg *ipsec.IPsecConfig, log *slog.Logger) {
	for _, refusal := range spdForwardBypassRefusals(dp, cfg.Policies) {
		log.LogAttrs(context.Background(), slog.LevelError, "ike: "+refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeDoctorIPsecSPDForwardBypass))...)
	}
}

// answerSPDForwardBypass answers ipsec-spd-forward-bypass from the engine's process: one
// error for each installed operator bypass with a port that forwarded traffic can
// match, judged on the addresses this host holds now, so an address that left after
// the commit is reported too.
//
// Safe for concurrent use: applyMu guards installedSPD, as it guards every member the
// apply writes.
func answerSPDForwardBypass(s *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	s.applyMu.Lock()
	installed := maps.Clone(s.installedSPD)
	s.applyMu.Unlock()
	refusals := spdForwardBypassRefusals(dataplane.Get(), installed)
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(refusals))
	for _, refusal := range refusals {
		message := refusal.Problem() + ". " + refusal.Why() + "."
		if cause := refusal.Unwrap(); cause != nil {
			message += " The read answered: " + cause.Error() + "."
		}
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorIPsecSPDForwardBypass,
			Severity: severityError,
			Message:  message,
			Fix:      refusal.Fix(),
		})
	}
	return out
}
