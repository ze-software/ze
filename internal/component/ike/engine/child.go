// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA creation and teardown
// Detail: ts_narrow.go -- traffic-selector narrowing, policy, and port encoding
// RFC: rfc/short/rfc7296.md -- Child SA keying material (Section 2.17), Traffic Selectors (Section 2.9)

package engine

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
)

// isXFRMUnsupported reports whether an install failure names a platform that offers
// no XFRM at all.
//
// The errno is ambiguous and stays ambiguous. A kernel without the esp4 module
// answers EPROTONOSUPPORT, and so does a kernel that refuses the key the state
// carries. Nothing in the errno separates the two, so this predicate does not try.
//
// The ambiguity is a reason to report the outcome honestly, not a reason to stay
// silent. Either way the Child SA carries no ESP. createFirstChildSA therefore
// records ESPInstalled false and the peer reports degraded rather than up
// (ai/rules/evidence.md). The tolerance survives, because a platform with
// no XFRM must still run the control plane. The false "everything is fine" does not.
func isXFRMUnsupported(err error) bool {
	if errors.Is(err, dataplane.ErrNotSupported) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ENOPROTOOPT ||
			errno == syscall.EPROTONOSUPPORT ||
			errno == syscall.EAFNOSUPPORT ||
			errno == syscall.ENOSYS
	}
	s := err.Error()
	return strings.Contains(s, "protocol not supported") ||
		strings.Contains(s, "function not implemented")
}

const (
	protoESP = 50
	// modeTunnel and modeTransport alias the dataplane vocabulary rather than repeating
	// its numbers. Two enumerations for one concept is the hazard kernelXFRMMode's own
	// comment records (dataplane.go): a mode that reached the kernel one number off was
	// accepted silently and protected the traffic in the wrong mode.
	modeTunnel    = dataplane.ModeTunnel
	modeTransport = dataplane.ModeTransport
	defaultReqID  = 1
	replayWindow  = 32
)

// ChildSA holds the state for one ESP Child SA pair (inbound + outbound).
type ChildSA struct {
	InboundSPI  uint32
	OutboundSPI uint32
	LocalAddr   net.IP
	RemoteAddr  net.IP
	IfID        uint32
	TSLocal     *net.IPNet
	TSRemote    *net.IPNet
	Keys        *crypto.ChildSAKeys
	ESPGroup    ipsec.ESPGroup
	ReqID       uint32
	NATDetected bool

	// Owner is the configured peer this Child SA belongs to. It is the identity the
	// dataplane uses to tell this peer's re-install of a selector from a DIFFERENT
	// peer's takeover of it, which the kernel cannot tell apart on its own
	// (dataplane.SPParams.Owner). A rekey inherits it, so the replacement upserts the
	// retired pair's policies instead of being refused.
	Owner string

	// Selectors is the negotiated selector set in TSi/TSr orientation, carrying the
	// port and protocol of each entry as well as its prefix.
	//
	// It is the SCOPE CURRENTLY IN USE that RFC 7296 Section 2.9.2 forbids a rekey from
	// narrowing below, so respondChildRekey passes it as the narrowing floor. TSLocal
	// and TSRemote above hold only the first pair's prefixes, which cannot express the
	// scope of a multi-selector or port-restricted SA.
	Selectors []tsPair

	// Mode is the encapsulation mode this Child SA was installed with:
	// dataplane.ModeTunnel or dataplane.ModeTransport.
	//
	// RFC 7296 Section 1.3.1 makes tunnel the default and transport the negotiated
	// exception, so a Child SA built without a decision carries tunnel. It is read at
	// every SA and policy install, and a transport-mode install carries NO tunnel
	// endpoints (dataplane.tunnelEndpoints refuses them).
	Mode uint8

	// UDPEncap records that this Child SA's ESP is wrapped in UDP on port 4500.
	//
	// Two independent RFC conditions set it, and either alone is enough.
	//
	// RFC 7296 Section 2.23 MUST: "if a NAT is detected, both devices MUST use UDP
	// encapsulation for ESP." That is NATDetected.
	//
	// The same section also lets either side encapsulate
	// "irrespective of the choice made by the other side",
	// so a peer can encapsulate with no NAT present. A peer that does runs its IKE on
	// port 4500 too, so the IKE SA's own port is the signal. That is localPort. It is
	// also how ze meets RFC7296-2.23-11 MUST (rfc/full/rfc7296.txt:3624):
	// "Implementations MUST process received UDP-encapsulated ESP packets even when
	// no NAT was detected" -- the floated port is what puts the ESP-in-UDP template on
	// the INBOUND state.
	//
	// In production the second implies the first, because every site that sets
	// NATDetected also floats the SA. They are kept separate because they answer
	// different questions, and merging them is the defect
	// plan/spec-fixit-ike-responder-natt-port-float.md records.
	//
	// KNOWN INTEROP CONFLICT, unresolved and NOT to be "fixed" by deleting the
	// localPort disjunct. strongSwan 5.9.14 moves IKE to 4500 after IKE_SA_INIT with
	// no NAT present, which RFC 7296 Section 2.23 permits
	// (rfc/full/rfc7296.txt:3538), and then installs ESP states with NO encapsulation.
	// ze floats, encapsulates, and the peer's kernel drops every packet. MEASURED:
	// ze's outbound state carries "encap type espinudp" with oseq 0x4 while
	// strongSwan's carries no encap line and counts XfrmInStateMismatch 4. Interop
	// scenarios 07-responder-psk and 18-cookie-challenge fail on exactly this.
	//
	// Dropping the disjunct trades RFC7296-2.23-11 for that interop, and splitting the
	// flag per direction does not help: a Linux XFRM inbound state accepts exactly ONE
	// ESP form, so an encapsulated-receive state refuses the bare ESP strongSwan
	// sends. The real fix is dual-form receive, owned by
	// plan/spec-ipsec-esp-dual-form-receive.md. Awaiting an owner decision.
	UDPEncap bool

	// LocalIsInitiator is true when this side sent Ni for this Child SA's KEYMAT
	// (i.e. we are the CREATE_CHILD_SA / IKE_AUTH exchange initiator). RFC 7296
	// Section 2.17: traffic from the initiator to the responder is keyed with the
	// EncryptKeyI/IntegKeyI half of KEYMAT and the reverse with the R half, so our
	// send (outbound) SA uses the I half when true and the R half when false. For
	// the first Child SA this equals the IKE SA role (sa.IsInitiator).
	LocalIsInitiator bool

	// ESPInstalled is true when the dataplane holds both states and both policies
	// for this Child SA. It is false when no dataplane is configured, and false when
	// the install failed on a platform that reports XFRM as unsupported.
	//
	// A false value means the tunnel forwards no encrypted traffic. The peer metric
	// reads it, so an operator sees degraded rather than up (metrics.go).
	ESPInstalled bool
}

// Clear zeroes key material.
func (c *ChildSA) Clear() {
	if c.Keys != nil {
		c.Keys.Clear()
	}
}

// generateESPSPI generates a random 32-bit SPI for ESP.
// RFC 4303: SPI value 0 is reserved.
func generateESPSPI() (uint32, error) {
	var buf [4]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0, err
		}
		spi := binary.BigEndian.Uint32(buf[:])
		if spi != 0 {
			return spi, nil
		}
	}
}

// initiatorFirstChildSA creates the first Child SA of an IKE SA this node initiated.
//
// It reads the SA's OWN ESP group, and no other. handleAuthResponse (fsm.go) narrows that
// group to the proposal the responder accepted. selectResponderESP (responder.go) narrows
// the same field on the responder side. RFC 7296 Section 3.3.6 makes the accepted offer
// the set that keys the Child SA. A caller that passed the session's unnarrowed
// configuration would key from the first CONFIGURED proposal, while the peer keyed from
// the accepted one.
//
// runEstablished (established.go) is its only production caller. The group lives here
// rather than in that caller's argument list so no site can supply a different one.
func initiatorFirstChildSA(
	sa *SA,
	peer ipsec.SiteToSitePeer,
	ifID uint32,
	dp dataplane.Dataplane,
	log *slog.Logger,
) (*ChildSA, error) {
	return createFirstChildSA(sa, sa.ESPGroup, peer.LocalAddress, peer.RemoteAddress, ifID, dp, log)
}

// createFirstChildSA creates the initial Child SA after IKE_AUTH completes.
// RFC 7296 Section 2.17: KEYMAT = prf+(SK_d, Ni | Nr).
func createFirstChildSA(
	sa *SA,
	espGroup ipsec.ESPGroup,
	localAddr, remoteAddr string,
	ifID uint32,
	dp dataplane.Dataplane,
	log *slog.Logger,
) (*ChildSA, error) {
	if sa.SKKeys == nil {
		return nil, fmt.Errorf("child-sa: no IKE keys available")
	}
	if len(espGroup.Proposals) == 0 {
		return nil, fmt.Errorf("child-sa: no ESP proposals configured")
	}

	prop := espGroup.Proposals[0]
	enc, integ := espTransforms(prop)

	// RFC 7296 Section 2.17: KEYMAT = prf+(SK_d, Ni | Nr) in absolute initiator/
	// responder order, not this side's Local/Remote order (identical for an
	// initiator SA; swapped for a responder SA).
	keys, err := crypto.DeriveChildSAKeys(
		sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		sa.initiatorNonce(), sa.responderNonce(),
		enc, integ,
	)
	if err != nil {
		return nil, fmt.Errorf("child-sa: key derivation: %w", err)
	}

	var inSPI, outSPI uint32
	if sa.ChildInboundSPI != 0 {
		inSPI = sa.ChildInboundSPI
	} else {
		inSPI, err = generateESPSPI()
		if err != nil {
			keys.Clear()
			return nil, fmt.Errorf("child-sa: generate inbound SPI: %w", err)
		}
	}
	if sa.ChildOutboundSPI != 0 {
		outSPI = sa.ChildOutboundSPI
	} else {
		outSPI, err = generateESPSPI()
		if err != nil {
			keys.Clear()
			return nil, fmt.Errorf("child-sa: generate outbound SPI: %w", err)
		}
	}

	srcIP := net.ParseIP(localAddr)
	dstIP := net.ParseIP(remoteAddr)
	if srcIP == nil || dstIP == nil {
		keys.Clear()
		return nil, fmt.Errorf("child-sa: invalid addresses local=%q remote=%q", localAddr, remoteAddr)
	}

	tsLocal := ipToFullNet(srcIP)
	tsRemote := ipToFullNet(dstIP)
	// RFC 7296 Section 2.9: TSi is the initiator's selector, TSr the responder's.
	// Our local selector is therefore TSi when we are the initiator and TSr when we
	// are the responder (initiator path unchanged: local=TSi, remote=TSr).
	negLocal, negRemote := sa.NegotiatedTSi, sa.NegotiatedTSr
	if !sa.IsInitiator {
		negLocal, negRemote = sa.NegotiatedTSr, sa.NegotiatedTSi
	}
	if negLocal != nil {
		tsLocal = negLocal
	}
	if negRemote != nil {
		tsRemote = negRemote
	}

	// RFC 7296 Section 1.3.1: "Except when using this option to negotiate transport
	// mode, all Child SAs will use tunnel mode." The mode is therefore tunnel unless
	// USE_TRANSPORT_MODE was both requested and echoed (sa.UseTransportMode).
	mode := modeTunnel
	if sa.UseTransportMode {
		mode = modeTransport
	}

	child := &ChildSA{
		InboundSPI:       inSPI,
		OutboundSPI:      outSPI,
		LocalAddr:        srcIP,
		RemoteAddr:       dstIP,
		IfID:             ifID,
		TSLocal:          tsLocal,
		TSRemote:         tsRemote,
		Owner:            sa.PeerName,
		Selectors:        sa.NegotiatedPairs,
		Mode:             mode,
		Keys:             keys,
		ESPGroup:         espGroup,
		ReqID:            defaultReqID,
		NATDetected:      sa.NATDetected,
		UDPEncap:         sa.NATDetected || sa.localPort == transport.NATTPort,
		LocalIsInitiator: sa.IsInitiator,
	}

	if dp == nil {
		log.Debug("child-sa: no dataplane, skipping SA install")
		return child, nil
	}

	if err := installChildSA(child, prop, dp, log); err != nil {
		log.Debug("child-sa: install error", "error", err, "xfrm_unsupported", isXFRMUnsupported(err))
		if isXFRMUnsupported(err) {
			warnDegraded(child, log, err)
		} else {
			keys.Clear()
			return nil, err
		}
	} else {
		child.ESPInstalled = true
	}

	return child, nil
}

// warnDegraded reports a Child SA whose dataplane install was refused as unsupported.
// The control plane continues, and the tunnel forwards no encrypted traffic.
//
// The message names the consequence rather than the platform. The errno cannot tell
// a kernel with no XFRM from a kernel that refused this state. An operator acts on
// the same fact in both cases (ai/rules/cli.md).
func warnDegraded(child *ChildSA, log *slog.Logger, err error) {
	child.ESPInstalled = false
	log.Warn("child-sa: dataplane refused the ESP state, tunnel is degraded and carries no encrypted traffic",
		"in-spi", child.InboundSPI, "out-spi", child.OutboundSPI, "ifid", child.IfID, "error", err)
}

func installChildSA(child *ChildSA, prop ipsec.ESPProposal, dp dataplane.Dataplane, log *slog.Logger) error {
	isAEAD := prop.Encryption.IsAEAD()
	encAlgo := prop.Encryption.String()
	authAlgo := prop.Hash.String()

	// A Child SA built before the mode field existed, or by a test that fills the struct
	// literally, carries the zero value. Zero is not a mode in the dataplane vocabulary
	// (its constants are 1-based so an unset field is invalid), so it resolves to tunnel
	// here rather than reaching kernelXFRMMode and failing the install.
	mode := child.Mode
	if mode == 0 {
		mode = modeTunnel
	}

	// RFC 7296 Section 2.17: the SA carrying data the exchange initiator sends is
	// keyed with the EncryptKeyI/IntegKeyI half; the responder's send SA uses the R
	// half. When we are the exchange initiator our SEND (outbound) SA uses the I
	// half and our RECEIVE (inbound) SA uses the R half; otherwise the two swap.
	// For an initiator-role Child SA (LocalIsInitiator=true) this yields inbound=R /
	// outbound=I, identical to the former hardcoded assignment.
	inEnc, inInteg := child.Keys.EncryptKeyI, child.Keys.IntegKeyI
	outEnc, outInteg := child.Keys.EncryptKeyR, child.Keys.IntegKeyR
	if child.LocalIsInitiator {
		inEnc, inInteg = child.Keys.EncryptKeyR, child.Keys.IntegKeyR
		outEnc, outInteg = child.Keys.EncryptKeyI, child.Keys.IntegKeyI
	}

	// Inbound SA: remote sends to us, keyed with the peer's send-direction keys.
	inbound := dataplane.SAParams{
		SPI:       child.InboundSPI,
		Src:       child.RemoteAddr,
		Dst:       child.LocalAddr,
		IfID:      child.IfID,
		Proto:     protoESP,
		Mode:      mode,
		ReqID:     child.ReqID,
		ReplayWin: replayWindow,
		EncAlgo:   encAlgo,
		EncKey:    inEnc,
		AuthAlgo:  authAlgo,
		AuthKey:   inInteg,
		IsAEAD:    isAEAD,
	}

	// RFC 3948: UDP encapsulation follows EITHER the NAT verdict OR the port this SA
	// runs on. createFirstChildSA sets UDPEncap from a disjunction, so either condition
	// alone is enough. RFC 7296 Section 2.23 lets either side encapsulate
	// "irrespective of the choice made by the other side",
	// and a peer that encapsulates ESP runs its IKE on port 4500 too. The float is
	// therefore a second signal beside the NAT verdict, and it is set when an
	// AUTHENTICATED message arrives on port 4500 (sa.adoptAuthenticatedEndpoint).
	//
	// RFC 7296 Section 2.23 MUST NOT (rfc/full/rfc7296.txt:3544): "UDP encapsulation
	// MUST NOT be done on port 500." Both ports below are the NAT-T constant, and the
	// branch runs only for a floated SA, so no path here can encapsulate on port 500.
	//
	// KERNEL CONSTRAINT, and the receive path now works around it rather than accepting
	// it. On Linux XFRM one inbound state accepts exactly ONE of the two ESP forms.
	//
	// MEASURED: a state carrying an ESP-in-UDP template rejects bare ESP with
	// XfrmInStateMismatch, and a state without one rejects encapsulated ESP the same
	// way. TestEncapKernelBindsOneESPFormPerState records that truth table. It installs
	// its two states on two DISTINCT SPIs.
	//
	// MEASURED, and it was previously only REASONED: two states on one SPI do not help,
	// and the mechanism is not the one the old comment gave. The second state cannot be
	// INSTALLED at all. TestEncapTwoStatesOneSPI records "file exists" both with
	// identical addresses and with a differing source, because the uniqueness key and
	// the lookup key are the same tuple.
	//
	// The template below therefore states which form the KERNEL serves directly. The
	// other form is served beside it, through AcceptBothESPForms, so this SA meets
	// RFC 7296 Section 2.23 "at any time" WITHIN one Child SA and not only across SAs.
	if child.UDPEncap {
		inbound.UDPEncap = true
		inbound.UDPEncapSPort = transport.NATTPort
		inbound.UDPEncapDPort = transport.NATTPort
	}

	// RFC 7296 Section 2.23 MUST (rfc/full/rfc7296.txt:3544-3548): "all devices MUST be
	// able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP
	// packets at any time". The obligation binds because Ze exchanges NAT_DETECTION_*_IP
	// payloads in IKE_SA_INIT, so NAT-T is supported and the antecedent is true.
	//
	// It is set on the INBOUND state only. Reception is what the MUST governs; the form
	// Ze SENDS is a separate decision taken below.
	inbound.AcceptBothESPForms = true

	if err := dp.InstallSA(inbound); err != nil {
		return fmt.Errorf("child-sa: install inbound: %w", err)
	}
	log.Debug("child-sa: installed inbound SA", "spi", child.InboundSPI, "ifid", child.IfID)

	// Outbound SA: we send to remote, keyed with our send-direction keys.
	outbound := dataplane.SAParams{
		SPI:       child.OutboundSPI,
		Src:       child.LocalAddr,
		Dst:       child.RemoteAddr,
		IfID:      child.IfID,
		Proto:     protoESP,
		Mode:      mode,
		ReqID:     child.ReqID,
		ReplayWin: replayWindow,
		EncAlgo:   encAlgo,
		EncKey:    outEnc,
		AuthAlgo:  authAlgo,
		AuthKey:   outInteg,
		IsAEAD:    isAEAD,
	}

	// The SEND form follows the NAT VERDICT, and nothing else.
	//
	// RFC 7296 Section 2.23 (rfc/full/rfc7296.txt:3550-3551): "if a NAT is detected, both
	// devices MUST use UDP encapsulation for ESP". With no NAT the same paragraph leaves
	// the choice open: "Either side can decide whether or not to use UDP encapsulation
	// for ESP irrespective of the choice made by the other side" (:3548-3550), and
	// "sending ESP with UDP encapsulation is not required" on port 4500 (:3542-3543).
	//
	// The PORT is deliberately not a term here, and that is the fix for a measured
	// interop failure. strongSwan floats to port 4500 whenever the peer supports NAT-T,
	// even with no NAT present, because MOBIKE asks it to (ike_natd.c process_i, with
	// mobike defaulting to yes). Its ESP then stays BARE, because COND_NAT_ANY is false.
	// Reading that float as an encapsulation signal made Ze encapsulate toward a peer
	// that expects bare ESP, and scenario 07-responder-psk recorded the result: the
	// tunnel established and strongSwan accepted no ESP at all.
	//
	// child.UDPEncap still carries the port term, and the inbound template above still
	// uses it. That asymmetry is deliberate: Ze RECEIVES the encapsulated form on any
	// floated SA, which is what RFC7296-2.23-11 requires, while SENDING the form the NAT
	// verdict calls for.
	if child.NATDetected {
		outbound.UDPEncap = true
		outbound.UDPEncapSPort = transport.NATTPort
		outbound.UDPEncapDPort = transport.NATTPort
	}

	if err := dp.InstallSA(outbound); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		return fmt.Errorf("child-sa: install outbound: %w", err)
	}
	log.Debug("child-sa: installed outbound SA", "spi", child.OutboundSPI, "ifid", child.IfID)

	// The policies below can capture ze's own IKE traffic whenever the negotiated
	// selector covers the two addresses IKE runs between, which the ordinary
	// 0.0.0.0/0 site-to-site selector always does. The exemption that stops it is
	// installed once, at engine start, and it outlives every Child SA: see
	// installIKEBypass in bypass.go for why it is not installed here.
	if err := dp.InstallPolicy(childPolicyParams(child, dataplane.SADirIn)); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		_ = dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP)
		return fmt.Errorf("child-sa: install inbound policy: %w", err)
	}

	if err := dp.InstallPolicy(childPolicyParams(child, dataplane.SADirOut)); err != nil {
		// Owner-aware, so a peer whose install was refused cannot roll back over the
		// owning peer's live inbound policy (dataplane.SPParams.Owner).
		_ = dp.RemovePolicyParams(childPolicyParams(child, dataplane.SADirIn))
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		_ = dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP)
		return fmt.Errorf("child-sa: install outbound policy: %w", err)
	}

	log.Info("child-sa: installed", "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI, "ifid", child.IfID)
	return nil
}

// installChildTolerant installs a Child SA, tolerating platforms without XFRM/ESP
// support the same way createFirstChildSA does: on an ErrNotSupported install
// failure it logs and returns nil so the control plane proceeds (the rekey must
// not tear down a tunnel that the first Child SA was allowed to establish
// without a dataplane). Real install errors are returned; the caller clears keys.
func installChildTolerant(child *ChildSA, prop ipsec.ESPProposal, dp dataplane.Dataplane, log *slog.Logger) error {
	if dp == nil {
		return nil
	}
	if err := installChildSA(child, prop, dp, log); err != nil {
		if isXFRMUnsupported(err) {
			warnDegraded(child, log, err)
			return nil
		}
		return err
	}
	child.ESPInstalled = true
	return nil
}

// childPolicyParams builds the Security Policy this Child SA installs for one
// direction.
//
// Install AND remove both go through it, so the delete selector is the install
// selector by construction. The kernel identifies a policy by its whole selector and by
// nothing else, so a delete that rebuilt only some of the fields would either miss the
// policy or match a wider one than it meant to. It is also what carries the Owner the
// dataplane refuses a foreign takeover on (dataplane.SPParams.Owner).
//
// Src/Dst are the traffic selectors, the inner traffic the policy matches.
// TunnelSrc/TunnelDst are the tunnel endpoints, the outer IP header addresses the
// kernel resolves the policy to a state through. Both pairs reverse with the direction.
//
// A TRANSPORT-mode policy carries NEITHER endpoint. RFC 4301 Section 4.4.1.2 leaves a
// transport-mode template's addresses unused, and dataplane.tunnelEndpoints REJECTS a
// non-tunnel policy that carries any, so supplying them would fail the install outright
// rather than be ignored.
func childPolicyParams(child *ChildSA, dir dataplane.SADir) dataplane.SPParams {
	// A Child SA built by a test that fills the struct literally carries the zero
	// value, which is not a mode in the dataplane vocabulary. Resolved here exactly as
	// installChildSA resolves it, so the two agree.
	mode := child.Mode
	if mode == 0 {
		mode = modeTunnel
	}

	src, dst := child.TSLocal, child.TSRemote
	tunnelSrc, tunnelDst := child.LocalAddr, child.RemoteAddr
	srcPort, dstPort := selectorPort(child, true), selectorPort(child, false)
	if dir == dataplane.SADirIn {
		src, dst = child.TSRemote, child.TSLocal
		tunnelSrc, tunnelDst = child.RemoteAddr, child.LocalAddr
		srcPort, dstPort = selectorPort(child, false), selectorPort(child, true)
	}
	if mode == modeTransport {
		tunnelSrc, tunnelDst = nil, nil
	}

	return dataplane.SPParams{
		Src: src,
		Dst: dst,
		Dir: dir,
		// Stated rather than left to the zero value. SPActionProtect IS zero, chosen so a
		// caller who forgets the field protects traffic instead of passing it in the
		// clear (dataplane.go). Naming it here says the choice was made, and it keeps the
		// contrast with the IKE bypass (bypass.go) legible.
		Action:     dataplane.SPActionProtect,
		Owner:      child.Owner,
		Proto:      protoESP,
		Mode:       mode,
		IfID:       child.IfID,
		ReqID:      child.ReqID,
		Priority:   dataplane.PriorityChildSA,
		UpperProto: selectorProto(child),
		SrcPort:    srcPort,
		DstPort:    dstPort,
		TunnelSrc:  tunnelSrc,
		TunnelDst:  tunnelDst,
	}
}

// firstSharingSelector returns the first Child SA in keep that still answers to child's
// policies, or nil when none does.
//
// It exists because a Child SA's policies can outlive it in more than one way. A
// make-before-break REKEY leaves the retired pair and the replacement on one selector,
// and a parallel RE-INITIATION (RFC 7296 Section 2.4) leaves the old owner's child and
// the new handshake's pending child on one selector. A caller that considered only one
// of the two survivors would drop the policy the other one needs, so both are offered
// here and the first match wins.
func firstSharingSelector(child *ChildSA, keep ...*ChildSA) *ChildSA {
	for _, k := range keep {
		if samePolicySelector(child, k) {
			return k
		}
	}
	return nil
}

// samePolicySelector reports whether two Child SAs answer to the SAME pair of kernel
// policies.
//
// It compares exactly the fields installChildSA feeds into SPParams, because the
// kernel identifies a policy by its whole selector and by nothing else: the traffic
// selectors, the XFRM if_id, and the upper-layer protocol and ports the negotiated
// selector narrows to. The SPIs are deliberately absent -- they identify STATES, and
// a rekey replaces states while leaving the selector alone.
func samePolicySelector(a, b *ChildSA) bool {
	if a == nil || b == nil || a.TSLocal == nil || a.TSRemote == nil || b.TSLocal == nil || b.TSRemote == nil {
		return false
	}
	return a.TSLocal.String() == b.TSLocal.String() &&
		a.TSRemote.String() == b.TSRemote.String() &&
		a.IfID == b.IfID &&
		selectorProto(a) == selectorProto(b) &&
		selectorPort(a, true) == selectorPort(b, true) &&
		selectorPort(a, false) == selectorPort(b, false)
}

// removeChildSA tears down an installed Child SA from the dataplane, states and
// policies both. Use it when nothing else needs the selector.
func removeChildSA(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	removeChildSAExcept(child, nil, dp, log)
}

// removeChildSAExcept tears down a Child SA, keeping its POLICIES when keep still
// answers to them.
//
// A make-before-break rekey puts two Child SAs on one pair of policies.
// newRekeyedChild (rekey.go) inherits TSLocal, TSRemote, IfID, ReqID and Mode from
// the retired pair, so the replacement's install upserts the SAME selector rather
// than adding a second one (xfrmBackend.InstallPolicy). The kernel then holds
// exactly one policy per direction, shared by both pairs.
//
// Removing the retired pair therefore removed the LIVE pair's policy, and the tunnel
// stopped forwarding at the moment a rekey SUCCEEDED. MEASURED against strongSwan:
// interop scenario 05 reported "no ESP traffic after the rekey" with both Child SAs
// installed and healthy, because the peer's Delete for the old SPI took the
// survivor's policy with it.
//
// The states are always removed: they are keyed by SPI and are never shared.
//
// keep is nil when the whole tunnel is going away, and then the policies go too. At
// session teardown the retired pair is passed keep = the live child, so its policy
// survives the first call and is removed by the second -- once, in total.
func removeChildSAExcept(child, keep *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil || child == nil {
		return
	}
	dropPolicy := !samePolicySelector(child, keep)
	removeChildSAOutgoing(child, dp, log, dropPolicy)
	removeChildSAIncoming(child, dp, log, dropPolicy)
	child.Clear()
	log.Info("child-sa: removed", "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI,
		"policies-removed", dropPolicy)
}

// removeChildSAOutgoing removes the half of a Child SA pair this node SENDS on.
//
// RFC 7296 Section 1.4.1 needs the two halves separable: in the crossing case a node
// deletes "the outgoing SAs while processing the request and the incoming SAs while
// processing the response". Callers that close a whole pair use removeChildSA above.
func removeChildSAOutgoing(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger, dropPolicy bool) {
	if dp == nil || child == nil {
		return
	}
	if dropPolicy {
		// RemovePolicyParams, not the three-argument RemovePolicy: it names the whole
		// selector the install used, and it carries the Owner the dataplane refuses a
		// foreign delete on (dataplane.SPParams.Owner). A refusal here means another
		// peer owns the selector, which is a takeover the install should already have
		// stopped, so it is logged at Warn rather than swallowed.
		if err := dp.RemovePolicyParams(childPolicyParams(child, dataplane.SADirOut)); err != nil {
			logPolicyRemoveFailure(log, "outbound", child, err)
		}
	}
	if err := dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP); err != nil {
		log.Debug("child-sa: remove outbound SA", "error", err)
	}
}

// removeChildSAIncoming removes the half of a Child SA pair this node RECEIVES on.
// The companion of removeChildSAOutgoing above.
func removeChildSAIncoming(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger, dropPolicy bool) {
	if dp == nil || child == nil {
		return
	}
	if dropPolicy {
		if err := dp.RemovePolicyParams(childPolicyParams(child, dataplane.SADirIn)); err != nil {
			logPolicyRemoveFailure(log, "inbound", child, err)
		}
	}
	if err := dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP); err != nil {
		log.Debug("child-sa: remove inbound SA", "error", err)
	}
}

// logPolicyRemoveFailure reports a policy that could not be removed, at the level the
// CAUSE deserves.
//
// A refusal on ownership is not routine cleanup noise. It means a different peer holds
// this selector, so this Child SA's policy was never installed and something upstream
// let two peers negotiate one selector. A netlink failure on teardown is ordinary (the
// policy is often already gone), so it stays at Debug.
func logPolicyRemoveFailure(log *slog.Logger, dir string, child *ChildSA, err error) {
	var owned *dataplane.PolicyOwnedError
	if errors.As(err, &owned) {
		log.Warn("child-sa: refused to remove a policy another peer owns",
			"direction", dir, "peer", child.Owner, "owner", owned.HeldBy, "error", err)
		return
	}
	log.Debug("child-sa: remove policy", "direction", dir, "error", err)
}

// selectorProto returns the IP protocol the negotiated selector restricts the policy to,
// or 0 for every protocol.
//
// Only the FIRST negotiated pair is read, because SPParams carries one selector and the
// policy install is one pair. A multi-selector answer whose entries disagree on protocol
// would need one policy per entry, which the current install shape does not express;
// narrowSelectors leads with the initiator's first choice, so the first pair is the one
// the peer asked for (RFC 7296 Section 2.9).
func selectorProto(child *ChildSA) uint8 {
	if len(child.Selectors) == 0 {
		return 0
	}
	return child.Selectors[0].I.Proto
}

// selectorPort maps a negotiated port form to the kernel's port-plus-mask selector.
//
// local selects which half of the pair to read: the LOCAL side of the Child SA when
// true, the remote side when false. The caller supplies it per direction, because an
// outbound policy's source is the local side while an inbound policy's source is the
// remote one.
//
// RFC 7296 Section 3.13.1 gives three port forms and each maps to exactly one selector:
//
//	ANY (0..65535)   -> no constraint. Mask 0, which is what every policy carried before
//	                    ports existed, so an unconfigured peer programs the same bytes.
//	single port N    -> Port N, Mask 0xffff.
//	OPAQUE (65535/0) -> Port 0, Mask 0xffff. An opaque port is one that is NOT VISIBLE in
//	                    the packet, and a packet with no visible transport port presents
//	                    port 0 to the selector. It is deliberately NOT mapped to Mask 0:
//	                    Section 3.13.1 records that ANY includes OPAQUE, so answering an
//	                    OPAQUE negotiation with an any-port policy would program more
//	                    traffic than was negotiated.
func selectorPort(child *ChildSA, local bool) dataplane.PortMatch {
	if len(child.Selectors) == 0 {
		return dataplane.AnyPortMatch()
	}
	pair := child.Selectors[0]
	// Selectors are in TSi/TSr orientation. TSi is this side when we are the initiator.
	side := pair.R
	if local == child.LocalIsInitiator {
		side = pair.I
	}
	switch side.Port.Form {
	case ipsec.PortSingle:
		return dataplane.ExactPortMatch(side.Port.Port)
	case ipsec.PortOpaque:
		return dataplane.ExactPortMatch(0)
	default:
		return dataplane.AnyPortMatch()
	}
}

func ipToFullNet(ip net.IP) *net.IPNet {
	if ip4 := ip.To4(); ip4 != nil {
		return &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
}
