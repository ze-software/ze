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
	"slices"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
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
	// RFC 4303 Section 3.4.3 prefers a 64-packet anti-replay window and says
	// it SHOULD be the default.
	replayWindow = 64
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

	// PolicyPriority is where this Child SA's SPD entries sit in the operator's ordering
	// of the Security Policy Database (ipsec.SiteToSitePeer.PolicyPriority). LOWER VALUE
	// MEANS HIGHER PRECEDENCE. A rekey inherits it, because a rekey replaces states and
	// keeps the entry, so the entry's rank must not move under the operator.
	//
	// The zero value is not a rank. childPolicies resolves it to
	// dataplane.PriorityChildSA, exactly as it resolves an unset Mode, so a Child SA
	// built by a test that fills the struct literally installs at the documented default
	// instead of at rank 0, which would outrank the IKE control-plane bypass.
	PolicyPriority uint32

	// Owner is the configured peer this Child SA belongs to. It is the identity the
	// dataplane uses to tell this peer's re-install of a selector from a DIFFERENT
	// peer's takeover of it, which the kernel cannot tell apart on its own
	// (dataplane.SPParams.Owner). A rekey inherits it, so the replacement upserts the
	// retired pair's policies instead of being refused.
	Owner string

	// innerGuard is the cross-peer inner-selector registry this Child SA is recorded in
	// while the dataplane holds it, with the remote selectors its peer's operator
	// configured (RFC 3948 Section 5.1, inner_selectors.go). installChildSA claims it
	// and removeChildSAExcept releases it. A rekey inherits it.
	innerGuard innerSelectorGuard

	// Selectors is the negotiated selector set in TSi/TSr orientation, carrying the
	// port and protocol of each entry as well as its prefix.
	//
	// It is the SCOPE CURRENTLY IN USE that RFC 7296 Section 2.9.2 forbids a rekey from
	// narrowing below, so respondChildRekey passes it as the narrowing floor. TSLocal
	// and TSRemote above hold only the first pair's prefixes, which cannot express the
	// scope of a multi-selector or port-restricted SA.
	//
	// SelectorsLocalIsTSi below says which half of each pair is THIS node's side. The
	// two are read together and MUST travel together: a set of selectors whose
	// orientation is unknown cannot be turned into a policy.
	Selectors []tsPair

	// SelectorsLocalIsTSi is true when the I half of Selectors is this node's side.
	//
	// RFC 7296 Section 2.9 makes TSi the selector of the exchange INITIATOR, so the
	// orientation of Selectors is fixed by whoever started the exchange that negotiated
	// it. That exchange is over. A later rekey does not move the stored selectors, so it
	// MUST NOT move this flag either: newRekeyedChild inherits both unchanged.
	//
	// It is NOT LocalIsInitiator. That field is the KEYMAT role of the exchange that
	// keyed THIS pair (Section 2.17) and it flips every time the other end starts the
	// rekey. Reading it to orient a selector set that did not move is how a role-flipping
	// rekey installed a PORT-SWAPPED policy: the kernel then protects the peer's port as
	// if it were ours, and samePolicySelector stops recognizing the pair the replacement
	// shares its policy with.
	SelectorsLocalIsTSi bool

	// Mode is the encapsulation mode this Child SA was installed with:
	// dataplane.ModeTunnel or dataplane.ModeTransport.
	//
	// RFC 7296 Section 1.3.1 makes tunnel the default and transport the negotiated
	// exception, so a Child SA built without a decision carries tunnel. It is read at
	// every SA and policy install, and a transport-mode install carries NO tunnel
	// endpoints (dataplane.tunnelEndpoints refuses them).
	Mode uint8

	// UDPEncap records that this Child SA RECEIVES UDP-encapsulated ESP, on the local
	// port childEncapPorts (create_child.go) answers: the MOBIKE local path's port when
	// one is recorded, else the port the bound NAT-T socket holds, which is never
	// assumed to be 4500. It is the form the inbound XFRM template selects, and nothing else: the form ze
	// SENDS follows the NAT verdict alone, and installChildSA states why.
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
	// In production the FIRST implies the second: every site that sets NATDetected
	// also floats the SA. They are kept separate because they answer different
	// questions, and merging them is a defect recorded by
	// spec-fixit-ike-responder-natt-port-float.
	//
	// The implication does NOT run the other way, and that is why the disjunct below
	// is not redundant. A peer that moved to 4500 with no NAT present floats
	// localPort through adoptAuthenticatedEndpoint (sa.go) and sets NATDetected
	// nowhere. That case is the one this leaf exists to catch.
	//
	// The port term decided the SEND form too until 2026-08-02, and that broke interop.
	// strongSwan 5.9.14 moves IKE to 4500 after IKE_SA_INIT with no NAT present, which
	// RFC 7296 Section 2.23 permits (rfc/full/rfc7296.txt:3538), and then sends BARE
	// ESP. Ze read the float as an encapsulation signal and encapsulated toward it, so
	// the peer's kernel dropped every packet. MEASURED: ze's outbound state carried
	// "encap type espinudp" with oseq 0x4 while strongSwan's carried no encap line and
	// counted XfrmInStateMismatch 4, and scenarios responder-psk and cookie-challenge
	// failed on exactly that. The send form now reads NATDetected alone, so this leaf
	// governs reception only and the conflict is gone.
	//
	// The disjunct MUST NOT be deleted to chase that interop either. Dropping it trades
	// away RFC7296-2.23-11, which asks that the encapsulated form be RECEIVED with no
	// NAT detected. One Linux XFRM inbound state still binds exactly ONE ESP form (Linux
	// limitation LL-XFRM-STATE-ONE-ESP-FORM), so the bare form strongSwan sends is served beside the kernel instead
	// (SAParams.AcceptBothESPForms, dataplane/espform.go).
	UDPEncap bool
	// udpLocalPort and udpRemotePort are the UDP encapsulation ports, taken from the IKE
	// SA by childEncapPorts whenever UDPEncap is set (RFC 3948 Section 2.1): this node's
	// NAT-T socket port and the peer's authenticated IKE port. Zero when UDPEncap is
	// false, where no state reads them. A MOBIKE move or a rebind migrates them.
	udpLocalPort, udpRemotePort uint16

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

	// dataplaneRemoving is protected by dataplaneObservation. A published child
	// remains inconclusive between removal and its replacement/clear publication.
	dataplaneRemoving bool
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
	enc, integ, err := resolveESPTransforms(prop)
	if err != nil {
		return nil, fmt.Errorf("child-sa: %w", err)
	}

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

	var inSPI uint32
	if sa.ChildInboundSPI == 0 {
		inSPI, err = generateESPSPI()
		if err != nil {
			keys.Clear()
			return nil, fmt.Errorf("child-sa: generate inbound SPI: %w", err)
		}
	} else {
		inSPI = sa.ChildInboundSPI
	}
	// The outbound SPI is the PEER's number, from SAr2 or SAi2, and never ze's. RFC 4303
	// Section 2.1 reserves 0, so no peer SPI reaches this field as 0: handleAuthResponse
	// (fsm.go) and buildAuthResponse (responder.go) refuse it. A 0 here therefore means
	// no peer SPI was recorded, and the Child SA is refused rather than keyed on a
	// number the peer never allocated.
	if sa.ChildOutboundSPI == 0 {
		keys.Clear()
		return nil, errors.New("child-sa: no peer ESP SPI recorded")
	}
	outSPI := sa.ChildOutboundSPI

	// RFC 7296 Section 2.9: TSi is the initiator's selector, TSr the responder's.
	// Our local selector is therefore TSi when we are the initiator and TSr when we
	// are the responder (initiator path unchanged: local=TSi, remote=TSr).
	negLocal, negRemote := sa.NegotiatedTSi, sa.NegotiatedTSr
	if !sa.IsInitiator {
		negLocal, negRemote = sa.NegotiatedTSr, sa.NegotiatedTSi
	}

	// RFC 7296 Section 1.3.1: "Except when using this option to negotiate transport
	// mode, all Child SAs will use tunnel mode." The mode is therefore tunnel unless
	// USE_TRANSPORT_MODE was both requested and echoed (sa.UseTransportMode).
	mode := modeTunnel
	if sa.UseTransportMode {
		mode = modeTransport
	}

	child, err := newChildSA(sa, localAddr, remoteAddr, &childSpec{
		inSPI:    inSPI,
		outSPI:   outSPI,
		keys:     keys,
		espGroup: espGroup,
		ifID:     ifID,
		tsLocal:  negLocal,
		tsRemote: negRemote,
		// sa.NegotiatedPairs is in the TSi/TSr orientation of the exchange that produced
		// it, and the first Child SA's selectors come from IKE_AUTH. This node's IKE_AUTH
		// role IS sa.IsInitiator, so that is the orientation, exactly as it is for the
		// TSLocal/TSRemote swap above.
		selectors:           sa.NegotiatedPairs,
		selectorsLocalIsTSi: sa.IsInitiator,
		mode:                mode,
		localIsInitiator:    sa.IsInitiator,
	})
	if err != nil {
		keys.Clear()
		// The responder's IKE_AUTH refusal names the address fix from this sentinel
		// (childRefusedResponderFix, buildChildlessAuthResponse).
		return nil, fmt.Errorf("%w: %w", errChildAddress, err)
	}

	if dp == nil {
		log.Debug("child-sa: no dataplane, skipping SA install")
		return child, nil
	}

	if err := installChildSA(child, prop, dp, log); err != nil {
		log.Debug("child-sa: install error", "error", err, "xfrm_unsupported", isXFRMUnsupported(err))
		if !isXFRMUnsupported(err) {
			keys.Clear()
			// The responder's IKE_AUTH refusal names the install fix from this sentinel
			// (childRefusedResponderFix, buildChildlessAuthResponse).
			return nil, fmt.Errorf("%w: %w", errChildInstall, err)
		}

		warnDegraded(child, log, err)
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

// installChildSA installs a Child SA into the dataplane, after recording its tunnel
// selectors in the cross-peer registry.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from
// occurring." The narrowing filter (dropForeignInnerOverlaps) already dropped every pair
// another peer held, but two peer sessions run concurrently, so both can pass it before
// either installs. The record is therefore taken here, atomically with its check and
// before any dataplane call, and a Child SA that lost the race is refused with an error
// wrapping errTSUnacceptable. Every responder installs before it answers, so that error
// still reaches the peer as TS_UNACCEPTABLE. A failed install releases the record, so
// a Child SA the dataplane never held never locks another peer out.
func installChildSA(child *ChildSA, prop ipsec.ESPProposal, dp dataplane.Dataplane, log *slog.Logger) error {
	if err := child.innerGuard.claim(child); err != nil {
		return err
	}
	if err := installChildDataplane(child, prop, dp, log); err != nil {
		child.innerGuard.release(child)
		return err
	}
	return nil
}

// installChildDataplane writes the Child SA's states and policies into the dataplane.
func installChildDataplane(child *ChildSA, prop ipsec.ESPProposal, dp dataplane.Dataplane, log *slog.Logger) error {
	beginDataplaneWrite()
	defer endDataplaneWrite()

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
		Dir:       dataplane.SADirIn,
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

	// A tunnel-mode inbound state carries no selector. Linux enforces the negotiated
	// selectors of a tunnel-mode SA through the inbound require-policy installed below:
	// __xfrm_policy_check drops a decapsulated packet that matches no inbound policy
	// while its secpath holds a tunnel-mode state (XfrmInNoPols). A transport-only
	// secpath passes that check, so a transport-mode state carries the selector itself,
	// and the kernel then drops a packet outside it (XfrmInStateMismatch).
	//
	// The selector is built from the inbound policy, so the state and the policy cannot
	// disagree. One XFRM state holds ONE selector (Linux limitation
	// LL-XFRM-STATE-ONE-SELECTOR, docs/architecture/linux-limitations.md), so a transport-mode Child SA carries
	// exactly one negotiated pair (inboundStateSelector).
	if mode == modeTransport {
		// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1,
		// the entry for an inbound SA in the SAD MUST be initially populated with the
		// value or values negotiated at the time the SA was created."
		sel, err := inboundStateSelector(childPolicies(child, dataplane.SADirIn))
		if err != nil {
			return fmt.Errorf("child-sa: inbound state selector: %w", err)
		}
		inbound.Sel = sel
	}
	// The ports are the IKE SA's, as childEncapPorts answered them when the Child SA
	// was built. They are set whenever UDPEncap is, and NATDetected implies UDPEncap, so
	// every branch below that reads them finds them set.
	localPort, remotePort := child.udpLocalPort, child.udpRemotePort
	// The second half of that check. takeEncapPorts is a step each builder must
	// remember, so a child that skipped it would install an ESP-in-UDP template on port
	// 0 here, and the XFRM backend passes the ports through without a check. It is
	// refused before anything is installed.
	if child.UDPEncap {
		if localPort == 0 {
			return fmt.Errorf("%w: peer %s: Child SA reached install with no local port (ze defect)",
				errChildEncapPort, child.Owner)
		}
		if remotePort == 0 {
			return fmt.Errorf("%w: peer %s: Child SA reached install with no peer port (ze defect)",
				errChildEncapPort, child.Owner)
		}
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
	// MUST NOT be done on port 500." This node's port below is the NAT-T socket's, and
	// childEncapPorts refuses a Child SA whose own port would be 500. The PEER's port is
	// whatever its authenticated IKE endpoint is, 500 included when a NAT mapped it
	// there, because RFC 3948 Section 2.1 makes it the port IKE uses.
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
		inbound.UDPEncapSPort = remotePort
		inbound.UDPEncapDPort = localPort
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
		Dir:       dataplane.SADirOut,
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
	// that expects bare ESP, and scenario responder-psk recorded the result: the
	// tunnel established and strongSwan accepted no ESP at all.
	//
	// child.UDPEncap still carries the port term, and the inbound template above still
	// uses it. That asymmetry is deliberate: Ze RECEIVES the encapsulated form on any
	// floated SA, which is what RFC7296-2.23-11 requires, while SENDING the form the NAT
	// verdict calls for.
	if child.NATDetected {
		outbound.UDPEncap = true
		outbound.UDPEncapSPort = localPort
		outbound.UDPEncapDPort = remotePort
	}

	if err := dp.InstallSA(outbound); err != nil {
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		return fmt.Errorf("child-sa: install outbound: %w", err)
	}
	log.Debug("child-sa: installed outbound SA", "spi", child.OutboundSPI, "ifid", child.IfID)

	// The policies below can capture ze's own IKE traffic whenever the negotiated
	// selector covers the two addresses IKE runs between, which the ordinary
	// 0.0.0.0/0 site-to-site selector always does. The exemption that stops it is
	// installed once, on the first configuration apply, and it outlives every Child SA: see
	// installIKEBypass in bypass.go for why it is not installed here.
	//
	// RFC 4301 Section 4.4.2: one inbound and one outbound policy per negotiated pair,
	// every one protecting with this SA. The inbound ones go first, so no outbound
	// traffic is sent before the answering direction is accepted.
	inPolicies := childPolicies(child, dataplane.SADirIn)
	policies := slices.Concat(inPolicies, childPolicies(child, dataplane.SADirOut))
	for i, p := range policies {
		err := dp.InstallPolicy(p)
		if err == nil {
			continue
		}
		direction := "outbound"
		if i < len(inPolicies) {
			direction = "inbound"
		}
		// Owner-aware, so a peer whose install was refused cannot roll back over the
		// owning peer's live policy (dataplane.SPParams.Owner). Through removeZePolicy,
		// so a rollback removal that fails leaves its entry in the SPD ledger
		// (spd_left.go) and releaseSPD keeps the catch-all beneath it at exit.
		for _, installed := range policies[:i] {
			if rmErr := removeZePolicy(dp, installed); rmErr != nil {
				log.Warn("child-sa: could not roll back a policy after a failed install; it stays installed",
					"dir", dirLabel(installed.Dir), "src", installed.Src, "dst", installed.Dst, "error", rmErr)
			}
		}
		_ = dp.RemoveSA(child.InboundSPI, child.LocalAddr, protoESP)
		_ = dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP)
		return fmt.Errorf("child-sa: install %s policy %v -> %v: %w", direction, p.Src, p.Dst, err)
	}

	setChildRemoving(child, false)
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

// childPolicies builds the Security Policies this Child SA installs for one direction:
// one per negotiated selector pair, every one protecting with the same SA.
//
// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1, the
// entry for an inbound SA in the SAD MUST be initially populated with the value or
// values negotiated at the time the SA was created." A peer answer of several TSi/TSr
// pairs negotiates several values, and each reaches the dataplane as its own policy.
// XFRM resolves every one of them to the one state through the template (ReqID, SPI
// and addresses), and VPP through the SAD id, so several policies share one SA.
//
// A Child SA that negotiated no selector set (Selectors empty) installs the one policy
// its TSLocal and TSRemote describe, every protocol and every port.
//
// Install AND remove both go through it, so the delete selectors are the install
// selectors by construction.
func childPolicies(child *ChildSA, dir dataplane.SADir) []dataplane.SPParams {
	if len(child.Selectors) == 0 {
		return []dataplane.SPParams{childPairPolicy(child, dir, policyPair{
			local:  child.TSLocal,
			remote: child.TSRemote,
		})}
	}
	out := make([]dataplane.SPParams, 0, len(child.Selectors))
	for _, pair := range child.Selectors {
		pp, _, ok := childPolicyPair(child, pair)
		if !ok {
			continue
		}
		out = append(out, childPairPolicy(child, dir, pp))
	}
	return out
}

// childPolicyPair converts one negotiated pair of child to this node's orientation, and
// returns its remote half as negotiated. ok is false for a pair that installs no policy.
// childPolicies and the cross-peer registry (innerSelectorPairOf) both build from it, so
// what the registry compares is what the dataplane installs.
func childPolicyPair(child *ChildSA, pair tsPair) (policyPair, tsSelector, bool) {
	// Selectors are in TSi/TSr orientation, and SelectorsLocalIsTSi says which half
	// is ours. LocalIsInitiator answers a DIFFERENT question -- which KEYMAT half
	// keys this pair -- and it flips at a rekey the peer starts while the selectors
	// stay put.
	local, remote := pair.R, pair.I
	if child.SelectorsLocalIsTSi {
		local, remote = pair.I, pair.R
	}
	// RFC 4301 Section 4.4.2: "the entry for an inbound SA in the SAD MUST be
	// initially populated with the value or values negotiated at the time the SA
	// was created." A pair's two halves each carry a protocol, and a packet matches
	// the pair only when it matches both (RFC 7296 Section 3.13), so the protocol the
	// pair negotiated is their intersection: TSi "any" with TSr TCP is TCP. Reading
	// one half alone installs "any" for that pair, wider than the negotiation.
	//
	// A pair whose protocols are disjoint matches no packet, so it protects nothing
	// and installs no policy. crossPairs and acceptablePairs never produce one.
	proto, ok := intersectProto(pair.I.Proto, pair.R.Proto)
	if !ok {
		return policyPair{}, tsSelector{}, false
	}
	return policyPair{
		local:      local.Net,
		remote:     remote.Net,
		upperProto: proto,
		localPort:  selectorPort(local),
		remotePort: selectorPort(remote),
	}, remote, true
}

// policyPair is one negotiated selector pair in this node's orientation, in the form a
// policy is built from.
type policyPair struct {
	local, remote         *net.IPNet
	upperProto            uint8
	localPort, remotePort dataplane.PortMatch
}

// childPairPolicy builds the Security Policy this Child SA installs for one
// direction and one negotiated pair. The kernel identifies a policy by its whole selector and by
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
func childPairPolicy(child *ChildSA, dir dataplane.SADir, pair policyPair) dataplane.SPParams {
	// A Child SA built by a test that fills the struct literally carries the zero
	// value, which is not a mode in the dataplane vocabulary. Resolved here exactly as
	// installChildSA resolves it, so the two agree.
	mode := child.Mode
	if mode == 0 {
		mode = modeTunnel
	}

	// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the
	// entries to express a desired access control policy."
	//
	// An unset rank resolves to the default the same way the mode above does, and it
	// MUST NOT be passed through. Rank 0 outranks the IKE control-plane bypass, so the
	// entry would capture the exchange that builds and rekeys the SA it protects
	// (dataplane.PriorityIKEBypass).
	priority := int(child.PolicyPriority)
	if priority == 0 {
		priority = dataplane.PriorityChildSA
	}

	src, dst := pair.local, pair.remote
	tunnelSrc, tunnelDst := child.LocalAddr, child.RemoteAddr
	srcPort, dstPort := pair.localPort, pair.remotePort
	// The SA this policy protects with, for a dataplane that binds by SA id rather
	// than by template addresses (dataplane.SPParams.SAID). It is the SPI of the SA
	// installed for the SAME direction.
	said := child.OutboundSPI
	if dir == dataplane.SADirIn {
		src, dst = pair.remote, pair.local
		tunnelSrc, tunnelDst = child.RemoteAddr, child.LocalAddr
		srcPort, dstPort = pair.remotePort, pair.localPort
		said = child.InboundSPI
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
		Priority:   priority,
		UpperProto: pair.upperProto,
		SrcPort:    srcPort,
		DstPort:    dstPort,
		TunnelSrc:  tunnelSrc,
		TunnelDst:  tunnelDst,
		SAID:       said,
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
//
// A Child SA installs one policy per negotiated pair (childPolicies), so the two answer
// to the same policies only when every pair matches, in order. A partial overlap is
// not sharing: removing the retired pair's policies would then take the survivor's
// shared ones with it, and keeping them all would leave the retired pair's own.
func samePolicySelector(a, b *ChildSA) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return false
	}
	pa, pb := childPolicies(a, dataplane.SADirOut), childPolicies(b, dataplane.SADirOut)
	if len(pa) != len(pb) {
		return false
	}
	for i := range pa {
		if !samePolicyEntry(pa[i], pb[i]) {
			return false
		}
	}
	return true
}

// samePolicyEntry reports whether two policies carry the same kernel selector. A
// policy with no prefix on either side is never the same as anything, because the
// selector it would install is not one this Child SA negotiated.
func samePolicyEntry(a, b dataplane.SPParams) bool {
	if a.Src == nil || a.Dst == nil || b.Src == nil || b.Dst == nil {
		return false
	}
	return a.Src.String() == b.Src.String() &&
		a.Dst.String() == b.Dst.String() &&
		a.IfID == b.IfID &&
		a.UpperProto == b.UpperProto &&
		a.SrcPort == b.SrcPort &&
		a.DstPort == b.DstPort
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
// interop scenario child-rekey reported "no ESP traffic after the rekey" with both Child SAs
// installed and healthy, because the peer's Delete for the old SPI took the
// survivor's policy with it.
//
// The states are always removed: they are keyed by SPI and are never shared.
//
// keep is nil when the whole tunnel is going away, and then the policies go too. At
// session teardown the retired pair is passed keep = the live child, so its policy
// survives the first call and is removed by the second -- once, in total.
func removeChildSAExcept(child, keep *ChildSA, dp dataplane.Dataplane, log *slog.Logger) {
	beginDataplaneWrite()
	defer endDataplaneWrite()

	if child == nil {
		return
	}
	// RFC 3948 Section 5.1: the record is released before the nil-dataplane return
	// below, so no removal path leaves another peer locked out of these selectors.
	child.innerGuard.release(child)
	if dp == nil {
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
	beginDataplaneWrite()
	defer endDataplaneWrite()

	if dp == nil || child == nil {
		return
	}
	setChildRemoving(child, true)
	if dropPolicy {
		// RemovePolicyParams, not the three-argument RemovePolicy: it names the whole
		// selector the install used, and it carries the Owner the dataplane refuses a
		// foreign delete on (dataplane.SPParams.Owner). A refusal here means another
		// peer owns the selector, which is a takeover the install should already have
		// stopped, so it is logged at Warn rather than swallowed.
		for _, p := range childPolicies(child, dataplane.SADirOut) {
			if err := removeZePolicy(dp, p); err != nil {
				logPolicyRemoveFailure(log, "outbound", child, err)
			}
		}
	}
	if err := dp.RemoveSA(child.OutboundSPI, child.RemoteAddr, protoESP); err != nil {
		log.Debug("child-sa: remove outbound SA", "error", err)
	}
}

// removeChildSAIncoming removes the half of a Child SA pair this node RECEIVES on.
// The companion of removeChildSAOutgoing above.
func removeChildSAIncoming(child *ChildSA, dp dataplane.Dataplane, log *slog.Logger, dropPolicy bool) {
	beginDataplaneWrite()
	defer endDataplaneWrite()

	if dp == nil || child == nil {
		return
	}
	setChildRemoving(child, true)
	if dropPolicy {
		for _, p := range childPolicies(child, dataplane.SADirIn) {
			if err := removeZePolicy(dp, p); err != nil {
				logPolicyRemoveFailure(log, "inbound", child, err)
			}
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
// let two peers negotiate one selector. A netlink failure on teardown stays at Debug:
// ENOENT (the policy is already gone) is ordinary, and any other failure is recorded in
// the SPD ledger (removeZePolicy, spd_left.go), which releaseSPD reports at Warn on exit.
func logPolicyRemoveFailure(log *slog.Logger, dir string, child *ChildSA, err error) {
	if owned, ok := errors.AsType[*dataplane.PolicyOwnedError](err); ok {
		log.Warn("child-sa: refused to remove a policy another peer owns",
			"direction", dir, "peer", child.Owner, "owner", owned.HeldBy, "error", err)
		return
	}
	log.Debug("child-sa: remove policy", "direction", dir, "error", err)
}

// selectorPort maps a negotiated port form to the kernel's port-plus-mask selector.
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
func selectorPort(side tsSelector) dataplane.PortMatch {
	switch side.Port.Form {
	case ipsec.PortSingle:
		return dataplane.ExactPortMatch(side.Port.Port)
	case ipsec.PortOpaque:
		return dataplane.ExactPortMatch(0)
	case ipsec.PortAny, 0:
		return dataplane.AnyPortMatch()
	default:
		panic("BUG: unknown traffic selector port form")
	}
}

// inboundStateSelector builds the selector a transport-mode inbound state carries
// from the inbound policies of the same Child SA.
//
// RFC 4301 Section 4.4.2: "For each of the selectors defined in Section 4.4.1.1, the
// entry for an inbound SA in the SAD MUST be initially populated with the value or
// values negotiated at the time the SA was created."
//
// An XFRM state carries ONE selector (x->sel), and one SPI names one state (Linux
// limitation LL-XFRM-STATE-ONE-SELECTOR, docs/architecture/linux-limitations.md), so the
// state holds the negotiated values exactly only when exactly one pair was negotiated.
// Both producers guarantee that in transport mode: the responder narrows to one pair
// (narrowChildSelectors) and the initiator refuses an answer of several
// (recordInitiatorSelectors, ts_narrow.go). Several policies here are therefore a Ze
// defect, refused rather than widened: a selector wider than the pairs would admit a
// decrypted packet the SA never negotiated (RFC 4301 Section 5.2).
func inboundStateSelector(policies []dataplane.SPParams) (*dataplane.SASelector, error) {
	if len(policies) != 1 {
		return nil, fmt.Errorf("a transport-mode Child SA holds %d negotiated pairs, and its one state selector holds exactly one", len(policies))
	}
	p := policies[0]
	return &dataplane.SASelector{
		Src:        p.Src,
		Dst:        p.Dst,
		UpperProto: p.UpperProto,
		SrcPort:    p.SrcPort,
		DstPort:    p.DstPort,
	}, nil
}

func ipToFullNet(ip net.IP) *net.IPNet {
	if ip4 := ip.To4(); ip4 != nil {
		return &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
}
