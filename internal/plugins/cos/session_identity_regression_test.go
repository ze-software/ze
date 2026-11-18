// Design: docs/architecture/traffic/cos-dynamic.md -- independent subscriber policy

//go:build ze_l2tp

package cos

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestCoSEqualCoordinatesKeepIndependentInterfaces drives the existing event
// boundary with two admitted sources using equal wire labels and different
// PPP/access interfaces. Both installed policies must remain independently
// owned, and the first source's departure must leave the second in place.
// Two explicit immutable Birth inputs keep its labels, interfaces and assertions
// fixed without claiming that this fixture drives native session allocation.
func TestCoSEqualCoordinatesKeepIndependentInterfaces(t *testing.T) {
	bus, backend := setupTestHandler(t)
	const tunnelID, sessionID = uint16(64511), uint16(64512)
	const firstInterface = "eth-cos-identity-a"
	const secondInterface = "eth-cos-identity-b"
	firstBirth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	secondBirth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	require.NotEqual(t, firstBirth, secondBirth)
	firstID, secondID := firstBirth.String(), secondBirth.String()
	l2tp.StoreSessionMetadata(firstID, &l2tp.AuthMetadata{CoSProfile: "residential"})
	l2tp.StoreSessionMetadata(secondID, &l2tp.AuthMetadata{CoSProfile: "residential"})
	t.Cleanup(func() {
		l2tp.ClearSessionMetadata(firstID)
		l2tp.ClearSessionMetadata(secondID)
	})
	t.Cleanup(func() {
		sessionStore.Range(func(key, value any) bool {
			state, valid := value.(sessionCoSState)
			if !valid {
				return true
			}
			if state.accessInterface == firstInterface {
				sessionStore.Delete(key)
			}
			if state.accessInterface == secondInterface {
				sessionStore.Delete(key)
			}
			return true
		})
	})
	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		Birth: firstBirth, SubscriberID: firstID,
		TunnelID: tunnelID, SessionID: sessionID, Interface: "ppp-cos-a", AccessInterface: firstInterface,
	})
	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		Birth: secondBirth, SubscriberID: secondID,
		TunnelID: tunnelID, SessionID: sessionID, Interface: "ppp-cos-b", AccessInterface: secondInterface,
	})
	require.Equal(t, 2, backend.callCount(), "both sources must install their permitted profile")
	installed := func() map[string]string {
		states := make(map[string]string)
		sessionStore.Range(func(_, value any) bool {
			state, valid := value.(sessionCoSState)
			if !valid {
				return true
			}
			if state.accessInterface == firstInterface {
				states[firstInterface] = state.profileName
			}
			if state.accessInterface == secondInterface {
				states[secondInterface] = state.profileName
			}
			return true
		})
		return states
	}
	require.Equal(t, map[string]string{
		firstInterface: "residential", secondInterface: "residential",
	}, installed(), "equal wire labels must not overwrite either subscriber's installed policy")
	emitSessionDown(t, bus, &l2tpevents.SessionDownPayload{Birth: firstBirth, SubscriberID: firstID, TunnelID: tunnelID, SessionID: sessionID})
	require.Equal(t, 3, backend.callCount(), "only the first departure should add a revert")
	last, available := backend.lastCall()
	require.True(t, available)
	require.Equal(t, firstInterface, last.ifaceName, "the first departure must use its own captured access interface")
	require.Equal(t, map[string]string{secondInterface: "residential"}, installed())
}
