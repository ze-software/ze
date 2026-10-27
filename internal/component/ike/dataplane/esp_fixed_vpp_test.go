// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact VPP ESP installation
//go:build ze_vpp

package dataplane

import (
	"bytes"
	"encoding/binary"
	"testing"

	"go.fd.io/govpp/binapi/ipsec_types"
)

// TestESPFixedVPPInstall checks emitted binapi fields, not just name lookup, and
// makes a VPP refusal observable rather than silently installing another cipher.
func TestESPFixedVPPInstall(t *testing.T) {
	for _, aead := range []bool{false, true} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.IsAEAD = aead
		p.EncAlgo, p.AuthAlgo = "3des", "sha1"
		p.EncKey, p.AuthKey = bytes.Repeat([]byte{1}, 24), bytes.Repeat([]byte{2}, 20)
		if aead {
			p.EncAlgo, p.AuthAlgo = "chacha20poly1305", ""
			p.EncKey, p.AuthKey = bytes.Repeat([]byte{3}, 36), nil
		}
		if err := b.InstallSA(p); err != nil {
			t.Fatal(err)
		}
		entry := sentSAD(t, ch).Entry
		if aead {
			if entry.CryptoAlgorithm != ipsec_types.IPSEC_API_CRYPTO_ALG_CHACHA20_POLY1305 || entry.IntegrityAlgorithm != ipsec_types.IPSEC_API_INTEG_ALG_NONE {
				t.Fatalf("wrong ChaCha algorithms: %+v", entry)
			}
			if entry.CryptoKey.Length != 32 || !bytes.Equal(entry.CryptoKey.Data, p.EncKey[:32]) || entry.Salt != binary.BigEndian.Uint32(p.EncKey[32:]) {
				t.Fatal("ChaCha key/salt split changed")
			}
		} else {
			if entry.CryptoAlgorithm != ipsec_types.IPSEC_API_CRYPTO_ALG_3DES_CBC || entry.IntegrityAlgorithm != ipsec_types.IPSEC_API_INTEG_ALG_SHA1_96 {
				t.Fatalf("wrong 3DES algorithms: %+v", entry)
			}
			if entry.CryptoKey.Length != 24 || entry.IntegrityKey.Length != 20 || !bytes.Equal(entry.CryptoKey.Data, p.EncKey) || !bytes.Equal(entry.IntegrityKey.Data, p.AuthKey) {
				t.Fatal("3DES/SHA1 key material changed")
			}
		}
		refusing, channel := newCapturingBackend()
		channel.refuse = map[string]int32{"ipsec_sad_entry_add_del_v3": -1}
		if err := refusing.InstallSA(p); err == nil {
			t.Fatal("VPP refusal hidden")
		}
		for _, msg := range channel.sent {
			if msg.GetMessageName() != "ipsec_sad_entry_add_del_v3" {
				t.Fatalf("unexpected fallback request %s", msg.GetMessageName())
			}
		}
	}
}
