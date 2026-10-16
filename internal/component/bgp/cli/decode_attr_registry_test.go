package cli

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestDecodeAttributeUsesRegisteredJSONDecoder verifies attribute 29 is
// rendered through the decoder a plugin registered, and that with no plugin
// registered it falls back to the unknown-attribute path without a panic.
//
// VALIDATES: AC-9 / AC-10: decode_update.go resolves code 29 through
// registry.AttributeJSONDecoderFor and names no plugin.
// PREVENTS: `ze bgp decode` losing the bgp-ls rendering, or panicking in a
// build without the BGP-LS plugin. Not parallel: it replaces the registry.
func TestDecodeAttributeUsesRegisteredJSONDecoder(t *testing.T) {
	saved := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(saved) })
	registry.Reset()

	value := []byte{0x04, 0x04, 0x00, 0x01, 0xff}
	if _, _, understood := renderAttributeZe(29, value); understood {
		t.Fatal("attribute 29 understood with no registered decoder")
	}

	if err := registry.Register(registry.Registration{
		Name:        "test-attr-decoder",
		Description: "attribute JSON decoder test plugin",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		AttributeJSONDecoders: map[uint8]registry.AttributeJSONDecoder{
			29: {Key: "bgp-ls", Decode: func([]byte) map[string]any { return map[string]any{"seen": true} }},
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	key, rendered, understood := renderAttributeZe(29, value)
	if !understood || key != "bgp-ls" {
		t.Fatalf("renderAttributeZe(29) = %q, %v, %v; want bgp-ls through the registered decoder", key, rendered, understood)
	}
}
