// Design: docs/functional-tests.md -- How an ExaBGP Compatibility Case Passes
// RFC: rfc/short/rfc4271.md Section 4.3
// Related: exabgp_server.go -- serveExaBGPConnection, runExaBGPGroup
// Related: exabgp_case.go -- readExaBGPRoute, the options read here
//
// What the mock does only for ExaBGP's 4.2 and 5.0 fixtures
// (test/exabgp-compat/old-4.2, old-5.0): match decoded IPv4 routes the way the
// 4.2 mock does, answer each message with a Cease under the 4.2
// send-notification option, and name the 5.0 mock's unknown message type.

//go:build ze_bgp

package bgp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strconv"
	"syscall"
)

// exabgpUnknownMessageType is the type the 5.0 mock's unknown message carries
// (`bytes([255])` in qa/old/5.0/qa/sbin/bgp): a type no BGP message has.
const exabgpUnknownMessageType = 255

// exabgpNotifyCommunication is the text the 4.2 mock's Cease carries.
const exabgpNotifyCommunication = "closing session because we can"

// exabgpDecodedRoutes holds the routes of an UPDATE that a route group has not
// consumed yet. The 4.2 mock moves to the next group in the middle of an UPDATE
// once the current group is complete, so the rest of that UPDATE's routes are
// owed to the groups after it rather than dropped. It holds the routes of one
// UPDATE, which the message length bounds.
// Not safe for concurrent use: one session's walk owns it.
type exabgpDecodedRoutes struct {
	pending []string
}

// matchExaBGPRoutes reads from the speaker until every route of one 4.2 route
// group has arrived, as the 4.2 mock's handle_keepalive does: each UPDATE is
// decoded into its IPv4 routes, an End-of-RIB and an UPDATE with no IPv4 route
// field are skipped, and a route the group does not name fails the case. The
// 4.2 mock decodes no other message, so a KEEPALIVE, an OPEN or a
// ROUTE-REFRESH is read past; a NOTIFICATION ends the case.
func matchExaBGPRoutes(wire exabgpWire, group []exabgpStep, recorder *frameRecorder, decoded *exabgpDecodedRoutes) error {
	remaining := make([]string, 0, len(group))
	for _, step := range group {
		remaining = append(remaining, step.route)
	}
	for len(remaining) > 0 {
		if len(decoded.pending) == 0 {
			if err := readExaBGPRoutes(wire, group[0].sequence, remaining, recorder, decoded); err != nil {
				return err
			}
			continue
		}
		route := decoded.pending[0]
		decoded.pending = decoded.pending[1:]
		found := slices.Index(remaining, route)
		if found >= 0 {
			remaining = slices.Delete(remaining, found, found+1)
			continue
		}
		if recorder.capturing() {
			recorder.mismatch()
			continue
		}
		return fmt.Errorf("unexpected route %s; %d expected routes remain (first: %s)", route, len(remaining), remaining[0])
	}
	return nil
}

// readExaBGPRoutes reads the speaker's messages until one UPDATE carries at
// least one IPv4 route, and leaves its routes in decoded.
func readExaBGPRoutes(wire exabgpWire, sequence int, remaining []string, recorder *frameRecorder, decoded *exabgpDecodedRoutes) error {
	for {
		messageType, body, err := wire.read()
		if err != nil {
			return fmt.Errorf("sequence %d: %d expected route(s) never arrived (first: %s): %w",
				sequence, len(remaining), remaining[0], err)
		}
		if messageType == bgpNotification {
			return fmt.Errorf("sequence %d: the speaker sent NOTIFICATION %X while %d expected route(s) remain (first: %s)",
				sequence, body, len(remaining), remaining[0])
		}
		if messageType != bgpUpdate {
			continue
		}
		recorder.record(speakerMessage(messageType, body))
		routes, err := exabgpLegacyRoutes(body)
		if err != nil {
			return fmt.Errorf("sequence %d: %w", sequence, err)
		}
		if len(routes) == 0 {
			continue
		}
		decoded.pending = routes
		return nil
	}
}

// exabgpLegacyRoutes decodes the withdrawn routes, then the NLRI, of an UPDATE
// body into the 4.2 mock's text: `withdraw:<prefix>` and `announce:<prefix>`,
// the prefix written with its four octets, those past its length zero.
//
// RFC 4271 Section 4.3: the UPDATE body is
//
//	+-----------------------------------------------------+
//	|   Withdrawn Routes Length (2 octets)                |
//	+-----------------------------------------------------+
//	|   Withdrawn Routes (variable)                       |
//	+-----------------------------------------------------+
//	|   Total Path Attribute Length (2 octets)            |
//	+-----------------------------------------------------+
//	|   Path Attributes (variable)                        |
//	+-----------------------------------------------------+
//	|   Network Layer Reachability Information (variable) |
//	+-----------------------------------------------------+
//
// and each route is "a 2-tuple of the form <length, prefix>", where "The
// Length field indicates the length in bits of the IP address prefix" and "the
// Prefix field contains an IP address prefix, followed by enough trailing bits
// to make the end of the field fall on an octet boundary.".
func exabgpLegacyRoutes(body []byte) ([]string, error) {
	if len(body) < 2 {
		return nil, errors.New("UPDATE body is shorter than its Withdrawn Routes Length")
	}
	withdrawnEnd := 2 + int(binary.BigEndian.Uint16(body[0:2]))
	if withdrawnEnd+2 > len(body) {
		return nil, fmt.Errorf("UPDATE Withdrawn Routes Length %d runs past the %d-octet body", withdrawnEnd-2, len(body))
	}
	attributesEnd := withdrawnEnd + 2 + int(binary.BigEndian.Uint16(body[withdrawnEnd:withdrawnEnd+2]))
	if attributesEnd > len(body) {
		return nil, fmt.Errorf("UPDATE Total Path Attribute Length %d runs past the %d-octet body", attributesEnd-withdrawnEnd-2, len(body))
	}
	routes, err := appendExaBGPPrefixes(nil, exabgpRouteWithdraw, body[2:withdrawnEnd])
	if err != nil {
		return nil, err
	}
	return appendExaBGPPrefixes(routes, exabgpRouteAnnounce, body[attributesEnd:])
}

// appendExaBGPPrefixes appends one `<kind>:<prefix>` per route of a route field.
// The field's length bounds the loop: each pass consumes at least one octet.
func appendExaBGPPrefixes(routes []string, kind string, field []byte) ([]string, error) {
	const ipv4PrefixBitsMax = 32
	for len(field) > 0 {
		bits := int(field[0])
		if bits > ipv4PrefixBitsMax {
			return nil, fmt.Errorf("UPDATE %s route has a prefix length of %d bits, longer than an IPv4 address", kind, bits)
		}
		octets := (bits + 7) / 8
		if 1+octets > len(field) {
			return nil, fmt.Errorf("UPDATE %s route of %d bits needs %d octets, %d remain", kind, bits, octets, len(field)-1)
		}
		var address [4]byte
		copy(address[:], field[1:1+octets])
		routes = append(routes, kind+":"+netip.AddrFrom4(address).String()+"/"+strconv.Itoa(bits))
		field = field[1+octets:]
	}
	return routes, nil
}

// notifyExaBGPUntilClosed is the 4.2 mock under send-notification: every
// message the speaker sends after the OPEN exchange is answered with a Cease
// carrying the 4.2 mock's text, and the case passes when the speaker closes the
// connection. A speaker that keeps the session until the message timeout or
// the session's end fails the case, so the pass is the close, never silence.
func notifyExaBGPUntilClosed(wire exabgpWire) error {
	notification := exabgpCeaseWithData([]byte(exabgpNotifyCommunication))
	for {
		_, _, err := wire.read()
		if exabgpSessionClosed(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("the speaker kept the session after the mock's NOTIFICATION: %w", err)
		}
		// A speaker that sent two messages before it read the first Cease has
		// closed by the time the second is answered, so that write fails on
		// the close the case waits for.
		_, err = wire.connection.Write(notification)
		if exabgpSessionClosed(err) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// exabgpSessionClosed answers whether an error is the speaker closing the
// connection: an end of stream on a read, a reset or broken pipe on a write.
func exabgpSessionClosed(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	return errors.Is(err, syscall.EPIPE)
}

// exabgpCeaseWithData builds the 4.2 mock's NOTIFICATION: Cease, subcode 0,
// the data after it.
//
// RFC 4271 Section 4.5: the NOTIFICATION body is Error code (1 octet), Error
// subcode (1 octet), Data (variable).
func exabgpCeaseWithData(data []byte) []byte {
	const (
		errorCodeCease    = 6
		subcodeUnspecific = 0
	)
	body := make([]byte, 0, 2+len(data))
	body = append(body, errorCodeCease, subcodeUnspecific)
	body = append(body, data...)
	return speakerMessage(bgpNotification, body)
}
