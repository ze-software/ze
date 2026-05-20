// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE_SA_INIT initiator logic
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT exchange (Section 1.2)
package engine

import (
	"encoding/binary"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
)

// newInitiatorSA creates a new SA for the initiator side.
func newInitiatorSA(peerName string, peer ipsec.SiteToSitePeer, ikeGroup ipsec.IKEGroup) (*SA, error) {
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
	proposals := buildWireIKEProposals(ikeGroup)
	dhGroupID := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)

	payloads := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: proposals}},
		{Payload: &wire.PayloadKE{
			DHGroup:         uint16(dhGroupID),
			KeyExchangeData: sa.LocalDH.PublicKey,
		}},
		{Payload: &wire.PayloadNonce{NonceData: sa.LocalNonce}},
		{Payload: buildSignatureHashAlgosNotify()},
	}

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

	buf := make([]byte, 4096)
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

// buildWireIKEProposals converts config IKE proposals to wire format.
func buildWireIKEProposals(ikeGroup ipsec.IKEGroup) []wire.Proposal {
	proposals := make([]wire.Proposal, 0, len(ikeGroup.Proposals))
	for _, p := range ikeGroup.Proposals {
		enc := lookupEncryption(p.Encryption)
		prf := lookupPRF(p.Hash)
		integ := lookupIntegrity(p.Hash)
		dh := uint16(p.DHGroup)

		transforms := []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: uint16(enc.ID), Attrs: encAttrs(enc)},
			{Type: wire.TransformTypePRF, ID: uint16(prf.ID)},
			{Type: wire.TransformTypeINTG, ID: uint16(integ.ID)},
			{Type: wire.TransformTypeDH, ID: dh},
		}

		proposals = append(proposals, wire.Proposal{
			Number:     uint8(p.Number),
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
		out = append(out, crypto.IKEProposal{
			Number:     p.Number,
			Encryption: lookupEncryption(p.Encryption),
			PRF:        lookupPRF(p.Hash),
			Integrity:  lookupIntegrity(p.Hash),
			DHGroup:    crypto.DHGroupTransform{ID: crypto.DHGroupID(p.DHGroup)},
		})
	}
	return out
}

// wireProposalsToIKE converts wire SA proposals to crypto proposals for negotiation.
func wireProposalsToIKE(wireProps []wire.Proposal) []crypto.IKEProposal {
	out := make([]crypto.IKEProposal, 0, len(wireProps))
	for _, wp := range wireProps {
		p := crypto.IKEProposal{Number: uint16(wp.Number)}
		for _, t := range wp.Transforms {
			switch t.Type {
			case wire.TransformTypeENCR:
				p.Encryption.ID = crypto.EncryptionID(t.ID)
				for _, a := range t.Attrs {
					if a.Type == wire.AttrTypeKeyLength {
						p.Encryption.KeyLength = a.Value
					}
				}
			case wire.TransformTypePRF:
				p.PRF.ID = crypto.PRFID(t.ID)
			case wire.TransformTypeINTG:
				p.Integrity.ID = crypto.IntegrityID(t.ID)
			case wire.TransformTypeDH:
				p.DHGroup.ID = crypto.DHGroupID(t.ID)
			case wire.TransformTypeESN:
				// ESN not used for IKE proposals
			}
		}
		out = append(out, p)
	}
	return out
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
	binary.BigEndian.PutUint16(data[0:2], 2) // SHA2-256
	binary.BigEndian.PutUint16(data[2:4], 3) // SHA2-384
	binary.BigEndian.PutUint16(data[4:6], 4) // SHA2-512
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

func encAttrs(enc crypto.EncryptionTransform) []wire.TransformAttr {
	if enc.KeyLength != 0 {
		return []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: enc.KeyLength}}
	}
	return nil
}
