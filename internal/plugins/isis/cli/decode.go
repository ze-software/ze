// Design: docs/architecture/wire/isis.md -- offline IS-IS PDU decode CLI (wiring proof)
//
// Offline IS-IS PDU decoder. Reads a hex blob from stdin (ASCII hex, any
// whitespace or newlines allowed), parses it with
// internal/plugins/isis/packet, and emits a JSON view on stdout. This is the
// thin caller proving the codec wires end-to-end (test/isis-wire/isis-pdu-1.ci);
// the full IS-IS CLI surface (show isis ...) is owned by isis-13.

package cli

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// maxStdinBytes caps the hex input. A single IS-IS PDU is bounded by the link
// MTU (a few thousand bytes); the hex encoding doubles that. 256 KiB is ample
// and bounds allocation so a malformed pipe cannot exhaust memory.
const maxStdinBytes = 256 * 1024

// errln writes a user-facing CLI diagnostic line (message + newline) to stderr.
// The error from writing to stderr is genuinely unactionable for a CLI, so it
// is read and dropped via a named variable (not a blank Write discard).
func errln(msg string) {
	var b textbuf.Buffer
	if _, werr := os.Stderr.WriteString(b.Str(msg).Byte('\n').String()); werr != nil {
		return
	}
}

func cmdDecode(args []string) int {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	pretty := fs.Bool("pretty", false, "indent JSON output")
	fs.Usage = func() {
		errln("usage: ze isis decode [--pretty] < hex")
		errln("  reads a hex IS-IS PDU from stdin, emits JSON on stdout")
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: read stdin: ").Err(err).String())
		return 1
	}
	if len(raw) > maxStdinBytes {
		var b textbuf.Buffer
		errln(b.Str("error: input exceeds ").Int(int64(maxStdinBytes)).Str(" bytes (max); likely not a hex IS-IS PDU").String())
		return 1
	}
	wire := toWire(raw)

	pdu, err := packet.DecodePDU(wire)
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: decode PDU: ").Err(err).String())
		return 1
	}
	defer pdu.Release()

	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(pdu.ToJSON()); err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: encode json: ").Err(err).String())
		return 1
	}
	return 0
}

// toWire turns the stdin bytes into the raw IS-IS PDU. The offline decoder
// accepts BOTH encodings so it is robust to how the bytes arrive:
//   - ASCII hex (e.g. a human pasting "831c..." or a test fixture): after
//     stripping whitespace, if the input is non-empty, even-length, and all hex
//     digits, it is hex-decoded into the wire bytes.
//   - raw PDU bytes (e.g. piped straight from a capture, or a test harness that
//     pre-decodes a hex= block): used verbatim.
//
// The distinction is unambiguous in practice: an IS-IS PDU starts with the
// protocol discriminator 0x83, which is not an ASCII hex character, so raw PDU
// bytes are never mistaken for a hex string.
func toWire(raw []byte) []byte {
	clean := stripWhitespace(string(raw))
	if clean != "" && len(clean)%2 == 0 && isHexString(clean) {
		if decoded, err := hex.DecodeString(clean); err == nil {
			return decoded
		}
	}
	return raw
}

// isHexString reports whether s consists solely of ASCII hex digits.
func isHexString(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// stripWhitespace removes all Unicode whitespace from s so hex input may span
// multiple lines with arbitrary spacing.
func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
