// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// RFC: rfc/short/rfc4303.md -- ESP SA parameters
// Related: policy_owner.go -- who owns a policy selector, since the kernel cannot say

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// xfrmPolicyDel is the kernel call RemovePolicyParams makes.
//
// It is a variable so a test can drive the arm below it, where the ownership record and
// the kernel's own state can part. No real kernel refuses a well-formed delete on demand
// for a reason other than ENOENT, and driving that arm from the helper alone would leave
// the entry point itself unproven (ai/rules/evidence.md).
var xfrmPolicyDel = netlink.XfrmPolicyDel

// xfrmStateList and xfrmPolicyList are the kernel dumps the read surface makes.
//
// They are variables for the same reason xfrmPolicyDel above is: the mapping
// from a kernel record onto SAInfo and PolicyInfo is where a field is dropped,
// mis-scaled, or offset by one, and a test that can only reach it through a real
// netlink socket proves it on the machine that happens to run it. Driving the
// map from a fixture proves it everywhere, and the integration tests beside it
// still prove the dump itself against a real kernel.
var (
	xfrmStateList  = netlink.XfrmStateList
	xfrmPolicyList = netlink.XfrmPolicyList
)

type xfrmBackend struct {
	// mu serializes kernel mutations with a multi-message tunnel migration.
	mu sync.Mutex

	// espForms serves the ESP wire form the installed state refuses. One XFRM state
	// binds one form (Linux limitation LL-XFRM-STATE-ONE-ESP-FORM,
	// docs/architecture/linux-limitations.md), and RFC 7296 Section 2.23 requires
	// both to be received, so an
	// inbound state that asks for both is registered here as well as with the kernel.
	espForms *espFormReceiver

	// policies records who owns each installed policy selector, because the kernel
	// cannot. Every policy below is upserted, so without this a second peer on the
	// same selector would replace the first peer's policy instead of being refused.
	// See SPParams.Owner.
	policies policyOwners

	// migrationCleanup holds alternate SAD identities after a failed rollback
	// whose delete also failed. Ordinary Child SA teardown retries these entries.
	migrationCleanup map[SAIdentity]SAIdentity

	// guard is the application override guard (override_guard_linux.go), held only
	// while `vpn ipsec application-override deny` is applied.
	guard overrideGuard
}

func newXFRMBackend() (Dataplane, error) {
	b := &xfrmBackend{espForms: newESPFormReceiver(slogutil.Logger("ike.dataplane"))}
	if xfrmMigrationAvailable() {
		return &xfrmMobikeBackend{xfrmBackend: b}, nil
	}
	return b, nil
}

func (b *xfrmBackend) InstallSA(p SAParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	state, err := xfrmStateFromParams(p)
	if err != nil {
		return err
	}
	var peer, local netip.Addr
	if p.AcceptBothESPForms && p.UDPEncap {
		var okPeer, okLocal bool
		peer, okPeer = netip.AddrFromSlice(p.Src.To4())
		local, okLocal = netip.AddrFromSlice(p.Dst.To4())
		// Fail closed: a state installed here would receive one ESP form only. The branch
		// is unreachable from a negotiation today, because the IKE listener opens udp4
		// only (transport/udp.go), so no Child SA has IPv6 outer addresses; the gap is
		// owed after the first release (plan/post-release/features/spec-ike-ipv6-transport.md).
		if !okPeer || !okLocal {
			return opererr.New(
				"the Child SA from "+p.Src.String()+" to "+p.Dst.String()+
					" asks to receive both ESP wire forms over IPv6, and Ze cannot serve that yet",
				"Ze has no IPv6 IKE transport yet, so no IPv6 dual-form SA can be produced; "+
					"receiving both ESP forms over IPv6 is a gap owed after the first release, "+
					"and installing the SA anyway would drop one of the two forms",
				"set vpn ipsec site-to-site peer <peer> remote-address <IPv4 address>")
		}
	}
	if err := netlink.XfrmStateAdd(state); err != nil {
		return fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
	}

	// RFC 7296 Section 2.23 MUST (rfc/full/rfc7296.txt:3544-3548): both ESP forms are
	// received at any time. The state above serves the form its template selects. When
	// the caller asks for both, the other form is served beside the kernel.
	//
	// Only a TEMPLATED state needs the help. A template-free state already accepts bare
	// ESP natively, and the encapsulated form reaching it is refused by the kernel
	// before any userspace reader can see it (MEASURED:
	// TestEncapEncapsulatedESPHiddenFromUserspaceWhenSocketDecapsulates). RFC 3948
	// Section 2.1 makes that combination unreachable from a conforming peer: the
	// ESP-in-UDP ports "MUST be the same as that used by IKE traffic" and RFC 7296
	// forbids encapsulation on port 500, so a peer that encapsulates ESP runs its IKE on
	// port 4500 and its SA is templated here.
	if p.AcceptBothESPForms && p.UDPEncap {
		// The template's ports, not 4500/4500: a Child rekey after MOBIKE keeps the
		// translated IKE port, and the reader re-presents bare ESP to the replacement
		// inbound SA with the tuple its state matches from the first datagram on.
		target := espFormTarget{peer: peer, local: local, peerPort: p.UDPEncapSPort, localPort: p.UDPEncapDPort}
		// RFC 7296 Section 2.23
		if err := b.espForms.Watch(p.SPI, target); err != nil {
			// The state is already in the kernel, and this install is about to be
			// reported as failed. Leaving it behind would strand a state no caller
			// believes exists, and the next install of the same SPI would answer
			// "file exists" for a reason nobody could see.
			if delErr := netlink.XfrmStateDel(state); delErr != nil {
				b.espForms.logger().Debug("xfrm: remove state after the ESP form receiver refused",
					"spi", p.SPI, "error", delErr)
			}
			return fmt.Errorf("xfrm: spi=%d cannot receive both ESP forms, so it was not installed: %w", p.SPI, err)
		}
	}
	return nil
}

// xfrmStateFromParams maps one Child SA description onto the netlink state this backend
// hands the kernel. It is the whole of what ze controls about an installed SA, so it is
// also the whole of what ze could use to change the kernel's ECN behavior.
//
// RFC 7296 Section 2.24 MUST: tunnel encapsulators and decapsulators "MUST support the ECN
// full-functionality option for tunnels", and MUST implement the processing "to prevent
// discarding of ECN congestion indications". Linux copies the congestion indication on
// decapsulation UNLESS XFRM_STATE_NOECN is set in the state's flags. Nothing below sets
// it: netlink.XfrmState carries no general flags field, only ESN, AFUnspec, NoPMTUDisc and
// ICMP, which set XFRM_STATE_ESN, XFRM_STATE_AF_UNSPEC, XFRM_STATE_NOPMTUDISC and
// XFRM_STATE_ICMP alone, and DontEncapDSCP and OSeqMayWrap, which set
// XFRM_SA_XFLAG_DONT_ENCAP_DSCP and XFRM_SA_XFLAG_OSEQ_MAY_WRAP in the separate
// XFRMA_SA_EXTRA_FLAGS attribute, so the kernel default stands for every SA ze installs.
//
// It is split out of InstallSA so the mapping can be driven without a netlink socket: the
// tests that prove Section 2.24 call THIS, not a type literal.
func xfrmStateFromParams(p SAParams) (*netlink.XfrmState, error) {
	mode, ok := kernelXFRMMode(p.Mode)
	if !ok {
		return nil, fmt.Errorf("xfrm: state add spi=%d: unknown mode %d, want ModeTransport (%d) or ModeTunnel (%d)",
			p.SPI, p.Mode, ModeTransport, ModeTunnel)
	}
	// RFC 4301 Section 4.4.2.1: "Security Parameter Index (SPI): a 32-bit value
	// selected by the receiving end of an SA to uniquely identify the SA."
	// RFC 4303 Section 2.1: "The SPI value of zero (0) is reserved for local,
	// implementation-specific use and MUST NOT be sent on the wire." A peer's
	// proposal or a caller can hand over zero; no SAD entry is built for it.
	if p.SPI == 0 {
		return nil, errors.New("xfrm: state add: spi 0 is reserved and identifies no SA")
	}
	// RFC 4301 Section 4.4.2.1: "Tunnel header IP source and destination address --
	// both addresses must be either IPv4 or IPv6 addresses."
	if p.Mode == ModeTunnel && p.Src != nil && p.Dst != nil && (p.Src.To4() == nil) != (p.Dst.To4() == nil) {
		return nil, fmt.Errorf("xfrm: state add spi=%d: tunnel header %s -> %s mixes IPv4 and IPv6", p.SPI, p.Src, p.Dst)
	}
	state := &netlink.XfrmState{
		Src:   p.Src,
		Dst:   p.Dst,
		Proto: netlink.Proto(p.Proto),
		Mode:  netlink.Mode(mode),
		Spi:   int(p.SPI),
		Reqid: int(p.ReqID),
		Ifid:  int(p.IfID),
	}
	if p.ReplayWin > 0 {
		state.ReplayWindow = int(p.ReplayWin)
	}
	// RFC 4301 Section 4.4.2.1: "Sequence Counter Overflow: a flag indicating whether
	// overflow of the sequence number counter should generate an auditable event and
	// prevent transmission of additional packets on the SA, or whether rollover is
	// permitted." Ze never permits rollover: with the flag clear, the kernel's
	// xfrm_replay_overflow audits the overflow and refuses the packet rather than
	// wrapping the counter. This is a
	// guard, written out rather than left to the zero value, so a later change that
	// wants rollover has to delete this line and its reason.
	state.OSeqMayWrap = false

	// RFC 4552 OSPFv3: an explicit state selector (x->sel) lets one wildcard-address
	// SA (Src=Dst=::) be resolved for any OSPF flow (ff02::5, ff02::6, neighbor
	// unicast) instead of only flows whose daddr equals p.Dst. A transport-mode IKE
	// Child SA sets it to its negotiated selector (RFC 4301 Section 4.4.2), and the
	// kernel then drops a decrypted packet outside it (XfrmInStateMismatch). A
	// tunnel-mode IKE Child SA leaves p.Sel nil; the branch after this one marks
	// that state as serving either inner family.
	if p.Sel != nil {
		srcPort, err := xfrmSelectorPort("state source", p.Sel.SrcPort)
		if err != nil {
			return nil, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
		}
		dstPort, err := xfrmSelectorPort("state destination", p.Sel.DstPort)
		if err != nil {
			return nil, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
		}
		state.Selector = &netlink.XfrmPolicy{
			Src:     p.Sel.Src,
			Dst:     p.Sel.Dst,
			Proto:   netlink.Proto(p.Sel.UpperProto), // 0 = any, 89 = OSPF
			SrcPort: srcPort,
			DstPort: dstPort,

			// The masks travel with the ports: an ICMP type or code selector of 0, and
			// an ICMP code block, are a value the library cannot derive a mask from.
			SrcPortMask: p.Sel.SrcPort.Mask,
			DstPortMask: p.Sel.DstPort.Mask,
		}
	}
	// A zero selector is not "any family". The kernel's copy_from_user_state
	// (net/xfrm/xfrm_user.c) sets "x->sel.family = p->family" unless the message
	// carries XFRM_STATE_AF_UNSPEC, and xfrm_state_look_at then skips the state for
	// a flow of the other family: an IPv6 packet routed into an IPv4-outer SA finds
	// no state (XfrmOutNoStates). With the flag the selector family stays AF_UNSPEC,
	// __xfrm_init_state fills the tunnel inner mode of both families, and
	// xfrm_ip2inner_mode picks one per decapsulated packet. The SPD policies, one
	// per negotiated selector pair and bound to the Child SA's if_id, still decide
	// what enters the state. Transport mode keeps its explicit selector above, and
	// a tunnel-mode caller that hands a selector keeps that selector's family.
	if p.Mode == ModeTunnel && p.Sel == nil {
		// RFC 4301 Section 5.1.2: "IPsec allows the IP version of the
		// encapsulating header to be different from that of the inner header."
		state.AFUnspec = true
	}

	// RFC 4301 Section 6.2: "If an IPsec implementation receives an inbound ICMP error
	// message on an SA, and the IP and ICMP headers of the message do not match the
	// traffic selectors for the SA, the receiver MUST process the received message in a
	// special fashion." With XFRM_STATE_ICMP on the last state of the secpath, icmp_rcv
	// (net/ipv4/icmp.c) and icmpv6_rcv (net/ipv6/icmp.c) answer a failed inbound policy
	// check by checking again on the embedded header with addresses and ports reversed
	// (xfrm4_policy_check_reverse), and __xfrm_policy_check does the same for a state
	// selector (xfrm_selector_inner_icmp_match). Without it the message is dropped.
	// Linux refuses the flag on an SA flagged outbound, so it rides the inbound state.
	if p.ICMPErrorCheck {
		if p.Dir == SADirOut {
			// Only a Ze defect reaches this: no config sets the check per direction.
			return nil, fmt.Errorf("xfrm: state add spi=%d: BUG: the ICMP error payload check is set on an outbound SA, and Linux performs it on an inbound SA only; "+
				"the code that built this SAParams must set ICMPErrorCheck on the inbound state alone, as engine/child.go does, so report this as a Ze defect",
				p.SPI)
		}
		state.ICMP = true
	}

	// RFC 4302 (AH) vs RFC 4303 (ESP): AH sets an integrity transform only, ESP
	// sets encryption + integrity, and a combined-mode algorithm sets a single
	// AEAD transform. planStateAlgos isolates that decision from netlink so it is
	// unit-testable on any platform.
	plan := planStateAlgos(p)
	if plan.AEAD {
		name, err := xfrmAEADName(p.EncAlgo)
		if err != nil {
			return nil, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
		}
		state.Aead = &netlink.XfrmStateAlgo{
			Name:   name,
			Key:    p.EncKey,
			ICVLen: 128, // every entry of xfrmAEADNames carries a 16-octet ICV
		}
	}
	if plan.Crypt {
		name, err := xfrmEncName(p.EncAlgo)
		if err != nil {
			return nil, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
		}
		state.Crypt = &netlink.XfrmStateAlgo{
			Name: name,
			Key:  p.EncKey,
		}
	}
	if plan.Auth {
		auth, err := xfrmAuthName(p.AuthAlgo)
		if err != nil {
			return nil, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
		}
		state.Auth = &netlink.XfrmStateAlgo{
			Name:        auth.name,
			Key:         p.AuthKey, //nolint:gosec // AH/ESP integrity key, not a credential
			TruncateLen: auth.truncLen,
		}
	}

	// RFC 3948: UDP encapsulation for NAT-T.
	if p.UDPEncap {
		state.Encap = &netlink.XfrmStateEncap{
			Type:    netlink.XFRM_ENCAP_ESPINUDP,
			SrcPort: int(p.UDPEncapSPort),
			DstPort: int(p.UDPEncapDPort),
		}
	}

	// RFC 4301 Section 8.1
	noPMTUDisc, err := xfrmNoPMTUDisc(p)
	if err != nil {
		return nil, err
	}
	state.NoPMTUDisc = noPMTUDisc

	// RFC 4301 Section 5.1.2.1
	dontEncapDSCP, err := xfrmDontEncapDSCP(p)
	if err != nil {
		return nil, err
	}
	state.DontEncapDSCP = dontEncapDSCP

	return state, nil
}

// xfrmDontEncapDSCP projects the SA's outer DSCP treatment onto the one DSCP primitive
// Linux XFRM has, or refuses the value the kernel cannot write.
//
// RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain for
// which the DSCP value in the outer header is not appropriate, that value MUST be
// mapped to an appropriate value for the domain".
//
// xfrm4_tunnel_encap_add and xfrm6_tunnel_encap_add (net/xfrm/xfrm_output.c) write the
// outer DS field as the inner one, unless XFRM_SA_XFLAG_DONT_ENCAP_DSCP is set, when
// they write 0; INET_ECN_encapsulate then adds the ECN bits either way. Copy and a
// fixed 0 are therefore exact. Any other fixed value has no XFRM flag or attribute,
// so it is refused rather than installed as 0 or as a copy.
func xfrmDontEncapDSCP(p SAParams) (bool, error) {
	dontEncap, err := xfrmDSCPFlag(p.OuterDSCP)
	if err != nil {
		return false, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
	}
	return dontEncap, nil
}

// xfrmDSCPFlag is the predicate xfrmDontEncapDSCP and SAOptionsSupported share, so
// the commit-time refusal and the install-time one cannot disagree.
//
// RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain for
// which the DSCP value in the outer header is not appropriate, that value MUST be
// mapped to an appropriate value for the domain".
func xfrmDSCPFlag(d OuterDSCP) (bool, error) {
	v, fixed := d.Fixed()
	if !fixed {
		return false, nil
	}
	if v == 0 {
		return true, nil
	}
	return false, fmt.Errorf("%w: outer DSCP %d cannot be expressed: the Linux kernel copies the inner DSCP or writes 0 (XFRM_SA_XFLAG_DONT_ENCAP_DSCP) and has no attribute for another value; configure dscp-map copy or 0 on this peer",
		ErrSAOptionRefused, v)
}

// xfrmNoPMTUDisc projects the SA's DF treatment onto the one DF primitive Linux XFRM
// has, or refuses the treatment the kernel cannot express.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA."
//
// The kernel builds the outer IPv4 header in xfrm4_tunnel_encap_add
// (net/xfrm/xfrm_output.c), and the DF bit is one expression there:
//
//	top_iph->frag_off = (flags & XFRM_STATE_NOPMTUDISC) || small_ipv6 ?
//		0 : (XFRM_MODE_SKB_CB(skb)->frag_off & htons(IP_DF));
//
// With no flag the inner bit is copied, and XFRM_STATE_NOPMTUDISC writes it clear. No
// flag or attribute writes it set: XFRMA_IPTFS_DONT_FRAG applies to IP-TFS mode only,
// and ze installs tunnel and transport mode. A set request is therefore refused rather
// than installed as a copy that would carry a clear bit for every inner packet whose
// own DF is clear.
func xfrmNoPMTUDisc(p SAParams) (bool, error) {
	if err := outerHeaderTreatmentError(p); err != nil {
		return false, fmt.Errorf("xfrm: state add: %w", err)
	}
	noPMTUDisc, err := xfrmDFFlag(p.DF)
	if err != nil {
		return false, fmt.Errorf("xfrm: state add spi=%d: %w", p.SPI, err)
	}
	return noPMTUDisc, nil
}

// xfrmDFFlag is the predicate xfrmNoPMTUDisc and SAOptionsSupported share, so the
// commit-time refusal and the install-time one cannot disagree.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA."
func xfrmDFFlag(df DFTreatment) (bool, error) {
	switch df {
	case DFCopy:
		return false, nil
	case DFClear:
		return true, nil
	case DFSet:
		return false, fmt.Errorf("%w: DF set cannot be expressed: the Linux kernel's xfrm4_tunnel_encap_add copies the inner DF bit or clears it, and no XFRM flag sets it; configure df-bit copy or clear on this peer",
			ErrSAOptionRefused)
	}
	return false, fmt.Errorf("%w: DF treatment %d names none of copy, clear or set",
		ErrSAOptionRefused, df)
}

// SAOptionsSupported answers config verify for Linux XFRM from the predicates the
// install path uses: DF copy or clear, an outer DSCP copied or fixed at 0, and the
// RFC 4301 Section 6.2 ICMP error processing either way (XFRM_STATE_ICMP and
// XFRM_POLICY_ICMP carry it).
func (b *xfrmBackend) SAOptionsSupported(o SAOptions) error {
	// RFC 4301 Section 8.1
	if _, err := xfrmDFFlag(o.DF); err != nil {
		return fmt.Errorf("xfrm: %w", err)
	}
	// RFC 4301 Section 5.1.2.1
	if _, err := xfrmDSCPFlag(o.OuterDSCP); err != nil {
		return fmt.Errorf("xfrm: %w", err)
	}
	return nil
}

// RemoveSA deletes one inbound or outbound XFRM state and stops re-presenting bare ESP
// for its SPI. The caller MUST treat the returned error as a failed kernel delete; the
// ESP form is released either way.
//
// The release is deferred because it MUST run on every exit, the failed delete included.
// A second teardown of one Child SA is ordinary rather than exotic (RFC 7296 Section 1.4
// Delete processing, engine/delete.go): the state is already gone, XfrmStateDel answers
// ESRCH, and a Forget sitting below that error was never reached. The SPI then stayed
// watched forever and held a host-wide raw ESP socket open after the last tunnel was
// gone, which taxes every packet on the box rather than only this tunnel's.
//
// Forget is a no-op for an SA that was never watched, takes the SPI out of the fast
// path's map unless another SA on the same SPI still holds it, and releases the reader
// socket, the ze-espform device and the fast path once the last watched SA is removed.
func (b *xfrmBackend) RemoveSA(spi uint32, dst net.IP, proto uint8) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := &netlink.XfrmState{
		Dst:   dst,
		Proto: netlink.Proto(proto),
		Spi:   int(spi),
	}
	// The SA forgotten is the one that carries spi to dst, the pair XFRM keys the state on,
	// so removing an outbound SA whose peer-chosen SPI equals a watched inbound SPI leaves
	// that inbound SA served. Only an IPv4 inbound state is ever watched (InstallSA
	// refuses a dual-form SA over IPv6), so a destination with no IPv4 form names no
	// watched SA, and nothing is forgotten.
	if local, ok := netip.AddrFromSlice(dst.To4()); ok {
		defer b.espForms.Forget(spi, local)
	}
	if alternate, pending := b.migrationCleanup[IdentityOf(spi, dst, proto, 0)]; pending {
		extra := &netlink.XfrmState{
			Dst: net.IP(alternate.Dst.AsSlice()), Spi: int(alternate.SPI),
			Proto: netlink.Proto(alternate.Proto), Ifid: int(alternate.IfID),
		}
		extraErr := xfrmMigrationStateDel(extra)
		if errors.Is(extraErr, unix.ESRCH) || errors.Is(extraErr, unix.ENOENT) {
			extraErr = nil
		}
		if extraErr == nil {
			delete(b.migrationCleanup, IdentityOf(spi, dst, proto, 0))
		}
		oldErr := netlink.XfrmStateDel(state)
		if errors.Is(oldErr, unix.ESRCH) || errors.Is(oldErr, unix.ENOENT) {
			oldErr = nil
		}
		return errors.Join(extraErr, oldErr)
	}
	if err := netlink.XfrmStateDel(state); err != nil {
		return fmt.Errorf("xfrm: state del spi=%d: %w", spi, err)
	}
	return nil
}

// CatchAllSupported answers yes when the kernel's XFRM netlink socket opens. An XFRM
// policy is node-wide, so an entry bound to no interface is the ordinary kind, and the
// only question is whether the kernel offers XFRM at all: a kernel built without
// CONFIG_XFRM_USER refuses the socket. A process without CAP_NET_ADMIN still opens
// it, because the kernel checks the capability per message rather than at socket
// creation, so this probe answers yes for such a process. Its install then fails
// when the configuration is applied, and installUnmatched refuses that apply, so the
// catch-all still fails closed.
func (b *xfrmBackend) CatchAllSupported() error {
	handle, err := netlink.NewHandle(unix.NETLINK_XFRM)
	if err != nil {
		return fmt.Errorf("%w: xfrm: the XFRM netlink socket does not open: %w", ErrNotSupported, err)
	}
	handle.Close()
	return nil
}

// RequiresForwardPolicy reports Linux's separate SPD-I lookup for transit traffic.
// A tunnel packet for a protected host reaches XFRM_POLICY_FWD, not XFRM_POLICY_IN.
func (b *xfrmBackend) RequiresForwardPolicy() bool { return true }

func (b *xfrmBackend) InstallPolicy(p SPParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	pol, err := xfrmPolicyFromParams(p)
	if err != nil {
		return fmt.Errorf("xfrm: policy add: %w", err)
	}
	// EVERY policy is UPSERTED, never exclusively added.
	//
	// XfrmPolicyAdd sends XFRM_MSG_NEWPOLICY, which the kernel inserts exclusively,
	// so a second install of one selector answers EEXIST. That exclusivity is wrong
	// for both kinds of policy this backend installs, for the same underlying
	// reason: a selector is not owned by one installer.
	//
	// A Child SA rekey re-installs the SAME selector. newRekeyedChild
	// (engine/rekey.go) inherits TSLocal, TSRemote, IfID, ReqID and Mode from the
	// retired pair, so the replacement's policies match the retired pair's selector.
	// Their template usually matches too; after a NAT rebind the tunnel-mode
	// replacement takes the IKE SA's new peer address (takePeerTunnelAddress), and
	// the upsert is then what moves the template to it. Under NEWPOLICY the replacement's install
	// failed with EEXIST on every make-before-break rekey, the rekey response was
	// abandoned, and the tunnel died at the Child SA's hard lifetime. MEASURED
	// against strongSwan: "child-sa: install inbound policy: xfrm: policy add: file
	// exists", once per second until "child-sa: hard lifetime expired".
	//
	// The IKE bypass is peer-independent by design: every peer installs the same
	// four policies, so peer B's install would fail on peer A's.
	//
	// XfrmPolicyUpdate sends XFRM_MSG_UPDPOLICY, and the kernel derives exclusivity
	// from the message TYPE rather than from NLM_F_EXCL (net/xfrm/xfrm_user.c,
	// xfrm_add_policy: excl = nlmsg_type == XFRM_MSG_NEWPOLICY), so the update
	// replaces. strongSwan installs every policy the same way, and for the same
	// reason.
	//
	// This is idempotent AND safer than swallowing EEXIST: swallowing would leave ze
	// believing it installed a policy when a DIFFERENT one occupied that selector,
	// which is a guard that fails open (ai/rules/evidence.md).
	//
	// The upsert is what a rekey needs and what a DIFFERENT peer must not get. EEXIST
	// used to refuse the second peer loudly, and the claim below is what refuses it
	// now: the kernel has no per-peer identity to tell the two apart, so ze keeps one
	// (SPParams.Owner). Claimed BEFORE the netlink call, so a refused peer never
	// reaches the kernel at all.
	created, err := b.policies.claim(p)
	if err != nil {
		return err
	}
	if err := netlink.XfrmPolicyUpdate(pol); err != nil {
		// Undo only a record this call CREATED. When the selector was already this
		// owner's -- a Child SA rekey re-installing it -- the kernel still holds the
		// earlier policy, so forgetting the record here would let the next foreign peer
		// take a LIVE selector over. That is the very takeover the claim exists to stop.
		if created {
			if relErr := b.policies.release(p); relErr != nil {
				return fmt.Errorf("xfrm: policy update: %w (and the ownership record could not be released: %w)", err, relErr)
			}
		}
		return fmt.Errorf("xfrm: policy update: %w", err)
	}
	// Labeled only once the kernel holds the entry, so a refused install leaves no
	// role behind for a read to show.
	b.policies.label(p)
	return nil
}

// RemovePolicy deletes by the three-field selector only, so it carries no owner and
// cannot be refused on one. It forgets every record it could have matched, because a
// record outliving its kernel policy would refuse a later, legitimate install.
//
// Prefer RemovePolicyParams. It is the owner-aware form, and it is what every IKE
// caller uses.
func (b *xfrmBackend) RemovePolicy(src, dst *net.IPNet, dir SADir) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	pol := &netlink.XfrmPolicy{
		Src: src,
		Dst: dst,
		Dir: netlink.Dir(dir - 1),
	}
	if err := netlink.XfrmPolicyDel(pol); err != nil {
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	b.policies.releaseBySelector(src, dst, dir)
	return nil
}

// RemovePolicyParams deletes a policy by its whole selector, and REFUSES to delete one
// a different owner installed.
//
// That refusal is load-bearing. installChildSA (engine/child.go) rolls a failed policy
// install back by removing the policy of the other direction, so a peer whose install
// this backend just refused would otherwise take the owning peer's live policy down on
// its way out, and the owning peer's tunnel would blackhole with its states still
// installed (ai/rules/evidence.md).
func (b *xfrmBackend) RemovePolicyParams(p SPParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	pol, err := xfrmPolicyFromParams(p)
	if err != nil {
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	// THE RECORD'S LIFETIME MUST MATCH THE KERNEL'S, and a failed delete is the one place
	// the two can part. deleteThenRelease checks the owner first, so a refused delete
	// never reaches the kernel, and it drops the record only once the kernel confirms the
	// policy is gone. InstallPolicy above compensates for the mirror case with `if
	// created`; this side needs no compensation because it never gets ahead of the kernel.
	if err := b.policies.deleteThenRelease(p, func() error { return xfrmPolicyDel(pol) }); err != nil {
		if _, ok := errors.AsType[*PolicyOwnedError](err); ok {
			return err
		}
		return fmt.Errorf("xfrm: policy del: %w", err)
	}
	return nil
}

// xfrmPolicyFromParams builds the netlink policy shared by install and delete so
// the delete selector (Src, Dst, Dir, upper-layer Proto, Ifid) matches the
// installed policy exactly; the kernel identifies a policy by its whole selector.
// It rejects an unknown mode, because the template mode must agree with the mode
// of the state it resolves to.
//
// The template also carries the tunnel endpoints in tunnel mode. Those addresses
// are how the kernel resolves the policy to a state (RFC 4301 Section 4.4.1.2).
// tunnelEndpoints rejects an absent pair, so a 0.0.0.0 template never reaches the
// kernel. Such a template matched no state and the tunnel forwarded nothing.
func xfrmPolicyFromParams(p SPParams) (*netlink.XfrmPolicy, error) {
	// A BYPASS and a DISCARD carry no template at all, so both are built before every
	// check below. Those checks all validate the template: the mode the template
	// names, and the tunnel endpoints the kernel resolves the template to a state
	// through. A policy with no template resolves to no state by design, so applying
	// them here would reject a correct bypass for lacking fields it must not carry.
	//
	// XFRM_POLICY_ALLOW plus an empty Tmpls IS the SPD "BYPASS" disposition of RFC
	// 4301 Section 4.4.1. netlink omits the XFRMA_TMPL attribute entirely when Tmpls
	// is empty (vendor xfrm_policy_linux.go, xfrmPolicyAddOrUpdate), which is what
	// makes it a bypass rather than a protect policy with an empty template list.
	//
	// XFRM_POLICY_BLOCK is the DISCARD disposition of the same section. The kernel
	// drops a packet whose highest-precedence match carries it
	// (net/xfrm/xfrm_policy.c, xfrm_policy_check returns 0 for XFRM_POLICY_BLOCK),
	// which is what RFC 4301 Section 7.4 calls "DISCARDing ... using the normal SPD
	// packet classification mechanisms".
	if p.Action.isTemplateFree() {
		// A bypass or discard entry names no SA, so it has no SA to map an ICMP error
		// to: the request would install nothing the caller asked for.
		if p.ICMPErrorReturn {
			// Only a Ze defect reaches this: no config sets the mapping on such an entry.
			return nil, fmt.Errorf("BUG: the ICMP error return mapping is set on a template-free policy (action %d), which names no SA to carry the error; "+
				"the code that built this SPParams must set ICMPErrorReturn on an outbound protect entry alone, as engine/child.go childPairPolicy does, so report this as a Ze defect",
				p.Action)
		}
		kernelAction, err := xfrmPolicyAction(p.Action)
		if err != nil {
			return nil, err
		}
		srcPort, err := xfrmSelectorPort("source", p.SrcPort)
		if err != nil {
			return nil, err
		}
		dstPort, err := xfrmSelectorPort("destination", p.DstPort)
		if err != nil {
			return nil, err
		}
		return &netlink.XfrmPolicy{
			Src:     p.Src,
			Dst:     p.Dst,
			Dir:     netlink.Dir(p.Dir - 1),
			Proto:   netlink.Proto(p.UpperProto),
			SrcPort: srcPort,
			DstPort: dstPort,

			// RFC 4301 Section 4.4.1.1: an ICMP type and code selector travels in the
			// port pair, and its mask is what makes type 0, code 0 and a code block
			// exact (xfrmSelectorPort).
			SrcPortMask: p.SrcPort.Mask,
			DstPortMask: p.DstPort.Mask,

			Ifindex:  p.IfIndex,
			Priority: p.Priority,
			Action:   kernelAction,
			Ifid:     int(p.IfID),
		}, nil
	}
	// The kernel reads XFRM_POLICY_ICMP only on the policy an outbound XFRM_LOOKUP_ICMP
	// lookup finds (xfrm_lookup_with_ifid, net/xfrm/xfrm_policy.c), so the flag on an
	// inbound or forward policy would install a request nothing performs.
	if p.ICMPErrorReturn && p.Dir != SADirOut {
		// Only a Ze defect reaches this: no config sets the mapping per direction.
		return nil, fmt.Errorf("BUG: the ICMP error return mapping is set on a direction %d policy, and the kernel performs it on an outbound (%d) policy only; "+
			"the code that built this SPParams must set ICMPErrorReturn on the outbound entry alone, as engine/child.go childPairPolicy does, so report this as a Ze defect",
			p.Dir, SADirOut)
	}
	mode, ok := kernelXFRMMode(p.Mode)
	if !ok {
		return nil, fmt.Errorf("unknown mode %d, want ModeTransport (%d) or ModeTunnel (%d)",
			p.Mode, ModeTransport, ModeTunnel)
	}
	tmplSrc, tmplDst, err := tunnelEndpoints(p)
	if err != nil {
		return nil, err
	}
	srcPort, err := xfrmSelectorPort("source", p.SrcPort)
	if err != nil {
		return nil, err
	}
	dstPort, err := xfrmSelectorPort("destination", p.DstPort)
	if err != nil {
		return nil, err
	}
	return &netlink.XfrmPolicy{
		Src:     p.Src,
		Dst:     p.Dst,
		Dir:     netlink.Dir(p.Dir - 1),
		Proto:   netlink.Proto(p.UpperProto), // upper-layer selector (0 = any, 89 = OSPF)
		SrcPort: srcPort,
		DstPort: dstPort,

		// A negotiated ICMP selector carries type and code here (RFC 4301 Section
		// 4.4.1.1, engine/child.go childPairPolicy); the mask keeps type 0 exact.
		SrcPortMask: p.SrcPort.Mask,
		DstPortMask: p.DstPort.Mask,

		Ifindex:  p.IfIndex, // RFC 4552 §6 interface-based selector (0 = node-wide)
		Priority: p.Priority,
		Tmpls: []netlink.XfrmPolicyTmpl{{
			// Src/Dst are the outer tunnel-header addresses, not the selector above.
			// They stay nil in transport mode, where the kernel leaves them unused.
			Src:   tmplSrc,
			Dst:   tmplDst,
			Proto: netlink.Proto(p.Proto),
			Mode:  netlink.Mode(mode),
			Reqid: int(p.ReqID),
		}},
		Ifid: int(p.IfID),

		// RFC 4301 Section 6.2: "It then uses this extracted information to locate an
		// appropriate, active outbound SA, and transmits the error message via this SA."
		// icmp_route_lookup (net/ipv4/icmp.c), icmpv6_route_lookup (net/ipv6/icmp.c) and
		// xfrm_out_fwd_icmp (net/xfrm/xfrm_policy.c) make that lookup with
		// XFRM_LOOKUP_ICMP on the reversed header, and xfrm_lookup_with_ifid answers
		// -ENOENT to it unless the policy it finds carries XFRM_POLICY_ICMP.
		ICMP: p.ICMPErrorReturn,
	}, nil
}

// xfrmPolicyAction maps a template-free Ze disposition to the kernel policy action.
// It never defaults, because the two kernel actions differ in the direction their
// mistake fails: XFRM_POLICY_ALLOW over a discard passes the traffic the operator
// asked to stop, and XFRM_POLICY_BLOCK over a bypass black-holes traffic that was
// meant to cross the boundary in the clear.
//
// PROTECT never reaches here. It carries a template, so xfrmPolicyFromParams builds
// it on the other side of the isTemplateFree branch, and passing it in is a caller
// mistake this reports rather than guesses at.
func xfrmPolicyAction(a SPAction) (netlink.PolicyAction, error) {
	switch a {
	case SPActionBypass:
		return netlink.XFRM_POLICY_ALLOW, nil
	case SPActionDiscard:
		// RFC 4301 Section 4.4.1: "DISCARD -- to discard the traffic, i.e., not to
		// let the traffic traverse the IPsec boundary".
		return netlink.XFRM_POLICY_BLOCK, nil
	case SPActionProtect:
		return 0, fmt.Errorf(
			"%w: xfrm: a protect policy carries a template, so it is not built through the template-free path",
			ErrNotSupported)
	default:
		panic("BUG: template-free policy has an unknown action")
	}
}

// xfrmSelectorPort converts a PortMatch to the port number netlink writes into the XFRM
// selector, and REFUSES a match the selector cannot express.
//
// The kernel selector is a port plus a mask and matches a flow when
// (flow ^ sel) & mask is zero (net/xfrm/xfrm_policy.c, __xfrm4_selector_match). Every
// PortMatch is therefore expressible, provided the mask reaches the kernel with it:
// the caller writes p.Mask into XfrmPolicy.SrcPortMask or DstPortMask, which the
// vendored library sends as given (internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch,
// selFromPolicy). Upstream derives the mask from the port value instead, so exactly
// port 0 and a partial mask would install as any port without that patch
// (docs/architecture/linux-limitations.md, LL-NETLINK-XFRM-PORT-MASK).
//
// For ICMP and ICMPv6 the kernel reads the message type into the source port and the
// code into the destination port (include/net/xfrm.h, xfrm_flowi_sport and
// xfrm_flowi_dport), which is how an RFC 4301 Section 4.4.1.1 type and code selector
// reaches this function.
//
// A port bit outside the mask is refused: the kernel would ignore it, so the value
// would read as a constraint the installed policy does not carry.
func xfrmSelectorPort(side string, p PortMatch) (int, error) {
	if p.Port&^p.Mask != 0 {
		return 0, fmt.Errorf(
			"xfrm: %s port %d carries bits outside its mask %#04x, and the kernel ignores them, so the installed selector would not be the one asked for",
			side, p.Port, p.Mask)
	}
	return int(p.Port), nil
}

func (b *xfrmBackend) ListSAs(ifID uint32) ([]SAInfo, error) {
	states, err := xfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("xfrm: state list: %w", err)
	}
	out := make([]SAInfo, 0, len(states))
	for i := range states {
		s := &states[i]
		if ifID != 0 && uint32(s.Ifid) != ifID {
			continue
		}
		out = append(out, saInfoFromState(s))
	}
	return out, nil
}

// saInfoFromState maps one kernel SAD record onto SAInfo.
//
// Every field below comes from the DUMP. netlink fills Statistics, Limits, Mode,
// Reqid and ReplayWindow from the base message or its extended replay attribute,
// so no per-SA XFRM_MSG_GETSA round trip is needed.
//
// The algorithm arms carry Name and key LENGTH and never Key. netlink.XfrmState
// holds the live encryption and integrity keys, and this is the boundary where
// they stop.
func saInfoFromState(s *netlink.XfrmState) SAInfo {
	info := SAInfo{
		SPI:            uint32(s.Spi),
		Src:            s.Src,
		Dst:            s.Dst,
		IfID:           uint32(s.Ifid),
		Proto:          uint8(s.Proto),
		ReqID:          uint32(s.Reqid),
		ReplayWindow:   uint32(s.ReplayWindow),
		BytesCurrent:   s.Statistics.Bytes,
		PacketsCurrent: s.Statistics.Packets,
		BytesHard:      hardLimitFromKernel(s.Limits.ByteHard),
		PacketsHard:    hardLimitFromKernel(s.Limits.PacketHard),
		AddedAt:        unixOrZero(s.Statistics.AddTime),
		UsedAt:         unixOrZero(s.Statistics.UseTime),
	}
	if mode, ok := zeXFRMMode(uint8(s.Mode)); ok {
		info.Mode = mode
	}
	// RFC 7296 Section 3.3.2: an AEAD transform carries integrity itself, so the
	// kernel fills Aead and leaves Crypt and Auth nil. Reporting an empty
	// Integrity there is the truth about the negotiation, not a failed read.
	switch {
	case s.Aead != nil:
		info.Encryption = s.Aead.Name
		info.EncryptionKeyBits = len(s.Aead.Key) * 8
	case s.Crypt != nil:
		info.Encryption = s.Crypt.Name
		info.EncryptionKeyBits = len(s.Crypt.Key) * 8
	}
	if s.Auth != nil {
		info.Integrity = s.Auth.Name
		info.IntegrityKeyBits = len(s.Auth.Key) * 8
	}
	info.Options = saOptionsFromState(s)
	info.OptionsRead = true
	info.Dir = saDirFromKernel(s.SADir)
	return info
}

// saDirFromKernel maps the XFRMA_SA_DIR attribute of a dumped state. The kernel
// reports it only for a state installed with it (copy_to_user_state_extra,
// net/xfrm/xfrm_user.c, emits it when x->dir is set), so its absence, and any value
// outside the two directions, maps to the zero SADir: the dataplane holds no
// direction, and none is derived.
func saDirFromKernel(d netlink.SADir) SADir {
	switch d {
	case netlink.XFRM_SA_DIR_IN:
		return SADirIn
	case netlink.XFRM_SA_DIR_OUT:
		return SADirOut
	}
	return 0
}

// saOptionsFromState reads the per-SA options back from the flags the kernel reports
// for the state, the inverse of the projection in xfrmState (xfrmDFFlag,
// xfrmDSCPFlag, the ICMP arm). The dump carries all three: the base message's flags
// hold XFRM_STATE_NOPMTUDISC and XFRM_STATE_ICMP, and XFRMA_SA_EXTRA_FLAGS holds
// XFRM_SA_XFLAG_DONT_ENCAP_DSCP (netlink parseXfrmState).
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA." xfrm4_tunnel_encap_add (net/xfrm/xfrm_output.c) writes a clear outer DF
// under XFRM_STATE_NOPMTUDISC and copies the inner DF otherwise, so the flag reads
// back as clear and its absence as copy. No kernel state writes DF set.
//
// RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain
// for which the DSCP value in the outer header is not appropriate, that value MUST
// be mapped to an appropriate value for the domain". xfrm4_tunnel_encap_add and
// xfrm6_tunnel_encap_add write outer DSCP 0 under XFRM_SA_XFLAG_DONT_ENCAP_DSCP and
// copy the inner DSCP otherwise.
//
// RFC 4301 Section 6.2: "Thus, an IPsec implementation MUST be configurable to check
// that this payload header information is consistent with the SA via which it
// arrives." XFRM_STATE_ICMP on the last state of the secpath is that check: icmp_rcv
// and icmpv6_rcv then run xfrm4_policy_check_reverse and xfrm6_policy_check_reverse.
func saOptionsFromState(s *netlink.XfrmState) SAOptions {
	options := SAOptions{DF: DFCopy, OuterDSCP: OuterDSCPCopy, ICMPErrorCheck: s.ICMP}
	if s.NoPMTUDisc {
		options.DF = DFClear
	}
	if s.DontEncapDSCP {
		// FixedOuterDSCP refuses only a value above DSCPMax, and 0 is not.
		fixed, err := FixedOuterDSCP(0)
		if err != nil {
			panic("BUG: FixedOuterDSCP refused 0: " + err.Error())
		}
		options.OuterDSCP = fixed
	}
	return options
}

// hardLimitFromKernel maps XFRM_INF to the public unlimited value.
func hardLimitFromKernel(limit uint64) uint64 {
	if limit == nl.XFRM_INF {
		return 0
	}
	return limit
}

// unixOrZero converts a kernel epoch stamp, keeping "never" distinct from 1970.
//
// The kernel writes 0 into UseTime for an SA that has carried no packet. Passing
// that to time.Unix yields 1970-01-01, which renders as a real timestamp and
// reads as "last used a very long time ago" rather than "never used". The zero
// time.Time is the value a renderer can tell apart.
func unixOrZero(sec uint64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(int64(sec), 0).UTC()
}

func (b *xfrmBackend) ListPolicies() ([]PolicyInfo, error) {
	policies, err := xfrmPolicyList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("xfrm: policy list: %w", err)
	}
	out := make([]PolicyInfo, 0, len(policies))
	for i := range policies {
		out = append(out, b.policyInfoFromKernel(&policies[i]))
	}
	nameInterfaceSPDs(out)
	return out, nil
}

// policyInfoFromKernel maps one kernel SPD record onto PolicyInfo and resolves
// the peer that installed it, and the role and SPD name Ze recorded for it
// (policyOwners.annotate).
//
// THE OWNER JOIN IS THE POINT. The kernel identifies a policy by its selector
// alone and stores no installer, so the only way to name one is to invert the
// kernel row back into the SPParams key policyOwners recorded at install time.
//
// The dump retains prefix families and raw port masks. Ownership therefore
// compares the full selector, including partial masks and exact port zero, which
// ze itself installs for an ICMP type and code selector.
//
// A policy ze did not install resolves to no owner. That is the common case on a
// node running another IKE daemon, and OwnerKnown false is the answer, never a
// blank owner and never a nearest match.
func (b *xfrmBackend) policyInfoFromKernel(p *netlink.XfrmPolicy) PolicyInfo {
	info := PolicyInfo{
		Src:        p.Src,
		Dst:        p.Dst,
		SrcPort:    PortMatch{Port: uint16(p.SrcPort), Mask: p.SrcPortMask},
		DstPort:    PortMatch{Port: uint16(p.DstPort), Mask: p.DstPortMask},
		UpperProto: uint8(p.Proto),
		Priority:   p.Priority,
		IfIndex:    p.Ifindex,
		IfID:       uint32(p.Ifid),
		// XFRM_POLICY_ICMP, the flag xfrmPolicyFromParams sets for
		// SPParams.ICMPErrorReturn (netlink parseXfrmPolicy reads it into ICMP).
		ICMPErrorReturn: p.ICMP,
	}
	info.Dir, info.PerSocket = policyDirFromKernel(p.Dir)
	// An ALLOW policy with no template is the SPD BYPASS disposition of RFC 4301
	// Section 4.4.1, and a BLOCK policy is the DISCARD disposition of the same
	// section. A template present under ALLOW means the policy protects, which is
	// the zero value info already carries.
	//
	// BLOCK is read on its own action rather than on the empty template list. The
	// kernel accepts a template beside a BLOCK policy and ignores it, so a discard
	// installed by another daemon can carry one, and reading such a policy back as a
	// protect entry would report the opposite of what the kernel is doing.
	switch {
	case p.Action == netlink.XFRM_POLICY_BLOCK:
		info.Action = SPActionDiscard
	case p.Action == netlink.XFRM_POLICY_ALLOW && len(p.Tmpls) == 0:
		info.Action = SPActionBypass
	}
	if len(p.Tmpls) > 0 {
		t := &p.Tmpls[0]
		info.TunnelSrc = t.Src
		info.TunnelDst = t.Dst
		info.ReqID = uint32(t.Reqid)
		if mode, ok := zeXFRMMode(uint8(t.Mode)); ok {
			info.Mode = mode
		}
	}
	if info.PerSocket {
		// A socket policy is no SPD entry, so the owner ledger holds no record of it.
		// Its role says whether it is Ze's own (the ESP form reader's, marked by its
		// priority) or an application's override.
		info.Role = socketPolicyRole(info.Priority)
		return info
	}
	b.policies.annotate(&info)
	return info
}

// policyDirFromKernel maps the dump's direction. The kernel lists a per-socket
// policy with its direction offset by XFRM_POLICY_MAX (xfrm_sk_policy_link calls
// __xfrm_policy_link with XFRM_POLICY_MAX + dir), which the netlink package names
// XFRM_SOCKET_IN, XFRM_SOCKET_OUT and XFRM_SOCKET_FWD. A value outside the six maps
// to the zero SADir, which the listing renders as unknown, never to a direction it
// does not carry.
func policyDirFromKernel(d netlink.Dir) (dir SADir, perSocket bool) {
	switch d {
	case netlink.XFRM_DIR_IN:
		return SADirIn, false
	case netlink.XFRM_DIR_OUT:
		return SADirOut, false
	case netlink.XFRM_DIR_FWD:
		return SADirFwd, false
	case netlink.XFRM_SOCKET_IN:
		return SADirIn, true
	case netlink.XFRM_SOCKET_OUT:
		return SADirOut, true
	case netlink.XFRM_SOCKET_FWD:
		return SADirFwd, true
	}
	return 0, false
}

// zeXFRMMode is the inverse of kernelXFRMMode. The two constant sets are offset
// by one, so a missing inverse would silently report every tunnel as transport.
func zeXFRMMode(kernelMode uint8) (uint8, bool) {
	switch kernelMode {
	case kernelModeTransport:
		return ModeTransport, true
	case kernelModeTunnel:
		return ModeTunnel, true
	default:
		return 0, false
	}
}

func (b *xfrmBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.espForms.Close()
}

// Kernel transform names, spelled as the kernel's algorithm registry spells them
// (net/xfrm/xfrm_algo.c), keyed by the algorithm word the SA carries. The words are
// the ones ipsec.Encryption.String and ipsec.Hash.String render for a negotiated
// Child SA, and the ones the OSPFv3 resolver reads from ze-ospf-conf.yang for an
// RFC 4552 manual SA; xfrm_vocabulary_test.go gates the three tables against both
// modules, so a word one side adds alone turns that test red.
//
// A word no table holds is REFUSED (ErrNotSupported), never defaulted. Until
// 2026-09-14 each mapper answered AES-CBC, AES-GCM or HMAC-SHA256 for an unknown
// word, so the kernel installed a transform the peer had not negotiated and every
// packet on the SA failed its integrity check. vppCryptoAlg names the same defect.
const (
	xfrmEncAESCBC  = "cbc(aes)"
	xfrmAEADAESGCM = "rfc4106(gcm(aes))"
	xfrmAuthSHA256 = "hmac(sha256)"
)

// xfrmEncNames maps a non-AEAD cipher word to its kernel transform.
// enumeration: gated by TestXfrmCipherVocabularyMatchesModel
var xfrmEncNames = map[string]string{
	"aes128": xfrmEncAESCBC,
	"aes256": xfrmEncAESCBC,
	"3des":   "cbc(des3_ede)",
	// RFC 4552 §3 / RFC 2410: ESP with NULL encryption (integrity-only ESP).
	// The kernel's ealg registry (net/xfrm/xfrm_algo.c, ealg_list) names this
	// transform "ecb(cipher_null)" and accepts "cipher_null" as its compat alias
	// (xfrm_alg_name_match compares both). The interop scenario
	// esp-integrity-only-null has Ze install this name through XFRM for a Child SA
	// negotiated with strongSwan as ENCR_NULL, and carries traffic over it.
	"null": "ecb(cipher_null)",
}

// xfrmAEADNames maps a combined-mode cipher word to its kernel transform. Every entry
// carries a 16-octet ICV, which is why xfrmStateFromParams writes ICVLen 128 beside
// the name: AES-GCM is negotiated with the 16-octet ICV only (ENCR_AES_GCM_16), and
// ChaCha20-Poly1305 always carries a 16-octet tag (RFC 7634 Section 2). The AES-CCM
// words the model offers are absent on purpose: an ESP proposal naming one is refused
// at config parse (ipsec.EncryptionImplementedESP), so a CCM word reaching here is a
// defect and is refused rather than installed as AES-GCM.
var xfrmAEADNames = map[string]string{
	"aes128gcm":        xfrmAEADAESGCM,
	"aes256gcm":        xfrmAEADAESGCM,
	"chacha20poly1305": "rfc7539esp(chacha20,poly1305)",
}

// xfrmAuthTransform is one integrity algorithm as the kernel takes it: the transform
// name and the ICV length in bits (RFC 4868 Section 2.3 for the SHA-2 family, RFC 2404
// for HMAC-SHA-1-96).
type xfrmAuthTransform struct {
	name     string
	truncLen int
}

// xfrmAuthNames maps an integrity algorithm word to its kernel transform.
// enumeration: gated by TestXfrmAuthVocabularyMatchesModel
var xfrmAuthNames = map[string]xfrmAuthTransform{
	"sha256": {name: xfrmAuthSHA256, truncLen: 128},
	"sha384": {name: "hmac(sha384)", truncLen: 192},
	"sha512": {name: "hmac(sha512)", truncLen: 256},
	"sha1":   {name: "hmac(sha1)", truncLen: 96},
}

func xfrmEncName(algo string) (string, error) {
	name, ok := xfrmEncNames[algo]
	if !ok {
		return "", fmt.Errorf(
			"%w: xfrm: cipher %q is not one this backend can name to the kernel; installing it would encrypt with a different cipher",
			ErrNotSupported, algo)
	}
	return name, nil
}

func xfrmAEADName(algo string) (string, error) {
	name, ok := xfrmAEADNames[algo]
	if !ok {
		return "", fmt.Errorf(
			"%w: xfrm: AEAD cipher %q is not one this backend can name to the kernel; installing it would encrypt with a different cipher",
			ErrNotSupported, algo)
	}
	return name, nil
}

func xfrmAuthName(algo string) (xfrmAuthTransform, error) {
	transform, ok := xfrmAuthNames[algo]
	if !ok {
		return xfrmAuthTransform{}, fmt.Errorf(
			"%w: xfrm: integrity algorithm %q is not one this backend can name to the kernel; installing it would authenticate with a different algorithm",
			ErrNotSupported, algo)
	}
	return transform, nil
}
