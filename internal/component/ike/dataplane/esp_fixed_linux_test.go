// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- ESP transform installation
package dataplane

import (
	"bytes"
	"testing"
)

// TestESPFixedXFRMState checks the actual netlink state producer, including the
// ChaCha salt/tag and the separate 3DES/SHA1 key and truncation fields.
func TestESPFixedXFRMState(t *testing.T) {
	for _, aead := range []bool{false, true} {
		p := boundarySA(0x12345678)
		p.IsAEAD = aead
		p.EncAlgo, p.AuthAlgo = "3des", "sha1"
		p.EncKey, p.AuthKey = bytes.Repeat([]byte{1}, 24), bytes.Repeat([]byte{2}, 20)
		if aead {
			p.EncAlgo, p.AuthAlgo = "chacha20poly1305", ""
			p.EncKey, p.AuthKey = bytes.Repeat([]byte{3}, 36), nil
		}
		state, err := xfrmStateFromParams(p)
		if err != nil {
			t.Fatal(err)
		}
		if aead {
			if state.Aead == nil || state.Auth != nil || state.Crypt != nil {
				t.Fatalf("wrong AEAD state: %+v", state)
			}
			if state.Aead.Name != "rfc7539esp(chacha20,poly1305)" || state.Aead.ICVLen != 128 || !bytes.Equal(state.Aead.Key, p.EncKey) {
				t.Fatalf("wrong ChaCha transform: %+v", state.Aead)
			}
		} else {
			if state.Crypt == nil || state.Auth == nil || state.Aead != nil {
				t.Fatalf("wrong CBC state: %+v", state)
			}
			if state.Crypt.Name != "cbc(des3_ede)" || !bytes.Equal(state.Crypt.Key, p.EncKey) {
				t.Fatalf("wrong 3DES transform: %+v", state.Crypt)
			}
			if state.Auth.Name != "hmac(sha1)" || state.Auth.TruncateLen != 96 || !bytes.Equal(state.Auth.Key, p.AuthKey) {
				t.Fatalf("wrong SHA1 transform: %+v", state.Auth)
			}
		}
	}
}
