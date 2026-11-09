// Design: docs/architecture/testing/interop.md -- exact typed-NLRI retention.
package bgp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/evpn"
)

// typedDiscardRetained is the exact assigned NLRI in inject.msg: type 2,
// RD 172.30.0.9:1, zero ESI/tag, MAC 00:11:22:33:44:55, no IP and label 2.
// It is independent of the decoder and of the peer's summarized route count.
var typedDiscardRetained = [...]byte{
	2, 33, 0, 1, 172, 30, 0, 9, 0, 1,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 48, 0, 17, 34, 51, 68, 85, 0, 0, 0, 33,
}

// requireTypedEVPNWire consumes every reported route-bearing UPDATE, not merely
// a historical match. Malformed envelopes or a different assigned route cannot
// answer for the fixture's retained route, and a withdrawal cannot answer for an
// announcement. The requested capture must finish on the actual OPEN session.
func requireTypedEVPNWire(report string) error {
	for _, field := range []string{"ze-open", "capture-complete"} {
		value, printed := speakerReportField(report, field)
		if !printed || value != logValueYes {
			return fmt.Errorf("speaker %s = %q, want yes", field, value)
		}
	}
	for _, field := range []string{speakerFieldRouteUpdates, speakerFieldEVPNNLRI} {
		count, err := speakerReportCount(report, field)
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("speaker %s = %d, want exactly one", field, count)
		}
	}
	updates, routes := 0, 0
	for line := range strings.SplitSeq(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != speakerNoteMarker || fields[1] != "update-hex:" {
			continue
		}
		if len(fields) != 3 {
			return errors.New("malformed speaker UPDATE-hex note")
		}
		body, err := hex.DecodeString(fields[2])
		if err != nil {
			return fmt.Errorf("speaker UPDATE hex: %w", err)
		}
		update, err := parsedEmptyMPUpdate(body)
		if err != nil {
			return fmt.Errorf("speaker UPDATE envelope: %w", err)
		}
		if len(update.withdrawn) != 0 || len(update.nlri) != 0 {
			return errors.New("typed discard witness carries ordinary IPv4 NLRI")
		}
		seen := map[byte]bool{}
		for _, attribute := range update.attributes {
			if seen[attribute.code] {
				return fmt.Errorf("duplicate attribute %d in typed discard witness", attribute.code)
			}
			seen[attribute.code] = true
			if attribute.code != mpReach && attribute.code != mpUnreach {
				continue
			}
			value := attribute.value
			if attribute.code != mpReach || len(value) < 9 || value[0] != 0 || value[1] != 25 || value[2] != 70 || value[3] != 4 || value[8] != 0 {
				return errors.New("typed discard witness has wrong or truncated EVPN MP_REACH envelope")
			}
			section := value[9:]
			for len(section) != 0 {
				route, rest, err := evpn.ParseEVPN(section, false)
				if err != nil {
					return fmt.Errorf("malformed retained EVPN NLRI: %w", err)
				}
				consumed := len(section) - len(rest)
				if route.RouteType() != evpn.EVPNRouteType2 || !bytes.Equal(section[:consumed], typedDiscardRetained[:]) {
					return fmt.Errorf("retained EVPN NLRI %x, want exact fixture type 2 %x", section[:consumed], typedDiscardRetained)
				}
				routes++
				section = rest
			}
		}
		updates++
	}
	if updates != 1 || routes != 1 {
		return fmt.Errorf("typed discard witness has %d route-bearing UPDATEs and %d exact type-2 announcements, want one of each", updates, routes)
	}
	return nil
}
