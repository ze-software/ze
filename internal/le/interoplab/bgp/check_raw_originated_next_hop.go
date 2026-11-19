// Design: docs/architecture/testing/interop.md -- raw origination at a native receiver.
// Related: check_rfc.go -- the existing relay assertions run before this helper.
// Upstream: FRR 10.7.1 bgpd/bgp_route.c::bgp_update and bgp_withdraw log
// announcements (including denied input) separately from explicit withdrawals.
// https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_route.c
package bgp

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkRawOriginatedNextHop adds actual operator raw commands to the established
// relay scenario. FRR's own decode, rather than absence from its table, decides
// whether the forbidden announcement reached it. Both permitted controls are
// valid, selected paths with the exact third-party next hop and AS_PATH.
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
func checkRawOriginatedNextHop(ctx context.Context, check *interoplab.CheckContext) error {
	ze := netip.MustParseAddr(networkHostAddress(check.Network, 2))
	recipient := netip.MustParseAddr(networkHostAddress(check.Network, 3))
	thirdParty := netip.MustParseAddr(networkHostAddress(check.Network, 9))
	generation, err := queryFRRSessionGeneration(ctx, check.Lab, ze.String())
	if err != nil {
		return err
	}
	for index, fullPacket := range []bool{false, true} {
		octet := byte(92 + index)
		before := netip.MustParsePrefix(fmt.Sprintf("10.%d.0.0/24", octet))
		subject := netip.MustParsePrefix(fmt.Sprintf("10.%d.1.0/24", octet))
		after := netip.MustParsePrefix(fmt.Sprintf("10.%d.2.0/24", octet))
		owed := []byte{24, 10, octet, 0}
		if err := rawOriginatedNextHopSend(ctx, check.Lab, recipient, fullPacket,
			rawOriginatedNextHopBody(nil, owed, thirdParty)); err != nil {
			return err
		}
		if err := waitNextHopAutoFRR(ctx, check, peerFRR, before, thirdParty, ze, "65001"); err != nil {
			return err
		}
		// The first route is held by this actual receiver before it is withdrawn.
		// Refusing the unrelated announcement must not erase that owed withdrawal.
		if err := rawOriginatedNextHopSend(ctx, check.Lab, recipient, fullPacket,
			rawOriginatedNextHopBody(owed, []byte{24, 10, octet, 1}, recipient)); err != nil {
			return err
		}
		if err := rawOriginatedNextHopSend(ctx, check.Lab, recipient, fullPacket,
			rawOriginatedNextHopBody(nil, []byte{24, 10, octet, 2}, thirdParty)); err != nil {
			return err
		}
		if err := waitNextHopAutoFRR(ctx, check, peerFRR, after, thirdParty, ze, "65001"); err != nil {
			return err
		}
		if err := rawOriginatedNextHopDecode(ctx, check, ze.String(), before.String(), subject.String(), after.String()); err != nil {
			return err
		}
		absent := operation{kind: opFRRRouteAbsent, argument: before.String(), timeout: 30 * time.Second}
		if err := runOperation(ctx, check.Network, check.Lab, &absent); err != nil {
			return err
		}
	}
	afterGeneration, err := queryFRRSessionGeneration(ctx, check.Lab, ze.String())
	if err != nil {
		return err
	}
	if afterGeneration != generation {
		return errors.New("refusing raw peer-own next hops replaced the native session")
	}
	return waitZePeerState(ctx, check.Lab, recipient.String(), 30*time.Second)
}

// rawOriginatedNextHopDecode reads one log snapshot containing both permitted
// announcements and the original withdrawal. The later control bounds absence
// on the same TCP session; an unread or incomplete log cannot pass the check.
func rawOriginatedNextHopDecode(ctx context.Context, check *interoplab.CheckContext, peer, before, subject, after string) error {
	log, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second,
		Description: "FRR decodes the raw controls and original withdrawal",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, peerFRR, []string{cmdCat, frrLogPath}, nil)
	}, func(log string) bool {
		if !frrDecodedAnnouncement(log, peer, before) {
			return false
		}
		if !frrDecodedWithdrawal(log, peer, before) {
			return false
		}
		return frrDecodedAnnouncement(log, peer, after)
	})
	if err != nil {
		return err
	}
	if frrDecodedAnnouncement(log, peer, subject) {
		return fmt.Errorf("FRR decoded raw forbidden announcement %s from %s", subject, peer)
	}
	return nil
}

// rawOriginatedNextHopBody supplies valid independent ASN4 input. RFC 4271
// Section 4.3: Withdrawn-Length[0:2], Withdrawn, Attribute-Length[2+w:4+w],
// ORIGIN IGP, AS_SEQUENCE [65001], NEXT_HOP, then the legacy IPv4 NLRI.
func rawOriginatedNextHopBody(withdrawn, announced []byte, hop netip.Addr) []byte {
	nextHop := hop.As4()
	attrs := make([]byte, 0, 16+len(nextHop))
	attrs = append(attrs, 0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4)
	attrs = append(attrs, nextHop[:]...)
	body := make([]byte, 4+len(withdrawn)+len(attrs)+len(announced))
	binary.BigEndian.PutUint16(body[:2], uint16(len(withdrawn)))
	copy(body[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(body[offset:offset+2], uint16(len(attrs)))
	copy(body[offset+2:], attrs)
	copy(body[offset+2+len(attrs):], announced)
	return body
}

// rawOriginatedNextHopSend uses the real CLI transport already used by the raw
// AS-zero scenario. RFC 4271 Section 4.1: full frame Marker[0:16], Length[16:18],
// Type[18], Body[19:]. A body command instead names type update explicitly.
func rawOriginatedNextHopSend(ctx context.Context, lab interoplab.CheckerLab, recipient netip.Addr, fullPacket bool, body []byte) error {
	data := body
	typeWord := " type update"
	if fullPacket {
		data = make([]byte, 19+len(body))
		for index := range data[:16] {
			data[index] = 0xff
		}
		binary.BigEndian.PutUint16(data[16:18], uint16(len(data)))
		data[18] = 2
		copy(data[19:], body)
		typeWord = ""
	}
	command := zeCommand("send bgp " + recipient.String() + " raw hex " + hex.EncodeToString(data) + typeWord)
	_, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	return err
}
