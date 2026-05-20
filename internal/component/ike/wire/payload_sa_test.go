package wire

import (
	"bytes"
	"errors"
	"testing"
)

func TestPayloadSARoundtrip(t *testing.T) {
	sa := PayloadSA{
		Proposals: []Proposal{
			{
				Number:     1,
				ProtocolID: ProtocolIKE,
				SPISize:    0,
				Transforms: []Transform{
					{Type: TransformTypeENCR, ID: 12, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
					{Type: TransformTypePRF, ID: 5},
					{Type: TransformTypeINTG, ID: 12},
				},
			},
			{
				Number:     2,
				ProtocolID: ProtocolIKE,
				SPISize:    0,
				Transforms: []Transform{
					{Type: TransformTypeENCR, ID: 20, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
					{Type: TransformTypePRF, ID: 5},
					{Type: TransformTypeDH, ID: 14},
				},
			},
		},
	}

	buf := make([]byte, 1024)
	n := sa.WriteTo(buf, 0)

	var got PayloadSA
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if len(got.Proposals) != 2 {
		t.Fatalf("got %d proposals, want 2", len(got.Proposals))
	}
	if got.Proposals[0].Number != 1 {
		t.Errorf("proposal[0].Number = %d, want 1", got.Proposals[0].Number)
	}
	if got.Proposals[0].ProtocolID != ProtocolIKE {
		t.Errorf("proposal[0].ProtocolID = %d, want %d", got.Proposals[0].ProtocolID, ProtocolIKE)
	}
	if len(got.Proposals[0].Transforms) != 3 {
		t.Fatalf("proposal[0] transforms = %d, want 3", len(got.Proposals[0].Transforms))
	}
	if got.Proposals[0].Transforms[0].ID != 12 {
		t.Errorf("transform[0].ID = %d, want 12", got.Proposals[0].Transforms[0].ID)
	}
	if len(got.Proposals[0].Transforms[0].Attrs) != 1 {
		t.Fatalf("transform[0] attrs = %d, want 1", len(got.Proposals[0].Transforms[0].Attrs))
	}
	if got.Proposals[0].Transforms[0].Attrs[0].Value != 256 {
		t.Errorf("keylen = %d, want 256", got.Proposals[0].Transforms[0].Attrs[0].Value)
	}
	if len(got.Proposals[1].Transforms) != 3 {
		t.Fatalf("proposal[1] transforms = %d, want 3", len(got.Proposals[1].Transforms))
	}
}

func TestProposalTransformNested(t *testing.T) {
	// Proposal with ESP SPI (4 bytes) and multiple transforms
	sa := PayloadSA{
		Proposals: []Proposal{
			{
				Number:     1,
				ProtocolID: ProtocolESP,
				SPISize:    4,
				SPI:        []byte{0xDE, 0xAD, 0xBE, 0xEF},
				Transforms: []Transform{
					{Type: TransformTypeENCR, ID: 20, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 128}}},
					{Type: TransformTypeESN, ID: 0},
				},
			},
		},
	}

	buf := make([]byte, 512)
	n := sa.WriteTo(buf, 0)

	var got PayloadSA
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if len(got.Proposals) != 1 {
		t.Fatalf("got %d proposals, want 1", len(got.Proposals))
	}
	p := got.Proposals[0]
	if p.SPISize != 4 {
		t.Errorf("SPISize = %d, want 4", p.SPISize)
	}
	if !bytes.Equal(p.SPI, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("SPI = %x, want DEADBEEF", p.SPI)
	}
	if len(p.Transforms) != 2 {
		t.Fatalf("transforms = %d, want 2", len(p.Transforms))
	}
	if p.Transforms[0].Type != TransformTypeENCR || p.Transforms[0].ID != 20 {
		t.Errorf("transform[0] = type %d id %d, want ENCR/20", p.Transforms[0].Type, p.Transforms[0].ID)
	}
}

func TestPayloadSAEmpty(t *testing.T) {
	var sa PayloadSA
	err := sa.ReadFrom([]byte{})
	if !errors.Is(err, ErrNoProposals) {
		t.Errorf("ReadFrom empty = %v, want ErrNoProposals", err)
	}
}
