// VALIDATES: spec-plugin-ownership-imports Phase E -- a path attribute's JSON
// decoder is declared by the plugin that owns the attribute, and exactly one
// plugin owns each attribute code.
// PREVENTS: two plugins claiming one attribute code, which would leave the
// rendering `ze bgp decode` prints to map order, and a declaration that cannot
// render anything being accepted as live.

package registry

import (
	"errors"
	"testing"
)

// attrDecoderReg builds a registration that declares a JSON decoder for code.
func attrDecoderReg(name string, code uint8, dec AttributeJSONDecoder) Registration {
	reg := validReg(name)
	reg.AttributeJSONDecoders = map[uint8]AttributeJSONDecoder{code: dec}
	return reg
}

func renderSeen([]byte) map[string]any { return map[string]any{"seen": true} }

// TestRegisterRefusesDuplicateAttributeJSONDecoder registers one decoder for
// attribute 29, then a second plugin claiming 29. The second Register must fail
// with ErrDuplicateAttributeDecoder and leave the first decoder the answer.
func TestRegisterRefusesDuplicateAttributeJSONDecoder(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	first := AttributeJSONDecoder{Key: "bgp-ls", Decode: renderSeen}
	if err := Register(attrDecoderReg("owner-a", 29, first)); err != nil {
		t.Fatal(err)
	}
	err := Register(attrDecoderReg("owner-b", 29, AttributeJSONDecoder{Key: "other", Decode: renderSeen}))
	if !errors.Is(err, ErrDuplicateAttributeDecoder) {
		t.Fatalf("second claim of attribute 29: err = %v; want ErrDuplicateAttributeDecoder", err)
	}
	dec, ok := AttributeJSONDecoderFor(29)
	if !ok || dec.Key != "bgp-ls" {
		t.Fatalf("AttributeJSONDecoderFor(29) = %q, %v; want the first owner's bgp-ls", dec.Key, ok)
	}
	if err := Register(attrDecoderReg("owner-c", 30, AttributeJSONDecoder{Key: "x", Decode: renderSeen})); err != nil {
		t.Fatalf("a different code must register: %v", err)
	}
}

// TestRegisterRefusesIncompleteAttributeJSONDecoder proves a decoder with no
// JSON key or no decode function is refused at registration, where the plugin
// author sees it, rather than rendering nothing at decode time.
func TestRegisterRefusesIncompleteAttributeJSONDecoder(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	if err := Register(attrDecoderReg("no-key", 29, AttributeJSONDecoder{Decode: renderSeen})); !errors.Is(err, ErrInvalidAttributeDecoder) {
		t.Fatalf("empty key: err = %v; want ErrInvalidAttributeDecoder", err)
	}
	if err := Register(attrDecoderReg("no-func", 29, AttributeJSONDecoder{Key: "bgp-ls"})); !errors.Is(err, ErrInvalidAttributeDecoder) {
		t.Fatalf("nil Decode: err = %v; want ErrInvalidAttributeDecoder", err)
	}
	if _, ok := AttributeJSONDecoderFor(29); ok {
		t.Fatal("a refused declaration answered AttributeJSONDecoderFor(29)")
	}
}
