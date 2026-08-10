// Design: docs/architecture/wire/buffer-writer.md -- buffer-first encoding
// RFC: rfc/short/rfc7296.md -- IKEv2 message structure (Section 3)
//
// VALIDATES: Message.Len()/Payload.Len() report exactly what WriteTo writes, and
// Message.CheckedWriteTo rejects an undersized buffer without writing.
// PREVENTS: Len()/WriteTo drift silently reintroducing the fixed-buffer
// slice-bounds panic that CheckedWriteTo exists to remove.
package wire

import "testing"

// TestPayloadLenMatchesWriteTo pins Len() to WriteTo() for every payload type.
// Message.CheckedWriteTo sizes a fixed buffer from Len() before WriteTo indexes
// it directly, so any drift between the two reintroduces the slice-bounds panic
// this guard exists to remove (spec-fixit-fixed-buffer-overflow).
func TestPayloadLenMatchesWriteTo(t *testing.T) {
	cases := []struct {
		name string
		p    Payload
	}{
		{"SA-with-spi", &PayloadSA{Proposals: []Proposal{{
			Number: 1, ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4},
			Transforms: []Transform{
				{Type: TransformTypeENCR, ID: 12, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
				{Type: TransformTypeINTG, ID: 12},
			},
		}}}},
		{"SA-no-spi", &PayloadSA{Proposals: []Proposal{{
			Number: 1, ProtocolID: ProtocolIKE, SPISize: 0,
			Transforms: []Transform{{Type: TransformTypeENCR, ID: 12}},
		}}}},
		// SPISize claims 4 but the SPI slice is too short: WriteTo writes no SPI
		// bytes and zeroes SPISize, so Len() must not count them either.
		{"SA-spi-underrun", &PayloadSA{Proposals: []Proposal{{
			Number: 1, ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{9},
			Transforms: []Transform{{Type: TransformTypeENCR, ID: 12}},
		}}}},
		{"SA-multi-proposal", &PayloadSA{Proposals: []Proposal{
			{Number: 1, ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4},
				Transforms: []Transform{{Type: TransformTypeENCR, ID: 12}}},
			{Number: 2, ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{5, 6, 7, 8},
				Transforms: []Transform{{Type: TransformTypeENCR, ID: 20}, {Type: TransformTypeESN, ID: 0}}},
		}}},
		{"KE", &PayloadKE{DHGroup: 14, KeyExchangeData: make([]byte, 256)}},
		{"Nonce", &PayloadNonce{NonceData: make([]byte, 32)}},
		{"Notify-plain", &PayloadNotify{NotifyMsgType: NotifyNoProposalChosen}},
		{"Notify-data", &PayloadNotify{NotifyMsgType: NotifyInvalidKEPayload, NotificationData: []byte{0, 14}}},
		{"Notify-spi", &PayloadNotify{ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1, 2, 3, 4}, NotifyMsgType: NotifyRekeySA, NotificationData: []byte{7, 8}}},
		{"Notify-spi-underrun", &PayloadNotify{ProtocolID: ProtocolESP, SPISize: 4, SPI: []byte{1}, NotifyMsgType: NotifyRekeySA, NotificationData: []byte{7, 8}}},
		{"TS-v4", &PayloadTS{TSPayloadType: PayloadTypeTSi, TrafficSelectors: []TrafficSelector{{
			TSType: TSTypeIPv4AddrRange, EndPort: 65535, StartAddress: []byte{0, 0, 0, 0}, EndAddress: []byte{255, 255, 255, 255},
		}}}},
		{"TS-v6", &PayloadTS{TSPayloadType: PayloadTypeTSr, TrafficSelectors: []TrafficSelector{{
			TSType: TSTypeIPv6AddrRange, EndPort: 65535, StartAddress: make([]byte, 16), EndAddress: make([]byte, 16),
		}}}},
		{"SK", &PayloadSK{CipherText: make([]byte, 48)}},
		{"AUTH", &PayloadAUTH{AuthMethod: AuthMethodPSK, AuthData: make([]byte, 32)}},
		{"CERT", &PayloadCERT{CertEncoding: CertEncodingX509Sig, CertData: make([]byte, 40)}},
		{"CERTREQ", &PayloadCERTREQ{CertEncoding: CertEncodingX509Sig, CertAuthority: make([]byte, 20)}},
		{"CP", &PayloadCP{CFGType: CFGTypeReply, Attrs: []ConfigAttr{
			{Type: CPAttrInternalIP6Address, Value: make([]byte, 17)},
			{Type: CPAttrInternalIP6DNS, Value: make([]byte, 16)},
		}}},
		{"Delete", &PayloadDelete{ProtocolID: ProtocolESP, SPISize: 4, NumSPIs: 2, SPIs: []byte{1, 2, 3, 4, 5, 6, 7, 8}}},
		{"EAP", &PayloadEAP{Code: EAPCodeResponse, Identifier: 1, EAPData: make([]byte, 10)}},
		{"ID", &PayloadID{IDPayloadType: PayloadTypeIDi, IDType: IDTypeFQDN, IDData: []byte("peer.example")}},
		{"VendorID", &PayloadVendorID{VendorIDData: []byte("ze")}},
		{"Raw", &payloadRaw{PayloadType: 200, Data: make([]byte, 12)}},
	}

	buf := make([]byte, 8192)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.p.WriteTo(buf, 0)
			if want := tc.p.Len(); got != want {
				t.Fatalf("Len()=%d but WriteTo wrote %d bytes", want, got)
			}
		})
	}
}

// TestMessageLenMatchesWriteTo pins Message.Len() to Message.WriteTo() for a
// full payload chain, so CheckedWriteTo's up-front sizing is exact.
func TestMessageLenMatchesWriteTo(t *testing.T) {
	msg := &Message{
		Header: Header{
			InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
			MajorVersion: 2,
			ExchangeType: ExchangeIKESAInit,
			Flags:        FlagResponse,
		},
		Payloads: []PayloadEntry{
			{Payload: &PayloadSA{Proposals: []Proposal{{Number: 1, ProtocolID: ProtocolIKE, Transforms: []Transform{
				{Type: TransformTypeENCR, ID: 12, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
				{Type: TransformTypePRF, ID: 5},
				{Type: TransformTypeINTG, ID: 12},
				{Type: TransformTypeDH, ID: 14},
			}}}}},
			{Payload: &PayloadKE{DHGroup: 14, KeyExchangeData: make([]byte, 256)}},
			{Payload: &PayloadNonce{NonceData: make([]byte, 32)}},
			{Payload: &PayloadNotify{NotifyMsgType: NotifySignatureHashAlgorithms, NotificationData: []byte{0, 2, 0, 3, 0, 4}}},
			{Payload: &PayloadTS{TSPayloadType: PayloadTypeTSi, TrafficSelectors: []TrafficSelector{{
				TSType: TSTypeIPv4AddrRange, EndPort: 65535, StartAddress: []byte{0, 0, 0, 0}, EndAddress: []byte{255, 255, 255, 255},
			}}}},
		},
	}

	buf := make([]byte, 8192)
	got := msg.WriteTo(buf, 0)
	if want := msg.Len(); got != want {
		t.Fatalf("Message.Len()=%d but WriteTo wrote %d bytes", want, got)
	}
	// The header Length field must also equal the written length (RFC 7296 §3.1).
	if int(msg.Header.Length) != got {
		t.Fatalf("header Length=%d but %d bytes written", msg.Header.Length, got)
	}
}

// TestMessageCheckedWriteTo covers the fit / overflow decision.
func TestMessageCheckedWriteTo(t *testing.T) {
	msg := &Message{
		Header:   Header{MajorVersion: 2, ExchangeType: ExchangeIKESAInit, Flags: FlagResponse},
		Payloads: []PayloadEntry{{Payload: &PayloadNotify{NotifyMsgType: NotifyNoProposalChosen, NotificationData: make([]byte, 600)}}},
	}
	need := msg.Len()

	// A buffer one byte short is rejected, names the required length, writes nothing.
	small := make([]byte, need-1)
	if n, err := msg.CheckedWriteTo(small, 0); err == nil {
		t.Fatalf("expected error for undersized buffer, got n=%d", n)
	} else if n != 0 {
		t.Fatalf("expected 0 bytes written on error, got %d", n)
	}

	// An exactly-sized buffer succeeds and equals raw WriteTo.
	exact := make([]byte, need)
	n, err := msg.CheckedWriteTo(exact, 0)
	if err != nil {
		t.Fatalf("CheckedWriteTo on exact buffer: %v", err)
	}
	raw := make([]byte, need)
	if rn := msg.WriteTo(raw, 0); rn != n {
		t.Fatalf("CheckedWriteTo wrote %d, WriteTo wrote %d", n, rn)
	}
	for i := range n {
		if exact[i] != raw[i] {
			t.Fatalf("checked vs raw differ at byte %d", i)
		}
	}
}
