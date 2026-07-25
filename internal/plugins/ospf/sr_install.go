// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing reception->install
// driver. On the post-SPF hook (after the IP-route Installer applied, R-8), it reads
// remote SR capabilities (RI LSAs) and Prefix-SIDs (Extended Prefix LSAs) from the
// LSDB, computes each outgoing label from the SPF NEXT-HOP router's advertised SRGB
// (RFC 8665 §5: the pushed/swapped label is SRGB(next-hop).Label(index)), applies the
// NP/E/M PHP/Explicit-NULL rules ONLY when the next-hop IS the SID originator (this
// node is the penultimate hop) and swaps unconditionally at a transit hop, and emits
// an mpls-fib push/swap/pop toward the SPF next-hop. Stale FECs are withdrawn
// idempotently. Shared logic; only the LSDB carriage the maps are built from differs
// by address family.
// RFC: rfc/short/rfc8665.md (§5 outgoing label); rfc/short/rfc8666.md (§6)

package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// srNextHop is one resolved SPF next-hop: the next-hop address plus the Router ID
// at the far end of the first hop (the SPF next-hop router). The label source is
// this router's SRGB (RFC 8665 §5), and the NP/E/M rules apply only when it equals
// the SID originator (penultimate hop).
type srNextHop struct {
	Addr   netip.Addr
	Router types.RouterID
}

// srRoute is the slim SPF route view the installer needs: the destination prefix,
// its advertising router (originator), and the resolved next-hops.
type srRoute struct {
	Prefix   netip.Prefix
	Origin   types.RouterID
	NextHops []srNextHop
}

// srRemotePrefixSID is a Prefix-SID received for one prefix, with its originator
// and a Duplicate marker (RFC 8665 §5: multiple Prefix-SIDs for the same prefix are
// all ignored).
type srRemotePrefixSID struct {
	Originator types.RouterID
	SID        sr.PrefixSID
	Duplicate  bool
}

// srInstaller programs SR MPLS forwarding for one address family. It tracks the
// transit in-label installed per FEC so a FEC no longer present can be withdrawn.
type srInstaller struct {
	fib          *srFIB
	af           string
	explicitNull uint32
	active       map[netip.Prefix]uint32 // FEC -> transit in-label (0 = none)
}

func newSRInstaller(fib *srFIB, af string, explicitNull uint32) *srInstaller {
	return &srInstaller{fib: fib, af: af, explicitNull: explicitNull, active: make(map[netip.Prefix]uint32)}
}

// installRoutes computes and installs SR forwarding for the current SPF routes and
// withdraws any FEC that is no longer active. remoteCaps maps an originator to its
// advertised SRGB; algos maps an originator to its advertised SR-Algorithm list;
// mySRGB is this node's SRGB (for the transit swap in-label).
func (s *srInstaller) installRoutes(
	routes []srRoute,
	prefixSIDs map[netip.Prefix]srRemotePrefixSID,
	remoteCaps map[types.RouterID]sr.SRGB,
	algos map[types.RouterID][]uint8,
	mySRGB sr.SRGB,
) {
	if s.active == nil {
		s.active = make(map[netip.Prefix]uint32)
	}
	next := make(map[netip.Prefix]uint32)
	installedPush := 0
	for _, r := range routes {
		rs, ok := prefixSIDs[r.Prefix]
		if !ok || rs.Duplicate {
			if rs.Duplicate {
				srMetrics.Load().observeComputeError(s.af, "duplicate")
			}
			continue
		}
		// RFC 8665 §5 / RFC 8666 §6: a Prefix-SID for an algorithm Ze does not compute
		// (only Algorithm 0), or one whose algorithm the originator did not advertise, is
		// recorded but not installed.
		if rs.SID.Algorithm != 0 {
			srMetrics.Load().observeComputeError(s.af, "unknown-algorithm")
			continue
		}
		if !sr.HasAlgorithm(algos[rs.Originator], 0) {
			continue // non-SR / algorithm-not-advertised originator
		}
		// The transit swap in-label is this node's own SID label for the destination.
		var myLabel uint32
		myOK := false
		if !rs.SID.IsLabel {
			myLabel, myOK = mySRGB.Label(rs.SID.Index)
		}
		installedFEC := false
		for _, nh := range r.NextHops {
			outLabel, action, ok := s.forwarding(rs, nh, remoteCaps)
			if !ok {
				continue // next-hop not SR-capable, or index out of its SRGB
			}
			s.fib.installPrefixSID(r.Prefix, action, myLabel, myOK, outLabel, s.explicitNull, nh.Addr)
			installedPush++
			installedFEC = true
		}
		if installedFEC {
			next[r.Prefix] = myLabel
		}
	}
	// Withdraw FECs installed on a previous run that are no longer active.
	for fec, inLabel := range s.active {
		if _, still := next[fec]; still {
			continue
		}
		s.fib.removePush(fec)
		if inLabel != 0 {
			s.fib.removeSwap(inLabel)
			s.fib.removePop(inLabel)
		}
	}
	s.active = next
	srMetrics.Load().observeLabelsInstalled(s.af, "push", installedPush)
}

// forwarding resolves the outgoing label and forwarding action for one SPF next-hop
// toward a received Prefix-SID (RFC 8665 §5 / RFC 8666 §6). The pushed/swapped label
// is computed from the SRGB of the NEXT-HOP router (the far end of the first hop),
// NOT the SID originator: a SID index is global and each hop maps it through its own
// SRGB. The NP/E/M PHP/Explicit-NULL rules apply ONLY when the next-hop IS the
// originator (this node is the penultimate hop); at a transit hop the label is swapped
// unconditionally (ActionKeep). A next-hop that advertised no SRGB is not SR-capable
// (R-13): no install toward it. An out-of-range index yields no label.
func (s *srInstaller) forwarding(rs srRemotePrefixSID, nh srNextHop, remoteCaps map[types.RouterID]sr.SRGB) (uint32, sr.OutgoingAction, bool) {
	if rs.SID.IsLabel {
		// An absolute local label (V=1/L=1) is assigned by the originator and is only
		// meaningful where the next-hop is that originator (directly attached). Apply the
		// originator's NP/E/M flags; a transit next-hop cannot map an absolute label.
		if nh.Router != rs.Originator {
			return 0, sr.ActionKeep, false
		}
		return rs.SID.Label, sr.OutgoingActionFor(rs.SID.Flags), true
	}
	srgb, ok := remoteCaps[nh.Router]
	if !ok || srgb.Empty() {
		return 0, sr.ActionKeep, false // next-hop is not SR-capable (R-13)
	}
	label, ok := srgb.Label(rs.SID.Index)
	if !ok {
		srMetrics.Load().observeComputeError(s.af, "index-out-of-range")
		return 0, sr.ActionKeep, false
	}
	// Penultimate hop (next-hop == originator): apply the advertised PHP/E/M rules.
	// Transit hop: swap the label on unconditionally (ActionKeep).
	if nh.Router == rs.Originator {
		return label, sr.OutgoingActionFor(rs.SID.Flags), true
	}
	return label, sr.ActionKeep, true
}

// withdrawAll withdraws every currently-installed SR forwarding entry (SR disabled
// or the engine stopping).
func (s *srInstaller) withdrawAll() { s.installRoutes(nil, nil, nil, nil, sr.SRGB{}) }

// ---- Engine driver: build the maps from the LSDB and drive the installer ----

// srInstallFromRoutes is the post-SPF hook. It rebuilds the remote SR state from the
// LSDB and (re)installs SR forwarding for the current SPF routes. IPv4 reads the RFC 7684
// Extended Prefix Opaque LSAs; IPv6 reads the RFC 8362 Extended prefix LSAs
// (srRemotePrefixSIDs dispatches by address family). The label computation, NP/E/M
// forwarding decision and mpls-fib install are shared.
func (e *engine) srInstallFromRoutes() {
	if e.srInstaller == nil || e.spf == nil || e.lsdb == nil {
		return
	}
	cfg, enabled := srWire.get(e.cfg.RouterID)
	if !enabled || !cfg.Enabled {
		e.srInstaller.withdrawAll()
		return
	}
	remoteCaps, algos := e.srRemoteCapabilities()
	prefixSIDs := e.srRemotePrefixSIDs()
	mySRGB := sr.NewSRGB(cfg.SRGB)
	e.srInstaller.installRoutes(e.srRoutes(), prefixSIDs, remoteCaps, algos, mySRGB)
}

// srRoutes flattens the SPF route table into the installer's slim view.
func (e *engine) srRoutes() []srRoute {
	entries := e.spf.Routes()
	out := make([]srRoute, 0, len(entries))
	for _, r := range entries {
		if !r.Prefix.IsValid() || len(r.NextHops) == 0 {
			continue
		}
		hops := make([]srNextHop, 0, len(r.NextHops))
		for _, nh := range r.NextHops {
			if nh.Addr.IsValid() {
				hops = append(hops, srNextHop{Addr: nh.Addr, Router: nh.Router})
			}
		}
		if len(hops) == 0 {
			continue
		}
		out = append(out, srRoute{Prefix: r.Prefix, Origin: r.Origin, NextHops: hops})
	}
	return out
}

// srRemoteCapabilities reads every RI LSA in the LSDB and returns each originator's
// SRGB and advertised algorithm list (RFC 8665 §3.1/§3.2). The RI TLV stream is the
// same format in both families (RFC 8666 §4); only the carrier differs -- the IPv4
// family reads the RFC 7770 opaque RI LSA, the IPv6 family the native OSPFv3 RI LSA.
func (e *engine) srRemoteCapabilities() (map[types.RouterID]sr.SRGB, map[types.RouterID][]uint8) {
	caps := make(map[types.RouterID]sr.SRGB)
	algos := make(map[types.RouterID][]uint8)
	af := e.srAF()
	record := func(router types.RouterID, body []byte) {
		rc := srDecodeRemoteCapabilities(af, body)
		if rc.SRGB.Empty() && len(rc.Algorithms) == 0 {
			return
		}
		caps[router] = rc.SRGB
		algos[router] = rc.Algorithms
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		for _, sc := range riV3Scopes {
			for _, v := range e.lsdb.LSAViewsByType(sc.typ) {
				record(v.AdvertisingRouter, v.Body)
			}
		}
		return caps, algos
	}
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
		record(v.AdvertisingRouter, v.Body)
	}
	return caps, algos
}

// srRemotePrefixSIDs reads the received Prefix-SID per prefix from the address family's
// carriage. A prefix advertised with more than one (conflicting) Prefix-SID is marked
// Duplicate so it is ignored (RFC 8665 §5 / RFC 8666 §6). The IPv6 family reads the RFC
// 8362 Extended prefix LSAs (sr_reception_v6.go); the IPv4 family reads the RFC 7684
// Extended Prefix Opaque LSAs.
func (e *engine) srRemotePrefixSIDs() map[netip.Prefix]srRemotePrefixSID {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.srRemotePrefixSIDsV6()
	}
	out := make(map[netip.Prefix]srRemotePrefixSID)
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.ExtPrefixOpaqueType) {
		lsa, err := packet.DecodeExtPrefixLSA(v.Body)
		if err != nil {
			continue
		}
		for i := range lsa.Prefixes {
			tlv := lsa.Prefixes[i]
			if tlv.AF != 0 {
				continue // IPv4 unicast only in the Extended Prefix Opaque LSA
			}
			pfx := netip.PrefixFrom(netip.AddrFrom4(tlv.AddressPrefix), int(tlv.PrefixLength))
			for _, sub := range tlv.SubTLVs {
				if sub.Type != sr.V4TypePrefixSID {
					continue
				}
				ps, derr := sr.DecodePrefixSIDValue(sub.Value)
				if derr != nil {
					srMetrics.Load().observeMalformed(e.srAF(), "prefix-sid")
					continue
				}
				if existing, dup := out[pfx]; dup {
					existing.Duplicate = true
					out[pfx] = existing
					continue
				}
				out[pfx] = srRemotePrefixSID{Originator: v.AdvertisingRouter, SID: ps}
			}
		}
	}
	return out
}

// srAF returns this engine's SR address-family metric label.
func (e *engine) srAF() string {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return interfaceFamilyIPv6
	}
	return interfaceFamilyIPv4
}

// srExplicitNull returns the Explicit NULL label for this engine's address family
// (RFC 8665 §5 IPv4 label 0; RFC 8666 §6 IPv6 label 2).
func (e *engine) srExplicitNull() uint32 {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return sr.ExplicitNullV6
	}
	return sr.ExplicitNullV4
}

// srSourceTag returns the mpls-fib Source tag for this engine's address family.
func (e *engine) srSourceTag() uint16 {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return mplsSourceOSPFv3SR
	}
	return mplsSourceOSPFSR
}
