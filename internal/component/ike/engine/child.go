// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA creation and teardown
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
// (ai/rules/fail-closed-guards.md). The tolerance survives, because a platform with
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
	// port 4500 too, so the IKE SA's own port is the signal. That is localPort.
	//
	// In production the second implies the first, because every site that sets
	// NATDetected also floats the SA. They are kept separate because they answer
	// different questions, and merging them is the defect
	// plan/spec-fixit-ike-responder-natt-port-float.md records.
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

// GenerateESPSPI generates a random 32-bit SPI for ESP.
// RFC 4303: SPI value 0 is reserved.
func GenerateESPSPI() (uint32, error) {
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
		inSPI, err = GenerateESPSPI()
		if err != nil {
			keys.Clear()
			return nil, fmt.Errorf("child-sa: generate inbound SPI: %w", err)
		}
	}
	if sa.ChildOutboundSPI != 0 {
		outSPI = sa.ChildOutboundSPI
	} else {
		outSPI, err = GenerateESPSPI()
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
// the same fact in both cases (ai/rules/error-messages.md).
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
	// KERNEL CONSTRAINT, and it bounds what this can achieve. On Linux XFRM one inbound
	// state accepts exactly ONE of the two ESP forms.
	//
	// MEASURED: a state carrying an ESP-in-UDP template rejects bare ESP with
	// XfrmInStateMismatch, and a state without one rejects encapsulated ESP the same
	// way. TestEncapKernelBindsOneESPFormPerState records that truth table. It installs
	// its two states on two DISTINCT SPIs.
	//
	// REASONED, and not measured: two states on one SPI do not help either. The state
	// lookup is keyed on destination, SPI, protocol and family, so it returns the first
	// match and the mismatch check then drops the packet. No test installs two states on
	// one SPI. plan/spec-ipsec-esp-dual-form-receive.md carries this as an assumption to
	// validate, and it owns the work of lifting the constraint.
	if child.UDPEncap {
		inbound.UDPEncap = true
		inbound.UDPEncapSPort = transport.NATTPort
		inbound.UDPEncapDPort = transport.NATTPort
	}

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

	if child.UDPEncap {
		outbound.UDPEncap = true
		outbound.UDPEncapSPort = transport.NATTPort
		outbound.UDPEncapDPort = transport.NATTPort
	}

	if err := dp.InstallSA(outbound); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		return fmt.Errorf("child-sa: install outbound: %w", err)
	}
	log.Debug("child-sa: installed outbound SA", "spi", child.OutboundSPI, "ifid", child.IfID)

	// Install policies. Src/Dst are the traffic selectors, the inner traffic each
	// policy matches. TunnelSrc/TunnelDst are the tunnel endpoints, the outer IP
	// header addresses. The kernel resolves a policy to a state through the
	// endpoints, so each policy names the same pair as the SA it must resolve to.
	//
	// A TRANSPORT-mode policy carries NEITHER endpoint. RFC 4301 Section 4.4.1.2 leaves
	// a transport-mode template's addresses unused, and dataplane.tunnelEndpoints
	// REJECTS a non-tunnel policy that carries any, so supplying them here would fail the
	// install outright rather than be ignored.
	inTunnelSrc, inTunnelDst := child.RemoteAddr, child.LocalAddr
	outTunnelSrc, outTunnelDst := child.LocalAddr, child.RemoteAddr
	if mode == modeTransport {
		inTunnelSrc, inTunnelDst = nil, nil
		outTunnelSrc, outTunnelDst = nil, nil
	}

	if err := dp.InstallPolicy(dataplane.SPParams{
		Src:        child.TSRemote,
		Dst:        child.TSLocal,
		Dir:        dataplane.SADirIn,
		Proto:      protoESP,
		Mode:       mode,
		IfID:       child.IfID,
		ReqID:      child.ReqID,
		UpperProto: selectorProto(child),
		SrcPort:    selectorPort(child, false),
		DstPort:    selectorPort(child, true),
		TunnelSrc:  inTunnelSrc,
		TunnelDst:  inTunnelDst,
	}); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		_ = dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP)
		return fmt.Errorf("child-sa: install inbound policy: %w", err)
	}

	if err := dp.InstallPolicy(dataplane.SPParams{
		Src:        child.TSLocal,
		Dst:        child.TSRemote,
		Dir:        dataplane.SADirOut,
		Proto:      protoESP,
		Mode:       mode,
		IfID:       child.IfID,
		ReqID:      child.ReqID,
		UpperProto: selectorProto(child),
		SrcPort:    selectorPort(child, true),
		DstPort:    selectorPort(child, false),
		TunnelSrc:  outTunnelSrc,
		TunnelDst:  outTunnelDst,
	}); err != nil {
		_ = dp.RemovePolicy(child.TSRemote, child.TSLocal, dataplane.SADirIn)
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

// removeChildSA tears down an installed Child SA from the dataplane.
func removeChildSA(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil || child == nil {
		return
	}
	removeChildSAOutgoing(child, dp, log)
	removeChildSAIncoming(child, dp, log)
	child.Clear()
	log.Info("child-sa: removed", "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI)
}

// removeChildSAOutgoing removes the half of a Child SA pair this node SENDS on.
//
// RFC 7296 Section 1.4.1 needs the two halves separable: in the crossing case a node
// deletes "the outgoing SAs while processing the request and the incoming SAs while
// processing the response". Callers that close a whole pair use removeChildSA above.
func removeChildSAOutgoing(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil || child == nil {
		return
	}
	if err := dp.RemovePolicy(child.TSLocal, child.TSRemote, dataplane.SADirOut); err != nil {
		log.Debug("child-sa: remove outbound policy", "error", err)
	}
	if err := dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP); err != nil {
		log.Debug("child-sa: remove outbound SA", "error", err)
	}
}

// removeChildSAIncoming removes the half of a Child SA pair this node RECEIVES on.
// The companion of removeChildSAOutgoing above.
func removeChildSAIncoming(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil || child == nil {
		return
	}
	if err := dp.RemovePolicy(child.TSRemote, child.TSLocal, dataplane.SADirIn); err != nil {
		log.Debug("child-sa: remove inbound policy", "error", err)
	}
	if err := dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP); err != nil {
		log.Debug("child-sa: remove inbound SA", "error", err)
	}
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
