// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA creation and teardown
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
	protoESP     = 50
	modeTunnel   = 2
	defaultReqID = 1
	replayWindow = 32
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

	child := &ChildSA{
		InboundSPI:       inSPI,
		OutboundSPI:      outSPI,
		LocalAddr:        srcIP,
		RemoteAddr:       dstIP,
		IfID:             ifID,
		TSLocal:          tsLocal,
		TSRemote:         tsRemote,
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
		Mode:      modeTunnel,
		ReqID:     child.ReqID,
		ReplayWin: replayWindow,
		EncAlgo:   encAlgo,
		EncKey:    inEnc,
		AuthAlgo:  authAlgo,
		AuthKey:   inInteg,
		IsAEAD:    isAEAD,
	}

	// RFC 3948: UDP encapsulation follows the PORT this SA runs on, not the NAT
	// verdict. RFC 7296 Section 2.23 lets either side encapsulate
	// "irrespective of the choice made by the other side",
	// and a peer that encapsulates ESP runs its IKE on port 4500 too. The float is
	// therefore the signal, and it is set both when a NAT is detected and when an
	// AUTHENTICATED message arrives on port 4500 (sa.adoptAuthenticatedEndpoint).
	//
	// RFC 7296 Section 2.23 MUST NOT (rfc/full/rfc7296.txt:3544): "UDP encapsulation
	// MUST NOT be done on port 500." Both ports below are the NAT-T constant, and the
	// branch runs only for a floated SA, so no path here can encapsulate on port 500.
	//
	// MEASURED KERNEL CONSTRAINT, and it bounds what this can achieve. On Linux XFRM
	// one inbound state accepts exactly ONE of the two ESP forms.
	//
	// A state carrying an ESP-in-UDP template rejects bare ESP with
	// XfrmInStateMismatch. A state without one rejects encapsulated ESP the same way.
	// Two states on one SPI do not help. The state lookup is not encapsulation-aware,
	// so it returns the first and the mismatch check then drops the packet.
	// TestEncapKernelBindsOneESPFormPerState records the truth table.
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
		Mode:      modeTunnel,
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
	if err := dp.InstallPolicy(dataplane.SPParams{
		Src:       child.TSRemote,
		Dst:       child.TSLocal,
		Dir:       dataplane.SADirIn,
		Proto:     protoESP,
		Mode:      modeTunnel,
		IfID:      child.IfID,
		ReqID:     child.ReqID,
		TunnelSrc: child.RemoteAddr,
		TunnelDst: child.LocalAddr,
	}); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		_ = dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP)
		return fmt.Errorf("child-sa: install inbound policy: %w", err)
	}

	if err := dp.InstallPolicy(dataplane.SPParams{
		Src:       child.TSLocal,
		Dst:       child.TSRemote,
		Dir:       dataplane.SADirOut,
		Proto:     protoESP,
		Mode:      modeTunnel,
		IfID:      child.IfID,
		ReqID:     child.ReqID,
		TunnelSrc: child.LocalAddr,
		TunnelDst: child.RemoteAddr,
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
	if err := dp.RemovePolicy(child.TSLocal, child.TSRemote, dataplane.SADirOut); err != nil {
		log.Debug("child-sa: remove outbound policy", "error", err)
	}
	if err := dp.RemovePolicy(child.TSRemote, child.TSLocal, dataplane.SADirIn); err != nil {
		log.Debug("child-sa: remove inbound policy", "error", err)
	}
	if err := dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP); err != nil {
		log.Debug("child-sa: remove outbound SA", "error", err)
	}
	if err := dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP); err != nil {
		log.Debug("child-sa: remove inbound SA", "error", err)
	}
	child.Clear()
	log.Info("child-sa: removed", "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI)
}

// narrowTS computes the intersection of two IP networks.
// RFC 7296 Section 2.9: responder may narrow but never widen.
func narrowTS(proposed, allowed *net.IPNet) *net.IPNet {
	if proposed == nil || allowed == nil {
		return nil
	}
	if allowed.Contains(proposed.IP) && maskLen(proposed.Mask) >= maskLen(allowed.Mask) {
		return proposed
	}
	if proposed.Contains(allowed.IP) && maskLen(allowed.Mask) >= maskLen(proposed.Mask) {
		return allowed
	}
	return nil
}

func maskLen(m net.IPMask) int {
	ones, _ := m.Size()
	return ones
}

func ipToFullNet(ip net.IP) *net.IPNet {
	if ip4 := ip.To4(); ip4 != nil {
		return &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
}
