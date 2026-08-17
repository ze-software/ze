// VALIDATES: every full BGP frame embedded in a .ci fixture declares a Length
//            field equal to its actual byte count.
// PREVENTS: the defect class that made forward-overflow-two-tier (50 frames) and
//            role-otc-unicast-scope (2 frames) untestable for as long as they
//            existed. A frame one byte longer than it declares leaves the extra
//            byte in the stream, so the NEXT header read starts one byte late,
//            fails its 16-byte 0xFF marker check (message/header.go), and the
//            session is torn down. The daemon is correct; the fixture is not.
//            Nothing downstream notices, because the test then fails for a
//            plausible-looking reason (missing EoR, "no established peers") that
//            sends every reader hunting a product bug that is not there.

package runner

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// bgpMarkerHex is the 16-byte all-ones marker every BGP message begins with
// (RFC 4271 Section 4.1). A hex literal starting with it is a full frame whose
// bytes 16-17 are the Length field; anything else is a fragment (a `contains=`
// needle, an attribute blob) and carries no length to check.
const bgpMarkerHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"

// ciFrameOptOut lets a fixture declare a frame malformed ON PURPOSE, for a test
// whose subject IS the daemon's handling of a bad Length. Put it on the line
// above the frame or trailing it. Without an escape hatch the gate would block
// exactly the tests most worth having.
const ciFrameOptOut = "malformed-frame:"

var ciHexValue = regexp.MustCompile(`(?:hex|contains)=([0-9A-Fa-f]{38,})`)

func TestCIFrameLengthsWellFormed(t *testing.T) {
	root := filepath.Join("..", "..", "..", "test")
	if _, err := os.Stat(root); err != nil {
		// Deliberately fatal, not a skip. A gate that disappears when its input
		// moves is worse than no gate: it reads green forever
		// (ai/rules/evidence.md).
		t.Fatalf("test fixture tree not reachable from %s: %v", root, err)
	}

	type badFrame struct {
		file     string
		line     int
		declared int
		actual   int
	}
	var bad []badFrame
	frames := 0

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ is invisible to every gate, this one included
		// (test/draft/README.md).
		if d.IsDir() && isDraftPath(root, path) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".ci") {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // fixture path from the repo's own tree
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, ciFrameOptOut) {
				continue
			}
			if i > 0 && strings.Contains(lines[i-1], ciFrameOptOut) {
				continue
			}
			for _, m := range ciHexValue.FindAllStringSubmatch(line, -1) {
				raw := m[1]
				if len(raw)%2 != 0 || !strings.HasPrefix(strings.ToUpper(raw), bgpMarkerHex) {
					continue
				}
				b, decErr := hex.DecodeString(raw)
				if decErr != nil {
					continue
				}
				frames++
				declared := int(b[16])<<8 | int(b[17])
				if declared != len(b) {
					bad = append(bad, badFrame{file: path, line: i + 1, declared: declared, actual: len(b)})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	if frames == 0 {
		t.Fatal("scanned zero BGP frames: the extractor is broken, not the fixtures")
	}
	t.Logf("scanned %d full BGP frames", frames)

	for _, b := range bad {
		var tb textbuf.Buffer
		t.Errorf("%s", tb.Str(b.file).Byte(':').Int(int64(b.line)).
			Str(": BGP Length field declares ").Int(int64(b.declared)).
			Str(" bytes but the frame carries ").Int(int64(b.actual)).
			Str(". The surplus/shortfall desynchronises the stream and the next header fails its marker check. ").
			Str("Fix the bytes; if the malformation is the point of the test, mark it with `# ").
			Str(ciFrameOptOut).Str(" <reason>`").String())
	}
}

// bgpUpdateType is the BGP message type code for UPDATE (RFC 4271 Section 4.1).
const bgpUpdateType = 2

// checkUpdateStructure walks an UPDATE's interior and returns a description of the
// first structural fault, or "" when it is well formed.
//
// The outer Length check alone is not enough. reconnect.ci carried a LOCAL_PREF
// with flags 0x04 -- neither Optional nor Transitive, and with a low-order bit
// RFC 4271 Section 4.3 says MUST be zero -- while its outer Length was perfectly
// consistent, so it sailed through. A receiver must treat that as an error, which
// makes the fixture's stated purpose vacuous.
func checkUpdateStructure(b []byte) string {
	const hdr = 19
	if len(b) < hdr+4 {
		return "UPDATE shorter than the mandatory withdrawn-routes and attribute length fields"
	}
	var tb textbuf.Buffer
	off := hdr
	wlen := int(b[off])<<8 | int(b[off+1])
	off += 2
	if off+wlen > len(b) {
		return tb.Str("withdrawn-routes length ").Int(int64(wlen)).Str(" runs past the frame").String()
	}
	off += wlen
	if off+2 > len(b) {
		return "frame ends before the total-path-attribute length field"
	}
	alen := int(b[off])<<8 | int(b[off+1])
	off += 2
	attrEnd := off + alen
	if attrEnd > len(b) {
		return tb.Str("total path attribute length ").Int(int64(alen)).
			Str(" runs past the frame (").Int(int64(len(b) - off)).Str(" bytes remain)").String()
	}
	for off < attrEnd {
		if off+3 > attrEnd {
			return "path attribute header runs past the attribute block"
		}
		flags := b[off]
		code := b[off+1]
		// RFC 4271 Section 4.3: "The lower-order four bits of the Attribute Flags
		// octet are unused. They MUST be zero when sent and MUST be ignored when
		// received."
		if flags&0x0F != 0 {
			return tb.Str("attribute ").Int(int64(code)).Str(" flags 0x").
				Hex([]byte{flags}).Str(" sets a low-order bit that RFC 4271 S4.3 requires to be zero").String()
		}
		// A well-known attribute MUST be Transitive (RFC 4271 Section 4.3). The
		// well-known set is ORIGIN(1), AS_PATH(2), NEXT_HOP(3), LOCAL_PREF(5),
		// ATOMIC_AGGREGATE(6).
		optional := flags&0x80 != 0
		transitive := flags&0x40 != 0
		if !optional && !transitive {
			return tb.Str("attribute ").Int(int64(code)).Str(" flags 0x").
				Hex([]byte{flags}).Str(" is neither Optional nor Transitive; a well-known attribute MUST set Transitive (RFC 4271 S4.3)").String()
		}
		vlenSize, vlen := 1, int(b[off+2])
		if flags&0x10 != 0 {
			if off+4 > attrEnd {
				return "extended-length attribute header runs past the attribute block"
			}
			vlenSize, vlen = 2, int(b[off+2])<<8|int(b[off+3])
		}
		off += 2 + vlenSize + vlen
		if off > attrEnd {
			return tb.Str("attribute ").Int(int64(code)).Str(" length ").Int(int64(vlen)).
				Str(" runs past the attribute block").String()
		}
	}
	return ""
}

// TestCIFrameStructureWellFormed walks the interior of every UPDATE embedded in a
// .ci fixture.
//
// VALIDATES: withdrawn-routes and attribute lengths are self-consistent, and each
// path attribute's flags are legal per RFC 4271 Section 4.3.
// PREVENTS: the reconnect.ci class -- a frame whose outer Length is right but whose
// attributes are malformed, so the daemon must discard the UPDATE and the test
// asserts nothing while looking green.
func TestCIFrameStructureWellFormed(t *testing.T) {
	root := filepath.Join("..", "..", "..", "test")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("test fixture tree not reachable from %s: %v", root, err)
	}

	frames := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ is invisible to every gate, this one included
		// (test/draft/README.md).
		if d.IsDir() && isDraftPath(root, path) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".ci") {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // fixture path from the repo's own tree
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if strings.Contains(line, ciFrameOptOut) {
				continue
			}
			if i > 0 && strings.Contains(lines[i-1], ciFrameOptOut) {
				continue
			}
			for _, m := range ciHexValue.FindAllStringSubmatch(line, -1) {
				raw := m[1]
				if len(raw)%2 != 0 || !strings.HasPrefix(strings.ToUpper(raw), bgpMarkerHex) {
					continue
				}
				b, decErr := hex.DecodeString(raw)
				if decErr != nil || len(b) < 19 || b[18] != bgpUpdateType {
					continue
				}
				frames++
				if fault := checkUpdateStructure(b); fault != "" {
					var tb textbuf.Buffer
					t.Errorf("%s", tb.Str(path).Byte(':').Int(int64(i+1)).Str(": ").Str(fault).
						Str(". Fix the bytes; if the malformation is the point of the test, mark it with `# ").
						Str(ciFrameOptOut).Str(" <reason>`").String())
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if frames == 0 {
		t.Fatal("scanned zero UPDATE frames: the extractor is broken, not the fixtures")
	}
	t.Logf("scanned %d UPDATE frames", frames)
}
