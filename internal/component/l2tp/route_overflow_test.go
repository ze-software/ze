// Design: docs/guide/l2tp.md -- bounded subscriber framed-route updates
// Related: route_observer.go -- original Birth records and replay

package l2tp

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/doctor" // Register the daemon's actual recent-events check.
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/report"
)

// TestRouteObserverFramedOverflowPreservesUsefulBirths drives the actual
// callback, first with 64 routes, then with a 65-route new or existing Birth.
// Its exact replay comparison proves that refusing overflow leaves useful
// neighboring routes and the original accepted update effective.
// MUTATION: remove the framed-route count guard before record mutation.
func TestRouteObserverFramedOverflowPreservesUsefulBirths(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new birth", true: "existing birth"}[existing], func(t *testing.T) {
			bus := &recordingBus{}
			observer := newSubscriberRouteObserver(slog.Default(), bus)
			neighbor := fixedRouteFixtureBirth(t, 6401)
			other := fixedRouteFixtureBirth(t, 6402)
			overflow := fixedRouteFixtureBirth(t, 6403)
			observer.OnSessionIPUp(neighbor, 64101, 64102, "neighbor",
				netip.MustParseAddr("192.0.2.61"), routeOverflowFrames(64))
			observer.OnSessionIPUp(other, 64101, 64102, "other",
				netip.MustParseAddr("192.0.2.62"), nil)
			if existing {
				overflow = neighbor
			}
			before := routeOverflowEntries(bus.events())
			require.Len(t, before, 66, "64 framed routes and both original host routes must be useful")
			observer.mu.Lock()
			original := observer.records[neighbor]
			observer.mu.Unlock()
			observer.OnSessionIPUp(overflow, 64101, 64102, "overflow",
				netip.MustParseAddr("192.0.2.63"), routeOverflowFrames(65))
			require.Equal(t, before, routeOverflowEntries(bus.events()), "overflow emitted a partial update")
			injected, withdrawn, active := observer.Stats()
			require.Equal(t, uint64(2), injected, "refused overflow changed injection statistics")
			require.Zero(t, withdrawn)
			require.Equal(t, 2, active)
			observer.mu.Lock()
			retained := observer.records[neighbor]
			observer.mu.Unlock()
			require.Same(t, original, retained, "overflow replaced the original route receipt")
			boundary := len(bus.events())
			observer.reemitAll(6401)
			replay := bus.events()[boundary:]
			for _, event := range replay {
				batch, ok := event.payload.(*redistevents.RouteChangeBatch)
				require.True(t, ok)
				require.Equal(t, uint64(6401), batch.ReplayID)
			}
			require.ElementsMatch(t, before, routeOverflowEntries(replay), "overflow damaged accepted routes or their original source tokens")
		})
	}
}

// TestRouteObserverFramedOverflowReachesRuntimeDoctor drives the same real
// refusal and observes the operator log, immutable error event and registered
// runtime doctor check. A log handler probes both observer locks at reporting.
// MUTATION: omit the coded log, route-owned error event, context or unlock.
func TestRouteObserverFramedOverflowReachesRuntimeDoctor(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	birth := fixedRouteFixtureBirth(t, 6404)
	const tunnelID uint16 = 64103
	const sessionID uint16 = 64104
	var logs bytes.Buffer
	var observer *subscriberRouteObserver
	var probed, mapUnlocked, emitUnlocked bool
	handler := slog.NewJSONHandler(&logs, &slog.HandlerOptions{
		Level: slog.LevelWarn,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key != "code" {
				return attr
			}
			probed = true
			mapUnlocked = observer.mu.TryLock()
			if mapUnlocked {
				observer.mu.Unlock()
			}
			emitUnlocked = observer.emitMu.TryLock()
			if emitUnlocked {
				observer.emitMu.Unlock()
			}
			return attr
		},
	})
	observer = newSubscriberRouteObserver(slog.New(handler), &recordingBus{})
	observer.OnSessionIPUp(birth, tunnelID, sessionID, "overflow",
		netip.MustParseAddr("192.0.2.64"), routeOverflowFrames(65))
	require.NotEmpty(t, logs.Bytes(), "overflow has no covered operator warning")
	var warning map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &warning))
	require.True(t, probed, "warning omitted its doctor code")
	require.True(t, mapUnlocked, "reporting held the route record lock")
	require.True(t, emitUnlocked, "reporting held the route emission lock")
	code, ok := warning["code"].(string)
	require.True(t, ok)
	require.NotNil(t, diagnostic.Lookup(code), "warning code has no diagnostic metadata")
	require.Equal(t, birth.String(), warning["subscriber-id"])
	require.Equal(t, float64(tunnelID), warning["tunnel-id"])
	require.Equal(t, float64(sessionID), warning["session-id"])
	fix, ok := warning["fix"].(string)
	require.True(t, ok)
	require.NotEmpty(t, fix)
	var original *report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Source != SubsystemName {
			continue
		}
		if issue.Detail["subscriber-id"] != birth.String() {
			continue
		}
		copy := issue
		original = &copy
		break
	}
	require.NotNil(t, original, "overflow has no route-owned retained error event")
	require.Equal(t, code, original.Code)
	require.Equal(t, tunnelID, original.Detail["tunnel-id"])
	require.Equal(t, sessionID, original.Detail["session-id"])
	require.Equal(t, warning["error"], original.Detail["cause"])
	require.NotEmpty(t, original.Detail["cause"])
	require.Equal(t, fix, original.Fix)
	require.Contains(t, original.Message, birth.String())
	require.Contains(t, original.Message, "64103")
	require.Contains(t, original.Message, "64104")
	var rows []diagnostic.Diagnostic
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == "recent-events" {
			require.Contains(t, check.Codes, code, "the runtime check does not declare the log code")
			rows = check.Check(diagnostic.DoctorCheckContext{})
		}
	}
	for _, row := range rows {
		if !strings.Contains(row.Message, birth.String()) {
			continue
		}
		require.Equal(t, code, row.Code)
		require.Equal(t, fix, row.Fix)
		require.Contains(t, row.Message, original.Message)
		return
	}
	t.Fatal("the actual runtime recent-events check did not report the original overflow")
}

func routeOverflowFrames(count int) []FramedRoute {
	routes := make([]FramedRoute, count)
	for i := range routes {
		routes[i] = FramedRoute{
			Prefix: netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 64, byte(i), 0}), 24),
			Metric: uint32(i + 1),
		}
	}
	return routes
}

func routeOverflowEntries(events []emittedEvent) []redistevents.RouteChangeEntry {
	var entries []redistevents.RouteChangeEntry
	for _, event := range events {
		if batch, ok := event.payload.(*redistevents.RouteChangeBatch); ok {
			entries = append(entries, batch.Entries...)
		}
	}
	return entries
}
