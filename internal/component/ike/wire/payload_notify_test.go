package wire

import (
	"bytes"
	"testing"
)

func TestPayloadNotifyRoundtrip(t *testing.T) {
	notifyData := make([]byte, 20)
	for i := range notifyData {
		notifyData[i] = byte(i + 50)
	}
	p := PayloadNotify{
		ProtocolID:       0,
		SPISize:          0,
		NotifyMsgType:    NotifyNATDetectionSourceIP,
		NotificationData: notifyData,
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.ProtocolID != 0 {
		t.Errorf("ProtocolID = %d, want 0", got.ProtocolID)
	}
	if got.NotifyMsgType != NotifyNATDetectionSourceIP {
		t.Errorf("NotifyMsgType = %d, want %d", got.NotifyMsgType, NotifyNATDetectionSourceIP)
	}
	if !bytes.Equal(got.NotificationData, notifyData) {
		t.Error("NotificationData mismatch")
	}
}

func TestPayloadNotifyWithSPI(t *testing.T) {
	p := PayloadNotify{
		ProtocolID:    ProtocolESP,
		SPISize:       4,
		NotifyMsgType: NotifyRekeySA,
		SPI:           []byte{0xAA, 0xBB, 0xCC, 0xDD},
	}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadNotify
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.ProtocolID != ProtocolESP {
		t.Errorf("ProtocolID = %d, want %d", got.ProtocolID, ProtocolESP)
	}
	if got.SPISize != 4 {
		t.Errorf("SPISize = %d, want 4", got.SPISize)
	}
	if !bytes.Equal(got.SPI, p.SPI) {
		t.Errorf("SPI = %x, want %x", got.SPI, p.SPI)
	}
}
