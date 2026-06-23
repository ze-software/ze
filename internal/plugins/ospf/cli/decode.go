// Design: plan/spec-ospf-2-wire.md -- offline OSPFv2 packet decode CLI

package cli

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"unicode"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
)

const maxStdinBytes = 256 * 1024

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
		errln("usage: ze ospf-decode [--pretty] < hex")
		errln("  reads a hex OSPFv2 packet from stdin, emits JSON on stdout")
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
		errln(b.Str("error: input exceeds ").Int(int64(maxStdinBytes)).Str(" bytes (max); likely not a hex OSPFv2 packet").String())
		return 1
	}
	wire := toWire(raw)

	p, err := packet.DecodePacket(wire)
	if err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: decode packet: ").Err(err).String())
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(p.ToJSON()); err != nil {
		var b textbuf.Buffer
		errln(b.Str("error: encode json: ").Err(err).String())
		return 1
	}
	return 0
}

func toWire(raw []byte) []byte {
	clean := stripWhitespace(string(raw))
	if clean != "" && len(clean)%2 == 0 && isHexString(clean) {
		if decoded, err := hex.DecodeString(clean); err == nil {
			return decoded
		}
	}
	return raw
}

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
