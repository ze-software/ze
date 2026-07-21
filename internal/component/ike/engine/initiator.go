// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE_SA_INIT initiator logic
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT exchange (Section 1.2)
package engine

import (
	"encoding/binary"
	"errors"
	"net"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
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

	// Size the buffer to the message length so WriteTo (which indexes buf
	// directly) can never overrun. The request is built entirely from ze's own
	// configuration (proposals, our DH public key), so its length is not
	// remotely influenced; a Len()-sized allocation bounds it without an error
	// path. RFC 7296 Section 3.1. See spec-fixit-fixed-buffer-overflow.
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

// buildWireIKEProposals converts config IKE proposals to wire format.
func buildWireIKEProposals(ikeGroup ipsec.IKEGroup) []wire.Proposal {
	proposals := make([]wire.Proposal, 0, len(ikeGroup.Proposals))
	for _, p := range ikeGroup.Proposals {
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

// buildChildSAPayloads builds the SAi2, TSi, TSr payloads for IKE_AUTH.
// Returns the generated inbound ESP SPI and the three payloads.
func buildChildSAPayloads(sa *SA) (uint32, *wire.PayloadSA, *wire.PayloadTS, *wire.PayloadTS, error) {
	if len(sa.ESPGroup.Proposals) == 0 {
		return 0, nil, nil, nil, errors.New("no ESP proposals configured")
	}
	espSPI, err := GenerateESPSPI()
	if err != nil {
		return 0, nil, nil, nil, err
	}
	proposals := buildWireESPProposals(sa.ESPGroup, espSPI)
	saPayload := &wire.PayloadSA{Proposals: proposals}

	remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
	isV6 := remoteIP != nil && remoteIP.To4() == nil
	tsAny := anyTrafficSelector(isV6)

	tsi := &wire.PayloadTS{
		TSPayloadType:    wire.PayloadTypeTSi,
		TrafficSelectors: []wire.TrafficSelector{tsAny},
	}
	tsr := &wire.PayloadTS{
		TSPayloadType:    wire.PayloadTypeTSr,
		TrafficSelectors: []wire.TrafficSelector{tsAny},
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

// buildWireESPProposals converts an ESP group to wire SA proposals.
func buildWireESPProposals(espGroup ipsec.ESPGroup, spi uint32) []wire.Proposal {
	proposals := make([]wire.Proposal, 0, len(espGroup.Proposals))
	for _, p := range espGroup.Proposals {
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
		transforms = append(transforms, wire.Transform{
			Type: wire.TransformTypeESN, ID: 0,
		})
		proposals = append(proposals, wire.Proposal{
			Number:     uint8(p.Number),
			ProtocolID: wire.ProtocolESP,
			SPISize:    4,
			SPI:        spiBytes,
			Transforms: transforms,
		})
	}
	return proposals
}

// tsToIPNet converts the first traffic selector from a wire TS payload to *net.IPNet.
// Returns nil if the range is not CIDR-aligned.
func tsToIPNet(selectors []wire.TrafficSelector) *net.IPNet {
	if len(selectors) == 0 {
		return nil
	}
	ts := selectors[0]
	if len(ts.StartAddress) == 0 || len(ts.StartAddress) != len(ts.EndAddress) {
		return nil
	}
	mask := make(net.IPMask, len(ts.StartAddress))
	for i := range mask {
		mask[i] = ^(ts.StartAddress[i] ^ ts.EndAddress[i])
	}
	ones, bits := mask.Size()
	if ones == 0 && bits == 0 {
		return nil
	}
	ip := make(net.IP, len(ts.StartAddress))
	copy(ip, ts.StartAddress)
	masked := ip.Mask(mask)
	if !masked.Equal(ip) {
		return nil
	}
	return &net.IPNet{IP: ip, Mask: mask}
}
