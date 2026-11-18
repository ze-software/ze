// Design: docs/guide/l2tp.md -- original traffic custody and counter availability.
package cmd

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// Only read measurements are modeled. The embedded formatting capture refuses
// native ownership and mutation; these counters authorize neither.
type trafficCounterCapture struct {
	cqmCommandCapture
	counters     ppp.SessionCounters
	counterReads int
}

func (capture *trafficCounterCapture) Counters() (ppp.SessionCounters, error) {
	capture.counterReads++
	return capture.counters, nil
}

// MUTATION: omit a diagnostic key, fabricate zero for Unknown, or re-read a
// reusable interface name. Every old counter remains useful when measured.
func TestSessionTrafficPreservesCounterAvailability(t *testing.T) {
	for _, input := range []struct {
		name     string
		readings [4]ppp.CounterReading
		want     [4]any
	}{
		{name: "measured", readings: [4]ppp.CounterReading{ppp.MeasuredCounter(0), ppp.MeasuredCounter(12), ppp.MeasuredCounter(13), ppp.MeasuredCounter(14)}, want: [4]any{float64(0), float64(12), float64(13), float64(14)}},
		{name: "unknown"},
		{name: "mixed", readings: [4]ppp.CounterReading{ppp.MeasuredCounter(0), {}, ppp.MeasuredCounter(13), {}}, want: [4]any{float64(0), nil, float64(13), nil}},
	} {
		t.Run(input.name, func(t *testing.T) {
			original := cqmCommandCapture{snapshot: l2tp.SessionSnapshot{LocalSID: 7, PppInterface: "original-ppp"}}
			capture := &trafficCounterCapture{cqmCommandCapture: original, counters: ppp.SessionCounters{
				RxBytes: 1101, TxBytes: 2202, RxPackets: 33, TxPackets: 44,
				RxErrors: input.readings[0], TxErrors: input.readings[1], RxDropped: input.readings[2], TxDropped: input.readings[3],
			}}
			service := &cqmCommandService{fakeService: &fakeService{}, captured: capture}
			publishFake(t, service)
			response, err := handleSessionTraffic(nil, commandtest.Args("7"))
			require.NoError(t, err)
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			require.Equal(t, 1, capture.counterReads)
			require.Zero(t, service.unsafeReads)
			var row map[string]any
			require.NoError(t, json.Unmarshal([]byte(responseString(t, response)), &row))
			for index, key := range []string{"rx-errors", "tx-errors", "rx-dropped", "tx-dropped"} {
				value, exists := row[key]
				require.True(t, exists, "existing diagnostic key %s must remain present", key)
				require.Equal(t, input.want[index], value, key)
			}
			require.Equal(t, float64(1101), row["rx-bytes"])
			require.Equal(t, float64(2202), row["tx-bytes"])
			require.Equal(t, float64(33), row["rx-packets"])
			require.Equal(t, float64(44), row["tx-packets"])
		})
	}
}

type trafficInventoryService struct {
	*fakeService
	captures  map[subscriberid.L2TPBirth]*trafficCounterCapture
	selectors []l2tp.SessionSelector
}

func (service *trafficInventoryService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service.selectors = append(service.selectors, selector)
	captured := service.captures[selector.Birth]
	if captured == nil {
		return nil, l2tp.ErrSessionNotFound
	}
	return captured, nil
}

// MUTATION: aggregate traffic reselects by a reusable interface/SID instead of
// carrying each inventory row's original Birth through the native counter read.
// A stale original keeps its own error row while the useful survivor is read.
func TestSessionTrafficInventoryKeepsOriginalCapturesAndSurvivors(t *testing.T) {
	service := &trafficInventoryService{fakeService: &fakeService{}, captures: make(map[subscriberid.L2TPBirth]*trafficCounterCapture)}
	var rows []l2tp.SessionSnapshot
	for index, reference := range []string{"l2tp-00000000000000000000000000001917", "l2tp-00000000000000000000000000001918"} {
		birth, err := subscriberid.ParseL2TPBirth(reference)
		require.NoError(t, err)
		endpoint := []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:1917"), netip.MustParseAddrPort("127.0.0.1:1918")}[index]
		row := l2tp.SessionSnapshot{Birth: birth, SubscriberID: reference, LocalSID: 7, TunnelLocalTID: 17, Listener: endpoint, PppInterface: "reused-label"}
		rows = append(rows, row)
		if index == 0 {
			original := cqmCommandCapture{snapshot: row}
			service.captures[birth] = &trafficCounterCapture{cqmCommandCapture: original, counters: ppp.SessionCounters{RxBytes: 1917, TxBytes: 2917}}
		}
	}
	service.snapshot = l2tp.Snapshot{SessionCount: 2, Tunnels: []l2tp.TunnelSnapshot{{LocalTID: 17, Sessions: rows}}}
	publishFake(t, service)
	response, err := handleSessionTraffic(nil, commandtest.Args("all"))
	require.NoError(t, err)
	require.Equal(t, []l2tp.SessionSelector{
		{SID: 7, TID: 17, Listener: rows[0].Listener, Birth: rows[0].Birth},
		{SID: 7, TID: 17, Listener: rows[1].Listener, Birth: rows[1].Birth},
	}, service.selectors)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	var answer map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, response)), &answer))
	result, ok := answer["sessions"].([]any)
	require.True(t, ok, "traffic inventory must be a JSON array")
	require.Len(t, result, 2)
	useful, ok := result[0].(map[string]any)
	require.True(t, ok, "useful traffic row must be a JSON object")
	require.Equal(t, rows[0].SubscriberID, useful["subscriber-id"])
	require.Equal(t, float64(1917), useful["rx-bytes"])
	require.Equal(t, float64(2917), useful["tx-bytes"])
	stale, ok := result[1].(map[string]any)
	require.True(t, ok, "stale traffic row must be a JSON object")
	require.Equal(t, rows[1].SubscriberID, stale["subscriber-id"])
	require.Equal(t, rows[1].Listener.String(), stale["listener"])
	require.NotEmpty(t, stale["error"])
	require.NotContains(t, stale, "rx-bytes")
	require.Equal(t, 1, service.captures[rows[0].Birth].counterReads)
}
