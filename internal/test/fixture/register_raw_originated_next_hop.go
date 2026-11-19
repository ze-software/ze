// Design: docs/architecture/api/update-syntax.md -- raw originated UPDATE admission.
// Related: plugin_fixture_10.go -- bounded readiness and command-response helpers.
// RFC 4271 Section 5.1.3 -- see rfc/short/rfc4271.md.
package fixture

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/raw-originated-nexthop-peer-own", rawOriginatedNextHop, StopsDaemon)
}

// rawOriginatedNextHop exercises both operator raw forms after initial sync.
// The peer script judges exact announcements, next hops and withdrawals. The
// observer MUST wait for the final successful writes before requesting shutdown.
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
func rawOriginatedNextHop(ctx context.Context, _ []string) error {
	return Observe(ctx, "raw-originated-nh", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := fixture10WaitEOR(ctx, plugin, "under-test", 60); err != nil {
			return err
		}
		before, measured := fixture10PeerCounter(ctx, plugin, "127.0.0.1", "updates-sent")
		if !measured {
			return errors.New("raw next-hop observer could not read the established peer's UPDATE counter")
		}
		for index, fullPacket := range []bool{false, true} {
			octet := byte(90 + index)
			owed := []byte{24, 10, octet, 0}
			for _, input := range []struct {
				withdrawn []byte
				announced []byte
				hop       [4]byte
			}{
				{nil, owed, [4]byte{203, 0, 113, 1}},
				{owed, []byte{24, 10, octet, 1}, [4]byte{127, 0, 0, 1}},
				{nil, []byte{24, 10, octet, 2}, [4]byte{203, 0, 113, 1}},
			} {
				body := rawOriginatedNextHopBody(input.withdrawn, input.announced, input.hop)
				command := rawOriginatedNextHopCommand(body, fullPacket)
				if err := fixture10Call(ctx, plugin, command).requireDone("raw next-hop injection"); err != nil {
					return err
				}
			}
		}
		if !Poll(ctx, 60, fixture10PollDelay, func() bool {
			count, measured := fixture10PeerCounter(ctx, plugin, "127.0.0.1", "updates-sent")
			return measured && count >= before+6
		}) {
			return errors.New("raw next-hop observer did not see both permitted pairs and original withdrawals written")
		}
		return nil
	})
}

// rawOriginatedNextHopBody supplies independent fixed ASN4 fixture bytes.
// RFC 4271 Section 4.3: Withdrawn-Length[0:2], Withdrawn, Attribute-Length[2+w:4+w],
// ORIGIN IGP, AS_SEQUENCE [65000] (four-octet ASN), NEXT_HOP, then IPv4 NLRI.
func rawOriginatedNextHopBody(withdrawn, announced []byte, hop [4]byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8, 0x40, 3, 4, hop[0], hop[1], hop[2], hop[3]}
	body := make([]byte, 4+len(withdrawn)+len(attrs)+len(announced))
	binary.BigEndian.PutUint16(body[:2], uint16(len(withdrawn)))
	copy(body[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(len(attrs)))
	copy(body[offset+2:], attrs)
	copy(body[offset+2+len(attrs):], announced)
	return body
}

// rawOriginatedNextHopCommand invokes the actual raw command grammar.
// RFC 4271 Section 4.1: full packet Marker[0:16], Length[16:18], Type[18], Body[19:].
func rawOriginatedNextHopCommand(body []byte, fullPacket bool) string {
	if !fullPacket {
		return "send bgp 127.0.0.1 raw hex " + hex.EncodeToString(body) + " type update"
	}
	packet := make([]byte, 19+len(body))
	for index := range packet[:16] {
		packet[index] = 0xff
	}
	binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
	packet[18] = 2
	copy(packet[19:], body)
	return fmt.Sprintf("send bgp 127.0.0.1 raw hex %x", packet)
}
