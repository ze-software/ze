// Design: docs/guide/l2tp.md -- one qualified original for every session read.
// Related: session_capture.go -- immutable original snapshot/history/counter capability.
package cmd

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// This fixture supplies only explicit read evidence. The embedded capture's
// mutation methods refuse, so useful reads grant no native mutation authority.
type sessionReadCapture struct {
	cqmCommandCapture
	snapshotErr   error
	fsmErr        error
	counterErr    error
	snapshotReads int
	fsmReads      int
	counterReads  int
}

func (capture *sessionReadCapture) Snapshot() (l2tp.SessionSnapshot, error) {
	capture.snapshotReads++
	return capture.snapshot, capture.snapshotErr
}

func (capture *sessionReadCapture) FSMHistory() ([]l2tp.FSMTransition, error) {
	capture.fsmReads++
	return []l2tp.FSMTransition{{Timestamp: time.Unix(1000, 0), From: "negotiating", To: "established", Trigger: "original"}}, capture.fsmErr
}

func (capture *sessionReadCapture) Counters() (ppp.SessionCounters, error) {
	capture.counterReads++
	return ppp.SessionCounters{RxBytes: 1101, TxBytes: 2202, RxPackets: 33, TxPackets: 44}, capture.counterErr
}

type sessionReadService struct {
	*fakeService
	captured    *sessionReadCapture
	captureErr  error
	selectors   []l2tp.SessionSelector
	unsafeReads int
}

func (service *sessionReadService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service.selectors = append(service.selectors, selector)
	return service.captured, service.captureErr
}

func (service *sessionReadService) LookupSession(uint16) (l2tp.SessionSnapshot, bool) {
	service.unsafeReads++
	return l2tp.SessionSnapshot{LocalSID: 7, TunnelLocalTID: 99, Username: "wrong-original"}, true
}

func (service *sessionReadService) SessionFSMHistory(uint16) []l2tp.FSMTransition {
	service.unsafeReads++
	return []l2tp.FSMTransition{{From: "wrong-original", To: "ended"}}
}

type sessionReadCase struct {
	name    string
	handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
}

func sessionReadCases() []sessionReadCase {
	return []sessionReadCase{
		{name: "detail", handler: handleSession},
		{name: "observer", handler: handleObserver},
		{name: "fsm", handler: handleSessionHistory},
		{name: "traffic", handler: handleSessionTraffic},
	}
}

func newSessionReadService(t *testing.T) *sessionReadService {
	t.Helper()
	birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000001917")
	require.NoError(t, err)
	endpoint := netip.MustParseAddrPort("127.0.0.1:1917")
	capture := &sessionReadCapture{cqmCommandCapture: cqmCommandCapture{
		snapshot: l2tp.SessionSnapshot{Birth: birth, SubscriberID: birth.String(), LocalSID: 7, TunnelLocalTID: 17, Listener: endpoint, Username: "original", PppInterface: "original-ppp"},
		events:   []l2tp.ObserverEvent{{Birth: birth, Timestamp: time.Unix(1000, 0), Type: l2tp.ObserverEventEchoRTT, TunnelID: 17, SessionID: 7, RTT: 13 * time.Millisecond}},
	}}
	return &sessionReadService{fakeService: &fakeService{listeners: []l2tp.ListenerSnapshot{{Name: "edge", Addr: endpoint}}}, captured: capture}
}

// MUTATION: select any reader through a SID-only service method, or discard
// tunnel/listener qualifiers before CaptureSession. Useful output must belong
// to that same captured original, including traffic obtained without name lookup.
func TestSingleSessionReadersUseQualifiedOriginalCapture(t *testing.T) {
	for _, read := range sessionReadCases() {
		t.Run(read.name, func(t *testing.T) {
			service := newSessionReadService(t)
			publishFake(t, service)
			response, err := read.handler(nil, commandtest.Args("7", "tunnel-id", "17", "listener", "edge"))
			require.NoError(t, err)
			require.Equal(t, []l2tp.SessionSelector{{SID: 7, TID: 17, Listener: service.captured.snapshot.Listener}}, service.selectors, "every reader must select the qualified original exactly once")
			require.Zero(t, service.unsafeReads, "a later reusable-coordinate lookup splits the read")
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			var data map[string]any
			require.NoError(t, json.Unmarshal([]byte(responseString(t, response)), &data))
			switch read.name {
			case "detail":
				require.Equal(t, "original", data["username"])
				require.Equal(t, service.captured.snapshot.Birth.String(), data["birth"])
				require.Equal(t, 1, service.captured.snapshotReads)
			case "observer":
				require.Len(t, data["events"], 1)
				require.Equal(t, 1, service.captured.reads)
			case "fsm":
				transitions := data["transitions"].([]any)
				require.Len(t, transitions, 1)
				require.Equal(t, "original", transitions[0].(map[string]any)["trigger"])
				require.Equal(t, 1, service.captured.fsmReads)
			case "traffic":
				require.Equal(t, float64(1101), data["rx-bytes"])
				require.Equal(t, float64(2202), data["tx-bytes"])
				require.Equal(t, float64(33), data["rx-packets"])
				require.Equal(t, float64(44), data["tx-packets"])
				require.Equal(t, 1, service.captured.counterReads)
			}
		})
	}
}

// MUTATION: resolve a missing/ambiguous original with the first SID match, or
// retry a stale captured read through that reusable SID. Neither may disclose
// a successor's useful data; the error must retain the operator's next command.
func TestSingleSessionReadersRefuseAmbiguousAndStaleOriginal(t *testing.T) {
	for _, read := range sessionReadCases() {
		for _, refusal := range []string{"ambiguous", "missing", "stale"} {
			t.Run(read.name+"/"+refusal, func(t *testing.T) {
				service := newSessionReadService(t)
				switch refusal {
				case "ambiguous":
					service.captureErr = l2tp.ErrSessionAmbiguous
				case "missing":
					service.captureErr = l2tp.ErrSessionNotFound
				case "stale":
					service.captured.snapshotErr = l2tp.ErrSessionNotFound
					service.captured.fsmErr = l2tp.ErrSessionNotFound
					service.captured.historyErr = l2tp.ErrSessionNotFound
					service.captured.counterErr = l2tp.ErrSessionNotFound
				}
				publishFake(t, service)
				response, err := read.handler(nil, commandtest.Args("7"))
				require.NoError(t, err)
				require.Equal(t, []l2tp.SessionSelector{{SID: 7}}, service.selectors)
				require.Zero(t, service.unsafeReads)
				require.Equal(t, plugin.StatusError, response.Status)
				require.Contains(t, response.Error, "show l2tp sessions")
			})
		}
	}
}

// MUTATION: silently ignore an unknown, duplicate or malformed qualifier.
// Refuse before any original is captured, not after a reader sees its data.
func TestSingleSessionReadersRejectMalformedQualifiersBeforeSelection(t *testing.T) {
	for _, read := range sessionReadCases() {
		for _, arguments := range [][]string{
			{"7", "tunnel-id"}, {"7", "tunnel-id", "0"},
			{"7", "tunnel-id", "17", "tunnel-id", "18"},
			{"7", "listener"}, {"7", "listener", "unknown"},
			{"7", "listener", "-help"}, {"7", "unexpected", "value"},
		} {
			t.Run(read.name+"/"+arguments[len(arguments)-1], func(t *testing.T) {
				service := newSessionReadService(t)
				publishFake(t, service)
				response, err := read.handler(nil, commandtest.Args(arguments...))
				require.NoError(t, err)
				require.Equal(t, plugin.StatusError, response.Status)
				require.Empty(t, service.selectors)
				require.Zero(t, service.unsafeReads)
			})
		}
	}
}

// MUTATION: omit the bound listener when projecting list/detail coordinates.
// An absent listener remains absent instead of inventing an endpoint.
func TestSessionRowsExposeOriginalListenerForReadableQualification(t *testing.T) {
	endpoint := netip.MustParseAddrPort("127.0.0.1:1917")
	for _, detail := range []bool{false, true} {
		row := sessionJSON(&l2tp.SessionSnapshot{LocalSID: 7, TunnelLocalTID: 17, Listener: endpoint}, detail)
		require.Equal(t, endpoint.String(), row["listener"])
		row = sessionJSON(&l2tp.SessionSnapshot{LocalSID: 7, TunnelLocalTID: 17}, detail)
		_, exists := row["listener"]
		require.False(t, exists)
	}
}
