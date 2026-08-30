// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE_SA_INIT initiator logic
// Related: ts_narrow.go -- traffic-selector policy, narrowing, and port encoding
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT exchange (Section 1.2)
package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// newInitiatorSA creates a new SA for the initiator side.
func newInitiatorSA(peerName string, peer ipsec.SiteToSitePeer, ikeGroup ipsec.IKEGroup, espGroup ipsec.ESPGroup) (*SA, error) {
	spi, err := GenerateSPI()
	if err != nil {
		return nil, err
	}

	nonce, err := GenerateNonce(nonceLen)
	if err != nil {
		return nil, err
	}

	dhGroupID := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)
	dh, err := crypto.NewDHExchange(dhGroupID)
	if err != nil {
		return nil, err
	}

	return &SA{
		PeerName:     peerName,
		PeerCfg:      peer,
		IKEGroup:     ikeGroup,
		ESPGroup:     espGroup,
		InitiatorSPI: spi,
		IsInitiator:  true,
		State:        StateIdle,
		LocalNonce:   nonce,
		LocalDH:      dh,
		CreatedAt:    time.Now(),
	}, nil
}

// buildSAInitRequest constructs an IKE_SA_INIT request message.
// RFC 7296 Section 2.23: includes NAT_DETECTION_*_IP notify payloads.
func buildSAInitRequest(sa *SA, ikeGroup ipsec.IKEGroup) []byte {
	// RFC 7296 Section 1.2 MUST: a retry after INVALID_KE_PAYLOAD "MUST again propose
	// its full set of acceptable cryptographic suites because the rejection message was
	// unauthenticated and otherwise an active attacker could trick the endpoints into
	// negotiating a weaker suite than a stronger one that they both prefer". The offer
	// is therefore built from the whole configured group on every attempt, and is never
	// narrowed to the group the responder named.
	proposals := buildWireIKEProposals(ikeGroup)
	// The KE payload names the group of the key it actually carries, read from the
	// DHExchange rather than recomputed from the config index. The two were computed
	// independently from that one index and could already drift; after an
	// INVALID_KE_PAYLOAD retry they WOULD drift, because the retry replaces the key
	// without touching the config.
	dhGroupID := sa.LocalDH.GroupID

	payloads := make([]wire.PayloadEntry, 0, 6)
	if len(sa.Cookie) > 0 {
		// RFC 7296 Section 2.6 MUST: the retry must "include the COOKIE notification
		// containing the received data as the first payload, and all other payloads
		// unchanged". Message.WriteTo emits the slice in order, so position zero here
		// is position zero on the wire.
		payloads = append(payloads, wire.PayloadEntry{Payload: &wire.PayloadNotify{
			NotifyMsgType:    wire.NotifyCookie,
			NotificationData: sa.Cookie,
		}})
	}
	payloads = append(payloads,
		wire.PayloadEntry{Payload: &wire.PayloadSA{Proposals: proposals}},
		wire.PayloadEntry{Payload: &wire.PayloadKE{
			DHGroup:         uint16(dhGroupID),
			KeyExchangeData: sa.LocalDH.PublicKey,
		}},
		wire.PayloadEntry{Payload: &wire.PayloadNonce{NonceData: sa.LocalNonce}},
		wire.PayloadEntry{Payload: buildSignatureHashAlgosNotify()},
	)

	localIP := net.ParseIP(sa.PeerCfg.LocalAddress)
	remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	if localIP != nil && remoteIP != nil {
		payloads = append(payloads, buildNATDetectionPayloads(sa.InitiatorSPI, sa.ResponderSPI, localIP, remoteIP, transport.IKEPort)...)
	}

	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			MajorVersion: 2,
			MinorVersion: 0,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagInitiator,
			MessageID:    0,
		},
		Payloads: payloads,
	}

	// Size the buffer to the message length so WriteTo (which indexes buf
	// directly) can never overrun. A Len()-sized allocation bounds it without an
	// error path. RFC 7296 Section 3.1. See spec-fixit-fixed-buffer-overflow.
	//
	// One field IS remotely influenced: sa.Cookie holds octets a responder chose.
	// RFC 7296 Section 2.6 bounds it at 64, retrySAInit refuses anything outside
	// that bound before it reaches this field, and the allocation below is sized
	// from the message rather than fixed, so the bound is a conformance rule here
	// and not the thing that stops an overrun.
	buf := make([]byte, msg.Len())
	n := msg.WriteTo(buf, 0)
	return buf[:n]
}

// buildNATDetectionPayloads creates the NAT_DETECTION_SOURCE_IP and
// NAT_DETECTION_DESTINATION_IP notify payloads.
// RFC 7296 Section 2.23: SOURCE_IP uses our address, DESTINATION_IP uses the peer's address.
func buildNATDetectionPayloads(spiI, spiR [8]byte, localIP, remoteIP net.IP, port uint16) []wire.PayloadEntry {
	srcHash := transport.NATDetectionHash(spiI, spiR, localIP, port)
	dstHash := transport.NATDetectionHash(spiI, spiR, remoteIP, port)
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{
			NotifyMsgType:    wire.NotifyNATDetectionSourceIP,
			NotificationData: srcHash,
		}},
		{Payload: &wire.PayloadNotify{
			NotifyMsgType:    wire.NotifyNATDetectionDestIP,
			NotificationData: dstHash,
		}},
	}
}

// offerProposalNum returns the Proposal Num an offer puts on the proposal at index
// i. RFC 7296 Section 3.3 numbers the first Proposal of an offer one. Each later one
// is exactly one greater.
//
// The config key is a PRIORITY, not an index. The operator writes `proposal 10`. The
// parser sorts the group ascending by that key, so the slice order is the priority
// order. The wire number comes from the position. The key keeps its meaning, and the
// wire keeps the meaning the RFC gives it.
//
// The config parser bounds a group at ipsec.MaxProposalsPerGroup. The value i+1
// therefore always fits the one-octet field (ai/rules/protocol.md).
func offerProposalNum(i int) uint8 {
	return uint8(i + 1)
}

// buildWireIKEProposals converts config IKE proposals to a wire OFFER.
func buildWireIKEProposals(ikeGroup ipsec.IKEGroup) []wire.Proposal {
	proposals := make([]wire.Proposal, 0, len(ikeGroup.Proposals))
	for i, p := range ikeGroup.Proposals {
		enc := lookupEncryption(p.Encryption)
		prf := lookupPRF(p.Hash)
		integ := lookupIntegrity(p.Hash)
		dh := uint16(p.DHGroup)

		integID := uint16(integ.ID)
		if enc.IsAEAD {
			integID = uint16(crypto.AUTH_NONE)
		}

		transforms := []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: uint16(enc.ID), Attrs: encAttrs(enc)},
			{Type: wire.TransformTypePRF, ID: uint16(prf.ID)},
			{Type: wire.TransformTypeINTG, ID: integID},
			{Type: wire.TransformTypeDH, ID: dh},
		}

		proposals = append(proposals, wire.Proposal{
			Number:     offerProposalNum(i),
			ProtocolID: wire.ProtocolIKE,
			SPISize:    0,
			Transforms: transforms,
		})
	}
	return proposals
}

// buildIKEProposals converts config IKE proposals to crypto proposals.
func buildIKEProposals(ikeGroup ipsec.IKEGroup) []crypto.IKEProposal {
	out := make([]crypto.IKEProposal, 0, len(ikeGroup.Proposals))
	for _, p := range ikeGroup.Proposals {
		enc := lookupEncryption(p.Encryption)
		integ := lookupIntegrity(p.Hash)
		if enc.IsAEAD {
			integ = crypto.IntegrityTransform{ID: crypto.AUTH_NONE}
		}
		out = append(out, crypto.IKEProposal{
			Number:     p.Number,
			Encryption: enc,
			PRF:        lookupPRF(p.Hash),
			Integrity:  integ,
			DHGroup:    crypto.DHGroupTransform{ID: crypto.DHGroupID(p.DHGroup)},
		})
	}
	return out
}

// wireProposalsToIKE converts wire SA proposals to crypto proposals for negotiation. One
// wire proposal produces ONE crypto proposal for each complete parameter set it offers,
// so a proposal that carries alternatives produces several. See appendIKECombinations.
//
// RFC 7296 Section 3.3.6: a proposal that contains a Transform Type the responder does not
// understand MUST be considered unacceptable. This reader is the only place that sees the
// Transform Type of an offer, so it is the place that decides. RFC 7296 Section 3.3.3
// gives IKE the types ENCR, PRF, INTEG and D-H. Every other type, Extended Sequence
// Numbers included, is recorded in UnknownTransformType, and negotiation refuses that
// proposal. The proposals beside it are still processed.
func wireProposalsToIKE(wireProps []wire.Proposal) []crypto.IKEProposal {
	out := make([]crypto.IKEProposal, 0, len(wireProps))
	for _, wp := range wireProps {
		out = appendIKECombinations(out, wp)
	}
	return out
}

// maxIKECombinations bounds how many complete parameter sets ONE wire proposal expands
// to. The expansion below is a cross product. Without a bound, an unauthenticated peer
// CAN turn four lists of transforms into an arbitrarily large amount of work.
//
// The combinations are emitted in the peer's preference order. The cap therefore drops
// the peer's LEAST preferred combinations and keeps every one it asked for first. It only
// ever removes candidates, so it can never make Ze accept a set the peer did not offer.
const maxIKECombinations = 64

// appendIKECombinations expands one wire proposal into every complete IKE parameter set
// it offers. It appends them in the peer's preference order.
//
// RFC 7296 Section 3.3 lets one proposal carry SEVERAL transforms of the same type. That
// is the mandated encoding for alternatives. Section 3.3.5 requires two encryption key
// lengths to be offered as two separate ENCR transforms. The sender lists them in order
// of preference, and the responder selects one of each type that it supports.
//
// Reading only the last transform of each type therefore misreads the offer. A peer that
// offered [group 14, group NONE] was read as offering NONE alone. ikeProposalComplete
// refused it with NO_PROPOSAL_CHOSEN, although group 14 was on offer and configured.
// strongSwan offers alternatives this way, so that was a live interop failure.
//
// The expansion is an odometer with ENCR turning slowest and D-H fastest. The FIRST set
// emitted therefore pairs the peer's first choice of every type. crypto.negotiateIKE
// scans this list in order and returns the first entry any local proposal matches.
// Selection stays deterministic, and the peer's stated preference is what decides.
//
// A proposal that offers only transforms Ze does not support still matches nothing, and
// it is still refused. The expansion adds candidates. It never relaxes a comparison.
func appendIKECombinations(out []crypto.IKEProposal, wp wire.Proposal) []crypto.IKEProposal {
	base := crypto.IKEProposal{Number: uint16(wp.Number)}

	var (
		encs   []crypto.EncryptionTransform
		prfs   []crypto.PRFTransform
		integs []crypto.IntegrityTransform
		dhs    []crypto.DHGroupTransform
	)

	for _, t := range wp.Transforms {
		transformType := crypto.TransformType(t.Type)
		if !crypto.TransformTypeUnderstoodIKE(transformType) {
			if base.UnknownTransformType == 0 {
				base.UnknownTransformType = transformType
			}
			continue
		}
		switch t.Type {
		case wire.TransformTypeENCR:
			encs = append(encs, wireEncryptionTransform(t))
		case wire.TransformTypePRF:
			prfs = append(prfs, crypto.PRFTransform{ID: crypto.PRFID(t.ID)})
		case wire.TransformTypeINTG:
			integs = append(integs, crypto.IntegrityTransform{ID: crypto.IntegrityID(t.ID)})
		case wire.TransformTypeDH:
			dhs = append(dhs, crypto.DHGroupTransform{ID: crypto.DHGroupID(t.ID)})
		}
	}

	// RFC 7296 Section 3.3.6 makes a proposal that carries a Transform Type IKE does not
	// use unusable as a whole. Every combination inside it would be refused for that one
	// reason. A single entry carries the refusal exactly as it did before this expansion
	// existed, and it keeps the reported reason stable.
	if base.UnknownTransformType != 0 {
		return append(out, base)
	}

	// A type the peer omitted altogether keeps its zero value. ikeProposalComplete
	// refuses that as a missing mandatory transform. One zero-value element keeps the
	// refusal reachable. An empty list would collapse the cross product to nothing, and
	// the proposal would vanish silently instead of being refused.
	if len(encs) == 0 {
		encs = []crypto.EncryptionTransform{{}}
	}
	if len(prfs) == 0 {
		prfs = []crypto.PRFTransform{{}}
	}
	if len(integs) == 0 {
		integs = []crypto.IntegrityTransform{{}}
	}
	if len(dhs) == 0 {
		dhs = []crypto.DHGroupTransform{{}}
	}

	emitted := 0
	for _, enc := range encs {
		for _, prf := range prfs {
			for _, integ := range integs {
				for _, dh := range dhs {
					if emitted == maxIKECombinations {
						return out
					}
					p := base
					p.Encryption = enc
					p.PRF = prf
					p.Integrity = integ
					p.DHGroup = dh
					out = append(out, p)
					emitted++
				}
			}
		}
	}
	return out
}

// wireProposalsToESP converts wire SA proposals to crypto ESP proposals, so the
// initiator can check an accepted ESP offer against the proposals it sent. An AEAD
// proposal carries no integrity transform, and that absence reads as AUTH_NONE.
//
// A Transform Type ESP does not use is recorded in UnknownTransformType, exactly as
// wireProposalsToIKE records one. RFC 7296 Section 3.3.3 gives ESP the types ENCR and ESN,
// with INTEG and D-H optional.
func wireProposalsToESP(wireProps []wire.Proposal) []crypto.ESPProposal {
	out := make([]crypto.ESPProposal, 0, len(wireProps))
	for _, wp := range wireProps {
		p := crypto.ESPProposal{Number: uint16(wp.Number)}
		for _, t := range wp.Transforms {
			transformType := crypto.TransformType(t.Type)
			if !crypto.TransformTypeUnderstoodESP(transformType) {
				if p.UnknownTransformType == 0 {
					p.UnknownTransformType = transformType
				}
				continue
			}
			switch t.Type {
			case wire.TransformTypeENCR:
				p.Encryption = wireEncryptionTransform(t)
			case wire.TransformTypeINTG:
				p.Integrity.ID = crypto.IntegrityID(t.ID)
			case wire.TransformTypeESN, wire.TransformTypeDH:
				// RFC 7296 Section 3.3.2: neither is part of the suite this check
				// compares. The Extended Sequence Numbers selection is read from the wire
				// proposal instead, by acceptedESPESNConsistent, because crypto.ESPProposal
				// has no field to carry it. Ze offers no Diffie-Hellman group for a Child
				// SA.
			}
		}
		out = append(out, p)
	}
	return out
}

// buildESPProposals converts config ESP proposals to crypto proposals, which gives the
// accepted-offer check the set of suites this side offered.
func buildESPProposals(espGroup ipsec.ESPGroup) []crypto.ESPProposal {
	out := make([]crypto.ESPProposal, 0, len(espGroup.Proposals))
	for _, p := range espGroup.Proposals {
		enc, integ := espTransforms(p)
		out = append(out, crypto.ESPProposal{Number: p.Number, Encryption: enc, Integrity: integ})
	}
	return out
}

// acceptedOffer is the result of the initiator's re-check of a response. Protocol names
// which of the two proposal sets the responder answered with. The matching field holds
// the local proposal that the accepted offer agrees with.
//
// RFC 7296 Section 3.3.6 puts the parameters of the exchange in the responder's answer.
// It "MUST select a single complete set of parameters from the offers". The caller
// therefore KEYS from the matching field below. A caller that reads the error alone
// learns that the answer was consistent, and then keys from its own first proposal. That
// is the two ends disagreeing with nothing to detect it.
type acceptedOffer struct {
	Protocol uint8
	IKE      crypto.IKEProposal
	// ESP is the offer as the peer returned it. RFC 7296 Section 3.3.6 has the
	// attributes of a selected transform come back unmodified, so this is the peer's
	// own encoding. crypto.NegotiateESP returns the remote proposal, whose integrity
	// transform carries an id with no key length (wireProposalsToESP). It names the
	// accepted suite. It is NOT the shape key derivation reads.
	ESP crypto.ESPProposal
	// ESPConfig is the configured proposal that the accepted offer agrees with. An ESP
	// caller uses this field. It is the ESP equivalent of the IKE field above.
	// crypto.matchIKE returns the LOCAL proposal, so IKE already carries the local key
	// lengths. ESP does not. Each consumer needs this type anyway: espTransforms
	// derives the keys from it, and installChildSA (child.go) reads the dataplane
	// algorithm names off it.
	ESPConfig ipsec.ESPProposal
}

var (
	// errAcceptedOfferEmpty reports a response whose SA payload names no proposal.
	errAcceptedOfferEmpty = errors.New("ike: the accepted offer carries no proposal")
	// errAcceptedOfferProtocol reports an accepted offer for a protocol ze never offers.
	errAcceptedOfferProtocol = errors.New("ike: the accepted offer names an unknown protocol")
	// errAcceptedOfferMismatch reports an accepted offer that agrees with no proposal
	// this side sent.
	errAcceptedOfferMismatch = errors.New("ike: the accepted offer matches no proposal we sent")
	// errAcceptedOfferUnmapped reports an accepted ESP offer that the negotiation agreed
	// with, and that no configured proposal answers to. It cannot happen while
	// buildESPProposals derives its input from the same group. It is refused rather than
	// answered with a zero proposal, which would key an SA with no cipher.
	errAcceptedOfferUnmapped = errors.New("ike: the accepted ESP offer maps to no configured proposal")
	// errAcceptedOfferESN reports an accepted ESP offer whose Extended Sequence Numbers
	// selection is not the one this side offered.
	errAcceptedOfferESN = errors.New("ike: the accepted ESP offer selects an Extended Sequence Numbers value we did not offer")
)

// acceptedESPESNConsistent reports whether the Transform Type 5 selection in one accepted
// ESP proposal is the selection this side offered.
//
// RFC 7296 Section 3.3.6: "The initiator of an exchange MUST check that the accepted
// offer is consistent with one of its proposals, and if not MUST terminate the exchange"
// (rfc/full/rfc7296.txt:4906-4909). espProposalToWire offers exactly one value for the
// type, espESNNotExtended, so a responder that answers Extended Sequence Numbers has
// answered a suite this side never proposed. Section 2.7 allows the answer one transform
// of the type, so a second one is not a selection either.
//
// crypto.NegotiateESP does not reach this: crypto.ESPProposal carries no ESN field, and
// wireProposalsToESP drops the transform on the way in. The check therefore reads the
// wire proposal, where the value still exists.
//
// The failure it prevents is silent. Ze would install a 32-bit sequence space against a
// peer keyed for 64, the tunnel would come up, and no packet would survive the peer's
// anti-replay window.
func acceptedESPESNConsistent(p wire.Proposal) bool {
	esns := 0
	for _, t := range p.Transforms {
		if t.Type != wire.TransformTypeESN {
			continue
		}
		if t.ID != espESNNotExtended {
			return false
		}
		esns++
	}
	return esns <= 1
}

// espConfigForAccepted returns the configured ESP proposal that the accepted offer agrees
// with. It reads the transforms rather than the Proposal Num. RFC 7296 Section 3.3.1 makes
// that number the PEER's, so it names nothing in this node's configuration.
//
// The comparison is exact. The initiator check runs crypto.NegotiateESP under a key-length
// rule that accepts only the length this side offered (RFC 7296 Section 3.3.5, "the Key
// Length attribute ... is always returned unchanged").
func espConfigForAccepted(espGroup ipsec.ESPGroup, chosen crypto.ESPProposal) (ipsec.ESPProposal, bool) {
	for _, p := range espGroup.Proposals {
		enc, integ := espTransforms(p)
		if enc.ID == chosen.Encryption.ID &&
			enc.KeyLength == chosen.Encryption.KeyLength &&
			integ.ID == chosen.Integrity.ID {
			return p, true
		}
	}
	return ipsec.ESPProposal{}, false
}

// verifyAcceptedOffer checks a responder's accepted offer against the proposals this side
// sent, and returns the local proposal it agrees with.
//
// RFC 7296 Section 3.3.6: the initiator of an exchange MUST check that the accepted
// offer agrees with one of its proposals. It MUST stop the exchange when the offer does
// not. Every initiator response path calls this one helper. A response path added later
// that forgets the rule is therefore a visible omission rather than a silent one.
//
// It fails closed. A missing SA payload, an empty proposal list, and a protocol ze never
// offers are all refused.
func verifyAcceptedOffer(accepted *wire.PayloadSA, ikeGroup ipsec.IKEGroup, espGroup ipsec.ESPGroup) (acceptedOffer, error) {
	if accepted == nil || len(accepted.Proposals) == 0 {
		return acceptedOffer{}, errAcceptedOfferEmpty
	}
	switch accepted.Proposals[0].ProtocolID {
	case wire.ProtocolIKE:
		chosen, err := crypto.VerifyAcceptedIKE(wireProposalsToIKE(accepted.Proposals), buildIKEProposals(ikeGroup))
		if err != nil {
			return acceptedOffer{}, fmt.Errorf("%w: %w", errAcceptedOfferMismatch, err)
		}
		return acceptedOffer{Protocol: wire.ProtocolIKE, IKE: chosen}, nil
	case wire.ProtocolESP:
		for i := range accepted.Proposals {
			if !acceptedESPESNConsistent(accepted.Proposals[i]) {
				return acceptedOffer{}, errAcceptedOfferESN
			}
		}
		chosen, err := crypto.NegotiateESP(wireProposalsToESP(accepted.Proposals), buildESPProposals(espGroup))
		if err != nil {
			return acceptedOffer{}, fmt.Errorf("%w: %w", errAcceptedOfferMismatch, err)
		}
		local, ok := espConfigForAccepted(espGroup, chosen)
		if !ok {
			return acceptedOffer{}, errAcceptedOfferUnmapped
		}
		return acceptedOffer{Protocol: wire.ProtocolESP, ESP: chosen, ESPConfig: local}, nil
	default:
		return acceptedOffer{}, errAcceptedOfferProtocol
	}
}

// parseHashAlgoNotify extracts hash algorithm IDs from a SIGNATURE_HASH_ALGORITHMS
// notify payload data.
func parseHashAlgoNotify(data []byte) []uint16 {
	if len(data) < 2 {
		return nil
	}
	algos := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		algos = append(algos, binary.BigEndian.Uint16(data[i:i+2]))
	}
	return algos
}

// buildSignatureHashAlgosNotify creates a SIGNATURE_HASH_ALGORITHMS notify
// payload announcing SHA2-256, SHA2-384, SHA2-512.
// RFC 7427 Section 4.
func buildSignatureHashAlgosNotify() *wire.PayloadNotify {
	data := make([]byte, 6)
	binary.BigEndian.PutUint16(data[0:2], hashAlgoSHA2256)
	binary.BigEndian.PutUint16(data[2:4], hashAlgoSHA2384)
	binary.BigEndian.PutUint16(data[4:6], hashAlgoSHA2512)
	return &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifySignatureHashAlgorithms,
		NotificationData: data,
	}
}

func lookupEncryption(algo ipsec.EncryptionAlgo) crypto.EncryptionTransform {
	t, err := crypto.LookupEncryption(algo.String())
	if err != nil {
		return crypto.EncryptionTransform{}
	}
	return t
}

func lookupPRF(hash ipsec.HashAlgo) crypto.PRFTransform {
	t, err := crypto.LookupPRF(hash.String())
	if err != nil {
		return crypto.PRFTransform{}
	}
	return t
}

func lookupIntegrity(hash ipsec.HashAlgo) crypto.IntegrityTransform {
	t, err := crypto.LookupIntegrity(hash.String())
	if err != nil {
		return crypto.IntegrityTransform{}
	}
	return t
}

// espTransforms resolves the encryption and integrity transforms of one configured ESP
// proposal. RFC 7296 Section 3.3 makes the integrity transform NONE for an AEAD
// cipher, so a hash named beside such a cipher never becomes an integrity key.
//
// Every ESP key-derivation site calls this rather than pair lookupEncryption with
// lookupIntegrity. The sites that paired them put two integrity keys into an AEAD
// KEYMAT. That moved the responder encryption key 32 octets past the offset the peer
// reads it at. The wire offer stayed correct, because espProposalToWire omits the
// integrity transform for an AEAD cipher. Both kernels accepted their keys, and one
// direction of the tunnel decrypted nothing (ai/rules/evidence.md).
//
// The verdict comes from the Transform ID, never from the IsAEAD field, for the reason
// crypto.EncryptionID.IsAEAD gives.
func espTransforms(p ipsec.ESPProposal) (crypto.EncryptionTransform, crypto.IntegrityTransform) {
	enc := lookupEncryption(p.Encryption)
	if enc.ID.IsAEAD() {
		return enc, crypto.IntegrityTransform{ID: crypto.AUTH_NONE}
	}
	return enc, lookupIntegrity(p.Hash)
}

// wireEncryptionTransform reads one ENCR transform off the wire.
//
// It goes through crypto.NewEncryptionTransform, which fills the AEAD property from
// the Transform ID. Both readers of a peer's SA payload call it, so neither one has
// to remember that property on its own. A reader that filled the ID and the key
// length alone left IsAEAD at false for every cipher. That false value reads as a
// valid "not AEAD" answer (ai/rules/evidence.md).
func wireEncryptionTransform(t wire.Transform) crypto.EncryptionTransform {
	var keyLength uint16
	for _, a := range t.Attrs {
		if a.Type == wire.AttrTypeKeyLength {
			keyLength = a.Value
		}
	}
	return crypto.NewEncryptionTransform(crypto.EncryptionID(t.ID), keyLength)
}

func encAttrs(enc crypto.EncryptionTransform) []wire.TransformAttr {
	if enc.KeyLength != 0 {
		return []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: enc.KeyLength}}
	}
	return nil
}

// buildChildSAPayloads builds the SAi2, TSi, TSr payloads of an IKE_AUTH or
// CREATE_CHILD_SA request. The SA payload is an OFFER, so it carries every
// configured proposal numbered one upward (RFC 7296 Section 3.3). Returns the
// generated inbound ESP SPI and the three payloads.
func buildChildSAPayloads(sa *SA) (uint32, *wire.PayloadSA, *wire.PayloadTS, *wire.PayloadTS, error) {
	if len(sa.ESPGroup.Proposals) == 0 {
		return 0, nil, nil, nil, errors.New("no ESP proposals configured")
	}
	espSPI, err := generateESPSPI()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	saPayload := &wire.PayloadSA{Proposals: buildWireESPProposals(sa.ESPGroup, espSPI)}
	// A REQUEST proposes the operator's configured selectors, so a responder that
	// narrows has something of ours to narrow. An unconfigured peer still proposes the
	// wildcard (proposeChildTSPayloads).
	tsi, tsr := proposeChildTSPayloads(sa)
	return espSPI, saPayload, tsi, tsr, nil
}

// errNoAcceptedESPProposal reports a response built before any ESP proposal was
// selected. RFC 7296 Section 3.3.1 needs the accepted number, and a response that
// invented one would name a proposal the peer never sent.
var errNoAcceptedESPProposal = errors.New("ike auth: no accepted ESP proposal to answer with")

// buildChildSAResponsePayloads builds the SAr2, TSi, TSr payloads of an IKE_AUTH
// response. The SA payload is a RESPONSE, so it names exactly one proposal (RFC 7296
// Section 3.3). It carries the number the peer put on the proposal that was accepted
// (Section 3.3.1). selectResponderESP narrowed sa.ESPGroup to that proposal, and
// recorded its number.
func buildChildSAResponsePayloads(sa *SA) (uint32, *wire.PayloadSA, *wire.PayloadTS, *wire.PayloadTS, error) {
	if len(sa.ESPGroup.Proposals) == 0 {
		return 0, nil, nil, nil, errors.New("no ESP proposals configured")
	}
	if sa.ChildProposalNum == 0 {
		return 0, nil, nil, nil, errNoAcceptedESPProposal
	}
	espSPI, err := generateESPSPI()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	accepted := espProposalToWire(sa.ESPGroup.Proposals[0], espSPI, sa.ChildProposalNum)
	saPayload := &wire.PayloadSA{Proposals: []wire.Proposal{accepted}}

	// RFC 7296 Section 2.9: a RESPONSE carries the NARROWED selectors, which are a
	// subset of the initiator's proposal. narrowChildSelectors put that subset on the SA.
	//
	// This call used to be anyChildTSPayloads(sa), which answered every exchange with
	// 0.0.0.0-255.255.255.255, all ports, all protocols. That is a strict SUPERSET of any
	// non-wildcard proposal, so the responder was not merely failing to narrow: it was
	// widening, which rfc/full/rfc7296.txt:2393-2395 forbids.
	tsi, tsr := pairsToWire(sa.NegotiatedPairs)
	if tsi == nil || tsr == nil {
		return 0, nil, nil, nil, errTSUnacceptable
	}
	return espSPI, saPayload, tsi, tsr, nil
}

func anyTrafficSelector(ipv6 bool) wire.TrafficSelector {
	if ipv6 {
		return wire.TrafficSelector{
			TSType:       wire.TSTypeIPv6AddrRange,
			IPProtocol:   0,
			StartPort:    0,
			EndPort:      65535,
			StartAddress: make([]byte, 16),
			EndAddress:   []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		}
	}
	return wire.TrafficSelector{
		TSType:       wire.TSTypeIPv4AddrRange,
		IPProtocol:   0,
		StartPort:    0,
		EndPort:      65535,
		StartAddress: []byte{0, 0, 0, 0},
		EndAddress:   []byte{255, 255, 255, 255},
	}
}

// buildWireESPProposals converts an ESP group to a wire OFFER.
func buildWireESPProposals(espGroup ipsec.ESPGroup, spi uint32) []wire.Proposal {
	proposals := make([]wire.Proposal, 0, len(espGroup.Proposals))
	for i := range espGroup.Proposals {
		proposals = append(proposals, espProposalToWire(espGroup.Proposals[i], spi, offerProposalNum(i)))
	}
	return proposals
}

// espESNNotExtended is the one Transform Type 5 value ze can key. RFC 7296
// Section 3.3.2 defines 0 as "No Extended Sequence Numbers" and 1 as "Extended Sequence
// Numbers", and RFC 4303 Section 3.3.3 gives the second a 64-bit sequence space whose
// high-order half each peer maintains without transmitting it.
//
// Ze maintains no such half. dataplane.SAParams carries no ESN field, so neither backend
// can ask for one: xfrmStateFromParams (dataplane/xfrm_linux.go) builds the whole kernel
// state and never sets netlink.XfrmState.ESN, and vppSAFlags (dataplane/vpp.go) builds
// the whole flag word and never sets IPSEC_API_SAD_FLAG_USE_ESN. Every SA ze installs
// therefore counts 32 bits, and a peer that counted 64 would see every packet fail
// anti-replay.
//
// So this value is what ze OFFERS, what it ANSWERS with, and the only value it can
// accept from a peer. espProposalMatches (responder.go) refuses an offer that includes
// Transform Type 5 without it, and acceptedESPESNConsistent (below) refuses a response
// that selects anything else. Adding ESN to the datapath is plan/spec-ipsec-12-esn.md;
// until it lands, honesty about the absence is the conformant answer.
const espESNNotExtended uint16 = 0

// espProposalToWire encodes one configured ESP proposal under the given Proposal
// Num. An offer derives that number from the position (offerProposalNum). A response
// carries the number the peer put on the proposal that was accepted, which RFC 7296
// Section 3.3.1 requires the response to match.
func espProposalToWire(p ipsec.ESPProposal, spi uint32, number uint8) wire.Proposal {
	spiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(spiBytes, spi)
	enc := lookupEncryption(p.Encryption)
	transforms := []wire.Transform{
		{Type: wire.TransformTypeENCR, ID: uint16(enc.ID), Attrs: encAttrs(enc)},
	}
	if !p.Encryption.IsAEAD() {
		integ := lookupIntegrity(p.Hash)
		transforms = append(transforms, wire.Transform{
			Type: wire.TransformTypeINTG, ID: uint16(integ.ID),
		})
	}
	// RFC 7296 Section 3.3.3 makes Transform Type 5 mandatory in an ESP proposal, and
	// espESNNotExtended is the only value ze can key. One transform therefore says both
	// what ze offers as an initiator and what it answers with as a responder, and
	// RFC 7296 Section 2.7 has the answer carry exactly one transform of the type.
	transforms = append(transforms, wire.Transform{
		Type: wire.TransformTypeESN, ID: espESNNotExtended,
	})
	return wire.Proposal{
		Number:     number,
		ProtocolID: wire.ProtocolESP,
		SPISize:    4,
		SPI:        spiBytes,
		Transforms: transforms,
	}
}

// tsToIPNet was deleted with WP-7. It read selectors[0] only and discarded the port and
// the protocol, and it returned nil for any range that was not CIDR-aligned -- which made
// createFirstChildSA fall back to a /32 of the tunnel endpoint, silently substituting a
// policy the peer never proposed. wireToSelectors (ts_narrow.go) replaces it: it converts
// EVERY selector, keeps the port and the protocol, and narrows a non-prefix range INWARD
// to the largest prefix it contains rather than discarding it
// (ai/rules/no-layering.md: delete X before implementing Y).
