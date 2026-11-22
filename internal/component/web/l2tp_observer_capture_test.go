// Design: docs/architecture/web-workbench-pages.md -- captured L2TP detail history
//go:build ze_l2tp

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// A formatting-only capture carries copied read evidence and refuses every
// native or mutation operation. It supplies no fabricated native authority.
type cqmWebCapture struct {
	snapshot                    l2tp.SessionSnapshot
	events                      []l2tp.ObserverEvent
	snapshotErr, historyErr     error
	snapshotReads, historyReads int
}

func (capture *cqmWebCapture) Birth() subscriberid.L2TPBirth { return capture.snapshot.Birth }
func (capture *cqmWebCapture) Listener() netip.AddrPort      { return capture.snapshot.Listener }
func (capture *cqmWebCapture) Snapshot() (l2tp.SessionSnapshot, error) {
	capture.snapshotReads++
	return capture.snapshot, capture.snapshotErr
}
func (capture *cqmWebCapture) SessionEvents() ([]l2tp.ObserverEvent, error) {
	capture.historyReads++
	return append([]l2tp.ObserverEvent(nil), capture.events...), capture.historyErr
}
func (*cqmWebCapture) FSMHistory() ([]l2tp.FSMTransition, error) { return nil, l2tp.ErrSessionNotFound }
func (*cqmWebCapture) Counters() (ppp.SessionCounters, error) {
	return ppp.SessionCounters{}, l2tp.ErrSessionNotFound
}
func (*cqmWebCapture) RecordDisconnect(string, string, uint32) error { return l2tp.ErrSessionNotFound }
func (*cqmWebCapture) Teardown(context.Context, l2tpevents.TerminateCause, string) error {
	return l2tp.ErrSessionNotFound
}

type cqmWebService struct {
	*fakeL2TPService
	captured    l2tp.CapturedSession
	captureErr  error
	selectors   []l2tp.SessionSelector
	unsafeReads int
}

func (service *cqmWebService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service.selectors = append(service.selectors, selector)
	return service.captured, service.captureErr
}
func (service *cqmWebService) LookupSession(uint16) (l2tp.SessionSnapshot, bool) {
	service.unsafeReads++
	return l2tp.SessionSnapshot{}, false
}
func (service *cqmWebService) SessionEvents(subscriberid.L2TPBirth) []l2tp.ObserverEvent {
	service.unsafeReads++
	return nil
}

// MUTATION: a lookup/relookup detail handler cannot return this one capture's
// snapshot and useful history and trips the explicit unsafe-read counter.
func TestL2TPDetailReadsOneOriginalCaptureWithFullIdentity(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	require.NoError(t, err)
	generation, err := l2tpevents.NewObservationGeneration()
	require.NoError(t, err)
	capture := &cqmWebCapture{
		snapshot: l2tp.SessionSnapshot{Birth: birth, SubscriberID: birth.String(), Generation: generation,
			Listener: netip.MustParseAddrPort("127.0.0.1:1701"), LocalSID: 7, TunnelLocalTID: 17, Username: "alice", State: "established"},
		events: []l2tp.ObserverEvent{{Birth: birth, Timestamp: time.Unix(1000, 0), Type: l2tp.ObserverEventEchoRTT, TunnelID: 17, SessionID: 7, RTT: 13 * time.Millisecond}},
	}
	service := &cqmWebService{fakeL2TPService: &fakeL2TPService{}, captured: capture}
	publishFakeL2TP(t, service)
	request := httptest.NewRequestWithContext(t.Context(), "GET", "/l2tp/7?format=json", http.NoBody)
	recorder := httptest.NewRecorder()
	(&l2TPHandlers{}).handleL2TPDetail()(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []l2tp.SessionSelector{{SID: 7}}, service.selectors)
	require.Zero(t, service.unsafeReads)
	require.Equal(t, 1, capture.snapshotReads)
	require.Equal(t, 1, capture.historyReads)
	var data map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &data))
	snapshot, ok := data["Session"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, birth.String(), snapshot["subscriber-id"])
	require.Equal(t, "127.0.0.1:1701", snapshot["listener"])
	require.NotContains(t, snapshot, "Generation")
	require.NotContains(t, snapshot, "Birth")
	events, ok := data["Events"].([]any)
	require.True(t, ok)
	require.Len(t, events, 1)
	event, ok := events[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, birth.String(), event["subscriber-id"])
	require.Equal(t, float64(17), event["TunnelID"])
	require.Equal(t, float64(13*time.Millisecond), event["RTT"])
	encoded, err := json.Marshal(l2tp.SessionSnapshot{})
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "subscriber-id", "intentional empty formatting DTO remains serializable")
}

// MUTATION: choosing one ambiguous SID returns success; reading history after
// a stale snapshot or recapturing it exposes data beyond the original capture.
func TestL2TPDetailRefusesAmbiguousAndEndedOriginals(t *testing.T) {
	for _, refusal := range []struct {
		name                                string
		captureErr, snapshotErr, historyErr error
		status, snapshotReads, historyReads int
	}{
		{name: "ambiguous", captureErr: l2tp.ErrSessionAmbiguous, status: http.StatusConflict},
		{name: "absent", captureErr: l2tp.ErrSessionNotFound, status: http.StatusNotFound},
		{name: "ended snapshot", snapshotErr: l2tp.ErrSessionNotFound, status: http.StatusNotFound, snapshotReads: 1},
		{name: "ended history", historyErr: l2tp.ErrSessionNotFound, status: http.StatusNotFound, snapshotReads: 1, historyReads: 1},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			capture := &cqmWebCapture{snapshotErr: refusal.snapshotErr, historyErr: refusal.historyErr}
			service := &cqmWebService{fakeL2TPService: &fakeL2TPService{}, captured: capture, captureErr: refusal.captureErr}
			publishFakeL2TP(t, service)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), "GET", "/l2tp/7?format=json", http.NoBody)
			(&l2TPHandlers{}).handleL2TPDetail()(recorder, request)
			require.Equal(t, refusal.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), "Open /l2tp")
			require.Len(t, service.selectors, 1)
			require.Zero(t, service.unsafeReads)
			require.Equal(t, refusal.snapshotReads, capture.snapshotReads)
			require.Equal(t, refusal.historyReads, capture.historyReads)
		})
	}
}

// This existing fixture owns no original PPP terminal measurement.
func (*cqmWebCapture) TerminalCounters(context.Context) (l2tpevents.TrafficSample, error) {
	return l2tpevents.TrafficSample{}, l2tp.ErrSessionNotFound
}
