// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Child SA and IKE SA rekeying
// RFC: rfc/short/rfc7296.md -- Rekeying (Section 2.8), collision (Section 2.8.1)

package engine

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
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

// anyChildTSPayloads builds wildcard TSi/TSr payloads matching buildChildSAPayloads.
func anyChildTSPayloads(sa *SA) (*wire.PayloadTS, *wire.PayloadTS) {
	remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	isV6 := remoteIP != nil && remoteIP.To4() == nil
	tsAny := anyTrafficSelector(isV6)
	return &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{tsAny}},
		&wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{tsAny}}
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
	sa.NextMsgID++
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
	var nr []byte
	var outSPI uint32
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			nr = p.NonceData
		case *wire.PayloadSA:
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
	prop := old.ESPGroup.Proposals[0]
	keys, err := crypto.DeriveChildSAKeys(sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		pending.localNonce, nr, lookupEncryption(prop.Encryption), lookupIntegrity(prop.Hash))
	if err != nil {
		return nil, err
	}
	// We initiated this rekey (sent Ni), so our KEYMAT role is initiator.
	child := newRekeyedChild(old, pending.newInboundSPI, outSPI, keys, true)
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
func respondChildRekey(sa *SA, inner []wire.PayloadEntry, old *ChildSA, msgID uint32, dp dataplane.Dataplane, log *slog.Logger) ([]byte, *ChildSA, error) {
	var ni []byte
	var peerSPI uint32
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			ni = p.NonceData
		case *wire.PayloadSA:
			if s, err := espSPIFromSA(p); err == nil {
				peerSPI = s
			}
		}
	}
	if len(ni) == 0 || peerSPI == 0 {
		return nil, nil, fmt.Errorf("child rekey request: missing Ni(%d) or ESP SPI(%d)", len(ni), peerSPI)
	}

	nr, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, nil, err
	}
	inSPI, err := GenerateESPSPI()
	if err != nil {
		return nil, nil, err
	}
	prop := old.ESPGroup.Proposals[0]
	// Peer is the initiator here: KEYMAT = prf+(SK_d, Ni | Nr).
	keys, err := crypto.DeriveChildSAKeys(sa.Proposal.PRF.ID, sa.SKKeys.SK_d,
		ni, nr, lookupEncryption(prop.Encryption), lookupIntegrity(prop.Hash))
	if err != nil {
		return nil, nil, err
	}
	// The peer initiated this rekey (sent Ni); our KEYMAT role is responder.
	child := newRekeyedChild(old, inSPI, peerSPI, keys, false)
	if err := installChildTolerant(child, prop, dp, log); err != nil {
		keys.Clear()
		return nil, nil, err
	}

	tsi, tsr := anyChildTSPayloads(sa)
	inner2 := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: buildWireESPProposals(old.ESPGroup, inSPI)}},
		{Payload: &wire.PayloadNonce{NonceData: nr}},
		{Payload: tsi},
		{Payload: tsr},
	}
	resp, err := buildEncryptedMessageEx(sa, inner2, msgID, wire.ExchangeCreateChildSA, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		if dp != nil {
			removeChildSA(child, dp, log)
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
		InboundSPI:       inSPI,
		OutboundSPI:      outSPI,
		LocalAddr:        old.LocalAddr,
		RemoteAddr:       old.RemoteAddr,
		IfID:             old.IfID,
		TSLocal:          old.TSLocal,
		TSRemote:         old.TSRemote,
		Keys:             keys,
		ESPGroup:         old.ESPGroup,
		ReqID:            old.ReqID,
		NATDetected:      old.NATDetected,
		LocalIsInitiator: localIsInitiator,
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
	jitter := lifetimeJitter(lifetime)
	now := time.Now()
	soft := now.Add(lifetime - jitter)
	hard := now.Add(lifetime)
	return &lifetimeState{
		softTime: soft,
		hardTime: hard,
	}
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
	oldSA.NextMsgID++
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
	var nr, ker []byte
	var newResponderSPI [8]byte
	haveSPI := false
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			nr = p.NonceData
		case *wire.PayloadKE:
			ker = p.KeyExchangeData
		case *wire.PayloadSA:
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
	skKeys, err := crypto.DeriveSKKeys(
		oldSA.Proposal.PRF.ID, skeyseed, pending.localNonce, nr,
		pending.newInitiatorSPI[:], newResponderSPI[:],
		oldSA.Proposal.Encryption, oldSA.Proposal.Integrity)
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
		Proposal:      oldSA.Proposal,
		SKKeys:        skKeys,
		NextMsgID:     0,
		ExpectedMsgID: 0,
		NATDetected:   oldSA.NATDetected,
		BehindNAT:     oldSA.BehindNAT,
		CreatedAt:     time.Now(),
		EstablishedAt: time.Now(),
	}
	log.Info("ike-sa: rekeyed via CREATE_CHILD_SA",
		"old-ispi", SPIHex(oldSA.InitiatorSPI), "new-ispi", SPIHex(pending.newInitiatorSPI))
	return newSA, nil
}

// resolveRekeyCollision determines the winner of a simultaneous rekey.
// RFC 7296 Section 2.8.1: the exchange with the lowest nonce wins.
func resolveRekeyCollision(localNonce, remoteNonce []byte) bool {
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
func respondIKERekey(oldSA *SA, inner []wire.PayloadEntry, msgID uint32, log *slog.Logger) ([]byte, *SA, error) {
	if len(oldSA.IKEGroup.Proposals) == 0 {
		return nil, nil, fmt.Errorf("ike rekey: no IKE proposals configured")
	}
	var ni, kei []byte
	var peerNewSPI [8]byte
	haveSPI := false
	var remoteSA *wire.PayloadSA
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNonce:
			ni = p.NonceData
		case *wire.PayloadKE:
			kei = p.KeyExchangeData
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
		return nil, nil, fmt.Errorf("ike rekey request: missing Ni(%d)/KEi(%d)/SPI(%v)", len(ni), len(kei), haveSPI)
	}

	// Select a proposal we accept for the new IKE SA.
	chosen, err := crypto.NegotiateIKE(wireProposalsToIKE(remoteSA.Proposals), buildIKEProposals(oldSA.IKEGroup))
	if err != nil {
		return nil, nil, fmt.Errorf("ike rekey: %w", err)
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
