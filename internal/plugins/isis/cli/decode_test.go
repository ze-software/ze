// Design: plan/spec-isis-2-wire.md -- offline decode CLI unit test
package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// buildLSPHex encodes a minimal L2 LSP and returns its lowercase hex, for the
// CLI/JSON tests. It mirrors what `ze isis decode` consumes on stdin.
func buildLSPHex(t *testing.T) string {
	t.Helper()
	lsp := &packet.LSP{
		PDUType:           packet.PDUTypeL2LSP,
		RemainingLifetime: 1199,
		LSPID:             types.NewLSPID(types.NewSourceID(types.SystemID{0, 1, 0, 2, 0, 3}, 0), 0),
		SequenceNumber:    0x10,
		TypeBlock:         packet.LSPFlagISTypeL2,
	}
	buf := make([]byte, lsp.EncodedLen())
	n := lsp.WriteTo(buf, 0)
	return hex.EncodeToString(buf[:n])
}

// VALIDATES: Story 2 wiring -- a hex IS-IS PDU decodes through packet.DecodePDU
// and renders a JSON view with the right type and a valid checksum. This is the
// codec the `ze isis decode` CLI calls; the .ci test exercises the same path
// through the real binary.
// PREVENTS: the decode CLI silently regressing the header parse / dispatch.
func TestISISDecodeCLIJSON(t *testing.T) {
	h := buildLSPHex(t)
	wire, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}
	pdu, err := packet.DecodePDU(wire)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	view := pdu.ToJSON()
	if view.Type != "l2-lsp" {
		t.Errorf("type = %q, want l2-lsp", view.Type)
	}
	if view.LSP == nil {
		t.Fatal("LSP view nil")
	}
	if !view.LSP.ChecksumValid {
		t.Error("checksum-valid should be true for a freshly-encoded LSP")
	}
	if view.LSP.SequenceNumber != 0x10 {
		t.Errorf("sequence-number = %d, want 16", view.LSP.SequenceNumber)
	}
	// The JSON must marshal cleanly (the CLI emits it on stdout).
	if _, err := json.Marshal(view); err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
}

// VALIDATES: stripWhitespace removes spacing/newlines so multi-line hex input
// is accepted by the CLI.
func TestISISDecodeStripWhitespace(t *testing.T) {
	in := "  83 1b\n01 06\t01 \r\n12 01 00 03 "
	got := stripWhitespace(in)
	want := "831b01060112010003" // all whitespace removed
	if got != want {
		t.Errorf("stripWhitespace = %q, want %q", got, want)
	}
}

// VALIDATES: toWire accepts BOTH ASCII hex and raw PDU bytes, so the offline
// decoder works whether stdin carries a hex string (human / fixture) or the
// raw decoded PDU (a harness that pre-decodes the hex= block). An IS-IS PDU
// starts with 0x83 (not an ASCII hex char), so raw bytes are never confused
// with hex.
// PREVENTS: the decode CLI breaking when the .ci runner delivers raw bytes
// instead of an ASCII hex string.
func TestISISDecodeToWire(t *testing.T) {
	h := buildLSPHex(t)
	rawBytes, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	// ASCII hex form (with whitespace) decodes to the same wire bytes.
	hexInput := []byte("  " + h + "\n")
	if got := toWire(hexInput); !bytes.Equal(got, rawBytes) {
		t.Errorf("toWire(hex) = % x, want % x", got, rawBytes)
	}

	// Raw PDU bytes (starting 0x83) pass through verbatim.
	if got := toWire(rawBytes); !bytes.Equal(got, rawBytes) {
		t.Errorf("toWire(raw) = % x, want % x", got, rawBytes)
	}

	// Both forms decode to the same PDU.
	for _, in := range [][]byte{hexInput, rawBytes} {
		pdu, derr := packet.DecodePDU(toWire(in))
		if derr != nil {
			t.Fatalf("DecodePDU(toWire): %v", derr)
		}
		if pdu.LSP == nil {
			t.Fatal("expected LSP")
		}
	}
}
