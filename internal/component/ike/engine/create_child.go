// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- new Child SA creation on an established IKE SA
// Related: rekey.go -- the Child SA rekey exchange this one shares its payloads with
// Related: child.go -- createFirstChildSA, the IKE_AUTH Child SA built through newChildSA

package engine

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// childSpec is what one exchange agreed for a Child SA that replaces no other: the
// IKE_AUTH Child SA, or a CREATE_CHILD_SA creation on a childless IKE SA. A rekey
// inherits these fields from the pair it replaces instead (newRekeyedChild).
type childSpec struct {
	inSPI, outSPI uint32
	keys          *crypto.ChildSAKeys
	// espGroup holds exactly the one proposal the exchange accepted.
	espGroup ipsec.ESPGroup
	ifID     uint32
	// tsLocal and tsRemote are the negotiated prefixes, local first. Nil means the
	// host prefix of that side's tunnel endpoint.
	tsLocal, tsRemote *net.IPNet
	// selectors is the exchange's answer in its own TSi/TSr orientation, and
	// selectorsLocalIsTSi says whether this node's side is TSi in it.
	selectors           []tsPair
	selectorsLocalIsTSi bool
	mode                uint8
	// localIsInitiator is this node's role in THE EXCHANGE THAT KEYED the pair, which
	// selects the ESP key halves (RFC 7296 Section 2.17), whatever its IKE SA role.
	localIsInitiator bool
}

// newChildSA builds the Child SA one exchange negotiated, taking the tunnel endpoints,
// owner, policy rank and NAT encapsulation from the IKE SA. It installs nothing.
//
// RFC 4555 Section 3.3 takes tunnel endpoints from the IKE SA, so a MOBIKE SA's current
// path wins over the configured addresses.
func newChildSA(sa *SA, localAddr, remoteAddr string, spec *childSpec) (*ChildSA, error) {
	srcIP := net.ParseIP(localAddr)
	dstIP := net.ParseIP(remoteAddr)
	if sa.mobike.enabled {
		if sa.mobike.local != nil {
			srcIP = append(net.IP(nil), sa.mobike.local.IP...)
		}
		if remote := sa.remoteUDPAddr(); remote != nil {
			dstIP = append(net.IP(nil), remote.IP...)
		}
	}
	if srcIP == nil {
		return nil, fmt.Errorf("child-sa: invalid local address %q", localAddr)
	}
	if dstIP == nil {
		return nil, fmt.Errorf("child-sa: invalid remote address %q", remoteAddr)
	}
	tsLocal, tsRemote := spec.tsLocal, spec.tsRemote
	if tsLocal == nil {
		tsLocal = ipToFullNet(srcIP)
	}
	if tsRemote == nil {
		tsRemote = ipToFullNet(dstIP)
	}

	child := &ChildSA{
		InboundSPI:  spec.inSPI,
		OutboundSPI: spec.outSPI,
		LocalAddr:   srcIP,
		RemoteAddr:  dstIP,
		IfID:        spec.ifID,
		TSLocal:     tsLocal,
		TSRemote:    tsRemote,
		Owner:       sa.PeerName,
		// RFC 4301 Section 4.4.1: the operator orders the SPD entries, and these are two
		// of them.
		PolicyPriority:      sa.PeerCfg.PolicyPriority,
		Selectors:           spec.selectors,
		SelectorsLocalIsTSi: spec.selectorsLocalIsTSi,
		Mode:                spec.mode,
		Keys:                spec.keys,
		ESPGroup:            spec.espGroup,
		ReqID:               defaultReqID,
		NATDetected:         sa.NATDetected,
		UDPEncap:            sa.NATDetected || sa.localPort == transport.NATTPort,
		LocalIsInitiator:    spec.localIsInitiator,
	}
	if sa.mobike.enabled {
		if local := sa.mobike.local; local != nil {
			child.udpLocalPort = uint16(local.Port)
		}
		if remote := sa.remoteUDPAddr(); remote != nil {
			child.udpRemotePort = uint16(remote.Port)
		}
	}
	return child, nil
}

// negotiatedChildTS returns the first negotiated pair as local and remote prefixes. The
// pairs are in the orientation of the exchange that produced them, and localIsTSi says
// which side is this node's. An empty set answers nil, and newChildSA then uses the
// host prefixes of the tunnel endpoints.
func negotiatedChildTS(pairs []tsPair, localIsTSi bool) (*net.IPNet, *net.IPNet) {
	if len(pairs) == 0 {
		return nil, nil
	}
	first := pairs[0]
	if !localIsTSi {
		first = tsPair{I: first.R, R: first.I}
	}
	return first.I.Net, first.R.Net
}

// newChildRequest is a peer's CREATE_CHILD_SA request for a new Child SA, with the
// payloads RFC 7296 Section 1.3.1 makes mandatory present.
type newChildRequest struct {
	ni        []byte
	peerSPI   uint32
	offer     *wire.PayloadSA
	ke        *wire.PayloadKE
	tsi, tsr  *wire.PayloadTS
	transport bool
}

// parseNewChildRequest reads "HDR, SK {SA, Ni, [KEi,] TSi, TSr}" (RFC 7296 Section
// 1.3.1). A request missing a mandatory payload, or naming ESP SPI 0 (RFC 4303 Section
// 2.1 reserves it), is malformed, not unsatisfiable, so it draws INVALID_SYNTAX.
func parseNewChildRequest(inner []wire.PayloadEntry) (newChildRequest, error) {
	var req newChildRequest
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			req.ni = p.NonceData
		case *wire.PayloadKE:
			req.ke = p
		case *wire.PayloadSA:
			req.offer = p
			if s, err := espSPIFromSA(p); err == nil {
				req.peerSPI = s
			}
		case *wire.PayloadNotify:
			// RFC 7296 Section 1.3.1: "The USE_TRANSPORT_MODE notification MAY be
			// included in a request message that also includes an SA payload requesting
			// a Child SA."
			if p.NotifyMsgType == wire.NotifyUseTransportMode {
				req.transport = true
			}
		case *wire.PayloadTS:
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				req.tsi = p
			case wire.PayloadTypeTSr:
				req.tsr = p
			}
		}
	}
	if len(req.ni) == 0 {
		return req, fmt.Errorf("%w: new Child SA request missing Ni", errMalformedRequest)
	}
	if req.peerSPI == 0 {
		return req, fmt.Errorf("%w: new Child SA request carries no usable ESP SPI", errMalformedRequest)
	}
	if req.tsi == nil {
		return req, fmt.Errorf("%w: new Child SA request missing TSi", errMalformedRequest)
	}
	if req.tsr == nil {
		return req, fmt.Errorf("%w: new Child SA request missing TSr", errMalformedRequest)
	}
	return req, nil
}

// selectNewChildESP picks the first configured ESP proposal the peer's offer agrees
// with, under the pfs decision group carries (espDHMatch), and returns it with the
// peer's Proposal Num for that offer, which the answer echoes (RFC 7296 Section 3.3.1).
func selectNewChildESP(offer *wire.PayloadSA, espGroup ipsec.ESPGroup, group crypto.DHGroupID) (ipsec.ESPProposal, uint8, error) {
	for i := range espGroup.Proposals {
		our := espGroup.Proposals[i]
		accepted, ok, err := matchOfferedESP(offer, our, espDHMatch{Want: group})
		if err != nil {
			return ipsec.ESPProposal{}, 0, err
		}
		if !ok {
			continue
		}
		// RFC 7296 Section 3.3 numbers the first proposal of an offer one, so a
		// proposal numbered zero is malformed and is not answered.
		if accepted.Number == 0 {
			break
		}
		return our, accepted.Number, nil
	}
	return ipsec.ESPProposal{}, 0, crypto.ErrNoProposalChosen
}

// respondNewChild answers a peer CREATE_CHILD_SA request for a new Child SA on a
// childless IKE SA, installs the Child SA, and returns the SK-encrypted response. A
// nil Child SA with a response is an INVALID_KE_PAYLOAD answer. An error names the
// refusal notifyForRefusal maps to the error notify.
//
// RFC 7296 Section 1.3.1: "The responder replies (using the same Message ID to
// respond) with the accepted offer in an SA payload, a nonce in the Nr payload, and a
// Diffie-Hellman value in the KEr payload if KEi was included in the request and the
// selected cryptographic suite includes that group." The esp-group's pfs leaf decides whether the request must carry
// KEi, as on the rekey path (childRekeyDHGroup).
//
// espGroup is the session's CONFIGURED group. sa.ESPGroup is not read: on a childless
// IKE SA it is whatever IKE_AUTH left before its Child SA was refused.
func respondNewChild(sa *SA, inner []wire.PayloadEntry, espGroup ipsec.ESPGroup, msgID uint32,
	dp dataplane.Dataplane, log *slog.Logger,
) ([]byte, *ChildSA, error) {
	req, err := parseNewChildRequest(inner)
	if err != nil {
		return nil, nil, err
	}
	group, err := childRekeyDHGroup(sa, espGroup)
	if err != nil {
		return nil, nil, err
	}
	prop, proposalNum, err := selectNewChildESP(req.offer, espGroup, group)
	if err != nil {
		return nil, nil, fmt.Errorf("new Child SA request: %w", err)
	}
	if resp, mismatched, err := invalidKEAnswer(sa, group, req.ke, msgID, log); mismatched {
		return resp, nil, err
	}

	// RFC 7296 Section 1.3.1: "If the request is accepted, the response MUST also include
	// a notification of type USE_TRANSPORT_MODE." Acceptance is this node's decision
	// (decideResponderTransportMode), and it is made before the selectors are narrowed,
	// because transport mode narrows to single addresses.
	sa.PeerRequestedTransport = req.transport
	decideResponderTransportMode(sa)
	// RFC 7296 Section 2.9: narrow the initiator's proposal to the operator's policy. A
	// creation replaces no SA, so there is no scope in use and no floor.
	if err := narrowChildSelectors(sa, req.tsi, req.tsr, nil); err != nil {
		return nil, nil, err
	}
	tsi, tsr := pairsToWire(sa.NegotiatedPairs)
	if tsi == nil || tsr == nil {
		return nil, nil, fmt.Errorf("%w: the negotiated scope %s cannot be put on the wire",
			errTSUnacceptable, pairsText(sa.NegotiatedPairs))
	}

	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		return nil, nil, err
	}
	nr, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, nil, err
	}
	inSPI, err := generateESPSPI()
	if err != nil {
		return nil, nil, err
	}
	enc, integ, err := resolveESPTransforms(prop)
	if err != nil {
		return nil, nil, fmt.Errorf("new Child SA request: %w", err)
	}
	// The answer is encoded before anything is keyed or installed, so a proposal that
	// cannot be encoded leaves no Child SA behind.
	answer, err := espProposalWire(prop, inSPI, proposalNum, group)
	if err != nil {
		return nil, nil, fmt.Errorf("new Child SA request: %w", err)
	}
	// The peer sent Ni, so this node is the responder of the exchange whose nonces key
	// the pair (RFC 7296 Section 2.17).
	keys, ourPub, err := responderChildKeys(sa, group, req.ke, req.ni, nr, enc, integ)
	if err != nil {
		return nil, nil, err
	}

	mode := modeTunnel
	if sa.UseTransportMode {
		mode = modeTransport
	}
	tsLocal, tsRemote := negotiatedChildTS(sa.NegotiatedPairs, false)
	child, err := newChildSA(sa, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, &childSpec{
		inSPI:    inSPI,
		outSPI:   req.peerSPI,
		keys:     keys,
		espGroup: ipsec.ESPGroup{Name: espGroup.Name, Lifetime: espGroup.Lifetime, PFS: espGroup.PFS, Proposals: []ipsec.ESPProposal{prop}},
		ifID:     ifID,
		tsLocal:  tsLocal,
		tsRemote: tsRemote,
		// The peer sent Ni here, so its side is TSi in the set narrowChildSelectors
		// recorded, and this node's is TSr.
		selectors:           sa.NegotiatedPairs,
		selectorsLocalIsTSi: false,
		mode:                mode,
		localIsInitiator:    false,
	})
	if err != nil {
		keys.Clear()
		return nil, nil, err
	}
	if err := installChildTolerant(child, prop, dp, log); err != nil {
		keys.Clear()
		return nil, nil, err
	}

	// RFC 7296 Section 1.3.1: "HDR, SK {SA, Nr, [KEr,] TSi, TSr}".
	out := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{answer}}},
		{Payload: &wire.PayloadNonce{NonceData: nr}},
	}
	if group != dhGroupNone {
		out = append(out, wire.PayloadEntry{
			Payload: &wire.PayloadKE{DHGroup: uint16(group), KeyExchangeData: ourPub},
		})
	}
	if sa.UseTransportMode {
		out = append(out, wire.PayloadEntry{Payload: transportModeNotify()})
	}
	out = append(out, wire.PayloadEntry{Payload: tsi}, wire.PayloadEntry{Payload: tsr})
	resp, err := buildEncryptedMessageEx(sa, out, msgID, wire.ExchangeCreateChildSA, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		if dp != nil {
			removeChildSA(child, dp, log)
		}
		keys.Clear()
		return nil, nil, err
	}
	return resp, child, nil
}

// invalidKEAnswer builds the INVALID_KE_PAYLOAD answer to a Child SA request whose KEi
// is not in the group this node selected, and reports whether the request was answered
// that way. The exchange is then not keyed.
//
// RFC 7296 Section 3.4: "If the selected proposal uses a different Diffie-Hellman group
// (other than NONE), the message MUST be rejected with a Notify payload of type
// INVALID_KE_PAYLOAD." RFC 7296 Section 3.10.1 gives the payload two octets of data,
// the group the initiator must use.
func invalidKEAnswer(sa *SA, group crypto.DHGroupID, reqKE *wire.PayloadKE, msgID uint32, log *slog.Logger) ([]byte, bool, error) {
	if group == dhGroupNone {
		return nil, false, nil
	}
	if reqKE != nil && reqKE.DHGroup == uint16(group) {
		return nil, false, nil
	}
	got := wire.DHGroupNone
	if reqKE != nil {
		got = reqKE.DHGroup
	}
	log.Info("ike: peer Child SA request KE group mismatch", "peer", sa.PeerName,
		"want", uint16(group), "got", got)
	notify := &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifyInvalidKEPayload,
		NotificationData: []byte{byte(uint16(group) >> 8), byte(group)},
	}
	resp, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: notify}},
		msgID, wire.ExchangeCreateChildSA, initiatorFlag(sa)|wire.FlagResponse)
	return resp, true, err
}

// responderChildKeys keys a Child SA this node answers in a CREATE_CHILD_SA exchange and
// returns the KEr value the answer carries, nil without a group.
//
// RFC 7296 Section 2.17, the two KEYMAT forms. Without a group the seed is "Ni | Nr".
// With one, this node completes the exchange from the peer's KEi and the seed gains
// "g^ir (new)" in front, the fresh secret Perfect Forward Secrecy rests on. The KEr
// value is copied before the private half is cleared.
func responderChildKeys(sa *SA, group crypto.DHGroupID, reqKE *wire.PayloadKE, ni, nr []byte,
	enc crypto.EncryptionTransform, integ crypto.IntegrityTransform,
) (*crypto.ChildSAKeys, []byte, error) {
	if group == dhGroupNone {
		keys, err := crypto.DeriveChildSAKeys(sa.Proposal.PRF.ID, sa.SKKeys.SK_d, ni, nr, enc, integ)
		return keys, nil, err
	}
	dh, err := crypto.NewDHExchange(group)
	if err != nil {
		return nil, nil, err
	}
	ourPub := append([]byte(nil), dh.PublicKey...)
	sharedSecret, err := dh.SharedSecret(reqKE.KeyExchangeData)
	dh.Clear()
	if err != nil {
		return nil, nil, err
	}
	keys, err := crypto.DeriveChildSAKeysPFS(sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		sharedSecret, ni, nr, enc, integ)
	clear(sharedSecret)
	if err != nil {
		return nil, nil, err
	}
	return keys, ourPub, nil
}
