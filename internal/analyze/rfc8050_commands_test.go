// Design: docs/architecture/mrt.md — ADD-PATH at the real offline command handlers.
package analyze

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050CommandsReadSubtypeContext drives show, density and the content
// filter with actual OPEN evidence followed by one homogeneous ADD-PATH UPDATE.
// RFC requirement: RFC8050-x-4 positive -- actual OPEN context lets show and density count exactly two announcements and two withdrawals from a homogeneous ADD-PATH UPDATE, and content filtering accepts its AS_PATH without reading Path Identifiers as prefixes.
func TestRFC8050CommandsReadSubtypeContext(t *testing.T) {
	nlri4 := []byte{0, 0, 0, 1, 24, 10, 0, 0}
	nlri6 := []byte{0, 0, 0, 1, 32, 0x20, 1, 0x0d, 0xb8}
	nh := netip.MustParseAddr("2001:db8::1").As16()
	attrs := []byte{0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8}
	for _, attr := range []mrt.PathAttribute{mpReachAttr(2, 1, nh[:], nlri6), mpUnreachAttr(2, 1, nlri6)} {
		attrs = append(attrs, 0x80, attr.Code, byte(len(attr.Value)))
		attrs = append(attrs, attr.Value...)
	}
	body := buildUpdate(nlri4, attrs, nlri4)
	for _, subtype := range []uint16{8, 9, 10, 11} {
		fields := []byte{0xfd, 0xe8, 0xfd, 0xe9}
		if subtype == 9 || subtype == 11 {
			fields = []byte{0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe9}
		}
		fields = append(fields, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2)
		fields = append(fields, bytes.Repeat([]byte{0xff}, 16)...)
		fields = binary.BigEndian.AppendUint16(fields, uint16(19+len(body)))
		fields = append(fields, 2)
		fields = append(fields, body...)
		wire := mrtRecord(mrt.TypeBGP4MP, subtype, fields)
		wire = append(commandOpenPrelude(subtype, 3, 3), wire...)
		code, stdout, stderr := runSubcommandCapturing(t, wire, func() int { return runShow([]string{"-"}) })
		if code != 0 || stderr != "" || !strings.Contains(stdout, "W=2 A=2") {
			t.Fatalf("show subtype=%d code=%d out=%q err=%q", subtype, code, stdout, stderr)
		}
		code, stdout, stderr = runSubcommandCapturing(t, wire, func() int { return runDensity([]string{"-"}) })
		for _, want := range []string{"Total UPDATEs analyzed: 1", "Total announced NLRIs: 2", "Total withdrawn NLRIs: 2"} {
			if code != 0 || stderr != "" || !strings.Contains(stdout, want) {
				t.Fatalf("density subtype=%d want=%q code=%d out=%q err=%q", subtype, want, code, stdout, stderr)
			}
		}
		err := mrt.ReadFrom(bytes.NewReader(wire), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, record *mrt.MessageRecord) error {
			if record.BGPMessage.Bytes[18] != 2 {
				return nil
			}
			// AS4 is explicit here: this fixture's attribute is four-octet even
			// when testing the independent two-octet BGP4MP peer-header layout.
			match, err := matchMessageContent(record, true, &filterOpts{asPathRe: regexp.MustCompile("^65000$")})
			if err != nil || !match {
				t.Fatal("content filter rejected the ADD-PATH UPDATE")
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
}
