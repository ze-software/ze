package wire

import (
	"bytes"
	"errors"
	"testing"
)

func TestMessageChainRoundtrip(t *testing.T) {
	nonce := make([]byte, 32)
	for i := range nonce {
		nonce[i] = byte(i)
	}
	keyData := make([]byte, 64)
	for i := range keyData {
		keyData[i] = byte(i + 50)
	}
	notifyData := make([]byte, 20)
	for i := range notifyData {
		notifyData[i] = byte(i + 200)
	}

	msg := Message{
		Header: Header{
			InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			ResponderSPI: [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
			MajorVersion: 2,
			ExchangeType: ExchangeIKESAInit,
			Flags:        FlagInitiator,
			MessageID:    0,
		},
		Payloads: []PayloadEntry{
			{Payload: &PayloadSA{
				Proposals: []Proposal{
					{
						Number:     1,
						ProtocolID: ProtocolIKE,
						Transforms: []Transform{
							{Type: TransformTypeENCR, ID: 12, Attrs: []TransformAttr{{Type: AttrTypeKeyLength, Value: 256}}},
							{Type: TransformTypePRF, ID: 5},
							{Type: TransformTypeINTG, ID: 12},
							{Type: TransformTypeDH, ID: 14},
						},
					},
				},
			}},
			{Payload: &PayloadKE{DHGroup: 14, KeyExchangeData: keyData}},
			{Payload: &PayloadNonce{NonceData: nonce}},
			{Payload: &PayloadNotify{
				NotifyMsgType:    NotifyNATDetectionSourceIP,
				NotificationData: notifyData,
			}},
		},
	}

	buf := make([]byte, 4096)
	n := msg.WriteTo(buf, 0)

	var got Message
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if got.Header.MajorVersion != 2 {
		t.Errorf("MajorVersion = %d, want 2", got.Header.MajorVersion)
	}
	if got.Header.ExchangeType != ExchangeIKESAInit {
		t.Errorf("ExchangeType = %d, want %d", got.Header.ExchangeType, ExchangeIKESAInit)
	}
	if got.Header.Length != uint32(n) {
		t.Errorf("Length = %d, want %d", got.Header.Length, n)
	}
	if len(got.Payloads) != 4 {
		t.Fatalf("got %d payloads, want 4", len(got.Payloads))
	}

	// Verify SA
	sa, ok := got.Payloads[0].Payload.(*PayloadSA)
	if !ok {
		t.Fatalf("payload[0] = %T, want *PayloadSA", got.Payloads[0].Payload)
	}
	if len(sa.Proposals) != 1 {
		t.Fatalf("SA proposals = %d, want 1", len(sa.Proposals))
	}
	if len(sa.Proposals[0].Transforms) != 4 {
		t.Errorf("SA transforms = %d, want 4", len(sa.Proposals[0].Transforms))
	}

	// Verify KE
	ke, ok := got.Payloads[1].Payload.(*PayloadKE)
	if !ok {
		t.Fatalf("payload[1] = %T, want *PayloadKE", got.Payloads[1].Payload)
	}
	if ke.DHGroup != 14 {
		t.Errorf("KE.DHGroup = %d, want 14", ke.DHGroup)
	}
	if !bytes.Equal(ke.KeyExchangeData, keyData) {
		t.Error("KE data mismatch")
	}

	// Verify Nonce
	pn, ok := got.Payloads[2].Payload.(*PayloadNonce)
	if !ok {
		t.Fatalf("payload[2] = %T, want *PayloadNonce", got.Payloads[2].Payload)
	}
	if !bytes.Equal(pn.NonceData, nonce) {
		t.Error("Nonce data mismatch")
	}

	// Verify Notify
	notify, ok := got.Payloads[3].Payload.(*PayloadNotify)
	if !ok {
		t.Fatalf("payload[3] = %T, want *PayloadNotify", got.Payloads[3].Payload)
	}
	if notify.NotifyMsgType != NotifyNATDetectionSourceIP {
		t.Errorf("Notify type = %d, want %d", notify.NotifyMsgType, NotifyNATDetectionSourceIP)
	}
	if !bytes.Equal(notify.NotificationData, notifyData) {
		t.Error("Notify data mismatch")
	}
}

func TestMessageEmptyPayloads(t *testing.T) {
	// INFORMATIONAL DPD: header only, no payloads inside SK
	msg := Message{
		Header: Header{
			MajorVersion: 2,
			ExchangeType: ExchangeInformational,
			Flags:        FlagResponse,
			MessageID:    5,
		},
	}

	buf := make([]byte, 256)
	n := msg.WriteTo(buf, 0)

	var got Message
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(got.Payloads) != 0 {
		t.Errorf("got %d payloads, want 0", len(got.Payloads))
	}
	if got.Header.Length != HeaderLen {
		t.Errorf("Length = %d, want %d", got.Header.Length, HeaderLen)
	}
}

func TestMessageHeaderTruncated(t *testing.T) {
	var msg Message
	err := msg.ReadFrom(make([]byte, 10))
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("ReadFrom(10 bytes) = %v, want ErrTruncated", err)
	}
}

func TestMessageLengthExceedsData(t *testing.T) {
	buf := make([]byte, HeaderLen)
	var h Header
	h.MajorVersion = 2
	h.Length = 1000 // claims 1000 bytes but only 28 available
	h.WriteTo(buf, 0)

	var msg Message
	err := msg.ReadFrom(buf)
	if !errors.Is(err, ErrLengthMismatch) {
		t.Errorf("ReadFrom = %v, want ErrLengthMismatch", err)
	}
}
