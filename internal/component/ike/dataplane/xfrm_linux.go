// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// RFC: rfc/short/rfc4303.md -- ESP SA parameters
// Related: policy_owner.go -- who owns a policy selector, since the kernel cannot say

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// xfrmPolicyDel is the kernel call RemovePolicyParams makes.
//
// It is a variable so a test can drive the arm below it, where the ownership record and
// the kernel's own state can part. No real kernel refuses a well-formed delete on demand
// for a reason other than ENOENT, and driving that arm from the helper alone would leave
// the entry point itself unproven (ai/rules/evidence.md).
var xfrmPolicyDel = netlink.XfrmPolicyDel

type xfrmBackend struct {
	// espForms serves the ESP wire form the installed state refuses. One XFRM state
	// binds one form, and RFC 7296 Section 2.23 requires both to be received, so an
	// inbound state that asks for both is registered here as well as with the kernel.
	espForms *espFormReceiver

	// policies records who owns each installed policy selector, because the kernel
	// cannot. Every policy below is upserted, so without this a second peer on the
	// same selector would replace the first peer's policy instead of being refused.
	// See SPParams.Owner.
	policies policyOwners
}

func newXFRMBackend() (Dataplane, error) {
	return &xfrmBackend{espForms: newESPFormReceiver(slogutil.Logger("ike.dataplane"))}, nil
}

func (b *xfrmBackend) InstallSA(p SAParams) error {
	state, err := xfrmStateFromParams(p)
	if err != nil {
		return err
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
		peer, okPeer := netip.AddrFromSlice(p.Src.To4())
		local, okLocal := netip.AddrFromSlice(p.Dst.To4())
		if !okPeer || !okLocal {
			// IPv6 ESP-in-UDP is not served by this receiver. Refusing the install is
			// the honest answer: an SA that silently receives one form only carries no
			// traffic when the peer picks the other (ai/rules/protocol.md).
			return fmt.Errorf("xfrm: spi=%d asks to receive both ESP forms, which this backend serves for IPv4 only (src=%v dst=%v)", p.SPI, p.Src, p.Dst)
		}
		if err := b.espForms.Watch(p.SPI, peer, local); err != nil {
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
// decapsulation UNLESS XFRM_STATE_NOECN is set in the state's flags. Nothing below sets a
// flag, and netlink.XfrmState carries no general flags field to set one through, so the
// kernel default stands for every SA ze installs.
//
// It is split out of InstallSA so the mapping can be driven without a netlink socket: the
// tests that prove Section 2.24 call THIS, not a type literal.
func xfrmStateFromParams(p SAParams) (*netlink.XfrmState, error) {
	mode, ok := kernelXFRMMode(p.Mode)
	if !ok {
		return nil, fmt.Errorf("xfrm: state add spi=%d: unknown mode %d, want ModeTransport (%d) or ModeTunnel (%d)",
			p.SPI, p.Mode, ModeTransport, ModeTunnel)
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

	// RFC 4552 OSPFv3: an explicit state selector (x->sel) lets one wildcard-address
	// SA (Src=Dst=::) be resolved for any OSPF flow (ff02::5, ff02::6, neighbor
	// unicast) instead of only flows whose daddr equals p.Dst. IKE child SAs leave
	// p.Sel nil, so msg.Sel stays the zero value (byte-identical to before).
	if p.Sel != nil {
		state.Selector = &netlink.XfrmPolicy{
			Src:   p.Sel.Src,
			Dst:   p.Sel.Dst,
			Proto: netlink.Proto(p.Sel.UpperProto), // 0 = any, 89 = OSPF
		}
	}

	// RFC 4302 (AH) vs RFC 4303 (ESP): AH sets an integrity transform only, ESP
	// sets encryption + integrity, and a combined-mode algorithm sets a single
	// AEAD transform. planStateAlgos isolates that decision from netlink so it is
	// unit-testable on any platform.
	plan := planStateAlgos(p)
	if plan.AEAD {
		state.Aead = &netlink.XfrmStateAlgo{
			Name:   xfrmAEADName(p.EncAlgo),
			Key:    p.EncKey,
			ICVLen: 128,
		}
	}
	if plan.Crypt {
		state.Crypt = &netlink.XfrmStateAlgo{
			Name: xfrmEncName(p.EncAlgo),
			Key:  p.EncKey,
		}
	}
	if plan.Auth {
		state.Auth = &netlink.XfrmStateAlgo{
			Name:        xfrmAuthName(p.AuthAlgo),
			Key:         p.AuthKey, //nolint:gosec // AH/ESP integrity key, not a credential
			TruncateLen: xfrmAuthTruncLen(p.AuthAlgo),
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

	return state, nil
}

func (b *xfrmBackend) RemoveSA(spi uint32, dst net.IP, proto uint8) error {
	state := &netlink.XfrmState{
		Dst:   dst,
		Proto: netlink.Proto(proto),
		Spi:   int(spi),
	}
	if err := netlink.XfrmStateDel(state); err != nil {
		return fmt.Errorf("xfrm: state del spi=%d: %w", spi, err)
	}
	// The state is gone, so nothing must re-present bare ESP for it any more. Forget is
	// a no-op for an SPI that was never watched, and releases the raw sockets once the
	// last watched SA is removed.
	b.espForms.Forget(spi)
	return nil
}

func (b *xfrmBackend) InstallPolicy(p SPParams) error {
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
	// A Child SA rekey re-installs an IDENTICAL selector. newRekeyedChild
	// (engine/rekey.go) inherits TSLocal, TSRemote, IfID, ReqID and Mode from the
	// retired pair, so the replacement's policies match the retired pair's in every
	// field xfrmPolicyFromParams sets. Under NEWPOLICY the replacement's install
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
	return nil
}

// RemovePolicy deletes by the three-field selector only, so it carries no owner and
// cannot be refused on one. It forgets every record it could have matched, because a
// record outliving its kernel policy would refuse a later, legitimate install.
//
// Prefer RemovePolicyParams. It is the owner-aware form, and it is what every IKE
// caller uses.
func (b *xfrmBackend) RemovePolicy(src, dst *net.IPNet, dir SADir) error {
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
		var owned *PolicyOwnedError
		if errors.As(err, &owned) {
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
	// A BYPASS carries no template at all, so it is built before every check below.
	// Those checks all validate the template: the mode the template names, and the
	// tunnel endpoints the kernel resolves the template to a state through. A policy
	// with no template resolves to no state by design, so applying them here would
	// reject a correct bypass for lacking fields it must not carry.
	//
	// XFRM_POLICY_ALLOW plus an empty Tmpls IS the SPD "BYPASS" disposition of RFC
	// 4301 Section 4.4.1. netlink omits the XFRMA_TMPL attribute entirely when Tmpls
	// is empty (vendor xfrm_policy_linux.go, xfrmPolicyAddOrUpdate), which is what
	// makes it a bypass rather than a protect policy with an empty template list.
	if p.Action == SPActionBypass {
		srcPort, err := xfrmSelectorPort("source", p.SrcPort)
		if err != nil {
			return nil, err
		}
		dstPort, err := xfrmSelectorPort("destination", p.DstPort)
		if err != nil {
			return nil, err
		}
		return &netlink.XfrmPolicy{
			Src:      p.Src,
			Dst:      p.Dst,
			Dir:      netlink.Dir(p.Dir - 1),
			Proto:    netlink.Proto(p.UpperProto),
			SrcPort:  srcPort,
			DstPort:  dstPort,
			Ifindex:  p.IfIndex,
			Priority: p.Priority,
			Action:   netlink.XFRM_POLICY_ALLOW,
			Ifid:     int(p.IfID),
		}, nil
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
		Src:      p.Src,
		Dst:      p.Dst,
		Dir:      netlink.Dir(p.Dir - 1),
		Proto:    netlink.Proto(p.UpperProto), // upper-layer selector (0 = any, 89 = OSPF)
		SrcPort:  srcPort,
		DstPort:  dstPort,
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
	}, nil
}

// xfrmSelectorPort converts a PortMatch to the port number netlink writes into the XFRM
// selector, and REFUSES a match the selector cannot express.
//
// The kernel selector is a port plus a mask, but selFromPolicy derives the mask from the
// port: it sets a full mask only when the port is non-zero
// (vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go, selFromPolicy). So exactly
// two matches are expressible through this API:
//
//	any port      -> port 0, mask 0.
//	one port N>=1 -> port N, mask 0xffff.
//
// A match asking for "exactly port 0" cannot be built: writing 0 yields mask 0, which
// matches EVERY port. That is the OPAQUE port form of RFC 7296 Section 3.13.1, and
// installing it as any-port would protect more traffic than was negotiated. So it is
// refused here rather than widened (ai/rules/protocol.md).
func xfrmSelectorPort(side string, p PortMatch) (int, error) {
	if p.IsAny() {
		return 0, nil
	}
	if p.Mask != 0xffff {
		return 0, fmt.Errorf(
			"xfrm: %s port mask %#04x is not expressible: the kernel selector this backend builds carries either no port constraint or one exact port, so the mask must be 0 or 0xffff",
			side, p.Mask)
	}
	if p.Port == 0 {
		return 0, fmt.Errorf(
			"xfrm: %s port selector asks for exactly port 0, which this backend cannot express: netlink derives the port mask from the port value, so port 0 always matches every port; RFC 7296 Section 3.13.1 OPAQUE ports have no XFRM encoding",
			side)
	}
	return int(p.Port), nil
}

func (b *xfrmBackend) ListSAs(ifID uint32) ([]SAInfo, error) {
	states, err := netlink.XfrmStateList(netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("xfrm: state list: %w", err)
	}
	var out []SAInfo
	for i := range states {
		s := &states[i]
		if ifID != 0 && uint32(s.Ifid) != ifID {
			continue
		}
		out = append(out, SAInfo{
			SPI:  uint32(s.Spi),
			Src:  s.Src,
			Dst:  s.Dst,
			IfID: uint32(s.Ifid),
		})
	}
	return out, nil
}

func (b *xfrmBackend) Close() error { return b.espForms.Close() }

func xfrmEncName(algo string) string {
	switch algo {
	case "aes128", "aes256":
		return "cbc(aes)"
	case "3des":
		return "cbc(des3_ede)"
	case "null":
		// RFC 4552 §3 / RFC 2410: ESP with NULL encryption (authentication-only ESP).
		// NOTE: verify against target kernel -- the kernel's ealg registry names this
		// transform "cipher_null" (net/xfrm/xfrm_algo.c: ealg_list "cipher_null"), and
		// some kernels/iproute2 reject the "ecb(cipher_null)" spelling. Validate the
		// accepted string on the appliance kernel in QEMU (cannot be exercised here).
		return "ecb(cipher_null)"
	default:
		return "cbc(aes)"
	}
}

func xfrmAEADName(algo string) string {
	switch algo {
	case "aes128gcm", "aes256gcm":
		return "rfc4106(gcm(aes))"
	case "chacha20poly1305":
		return "rfc7539esp(chacha20,poly1305)"
	default:
		return "rfc4106(gcm(aes))"
	}
}

func xfrmAuthName(algo string) string {
	switch algo {
	case "sha256":
		return "hmac(sha256)"
	case "sha384":
		return "hmac(sha384)"
	case "sha512":
		return "hmac(sha512)"
	case "sha1":
		return "hmac(sha1)"
	default:
		return "hmac(sha256)"
	}
}

func xfrmAuthTruncLen(algo string) int {
	switch algo {
	case "sha256":
		return 128
	case "sha384":
		return 192
	case "sha512":
		return 256
	case "sha1":
		return 96
	default:
		return 128
	}
}
