// Design: docs/architecture/wire/isis.md -- offline decode rendering (JSON view of a PDU)
//
// This file produces a stable JSON view of a decoded PDU for the offline
// `ze isis decode` CLI (the wiring proof) and future diagnostics (isis-13). It
// is a COLD path (one PDU per CLI invocation), so encoding/json is acceptable
// (ai/rules/performance.md explicitly allows make([]byte)/JSON marshaling off
// the hot path). The runtime decode path does NOT use this; it consumes the
// typed views directly.

package packet

import "encoding/hex"

// JSONView is a JSON-serializable rendering of a decoded PDU. Exactly one body
// field is non-nil, matching the header's PDU type. Field names are stable
// kebab/lower tokens for diagnostics and test assertions.
type JSONView struct {
	Type             string `json:"type"`               // PDU type token, e.g. "l2-lsp"
	MaxAreaAddresses uint8  `json:"max-area-addresses"` // common-header field

	LANHello *lanHelloJSON `json:"lan-hello,omitempty"`
	P2PHello *p2pHelloJSON `json:"p2p-hello,omitempty"`
	LSP      *lspJSON      `json:"lsp,omitempty"`
	CSNP     *csnpJSON     `json:"csnp,omitempty"`
	PSNP     *psnpJSON     `json:"psnp,omitempty"`
}

type tlvJSON struct {
	Type  uint8  `json:"type"`
	Len   int    `json:"len"`
	Value string `json:"value"` // lowercase hex of the raw value
}

type lanHelloJSON struct {
	CircuitType uint8     `json:"circuit-type"`
	SystemID    string    `json:"system-id"`
	HoldingTime uint16    `json:"holding-time"`
	Priority    uint8     `json:"priority"`
	LANID       string    `json:"lan-id"`
	TLVs        []tlvJSON `json:"tlvs"`
}

type p2pHelloJSON struct {
	CircuitType    uint8     `json:"circuit-type"`
	SystemID       string    `json:"system-id"`
	HoldingTime    uint16    `json:"holding-time"`
	LocalCircuitID uint8     `json:"local-circuit-id"`
	TLVs           []tlvJSON `json:"tlvs"`
}

type lspJSON struct {
	RemainingLifetime uint16    `json:"remaining-lifetime"`
	LSPID             string    `json:"lsp-id"`
	SequenceNumber    uint32    `json:"sequence-number"`
	Checksum          uint16    `json:"checksum"`
	ChecksumValid     bool      `json:"checksum-valid"`
	Overload          bool      `json:"overload"`
	TypeBlock         uint8     `json:"type-block"`
	TLVs              []tlvJSON `json:"tlvs"`
}

type csnpJSON struct {
	SourceID   string    `json:"source-id"`
	StartLSPID string    `json:"start-lsp-id"`
	EndLSPID   string    `json:"end-lsp-id"`
	TLVs       []tlvJSON `json:"tlvs"`
}

type psnpJSON struct {
	SourceID string    `json:"source-id"`
	TLVs     []tlvJSON `json:"tlvs"`
}

// tlvsToJSON renders a TLV slice to its JSON form (type, length, hex value).
func tlvsToJSON(tlvs []TLV) []tlvJSON {
	out := make([]tlvJSON, 0, len(tlvs))
	for _, t := range tlvs {
		out = append(out, tlvJSON{
			Type:  t.Type,
			Len:   len(t.Value),
			Value: hex.EncodeToString(t.Value),
		})
	}
	return out
}

// ToJSON renders a decoded PDU to its JSON view. Cold path (CLI/diagnostics).
func (p PDU) ToJSON() JSONView {
	v := JSONView{
		Type:             p.Header.PDUType.String(),
		MaxAreaAddresses: p.Header.MaxAreaAddresses,
	}
	switch {
	case p.LANHello != nil:
		h := p.LANHello
		v.LANHello = &lanHelloJSON{
			CircuitType: uint8(h.CircuitType),
			SystemID:    h.SystemID.String(),
			HoldingTime: h.HoldingTime.Seconds(),
			Priority:    h.Priority,
			LANID:       h.LANID.String(),
			TLVs:        tlvsToJSON(h.TLVs),
		}
	case p.P2PHello != nil:
		h := p.P2PHello
		v.P2PHello = &p2pHelloJSON{
			CircuitType:    uint8(h.CircuitType),
			SystemID:       h.SystemID.String(),
			HoldingTime:    h.HoldingTime.Seconds(),
			LocalCircuitID: h.LocalCircuitID,
			TLVs:           tlvsToJSON(h.TLVs),
		}
	case p.LSP != nil:
		l := p.LSP
		v.LSP = &lspJSON{
			RemainingLifetime: l.RemainingLifetime.Seconds(),
			LSPID:             l.LSPID.String(),
			SequenceNumber:    uint32(l.SequenceNumber),
			Checksum:          l.Checksum,
			ChecksumValid:     l.VerifyChecksum(),
			Overload:          l.IsOverloaded(),
			TypeBlock:         l.TypeBlock,
			TLVs:              tlvsToJSON(l.TLVs),
		}
	case p.CSNP != nil:
		c := p.CSNP
		v.CSNP = &csnpJSON{
			SourceID:   c.SourceID.String(),
			StartLSPID: c.StartLSPID.String(),
			EndLSPID:   c.EndLSPID.String(),
			TLVs:       tlvsToJSON(c.TLVs),
		}
	case p.PSNP != nil:
		ps := p.PSNP
		v.PSNP = &psnpJSON{
			SourceID: ps.SourceID.String(),
			TLVs:     tlvsToJSON(ps.TLVs),
		}
	}
	return v
}
