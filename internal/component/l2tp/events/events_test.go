package events

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
)

// VALIDATES: AC-1 prerequisite -- typed handle exists with the expected
// namespace "l2tp" and event type "route-change".
func TestRouteChangeHandle_Registered(t *testing.T) {
	require.Equal(t, "l2tp", RouteChange.Namespace())
	require.Equal(t, redistevents.EventType, RouteChange.EventType())
}

// VALIDATES: ProtocolID is non-zero (RegisterProtocol succeeded).
func TestProtocolID_NonZero(t *testing.T) {
	require.NotEqual(t, redistevents.ProtocolUnspecified, ProtocolID,
		"ProtocolID must be allocated (non-zero)")
}

// VALIDATES: L2TP is listed as a producer by redistevents.Producers().
func TestL2TP_RegisteredAsProducer(t *testing.T) {
	prods := redistevents.Producers()
	require.True(t, slices.Contains(prods, ProtocolID),
		"l2tp ProtocolID must appear in Producers()")
}

// VALIDATES: AC-1 prerequisite -- SessionUp typed handle exists.
func TestSessionUpHandle_Registered(t *testing.T) {
	require.Equal(t, "l2tp", SessionUp.Namespace())
	require.Equal(t, SessionUpEvent, SessionUp.EventType())
}

// VALIDATES: SessionRateChange typed handle exists.
func TestSessionRateChangeHandle_Registered(t *testing.T) {
	require.Equal(t, "l2tp", SessionRateChange.Namespace())
	require.Equal(t, SessionRateChangeEvent, SessionRateChange.EventType())
}
