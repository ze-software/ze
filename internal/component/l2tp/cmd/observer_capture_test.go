// Design: docs/guide/command-reference.md -- captured original observer detail

package cmd

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	l2tp "github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// This formatting fixture has read-only evidence only. Every operation needing
// an admitted native owner explicitly refuses; it is never a successful native
// authority made from a nil embedded interface.
type cqmCommandCapture struct {
	snapshot   l2tp.SessionSnapshot
	events     []l2tp.ObserverEvent
	historyErr error
	reads      int
}

func (capture *cqmCommandCapture) Birth() subscriberid.L2TPBirth { return capture.snapshot.Birth }
func (capture *cqmCommandCapture) Listener() netip.AddrPort      { return capture.snapshot.Listener }
func (capture *cqmCommandCapture) Snapshot() (l2tp.SessionSnapshot, error) {
	return capture.snapshot, nil
}
func (capture *cqmCommandCapture) SessionEvents() ([]l2tp.ObserverEvent, error) {
	capture.reads++
	return append([]l2tp.ObserverEvent(nil), capture.events...), capture.historyErr
}
func (*cqmCommandCapture) FSMHistory() ([]l2tp.FSMTransition, error) {
	return nil, l2tp.ErrSessionNotFound
}
func (*cqmCommandCapture) Counters() (ppp.SessionCounters, error) {
	return ppp.SessionCounters{}, l2tp.ErrSessionNotFound
}
func (*cqmCommandCapture) RecordDisconnect(string, string, uint32) error {
	return l2tp.ErrSessionNotFound
}
func (*cqmCommandCapture) Teardown(context.Context, l2tpevents.TerminateCause, string) error {
	return l2tp.ErrSessionNotFound
}

type cqmCommandService struct {
	*fakeService
	captured    l2tp.CapturedSession
	captureErr  error
	selectors   []l2tp.SessionSelector
	unsafeReads int
	summaries   []l2tp.SessionSummary
}

func (service *cqmCommandService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service.selectors = append(service.selectors, selector)
	return service.captured, service.captureErr
}
func (service *cqmCommandService) LookupSession(uint16) (l2tp.SessionSnapshot, bool) {
	service.unsafeReads++
	return l2tp.SessionSnapshot{}, false
}
func (service *cqmCommandService) SessionEvents(subscriberid.L2TPBirth) []l2tp.ObserverEvent {
	service.unsafeReads++
	return nil
}
func (service *cqmCommandService) SessionSummaries() []l2tp.SessionSummary { return service.summaries }

// MUTATION: returning to LookupSession/Service.SessionEvents splits detail
// selection across owners and loses the captured original's useful history.
func TestObserverDetailUsesOneCapturedOriginalHistory(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	capture := &cqmCommandCapture{
		snapshot: l2tp.SessionSnapshot{Birth: birth, SubscriberID: birth.String(), LocalSID: 7, TunnelLocalTID: 17},
		events:   []l2tp.ObserverEvent{{Birth: birth, Timestamp: time.Unix(1000, 0), Type: l2tp.ObserverEventEchoRTT, TunnelID: 17, SessionID: 7, RTT: 13 * time.Millisecond}},
	}
	service := &cqmCommandService{fakeService: &fakeService{}, captured: capture}
	publishFake(t, service)
	response, err := handleObserver(nil, commandtest.Args("7"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status)
	require.Equal(t, []l2tp.SessionSelector{{SID: 7}}, service.selectors)
	require.Equal(t, 1, capture.reads)
	require.Zero(t, service.unsafeReads)
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, response)), &data))
	events, ok := data["events"].([]any)
	require.True(t, ok, "observer events must be a JSON array")
	require.Len(t, events, 1)
	event, ok := events[0].(map[string]any)
	require.True(t, ok, "observer event must be a JSON object")
	require.Equal(t, birth.String(), event["subscriber-id"])
	require.Equal(t, float64(17), event["tunnel-id"])
	require.Equal(t, 13.0, event["rtt-ms"])
}

// MUTATION: treating an ambiguous readable SID as a choice exposes one
// subscriber's history instead of an actionable explicit refusal.
func TestObserverDetailRefusesAmbiguousOrStaleCapture(t *testing.T) {
	for _, refusal := range []struct {
		name                   string
		captureErr, historyErr error
	}{
		{name: "ambiguous", captureErr: l2tp.ErrSessionAmbiguous},
		{name: "stale", historyErr: l2tp.ErrSessionNotFound},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			capture := &cqmCommandCapture{historyErr: refusal.historyErr}
			service := &cqmCommandService{fakeService: &fakeService{}, captured: capture, captureErr: refusal.captureErr}
			publishFake(t, service)
			response, err := handleObserver(nil, commandtest.Args("7"))
			require.NoError(t, err)
			require.Equal(t, plugin.StatusError, response.Status)
			require.Contains(t, response.Error, "show l2tp sessions")
			require.Len(t, service.selectors, 1)
			require.Zero(t, service.unsafeReads)
			if refusal.captureErr != nil {
				require.Zero(t, capture.reads)
			} else {
				require.Equal(t, 1, capture.reads)
			}
		})
	}
}

// MUTATION: conflating aggregate summary with detail capture changes bare/all
// behavior or omits the full identity and tunnel coordinate from summary JSON.
func TestObserverSummaryKeepsBareAndAllWithExplicitIdentity(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	service := &cqmCommandService{fakeService: &fakeService{}, summaries: []l2tp.SessionSummary{{Birth: birth, TunnelID: 17, SessionID: 7, EventCount: 1, LastEventType: "session-up", LastEventTime: time.Unix(1000, 0)}}}
	publishFake(t, service)
	for _, arguments := range [][]string{nil, {"all"}} {
		response, callErr := handleObserver(nil, commandtest.Args(arguments...))
		require.NoError(t, callErr)
		require.Equal(t, plugin.StatusDone, response.Status)
		encoded := responseString(t, response)
		require.Contains(t, encoded, `"subscriber-id":"`+birth.String()+`"`)
		require.Contains(t, encoded, `"tunnel-id":17`)
	}
	require.Empty(t, service.selectors)
	require.Zero(t, service.unsafeReads)
}
