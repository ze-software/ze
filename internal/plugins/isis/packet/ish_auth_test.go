// Design: docs/architecture/isis/isis-10-auth.md -- RFC 1195 Annex D ISH passwords.
// Goal: authenticate the ISO 9542 path without borrowing an IS-IS-only HMAC format.
// Method: sign a complete ISH and check password rotation, refusal, and wire framing.

package packet

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// A signed ISH carries TLV133 type1 and remains checksum-valid. Any configured
// receive password can accept it; a wrong password or missing TLV cannot.
func TestISHCleartextAuthentication(t *testing.T) {
	net, err := types.ParseNET("49.0001.0000.0000.0001.00")
	if err != nil {
		t.Fatal(err)
	}
	ish := ISH{NET: net, HoldingTime: 30, TLVs: []TLV{{Type: TLVProtocolsSupported, Value: []byte{0xcc}}}}
	var buf [254]byte
	end := ish.WriteTo(buf[:], 0)
	key := Key{Algorithm: AuthAlgoCleartext, Secret: []byte("current")}
	if err := VerifyISH(buf[:end], []Key{key}); !errors.Is(err, ErrAuthMissing) {
		t.Fatalf("unsigned ISH error = %v, want ErrAuthMissing", err)
	}
	signed, err := SignISH(buf[:end], key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signed[end:], []byte{133, 8, 1, 'c', 'u', 'r', 'r', 'e', 'n', 't'}) {
		t.Fatalf("authentication TLV = %x", signed[end:])
	}
	if !VerifyChecksum(signed) {
		t.Fatal("signed ISH checksum is invalid")
	}
	if err := VerifyISH(signed, []Key{{Algorithm: AuthAlgoCleartext, Secret: []byte("previous")}, key}); err != nil {
		t.Fatalf("rotation receive set: %v", err)
	}
	if err := VerifyISH(signed, []Key{{Algorithm: AuthAlgoCleartext, Secret: []byte("wrong")}}); !errors.Is(err, ErrAuthMismatch) {
		t.Fatalf("wrong password error = %v, want ErrAuthMismatch", err)
	}
	if _, err := SignISH(signed, key); !errors.Is(err, ErrAuthMalformed) {
		t.Fatalf("duplicate signature error = %v, want ErrAuthMalformed", err)
	}
}

// The signer refuses algorithms without a defined ISO9542 wire format and
// refuses insufficient caller capacity rather than allocating or truncating.
func TestISHSigningRefusesUnsupportedAndOversized(t *testing.T) {
	net, err := types.ParseNET("49.0001.0000.0000.0001.00")
	if err != nil {
		t.Fatal(err)
	}
	ish := ISH{NET: net, HoldingTime: 30}
	var buf [254]byte
	end := ish.WriteTo(buf[:], 0)
	if _, err := SignISH(buf[:end], Key{Algorithm: AuthAlgoHMACMD5, Secret: []byte("key")}); !errors.Is(err, ErrAuthUnsupported) {
		t.Fatalf("HMAC ISH error = %v, want ErrAuthUnsupported", err)
	}
	key := Key{Algorithm: AuthAlgoCleartext, Secret: []byte("key")}
	if _, err := SignISH(buf[:end:end], key); !errors.Is(err, ErrShortBuffer) {
		t.Fatalf("short signing buffer error = %v, want ErrShortBuffer", err)
	}
	key.Secret = make([]byte, 254)
	if _, err := SignISH(buf[:end], key); !errors.Is(err, ErrLength) {
		t.Fatalf("oversized signed ISH error = %v, want ErrLength", err)
	}
}
