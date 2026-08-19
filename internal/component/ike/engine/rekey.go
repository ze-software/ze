// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA and IKE SA rekeying
// Related: ts_narrow.go -- the narrowing floor of RFC 7296 Section 2.9.2
// RFC: rfc/short/rfc7296.md -- Rekeying (Section 2.8), collision (Section 2.8.1)

package engine

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// errTemporaryFailure reports a CREATE_CHILD_SA response whose only content is a
// TEMPORARY_FAILURE notify. RFC 7296 Section 2.25 gives its meaning. A temporary
// condition such as a rekey of the peer's own stopped the exchange. The recipient
// MUST wait before it retries. The caller arms the matching rekey hold.
var errTemporaryFailure = errors.New("ike: peer answered with TEMPORARY_FAILURE")

// hasTemporaryFailure reports whether a payload chain carries a TEMPORARY_FAILURE
// notify. RFC 7296 Section 2.25 keys the wait on this notification alone, so an
// exchange that fails any other way is retried on the next tick as before.
func hasTemporaryFailure(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok &&
			n.NotifyMsgType == wire.NotifyTemporaryFailure {
			return true
		}
	}
	return false
}

// errNoAdditionalSAs reports a rekey the peer refused with NO_ADDITIONAL_SAS. RFC
// 7296 Section 4 makes the answer to that refusal mandatory and specific, so it is
// distinguished from every other rekey failure: "If the responder rejects the
// CREATE_CHILD_SA request with a NO_ADDITIONAL_SAS notification, the implementation
// MUST be capable of instead deleting the old SA and creating a new one".
var errNoAdditionalSAs = errors.New("ike: peer answered with NO_ADDITIONAL_SAS")

// hasNoAdditionalSAs reports whether a payload chain carries a NO_ADDITIONAL_SAS
// notify. RFC 7296 Section 4 describes the peer that sends it as a minimal
// implementation, one that recognizes a CREATE_CHILD_SA request only to reject it.
// Retrying the same exchange against such a peer can never succeed, so this answer
// is separated from a transient failure.
func hasNoAdditionalSAs(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok &&
			n.NotifyMsgType == wire.NotifyNoAdditionalSAs {
			return true
		}
	}
	return false
}

// initiatorFlag returns the header Initiator flag for messages this side sends.
// RFC 7296 §3.1: the flag marks the original initiator of the IKE SA, regardless
// of who initiates a given exchange.
func initiatorFlag(sa *SA) uint8 {
	if sa.IsInitiator {
		return wire.FlagInitiator
	}
	return 0
}

// espSPIFromSA extracts the ESP SPI from the first ESP proposal in an SA payload.
func espSPIFromSA(p *wire.PayloadSA) (uint32, error) {
	for _, prop := range p.Proposals {
		if prop.ProtocolID == wire.ProtocolESP && len(prop.SPI) == 4 {
			return binary.BigEndian.Uint32(prop.SPI), nil
		}
	}
	return 0, fmt.Errorf("child rekey: no ESP SPI in SA payload")
}

// anyChildTSPayloads builds wildcard TSi/TSr payloads.
//
// It is the proposal an UNCONFIGURED peer makes: with no traffic-selector list the
// operator has stated no policy, so Ze asks for everything and lets the responder narrow.
// It is no longer used to build a RESPONSE, where answering with a wildcard widened the
// initiator's proposal (RFC 7296 Section 2.9).
func anyChildTSPayloads(sa *SA) (*wire.PayloadTS, *wire.PayloadTS) {
	remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	isV6 := remoteIP != nil && remoteIP.To4() == nil
	tsAny := anyTrafficSelector(isV6)
	return &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{tsAny}},
		&wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{tsAny}}
}

// proposeChildTSPayloads builds the TSi/TSr of a Child SA REQUEST.
//
// A peer with a configured traffic-selector list proposes exactly that list, so the
// operator's policy reaches the wire. A peer with no list proposes the wildcard, which
// is what every configuration written before the list existed did.
//
// RFC 7296 Section 2.9 puts the initiator's preference in the ORDER of the list, so the
// first configured selector is the first choice a conforming responder must honor.
// It also RECORDS what it proposed on the SA.
//
// RFC 7296 Section 2.9 lets the responder narrow the proposal, and never widen it.
// recordInitiatorSelectors (ts_narrow.go) checks the answer against this record. A proposal
// that is not recorded here is a ceiling the initiator cannot enforce. The producer writes
// the record for that reason, rather than the consumer rebuilding it.
func proposeChildTSPayloads(sa *SA) (*wire.PayloadTS, *wire.PayloadTS) {
	pairs := policyPairs(sa.PeerCfg, true)

	// RFC 7296 Section 2.23.1 constrains a transport-mode proposal to the IKE SA's own
	// address pair, so the rewrite happens BEFORE the wildcard fallback below. A
	// transport-mode peer never falls back to the wildcard: the wildcard carries a range,
	// and the MUST asks for exactly one address.
	if wantsTransportMode(sa) {
		pinned := transportSelectorPairs(sa, pairs)
		if tsi, tsr := pairsToWire(pinned); tsi != nil && tsr != nil {
			sa.ProposedChildPairs = pinned
			return tsi, tsr
		}
		sa.ProposedChildPairs = nil
		return anyChildTSPayloads(sa)
	}

	if len(pairs) == 0 {
		sa.ProposedChildPairs = nil
		return anyChildTSPayloads(sa)
	}
	tsi, tsr := pairsToWire(pairs)
	if tsi == nil || tsr == nil {
		// The wildcard went on the wire, so the wildcard is what was proposed. A record of
		// the pairs here states a ceiling the peer was never told about. The answer to a
		// wildcard is then refused because it goes past that ceiling.
		sa.ProposedChildPairs = nil
		return anyChildTSPayloads(sa)
	}
	sa.ProposedChildPairs = pairs
	return tsi, tsr
}

// initiateChildRekey builds the SK-encrypted CREATE_CHILD_SA request to replace
// oldChild and the pendingRekey state to correlate the response.
// RFC 7296 §1.3.2: N(REKEY_SA), SA, Ni, [KEi], TSi, TSr.
func initiateChildRekey(sa *SA, oldChild *ChildSA) ([]byte, *pendingRekey, error) {
	ni, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, nil, err
	}
	espSPI, saPayload, tsi, tsr, err := buildChildSAPayloads(sa)
	if err != nil {
		return nil, nil, err
	}

	// RFC 7296 §3.10: REKEY_SA notify carries the SPI of the SA being rekeyed.
	spiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spiBytes, oldChild.InboundSPI)
	rekeyNotify := &wire.PayloadNotify{
		ProtocolID:    wire.ProtocolESP,
		SPISize:       4,
		NotifyMsgType: wire.NotifyRekeySA,
		SPI:           spiBytes,
	}

	inner := []wire.PayloadEntry{
		{Payload: rekeyNotify},
		{Payload: saPayload},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: tsi},
		{Payload: tsr},
	}
	msgID := sa.NextMsgID
	msg, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeCreateChildSA, initiatorFlag(sa))
	if err != nil {
		return nil, nil, err
	}
	sa.advanceMsgID()
	return msg, &pendingRekey{
		kind:          rekeyChild,
		messageID:     msgID,
		sentMsg:       msg,
		sentAt:        time.Now(),
		localNonce:    ni,
		newInboundSPI: espSPI,
		oldChild:      oldChild,
	}, nil
}

// applyChildRekeyResponse processes the peer's CREATE_CHILD_SA response to our
// pending child rekey: derives keys from our Ni and the peer's Nr, and installs
// the new Child SA BEFORE the caller removes the old one (make-before-break).
// RFC 7296 §1.3.2, §2.17: KEYMAT = prf+(SK_d, Ni | Nr).
func applyChildRekeyResponse(sa *SA, pending *pendingRekey, inner []wire.PayloadEntry, dp dataplane.Dataplane, log *slog.Logger) (*ChildSA, error) {
	// RFC 7296 Section 2.25: a TEMPORARY_FAILURE answer means the peer is busy, not that
	// the response is malformed. It is read before the payload walk, because such a
	// response carries the notify alone and the walk below would report a missing Nr.
	if hasTemporaryFailure(inner) {
		return nil, errTemporaryFailure
	}
	// RFC 7296 Section 4: a NO_ADDITIONAL_SAS answer is refusal, not delay. It is read
	// here for the same reason as the notify above, and reported distinctly so the
	// caller can take the delete-and-create fallback the section makes mandatory
	// instead of retrying an exchange this peer will never accept.
	if hasNoAdditionalSAs(inner) {
		return nil, errNoAdditionalSAs
	}
	var nr []byte
	var outSPI uint32
	var accepted *wire.PayloadSA
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			nr = p.NonceData
		case *wire.PayloadSA:
			accepted = p
			s, err := espSPIFromSA(p)
			if err != nil {
				return nil, err
			}
			outSPI = s
		}
	}
	if len(nr) == 0 || outSPI == 0 {
		return nil, fmt.Errorf("child rekey response: missing Nr(%d) or ESP SPI(%d)", len(nr), outSPI)
	}

	old := pending.oldChild
	// RFC 7296 Section 3.3.6: the initiator checks the accepted offer against the ESP
	// proposals it sent. It stops the exchange when the two disagree. The replacement
	// Child SA then takes the suite the peer selected. That suite is not always the
	// first proposal of the offer. The section lets the responder "select a single
	// complete set of parameters from the offers", and buildWireESPProposals sends them
	// all. Keying from Proposals[0] would install a Child SA under an algorithm the peer
	// never agreed to, and the peer would drop every packet it carries.
	offer, err := verifyAcceptedOffer(accepted, sa.IKEGroup, old.ESPGroup)
	if err != nil {
		return nil, fmt.Errorf("child rekey response: %w", err)
	}
	prop := offer.ESPConfig
	rekeyEnc, rekeyInteg := espTransforms(prop)
	keys, err := crypto.DeriveChildSAKeys(sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		pending.localNonce, nr, rekeyEnc, rekeyInteg)
	if err != nil {
		return nil, err
	}
	// We initiated this rekey (sent Ni), so our KEYMAT role is initiator.
	child := newRekeyedChild(old, pending.newInboundSPI, outSPI, keys, true)
	// The replacement records the suite it is keyed with, and nothing else. It inherits
	// the retired SA's group. respondChildRekey reads Proposals[0] of that group when
	// the peer rekeys this SA next. An unselected proposal left in front of the accepted
	// one would make ze-build answer that rekey for an algorithm this pair never ran. Only the
	// child's own copy changes. The retired SA keeps its group.
	child.ESPGroup.Proposals = []ipsec.ESPProposal{prop}
	if err := installChildTolerant(child, prop, dp, log); err != nil {
		keys.Clear()
		return nil, err
	}
	return child, nil
}

// respondChildRekey processes a peer-initiated CREATE_CHILD_SA rekey request,
// installs the replacement Child SA (make-before-break; the old one is removed
// when the peer's INFORMATIONAL Delete arrives), and returns the SK-encrypted
// response echoing the request message ID. RFC 7296 §1.3.2.
//
// The request states the scope of the SA it asks for, so TSi and TSr are mandatory
// (RFC 7296 §1.3.3) and a request without them is refused. The answer is then built
// from the narrowing THIS request produced, never from a previous exchange's.
func respondChildRekey(sa *SA, inner []wire.PayloadEntry, old *ChildSA, msgID uint32, dp dataplane.Dataplane, log *slog.Logger) ([]byte, *ChildSA, error) {
	var ni []byte
	var peerSPI uint32
	var offer *wire.PayloadSA
	var reqTSi, reqTSr *wire.PayloadTS
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			ni = p.NonceData
		case *wire.PayloadSA:
			offer = p
			if s, err := espSPIFromSA(p); err == nil {
				peerSPI = s
			}
		case *wire.PayloadTS:
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				reqTSi = p
			case wire.PayloadTypeTSr:
				reqTSr = p
			}
		}
	}
	if len(ni) == 0 || peerSPI == 0 {
		// A request missing a mandatory payload is malformed, not unsatisfiable, so it
		// draws INVALID_SYNTAX rather than NO_PROPOSAL_CHOSEN (notify_error.go).
		return nil, nil, fmt.Errorf("%w: child rekey request missing Ni(%d) or ESP SPI(%d)",
			errMalformedRequest, len(ni), peerSPI)
	}
	if reqTSi == nil || reqTSr == nil {
		// TSi and TSr are part of the Child SA rekey request (RFC 7296 Section 1.3.3), and
		// Section 2.9 states what they carry: "TS payloads specify the selection criteria
		// for packets that will be forwarded over the newly set up SA." A request that omits
		// either payload states no complete criteria, so there is nothing to narrow and
		// nothing the answer could describe. This path used to skip the narrowing and answer
		// from sa.NegotiatedPairs, which announced the PREVIOUS exchange's scope for an SA
		// this request never proposed. The payload is missing rather than unsatisfiable, so the
		// refusal joins the case above and draws INVALID_SYNTAX.
		var absent string
		switch {
		case reqTSi == nil && reqTSr == nil:
			absent = "TSi and TSr"
		case reqTSi == nil:
			absent = "TSi"
		default:
			absent = "TSr"
		}
		return nil, nil, fmt.Errorf("%w: child rekey request missing %s", errMalformedRequest, absent)
	}

	// The suite that keys this replacement must be one the peer offered. The response
	// must name it by the peer's own Proposal Num (RFC 7296 Sections 3.3, 3.3.6). A
	// request that offers no such proposal is refused. Ze does not answer it with
	// keys for an algorithm the peer never asked for.
	accepted, ok := matchOfferedESPProposal(offer, old.ESPGroup.Proposals[0])
	if !ok || accepted.Number == 0 {
		return nil, nil, fmt.Errorf("child rekey request: %w", crypto.ErrNoProposalChosen)
	}

	// RFC 7296 Section 2.9.2 MUST NOT: "The responder MUST NOT narrow down the Traffic
	// Selectors narrower than the scope currently in use." The SA being replaced is the
	// scope in use, so it is passed as the narrowing FLOOR. Without it, a policy that
	// narrowed since the original SA came up would silently shrink a working tunnel at
	// its next rekey.
	//
	// The floor is compared against THIS request's payloads, so it has to be in THIS
	// exchange's orientation. The peer sent Ni here, so the peer's side is TSi. The stored
	// floor is in the orientation of whatever exchange negotiated it, which is the opposite
	// one whenever this node's side was TSi there.
	floor := old.Selectors
	if old.SelectorsLocalIsTSi {
		floor = swapPairs(floor)
	}
	if err := narrowChildSelectors(sa, reqTSi, reqTSr, floor); err != nil {
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
	prop := old.ESPGroup.Proposals[0]
	respEnc, respInteg := espTransforms(prop)
	// Peer is the initiator here: KEYMAT = prf+(SK_d, Ni | Nr).
	keys, err := crypto.DeriveChildSAKeys(sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		ni, nr, respEnc, respInteg)
	if err != nil {
		return nil, nil, err
	}
	// RFC 7296 Section 2.9: "TS payloads specify the selection criteria for packets that
	// will be forwarded over the newly set up SA." The answer is therefore a statement
	// about the Child SA installed below, and it carries that SA's scope in this exchange's
	// TSi/TSr orientation.
	//
	// The answer and the install are one set on every path that reaches here.
	// narrowChildSelectors ran for THIS request, and it refuses the exchange unless its
	// narrowing covers the scope in use, so the only answer left is the floor
	// (ts_narrow.go), which newRekeyedChild installs by inheriting old.Selectors.
	//
	// A scope no TS payload can carry is refused rather than answered, as the IKE_AUTH
	// responder refuses it (initiator.go). A wildcard here would widen the peer's proposal,
	// and a response without the payloads would drop the criteria Section 2.9 requires.
	tsi, tsr := pairsToWire(sa.NegotiatedPairs)
	if tsi == nil || tsr == nil {
		keys.Clear()
		return nil, nil, fmt.Errorf("%w: the negotiated scope %s cannot be put on the wire",
			errTSUnacceptable, pairsText(sa.NegotiatedPairs))
	}

	// The peer initiated this rekey (sent Ni); our KEYMAT role is responder.
	child := newRekeyedChild(old, inSPI, peerSPI, keys, false)
	if err := installChildTolerant(child, prop, dp, log); err != nil {
		keys.Clear()
		return nil, nil, err
	}
	inner2 := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{espProposalToWire(prop, inSPI, accepted.Number)}}},
		{Payload: &wire.PayloadNonce{NonceData: nr}},
		{Payload: tsi},
		{Payload: tsr},
	}
	resp, err := buildEncryptedMessageEx(sa, inner2, msgID, wire.ExchangeCreateChildSA, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		if dp != nil {
			// Rolling the replacement back. `old` is still live and shares these
			// policies, so removing them here would break the tunnel this rekey was
			// meant to preserve.
			removeChildSAExcept(child, old, dp, log)
		}
		return nil, nil, err
	}
	return resp, child, nil
}

// newRekeyedChild builds a replacement Child SA inheriting addresses/TS/ifID from
// the old one, with fresh SPIs and keys. localIsInitiator records whether we sent
// Ni for this rekey exchange (true when we initiated it), which selects the ESP
// send/receive key halves in installChildSA (RFC 7296 Section 2.17).
func newRekeyedChild(old *ChildSA, inSPI, outSPI uint32, keys *crypto.ChildSAKeys, localIsInitiator bool) *ChildSA {
	return &ChildSA{
		InboundSPI:  inSPI,
		OutboundSPI: outSPI,
		LocalAddr:   old.LocalAddr,
		RemoteAddr:  old.RemoteAddr,
		IfID:        old.IfID,
		TSLocal:     old.TSLocal,
		TSRemote:    old.TSRemote,
		Keys:        keys,
		ESPGroup:    old.ESPGroup,
		ReqID:       old.ReqID,
		NATDetected: old.NATDetected,
		// The replacement installs the SAME policy selector as the retired pair, so it
		// must claim it under the SAME owner. A replacement that dropped this would be
		// refused by the dataplane as a foreign peer taking the selector over
		// (dataplane.SPParams.Owner), and the rekey would fail at its policy install.
		Owner: old.Owner,
		// UDPEncap decides whether installChildSA gives the XFRM state an ESP-in-UDP
		// template (child.go). createFirstChildSA is its only other writer, so a rekeyed
		// child that does not inherit it installs a state with no template. The kernel
		// then refuses the encapsulated ESP the peer keeps sending, and a NAT-traversing
		// tunnel carries nothing from its first Child SA rekey onward.
		UDPEncap: old.UDPEncap,
		// Selectors and Mode are inherited for the same reason as UDPEncap above:
		// createFirstChildSA is their only other writer.
		//
		// Selectors is the SCOPE CURRENTLY IN USE that RFC 7296 Section 2.9.2 forbids the
		// NEXT rekey from narrowing below. A replacement that dropped it would give that
		// rekey no floor, so the second rekey of a tunnel could shrink it.
		//
		// Mode decides the encapsulation of every state and policy the replacement
		// installs. A replacement that dropped it would fall back to tunnel, so a
		// transport-mode tunnel would silently become a tunnel-mode one at its first
		// rekey -- the wrong-mode-without-an-error failure this package removes.
		Selectors: old.Selectors,
		// The orientation travels with the selectors it describes. This rekey did not
		// renegotiate them, so the half that was ours is still ours -- whichever end
		// started this exchange. Deriving it from localIsInitiator instead would swap the
		// policy's ports at the first rekey the other end initiates.
		SelectorsLocalIsTSi: old.SelectorsLocalIsTSi,
		Mode:                old.Mode,
		LocalIsInitiator:    localIsInitiator,
	}
}

// lifetimeState tracks SA lifetime for time-based and byte-based expiry.
type lifetimeState struct {
	softTime  time.Time
	hardTime  time.Time
	softBytes uint64
	byteCount uint64
}

func newLifetimeState(lifetimeSec uint32) *lifetimeState {
	if lifetimeSec == 0 {
		return nil
	}
	lifetime := time.Duration(lifetimeSec) * time.Second
	now := time.Now()
	hard := now.Add(lifetime)
	soft := now.Add(lifetime - rekeyLead(lifetime))
	return &lifetimeState{
		softTime: soft,
		hardTime: hard,
	}
}

// rekeyLead is how far before the hard time the soft (rekey) trigger sits.
//
// Two things set it. RFC 7296 Section 2.8 asks for headroom -- "Enough time should
// elapse between the time the new SA is established and the old one becomes unusable
// so that traffic can be switched over to the new SA" -- and the same section's "MUST
// NOT be used" makes the hard time a wall the owner loop will not move. A rekey that
// begins too late is therefore cut off rather than tolerated, so the trigger must
// leave a whole retransmit budget in front of that wall.
//
// lifetimeJitter alone cannot supply it: it returns a value in [0, 10%), so it can
// return zero and put the trigger exactly ON the hard time. Taking the larger of the
// jitter and the budget keeps the desynchronization Section 2.8 asks for (checklist
// row RFC7296-2.8-3) while giving the exchange room to finish.
//
// A short configured lifetime cannot fund the full budget, so the lead is capped at
// half the lifetime; the SA then rekeys at its midpoint.
func rekeyLead(lifetime time.Duration) time.Duration {
	budget := time.Duration(maxRetransmissions) * rekeyRetransmitTimeout
	budget = min(budget, lifetime/2)
	return max(lifetimeJitter(lifetime), budget)
}

// lifetimeJitter returns a random duration between 0 and 10% of the lifetime.
func lifetimeJitter(lifetime time.Duration) time.Duration {
	tenPercent := lifetime / 10
	if tenPercent <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(tenPercent)))
	if err != nil {
		return tenPercent / 2
	}
	return time.Duration(n.Int64())
}

// softExpired reports whether the soft (rekey trigger) lifetime has passed.
func (ls *lifetimeState) softExpired(now time.Time) bool {
	if ls == nil {
		return false
	}
	if !ls.softTime.IsZero() && !now.Before(ls.softTime) {
		return true
	}
	if ls.softBytes > 0 && ls.byteCount >= ls.softBytes {
		return true
	}
	return false
}

// hardExpired reports whether the hard (delete) lifetime has passed.
func (ls *lifetimeState) hardExpired(now time.Time) bool {
	if ls == nil {
		return false
	}
	return !ls.hardTime.IsZero() && !now.Before(ls.hardTime)
}

// ikeSPIFromSA extracts the 8-byte IKE SPI from the first IKE proposal.
func ikeSPIFromSA(p *wire.PayloadSA) ([8]byte, error) {
	var spi [8]byte
	for _, prop := range p.Proposals {
		if prop.ProtocolID == wire.ProtocolIKE && len(prop.SPI) == 8 {
			copy(spi[:], prop.SPI)
			return spi, nil
		}
	}
	return spi, fmt.Errorf("ike rekey: no IKE SPI in SA payload")
}

// initiateIKERekey builds the SK-encrypted CREATE_CHILD_SA request that rekeys the
// IKE SA and the pendingRekey holding our DH half until the response arrives.
// RFC 7296 §1.3.3: SA + Ni + KEi (KE mandatory). The DH exchange is real (the peer
// supplies KEr in its response), replacing the former self-DH local roll.
func initiateIKERekey(oldSA *SA, ikeGroup ipsec.IKEGroup) ([]byte, *pendingRekey, error) {
	if len(ikeGroup.Proposals) == 0 {
		return nil, nil, fmt.Errorf("ike rekey: no IKE proposals configured")
	}
	ni, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, nil, err
	}
	dhGroupID := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)
	dh, err := crypto.NewDHExchange(dhGroupID)
	if err != nil {
		return nil, nil, err
	}
	newSPI, err := GenerateSPI()
	if err != nil {
		dh.Clear()
		return nil, nil, err
	}

	props := buildWireIKEProposals(ikeGroup)
	spiBytes := make([]byte, 8)
	copy(spiBytes, newSPI[:])
	for i := range props {
		props[i].SPISize = 8
		props[i].SPI = spiBytes
	}
	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props}},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(ikeGroup.Proposals[0].DHGroup), KeyExchangeData: dh.PublicKey}},
	}
	msgID := oldSA.NextMsgID
	msg, err := buildEncryptedMessageEx(oldSA, inner, msgID, wire.ExchangeCreateChildSA, initiatorFlag(oldSA))
	if err != nil {
		dh.Clear()
		return nil, nil, err
	}
	oldSA.advanceMsgID()
	return msg, &pendingRekey{
		kind:            rekeyIKE,
		messageID:       msgID,
		sentMsg:         msg,
		sentAt:          time.Now(),
		localNonce:      ni,
		newInitiatorSPI: newSPI,
		dh:              dh,
	}, nil
}

// applyIKERekeyResponse processes the peer's CREATE_CHILD_SA response to our IKE
// SA rekey: completes DH from the peer's KEr, derives the new key hierarchy, and
// returns the replacement IKE SA (message-ID counters reset to 0, §2.8). The old
// SA's Child SAs continue to apply to the new IKE SA unchanged.
func applyIKERekeyResponse(oldSA *SA, pending *pendingRekey, inner []wire.PayloadEntry, log *slog.Logger) (*SA, error) {
	// RFC 7296 Section 2.25, as in applyChildRekeyResponse: the peer is busy, so the
	// caller waits rather than retrying at once.
	if hasTemporaryFailure(inner) {
		return nil, errTemporaryFailure
	}
	// RFC 7296 Section 4, as on the Child SA path: refusal, not delay.
	if hasNoAdditionalSAs(inner) {
		return nil, errNoAdditionalSAs
	}
	var nr, ker []byte
	var newResponderSPI [8]byte
	haveSPI := false
	var accepted *wire.PayloadSA
	var acceptedKE *wire.PayloadKE
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			nr = p.NonceData
		case *wire.PayloadKE:
			ker = p.KeyExchangeData
			acceptedKE = p
		case *wire.PayloadSA:
			accepted = p
			s, err := ikeSPIFromSA(p)
			if err != nil {
				return nil, err
			}
			newResponderSPI = s
			haveSPI = true
		}
	}
	if len(nr) == 0 || len(ker) == 0 || !haveSPI {
		return nil, fmt.Errorf("ike rekey response: missing Nr(%d)/KEr(%d)/SPI(%v)", len(nr), len(ker), haveSPI)
	}

	// RFC 7296 Section 3.4: the KE payload's Diffie-Hellman Group Num "MUST match a
	// Diffie-Hellman group specified in a proposal in the SA payload that is sent in
	// the same message". The rekey response carries the accepted proposal, so a KEr in
	// another group would key the replacement IKE SA under a group this node never
	// agreed to.
	if err := accepted.ValidateKEGroup(acceptedKE); err != nil {
		return nil, fmt.Errorf("ike rekey response: %w", err)
	}

	// RFC 7296 Section 3.3.6: the initiator checks the accepted offer against the IKE
	// proposals it sent. It stops the exchange when the two disagree. The new IKE SA
	// then takes the suite both sides agreed on. respondIKERekey negotiates for real,
	// so the offer it accepts is not always the first proposal of the old SA.
	offer, err := verifyAcceptedOffer(accepted, oldSA.IKEGroup, oldSA.ESPGroup)
	if err != nil {
		return nil, fmt.Errorf("ike rekey response: %w", err)
	}
	chosen := offer.IKE

	sharedSecret, err := pending.dh.SharedSecret(ker)
	if err != nil {
		return nil, err
	}
	skeyseed, err := crypto.DeriveRekeyedSKEYSEED(
		oldSA.Proposal.PRF.ID, oldSA.SKKeys.SK_d, sharedSecret, pending.localNonce, nr)
	if err != nil {
		clear(sharedSecret)
		return nil, err
	}
	// RFC 7296 Section 2.18: SKEYSEED comes from the PRF of the OLD SA. The new key
	// hierarchy comes from the suite the two sides agreed on for the new SA.
	skKeys, err := crypto.DeriveSKKeys(
		chosen.PRF.ID, skeyseed, pending.localNonce, nr,
		pending.newInitiatorSPI[:], newResponderSPI[:],
		chosen.Encryption, chosen.Integrity)
	clear(sharedSecret)
	clear(skeyseed)
	if err != nil {
		return nil, err
	}

	newSA := &SA{
		PeerName:     oldSA.PeerName,
		PeerCfg:      oldSA.PeerCfg,
		IKEGroup:     oldSA.IKEGroup,
		ESPGroup:     oldSA.ESPGroup,
		InitiatorSPI: pending.newInitiatorSPI,
		ResponderSPI: newResponderSPI,
		// We sent the CREATE_CHILD_SA request that rekeyed the IKE SA, so we are the
		// new SA's initiator regardless of our role on the old SA: we sent Ni, so the
		// new SK_ei is our send key. RFC 7296 Section 2.18.
		IsInitiator:   true,
		State:         StateEstablished,
		LocalNonce:    pending.localNonce,
		RemoteNonce:   nr,
		Proposal:      chosen,
		SKKeys:        skKeys,
		NextMsgID:     0,
		ExpectedMsgID: 0,
		NATDetected:   oldSA.NATDetected,
		BehindNAT:     oldSA.BehindNAT,
		CreatedAt:     time.Now(),
		EstablishedAt: time.Now(),
	}
	// RFC 7296 Section 2.18: the replacement SA carries the same peer over the same
	// path. It therefore inherits the float, the sockets, and the endpoint the old SA
	// authenticated. A replacement that started at port 500 would break every
	// NAT-traversing tunnel on its first rekey.
	newSA.inheritSendPath(oldSA)
	log.Info("ike-sa: rekeyed via CREATE_CHILD_SA",
		"old-ispi", SPIHex(oldSA.InitiatorSPI), "new-ispi", SPIHex(pending.newInitiatorSPI))
	return newSA, nil
}

// localNonceIsLower reports whether our nonce sorts below the peer nonce, octet by octet.
// RFC 7296 Section 2.8.1 closes the SA that carries the lowest of the four nonces. The
// endpoint that created that SA is the one that closes it. A caller that reads true
// therefore abandons its own exchange.
func localNonceIsLower(localNonce, remoteNonce []byte) bool {
	return bytes.Compare(localNonce, remoteNonce) < 0
}

// respondIKERekey processes a peer-initiated CREATE_CHILD_SA that rekeys the IKE SA
// (SA + Ni + KEi, no TS / no REKEY_SA). It completes a fresh DH exchange, derives the
// new IKE SA key hierarchy (RFC 7296 Section 2.18: SKEYSEED = prf(SK_d_old, g^ir_new
// | Ni | Nr)), and builds the CREATE_CHILD_SA response (SA + Nr + KEr) encrypted
// under the OLD SA keys, echoing the request message ID. The peer is the rekey
// initiator, so the new SA has IsInitiator=false (we send with SK_er). The old SA is
// removed once the peer's INFORMATIONAL Delete confirms the rekey (make-before-break).
// This closes spec-ipsec-13's deferred IKE-rekey responder. RFC 7296 Section 1.3.3.
//
// It returns a response and a nil SA when the KEi payload names a group other than the
// group of the proposal we selected. That answer is an INVALID_KE_PAYLOAD Notify, and the
// caller MUST send it without a swap to a new SA. RFC 7296 Section 1.3.
func respondIKERekey(oldSA *SA, inner []wire.PayloadEntry, msgID uint32, log *slog.Logger) ([]byte, *SA, error) {
	if len(oldSA.IKEGroup.Proposals) == 0 {
		return nil, nil, fmt.Errorf("ike rekey: no IKE proposals configured")
	}
	var ni, kei []byte
	var keiGroup uint16
	var peerNewSPI [8]byte
	haveSPI := false
	var remoteSA *wire.PayloadSA
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			ni = p.NonceData
		case *wire.PayloadKE:
			kei = p.KeyExchangeData
			keiGroup = p.DHGroup
		case *wire.PayloadSA:
			remoteSA = p
			s, err := ikeSPIFromSA(p)
			if err != nil {
				return nil, nil, err
			}
			peerNewSPI = s
			haveSPI = true
		}
	}
	if len(ni) == 0 || len(kei) == 0 || !haveSPI {
		// Malformed, as on the Child SA path above.
		return nil, nil, fmt.Errorf("%w: ike rekey request missing Ni(%d)/KEi(%d)/SPI(%v)",
			errMalformedRequest, len(ni), len(kei), haveSPI)
	}

	// Select a proposal we accept for the new IKE SA.
	chosen, err := crypto.NegotiateIKE(wireProposalsToIKE(remoteSA.Proposals), buildIKEProposals(oldSA.IKEGroup))
	if err != nil {
		return nil, nil, fmt.Errorf("ike rekey: %w", err)
	}
	logKeyLengthUpgrade(log, oldSA.PeerName, chosen)

	// RFC 7296 Section 1.3: the initiator guesses the group before it knows our choice.
	// When the KEi group differs from the group of the proposal we selected, we reject
	// the request and name our group in an INVALID_KE_PAYLOAD Notify. No key is derived
	// from a value computed over another group. The caller sends this answer and builds
	// no new SA. RFC 7296 Section 3.10.1 gives the payload two octets of data.
	if keiGroup != uint16(chosen.DHGroup.ID) {
		log.Info("ike: peer IKE rekey KE group mismatch", "peer", oldSA.PeerName,
			"want", uint16(chosen.DHGroup.ID), "got", keiGroup)
		grp := []byte{byte(uint16(chosen.DHGroup.ID) >> 8), byte(chosen.DHGroup.ID)}
		notify := &wire.PayloadNotify{
			NotifyMsgType:    wire.NotifyInvalidKEPayload,
			NotificationData: grp,
		}
		resp, err := buildEncryptedMessageEx(oldSA, []wire.PayloadEntry{{Payload: notify}},
			msgID, wire.ExchangeCreateChildSA, initiatorFlag(oldSA)|wire.FlagResponse)
		if err != nil {
			return nil, nil, err
		}
		return resp, nil, nil
	}

	dh, err := crypto.NewDHExchange(chosen.DHGroup.ID)
	if err != nil {
		return nil, nil, err
	}
	ourPub := append([]byte(nil), dh.PublicKey...)
	nr, err := GenerateNonce(nonceLen)
	if err != nil {
		dh.Clear()
		return nil, nil, err
	}
	ourNewSPI, err := GenerateSPI()
	if err != nil {
		dh.Clear()
		return nil, nil, err
	}

	sharedSecret, err := dh.SharedSecret(kei)
	dh.Clear()
	if err != nil {
		return nil, nil, err
	}
	skeyseed, err := crypto.DeriveRekeyedSKEYSEED(oldSA.Proposal.PRF.ID, oldSA.SKKeys.SK_d, sharedSecret, ni, nr)
	clear(sharedSecret)
	if err != nil {
		return nil, nil, err
	}
	// New SA: the peer (rekey initiator) is SPIi, we are SPIr; Ni is the peer's.
	newKeys, err := crypto.DeriveSKKeys(chosen.PRF.ID, skeyseed, ni, nr,
		peerNewSPI[:], ourNewSPI[:], chosen.Encryption, chosen.Integrity)
	clear(skeyseed)
	if err != nil {
		return nil, nil, err
	}

	now := oldSA.EstablishedAt
	newSA := &SA{
		PeerName:      oldSA.PeerName,
		PeerCfg:       oldSA.PeerCfg,
		IKEGroup:      oldSA.IKEGroup,
		ESPGroup:      oldSA.ESPGroup,
		InitiatorSPI:  peerNewSPI,
		ResponderSPI:  ourNewSPI,
		IsInitiator:   false, // peer initiated this rekey; we send with SK_er
		State:         StateEstablished,
		LocalNonce:    nr,
		RemoteNonce:   ni,
		Proposal:      chosen,
		SKKeys:        newKeys,
		NextMsgID:     0,
		ExpectedMsgID: 0,
		NATDetected:   oldSA.NATDetected,
		BehindNAT:     oldSA.BehindNAT,
		CreatedAt:     now,
		EstablishedAt: now,
	}
	// RFC 7296 Section 2.18, as on the initiator path above.
	newSA.inheritSendPath(oldSA)

	// Response SA carries our new IKE SPI on the chosen proposal.
	props := chosenIKEProposalToWire(chosen)
	spiBytes := make([]byte, 8)
	copy(spiBytes, ourNewSPI[:])
	for i := range props {
		props[i].SPISize = 8
		props[i].SPI = spiBytes
	}
	inner2 := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props}},
		{Payload: &wire.PayloadNonce{NonceData: nr}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(chosen.DHGroup.ID), KeyExchangeData: ourPub}},
	}
	resp, err := buildEncryptedMessageEx(oldSA, inner2, msgID, wire.ExchangeCreateChildSA, initiatorFlag(oldSA)|wire.FlagResponse)
	if err != nil {
		newKeys.Clear()
		return nil, nil, err
	}
	log.Info("ike-sa: responding to peer IKE rekey", "peer", oldSA.PeerName,
		"old-rspi", SPIHex(oldSA.ResponderSPI), "new-rspi", SPIHex(ourNewSPI))
	return resp, newSA, nil
}
