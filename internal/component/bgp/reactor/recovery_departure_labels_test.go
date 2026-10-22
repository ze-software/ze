// Design: docs/architecture/plugin/rib-storage-design.md -- causal recovery admission.
package reactor

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
)

// departureMultiLabelRD is the Route Distinguisher of the VPN case.
var departureMultiLabelRD = []byte{0, 0, 0xfd, 0xe8, 0, 0, 0, 1}

// departureMultiLabelNLRI is one announced route of fam carrying a stack of
// three labels, the bottom one with the S bit, then the RD for SAFI 128, then
// 192.0.2.0/24.
//
// RFC 8277 Section 2.2: "If the S bit is set to one in the Label field of the
// final label in the NLRI, the receiving speaker knows that it has reached the
// last label of the stack."
//
//	[Length:1][Label 1:3][Label 2:3][Label 3, S set:3][RD:8, SAFI 128 only][Prefix:3]
func departureMultiLabelNLRI(fam family.Family) []byte {
	raw := []byte{0, 0x00, 0x06, 0x40, 0x00, 0x06, 0x50, 0x00, 0x06, 0x61}
	if fam.SAFI == family.SAFIVPN {
		raw = append(raw, departureMultiLabelRD...)
	}
	raw = append(raw, 192, 0, 2)
	raw[0] = byte((len(raw) - 1) * 8)
	return raw
}

// departureWithdrawalNLRI is the withdrawal of that route: one Compatibility
// field 0x800000 in place of the whole stack, the length octet counting it.
//
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
//
//	[Length:1][Compatibility 0x800000:3][RD:8, SAFI 128 only][Prefix:3]
func departureWithdrawalNLRI(fam family.Family) []byte {
	raw := []byte{0, 0x80, 0x00, 0x00}
	if fam.SAFI == family.SAFIVPN {
		raw = append(raw, departureMultiLabelRD...)
	}
	raw = append(raw, 192, 0, 2)
	raw[0] = byte((len(raw) - 1) * 8)
	return raw
}

// departureMPUnreach answers the MP_UNREACH_NLRI value of one written UPDATE's
// path attributes, or nil when it holds none.
func departureMPUnreach(t *testing.T, attrs []byte) []byte {
	t.Helper()
	for len(attrs) != 0 {
		require.GreaterOrEqual(t, len(attrs), 3)
		flags, code := attrs[0], attrs[1]
		header, length := 3, int(attrs[2])
		if flags&byte(attribute.FlagExtLength) != 0 {
			require.GreaterOrEqual(t, len(attrs), 4)
			header, length = 4, int(binary.BigEndian.Uint16(attrs[2:4]))
		}
		require.GreaterOrEqual(t, len(attrs), header+length)
		if code == byte(attribute.AttrMPUnreachNLRI) {
			return attrs[header : header+length]
		}
		attrs = attrs[header+length:]
	}
	return nil
}

// TestRecoveryDepartureWithdrawsMultiLabelRouteWithCompatibility departs a
// source whose labelled route (VPN, SAFI 128, and labelled unicast, SAFI 4)
// carries three labels, through bgp-rr's own peer-down command form
// (sendPeerDown): native announcement hex, with and without the source's
// ADD-PATH framing, parsed by the real hex parser. The destination negotiated
// no ADD-PATH and the RIB names no survivor, so it is sent the withdrawal. That
// withdrawal MUST carry one Compatibility field 0x800000 and the route's RD and
// prefix, never the announcement's stack: a stack copied behind the
// Compatibility field is read by the receiver as RD and prefix octets, so it
// withdraws a route it never held and keeps this one.
// MUTATION: owe the departure's announcement bytes unchanged (oweDeparture).
func TestRecoveryDepartureWithdrawsMultiLabelRouteWithCompatibility(t *testing.T) {
	families := []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
	}
	for _, fam := range families {
		for _, addPath := range []bool{false, true} {
			t.Run(fam.String()+"/source-addpath="+strconv.FormatBool(addPath), func(t *testing.T) {
				r, _, destination, conn, ctxID := newSyncOrderRail(t)
				r.clock = clock.RealClock{}
				destination.sendingInitialRoutes.Store(0)
				destination.session.setSendCtxID(ctxID)
				destination.negotiated.Load().families[fam] = true
				destination.refreshForwardFacts()
				announced := departureMultiLabelNLRI(fam)
				failed := recoveryAdvertiseFailed(t, r, destination, ownershipRailBody(fam, false, announced, 0))
				setup := len(conn.written())
				recoveryOwnerStart(t, r, destination)
				owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
					return recoveryWithdrawAnswer(request), nil
				})
				t.Cleanup(owner.Close)

				// bgp-rr's sendPeerDown: "update hex nlri <family> [addpath] del <hex>".
				routeHex := hex.EncodeToString(announced)
				command := "nlri " + fam.String()
				if addPath {
					command += " addpath"
					routeHex = "00000007" + routeHex
				}
				command += " del " + routeHex
				parsed, err := update.ParseUpdateWire(strings.Fields(command), plugin.WireEncodingHex)
				require.NoError(t, err)
				require.Len(t, parsed.Groups, 1)
				batch := bgptypes.NLRIBatch{Family: fam, NLRIs: parsed.Groups[0].Withdraw,
					RecoverySource: failed.Settings().Address}
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				require.NoError(t, (&reactorAPIAdapter{r: r}).recoverNLRIBatch(ctx, batch,
					[]*Peer{destination}, plugin.ProcessSender("bgp-rr")))

				updates := recoveryWrittenUpdates(t, conn.written()[setup:])
				require.Len(t, updates, 1, "the departure withdraws the route once")
				value := departureMPUnreach(t, updates[0].PathAttributes)
				require.NotNil(t, value, "a labelled withdrawal travels in MP_UNREACH_NLRI")
				// RFC 4760 Section 4: [AFI:2][SAFI:1][Withdrawn Routes].
				require.Equal(t, []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}, value[:3])
				require.Equal(t, departureWithdrawalNLRI(fam), value[3:],
					"the withdrawal carries the Compatibility field 0x800000 in place of the three-label stack")
			})
		}
	}
}
