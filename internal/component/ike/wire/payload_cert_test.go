package wire

import (
	"bytes"
	"testing"
)

func TestPayloadCertRoundtrip(t *testing.T) {
	certData := make([]byte, 512)
	for i := range certData {
		certData[i] = byte(i)
	}
	p := PayloadCERT{CertEncoding: CertEncodingX509Sig, CertData: certData}

	buf := make([]byte, 1024)
	n := p.WriteTo(buf, 0)

	var got PayloadCERT
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.CertEncoding != CertEncodingX509Sig {
		t.Errorf("CertEncoding = %d, want %d", got.CertEncoding, CertEncodingX509Sig)
	}
	if !bytes.Equal(got.CertData, certData) {
		t.Error("CertData mismatch")
	}
}

func TestPayloadCertReqRoundtrip(t *testing.T) {
	// 3 CA hashes, 20 bytes each = 60 bytes total
	hashes := make([]byte, 60)
	for i := range hashes {
		hashes[i] = byte(i * 3)
	}
	p := PayloadCERTREQ{CertEncoding: CertEncodingX509Sig, CertAuthority: hashes}

	buf := make([]byte, 256)
	n := p.WriteTo(buf, 0)

	var got PayloadCERTREQ
	if err := got.ReadFrom(buf[:n]); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if got.CertEncoding != CertEncodingX509Sig {
		t.Errorf("CertEncoding = %d, want %d", got.CertEncoding, CertEncodingX509Sig)
	}
	if !bytes.Equal(got.CertAuthority, hashes) {
		t.Error("CertAuthority mismatch")
	}
	if len(got.CertAuthority) != 60 {
		t.Errorf("CertAuthority len = %d, want 60", len(got.CertAuthority))
	}
}
