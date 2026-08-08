package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestAnnouncePlanRefusal_CarriesItsOwnCause covers the backstop's REPORT, which is
// a separate question from whether it refuses.
//
// announceAttrs.add refuses an attribute with no wire form for a contributor that
// did not pre-check. Both rails pre-check today, so nothing reaches it, but when it
// does fire the rail sees only emit's ok=false: it answered that with
// errAnnounceTooLarge and "send fewer prefixes", on an announce whose size was never
// the problem. refusalCause is what separates the two, so the rail can name the next
// hop instead (ai/rules/cli.md: an operator-facing message must be true).
//
// It is driven at announceAttrs rather than at a rail because no rail can reach the
// backstop today: buildBatchAnnounceUpdate (reactor_api_batch.go) and
// buildRIBRouteUpdate (peer_rib_routes.go) both validate before they contribute, and
// neither ever contributes a stored MP_REACH. The plan IS the entry point of the
// behavior under test.
//
// VALIDATES: a refusal raised by the next-hop backstop carries
// attribute.ErrUnencodableNextHop, and a refusal raised by the region bound carries
// no cause at all, so a rail cannot report the two as the same thing.
// PREVENTS: a next-hop refusal reaching the operator as an oversize announce.
func TestAnnouncePlanRefusal_CarriesItsOwnCause(t *testing.T) {
	t.Run("no wire form names the next hop", func(t *testing.T) {
		var plan announceAttrs
		plan.begin()
		defer plan.release()

		// 2001:db8::/32 as MP_REACH NLRI, with a next hop that has no wire form.
		plan.add(attribute.NewMPReachNLRI(attribute.AFIIPv6, attribute.SAFIUnicast,
			[]netip.Addr{{}}, []byte{32, 0x20, 0x01, 0x0d, 0xb8}), nil)

		n, ok := plan.emit(nil, make([]byte, message.MaxMsgLen))
		require.False(t, ok, "an attribute with no wire form must not be emitted")
		assert.Equal(t, 0, n, "a refused emit must not report a length")
		require.ErrorIs(t, plan.refusalCause(), attribute.ErrUnencodableNextHop,
			"the rail must be able to tell this refusal from an oversize one")
	})

	t.Run("an oversize refusal carries no cause", func(t *testing.T) {
		// The discriminating half. A cause that were always non-nil would make every
		// refusal look like a next-hop failure, and the assertion above vacuous.
		var plan announceAttrs
		plan.begin()
		defer plan.release()

		lp := attribute.LocalPref(100)
		plan.add(lp, nil)

		n, ok := plan.emit(nil, make([]byte, attribute.AttrWireLen(lp)-1))
		require.False(t, ok, "an attribute that does not fit the region must be refused")
		assert.Equal(t, 0, n)
		assert.NoError(t, plan.refusalCause(),
			"an oversize refusal must leave the rail on its own oversize report")
	})

	t.Run("the first reason wins and so does its cause", func(t *testing.T) {
		// fail records once. A later refusal must not overwrite the cause the rail
		// will report, which is why the cause travels through fail rather than being
		// assigned beside the call.
		var plan announceAttrs
		plan.begin()
		defer plan.release()

		plan.add(attribute.NewMPReachNLRI(attribute.AFIIPv6, attribute.SAFIUnicast,
			[]netip.Addr{{}}, []byte{32, 0x20, 0x01, 0x0d, 0xb8}), nil)
		plan.add(attribute.LocalPref(100), nil)
		plan.add(attribute.LocalPref(200), nil) // duplicate type code: carries no cause

		require.ErrorIs(t, plan.refusalCause(), attribute.ErrUnencodableNextHop,
			"a later causeless refusal must not erase the first cause")
	})
}
