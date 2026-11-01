// Design: docs/architecture/config/exabgp-syntax.md -- ExaBGP flow route traffic-class above 63
// Overview: migrate_routes.go -- flow route conversion to Ze update blocks

package migration

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/nlri/flowspec"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// flowDSCPBits keeps the six least significant bits of a type 11 octet.
const flowDSCPBits = 0x3F

// flowTrafficClassDSCP answers an ExaBGP traffic-class value with each operand
// from 64 to 255 replaced by the DSCP in its six low bits, operators kept, and
// a note for the operator naming the route, empty when it replaced none.
//
// ExaBGP 4.2 and 5.0 accept a traffic-class up to 0xFFFF (ClassValue,
// class_value) and send it as one octet unchanged (IOperationByte.encode), so
// traffic-class 101 goes out as 0x65. RFC 8955 Section 4.2.2.11: "The six
// least significant bits contain the DSCP value. All other bits SHOULD be
// treated as 0." A receiver reads 0x65 as DSCP 37, and so does ExaBGP 6.0
// (flow.py _dscp). Ze refuses a DSCP above 63, so the migration writes 37: the
// same match on the remote router. An operand from 256 up fits no octet, and
// ExaBGP does not send it either, so it is left for ze to refuse.
func flowTrafficClassDSCP(route, value string) (string, string) {
	tokens := strings.Fields(value)
	changed := false
	for i, token := range tokens {
		terms := strings.Split(token, "&")
		for j, term := range terms {
			digits := strings.TrimLeft(term, "=<>!")
			// bitSize 8 refuses an operand no octet holds.
			n, err := strconv.ParseUint(digits, 10, 8)
			if err != nil || n <= flowDSCPBits {
				continue
			}
			terms[j] = term[:len(term)-len(digits)] + strconv.FormatUint(n&flowDSCPBits, 10)
			changed = true
		}
		tokens[i] = strings.Join(terms, "&")
	}
	if !changed {
		return value, ""
	}
	written := strings.Join(tokens, " ")
	var tb textbuf.Buffer
	tb.Str("flow route ").Str(route).Str(": ").Str(flowspec.KwTrafficClass).Byte(' ').Str(value).
		Str(" is migrated as ").Str(flowspec.KwTrafficClass).Byte(' ').Str(written).
		Str(": ExaBGP sends the value as one octet, and RFC 8955 Section 4.2.2.11 reads the DSCP from its six least significant bits only; check that this is the DSCP the route should match")
	return written, tb.String()
}
