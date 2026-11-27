//go:build ze_bgp

// Design: docs/architecture/doctor-and-health-checks.md -- every answer-2 reactor Warn carries a registered doctor code and raises on the report bus.
// Related: doctor_codes.go -- reactor doctor diagnostic codes and fixes.
// Related: register_diagnostic.go -- recent-events registrations for reactor codes.

package reactor

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/report"
)

type failingEventBus struct {
	err error
}

func (f failingEventBus) Emit(_, _ string, _ any) (int, error) {
	return 0, f.err
}

func (f failingEventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

type failingBFDHandle struct {
	key api.Key
}

func (h *failingBFDHandle) Key() api.Key                       { return h.key }
func (h *failingBFDHandle) Subscribe() <-chan api.StateChange  { return make(chan api.StateChange) }
func (h *failingBFDHandle) Unsubscribe(<-chan api.StateChange) {}
func (h *failingBFDHandle) Shutdown() error                    { return nil }
func (h *failingBFDHandle) Enable() error                      { return nil }

type failingBFDReleaseService struct {
	releaseErr error
	onEnsure   func()
}

func (s *failingBFDReleaseService) EnsureSession(req api.SessionRequest) (api.SessionHandle, error) {
	if s.onEnsure != nil {
		s.onEnsure()
	}
	return &failingBFDHandle{key: req.Key()}, nil
}

func (s *failingBFDReleaseService) ReleaseSession(api.SessionHandle) error {
	return s.releaseErr
}

func (s *failingBFDReleaseService) Snapshot() []api.SessionState { return nil }
func (s *failingBFDReleaseService) SessionDetail(string) (api.SessionState, bool) {
	return api.SessionState{}, false
}
func (s *failingBFDReleaseService) Profiles() []api.ProfileState { return nil }

func swapReactorLogger(lg *slog.Logger) func() {
	prev := reactorLogger
	reactorLogger = func() *slog.Logger { return lg }
	return func() { reactorLogger = prev }
}

func swapPeerLogger(lg *slog.Logger) func() {
	prev := peerLogger
	peerLogger = func() *slog.Logger { return lg }
	return func() { peerLogger = prev }
}

func swapCacheLogger(lg *slog.Logger) func() {
	prev := cacheLogger
	cacheLogger = func() *slog.Logger { return lg }
	return func() { cacheLogger = prev }
}

func newWarnCaptureLogger() (*slog.Logger, *syncBuffer) {
	sink := &syncBuffer{}
	return slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn})), sink
}

// TestEmitOperationListenerReadyReportsEmitFailure proves that a failed
// EventListenerReady EventBus emit during a BGP config operation emits a
// default-level coded WARN and raises a report-bus error.
func TestEmitOperationListenerReadyReportsEmitFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	lg, sink := newWarnCaptureLogger()
	t.Cleanup(swapReactorLogger(lg))

	emitErr := errors.New("event bus closed")
	r := &Reactor{eventBus: failingEventBus{err: emitErr}}
	a := &reactorAPIAdapter{r: r}
	localAddr := netip.MustParseAddr("192.0.2.1")

	a.emitOperationListenerReady(&PeerSettings{LocalAddress: localAddr})

	coded := closeRecordsWith(closeLogRecords(t, sink), "code", bgpserver.CodeDoctorBGPPluginEventsUndelivered)
	require.Len(t, coded, 1)
	assert.Equal(t, "WARN", coded[0]["level"])
	assert.Equal(t, localAddr.String(), coded[0]["address"])
	assert.NotEmpty(t, coded[0]["why"])
	assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, coded[0]["fix"])
	assert.Equal(t, emitErr.Error(), coded[0]["error"])

	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, reportSourceBGP, events[0].Source)
	assert.Equal(t, bgpserver.CodeDoctorBGPPluginEventsUndelivered, events[0].Code)
	assert.Equal(t, localAddr.String(), events[0].Subject)
	assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, events[0].Fix)
}

// TestPeerBFDReleaseFailureReportsWarnAndReportBus proves that BFD session
// release failures in both openBFDSession (canceled start) and releaseBFDClient
// emit a default-level coded WARN and raise on the report bus.
func TestPeerBFDReleaseFailureReportsWarnAndReportBus(t *testing.T) {
	releaseErr := errors.New("bfd engine unavailable")

	t.Run("canceled_start", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		lg, sink := newWarnCaptureLogger()
		t.Cleanup(swapPeerLogger(lg))

		peer := NewPeer(&PeerSettings{
			Address:      netip.MustParseAddr("198.51.100.2"),
			LocalAddress: netip.MustParseAddr("198.51.100.1"),
			BFD:          &BFDSettings{Enabled: true},
		})
		svc := &failingBFDReleaseService{
			releaseErr: releaseErr,
			onEnsure: func() {
				peer.bfd.mu.Lock()
				peer.bfd.canceled = true
				peer.bfd.mu.Unlock()
			},
		}
		prev := api.GetService()
		api.SetService(svc)
		t.Cleanup(func() { api.SetService(prev) })

		opened, again := peer.openBFDSession()
		assert.Nil(t, opened)
		assert.False(t, again)

		coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPBFDUnavailable)
		require.Len(t, coded, 1)
		assert.Equal(t, "WARN", coded[0]["level"])
		assert.Equal(t, peer.addrString, coded[0]["peer"])
		assert.NotEmpty(t, coded[0]["why"])
		assert.Equal(t, fixBFDUnavailable, coded[0]["fix"])
		assert.Equal(t, releaseErr.Error(), coded[0]["error"])

		events := report.Errors(0)
		require.Len(t, events, 1)
		assert.Equal(t, reportSourceBGP, events[0].Source)
		assert.Equal(t, codeDoctorBGPBFDUnavailable, events[0].Code)
		assert.Equal(t, peer.addrString, events[0].Subject)
		assert.Equal(t, fixBFDUnavailable, events[0].Fix)
	})

	t.Run("release_client", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		lg, sink := newWarnCaptureLogger()
		t.Cleanup(swapPeerLogger(lg))

		peer := NewPeer(&PeerSettings{
			Address: netip.MustParseAddr("198.51.100.3"),
		})
		handle := &failingBFDHandle{}
		peer.bfd.svc = &failingBFDReleaseService{releaseErr: releaseErr}
		peer.bfd.handle = handle
		peer.bfd.sub = handle.Subscribe()
		peer.bfd.stop = make(chan struct{})
		peer.bfd.done = make(chan struct{})

		rel := peer.releaseBFDClient()
		assert.True(t, rel.released)

		coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPBFDUnavailable)
		require.Len(t, coded, 1)
		assert.Equal(t, "WARN", coded[0]["level"])
		assert.Equal(t, peer.addrString, coded[0]["peer"])
		assert.NotEmpty(t, coded[0]["why"])
		assert.Equal(t, fixBFDUnavailable, coded[0]["fix"])
		assert.Equal(t, releaseErr.Error(), coded[0]["error"])

		events := report.Errors(0)
		require.Len(t, events, 1)
		assert.Equal(t, reportSourceBGP, events[0].Source)
		assert.Equal(t, codeDoctorBGPBFDUnavailable, events[0].Code)
		assert.Equal(t, peer.addrString, events[0].Subject)
		assert.Equal(t, fixBFDUnavailable, events[0].Fix)
	})
}

// TestPeerInitialSyncReportsRouteScopedAndDefaultOriginateFailures proves that
// default-originate send failures and route-scoped plugin route send failures
// in peer_initial_sync.go emit coded default-level WARNs and report-bus events.
func TestPeerInitialSyncReportsRouteScopedAndDefaultOriginateFailures(t *testing.T) {
	t.Run("default_originate", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		lg, sink := newWarnCaptureLogger()
		t.Cleanup(swapRoutesLogger(lg))

		peer, _ := newInitialSyncPeer(t, false, family.IPv4Unicast)
		peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.1")
		peer.settings.DefaultOriginate = map[string]bool{
			"ipv4/unicast": true,
		}

		peer.sendDefaultOriginateRoutes(peer.currentSession(), peer.negotiated.Load())

		coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPDefaultOriginateNotSent)
		require.Len(t, coded, 1)
		assert.Equal(t, "WARN", coded[0]["level"])
		assert.Equal(t, peer.addrString, coded[0]["peer"])
		assert.NotEmpty(t, coded[0]["why"])
		assert.Equal(t, fixDefaultOriginateNotSent, coded[0]["fix"])
		assert.NotEmpty(t, coded[0]["error"])

		events := report.Errors(0)
		require.Len(t, events, 1)
		assert.Equal(t, reportSourceBGP, events[0].Source)
		assert.Equal(t, codeDoctorBGPDefaultOriginateNotSent, events[0].Code)
		assert.Equal(t, peer.addrString, events[0].Subject)
		assert.Equal(t, fixDefaultOriginateNotSent, events[0].Fix)
	})

	for _, tc := range []struct {
		name  string
		group bool
	}{
		{name: "plugin_route_ungrouped", group: false},
		{name: "plugin_route_grouped", group: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			lg, sink := newWarnCaptureLogger()
			t.Cleanup(swapRoutesLogger(lg))

			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			peer.settings.PluginRoutes = []PluginRoute{
				{
					Family:  "ipv4/unicast",
					NLRI:    []byte{24, 198, 51, 100},
					NextHop: netip.MustParseAddr("10.0.0.1"),
					Group:   tc.group,
				},
			}

			peer.sendPluginRoutesVia(peer.currentSession(), func(*message.Update) error {
				return message.ErrAttributesTooLarge
			})

			coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPRouteNotSent)
			require.Len(t, coded, 1)
			assert.Equal(t, "WARN", coded[0]["level"])
			assert.Equal(t, peer.addrString, coded[0]["peer"])
			assert.NotEmpty(t, coded[0]["why"])
			assert.Equal(t, fixRouteNotSent, coded[0]["fix"])
			assert.Equal(t, message.ErrAttributesTooLarge.Error(), coded[0]["error"])

			events := report.Errors(0)
			require.Len(t, events, 1)
			assert.Equal(t, reportSourceBGP, events[0].Source)
			assert.Equal(t, codeDoctorBGPRouteNotSent, events[0].Code)
			assert.Equal(t, peer.addrString, events[0].Subject)
			assert.Equal(t, fixRouteNotSent, events[0].Fix)
		})
	}
}

// TestReactorEventBusEmitFailuresReportWarnAndReportBus proves that failed
// congestion and update-notification EventBus emissions in reactor.go emit
// default-level coded WARNs and raise on the report bus.
func TestReactorEventBusEmitFailuresReportWarnAndReportBus(t *testing.T) {
	emitErr := errors.New("event bus unavailable")

	t.Run("congestion", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		lg, sink := newWarnCaptureLogger()
		t.Cleanup(swapReactorLogger(lg))

		r := &Reactor{eventBus: failingEventBus{err: emitErr}}
		r.emitCongestionEventBus("203.0.113.10", bgpevents.EventCongested)

		coded := closeRecordsWith(closeLogRecords(t, sink), "code", bgpserver.CodeDoctorBGPPluginEventsUndelivered)
		require.Len(t, coded, 1)
		assert.Equal(t, "WARN", coded[0]["level"])
		assert.Equal(t, "203.0.113.10", coded[0]["peer"])
		assert.Equal(t, bgpevents.EventCongested, coded[0]["event"])
		assert.NotEmpty(t, coded[0]["why"])
		assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, coded[0]["fix"])
		assert.Equal(t, emitErr.Error(), coded[0]["error"])

		events := report.Errors(0)
		require.Len(t, events, 1)
		assert.Equal(t, reportSourceBGP, events[0].Source)
		assert.Equal(t, bgpserver.CodeDoctorBGPPluginEventsUndelivered, events[0].Code)
		assert.Equal(t, "203.0.113.10", events[0].Subject)
		assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, events[0].Fix)
	})

	t.Run("update_notification", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		lg, sink := newWarnCaptureLogger()
		t.Cleanup(swapReactorLogger(lg))

		r := &Reactor{eventBus: failingEventBus{err: emitErr}}
		r.emitUpdateNotificationEvent("203.0.113.11", "receive")

		coded := closeRecordsWith(closeLogRecords(t, sink), "code", bgpserver.CodeDoctorBGPPluginEventsUndelivered)
		require.Len(t, coded, 1)
		assert.Equal(t, "WARN", coded[0]["level"])
		assert.Equal(t, "203.0.113.11", coded[0]["peer"])
		assert.Equal(t, "receive", coded[0]["direction"])
		assert.NotEmpty(t, coded[0]["why"])
		assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, coded[0]["fix"])
		assert.Equal(t, emitErr.Error(), coded[0]["error"])

		events := report.Errors(0)
		require.Len(t, events, 1)
		assert.Equal(t, reportSourceBGP, events[0].Source)
		assert.Equal(t, bgpserver.CodeDoctorBGPPluginEventsUndelivered, events[0].Code)
		assert.Equal(t, "203.0.113.11", events[0].Subject)
		assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, events[0].Fix)
	})
}

// TestAckBatchReportsExpiredCacheEntry proves that an expired cache ID during
// refused-batch acknowledgement emits a coded default-level WARN and raises a
// report-bus error under codeDoctorBGPCachePressure.
func TestAckBatchReportsExpiredCacheEntry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	lg, sink := newWarnCaptureLogger()
	t.Cleanup(swapCacheLogger(lg))

	r := &Reactor{recentUpdates: newRecentUpdateCache(16)}
	a := &reactorAPIAdapter{r: r}

	a.ackBatch([]uint64{42}, "bgp-rs")

	coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPCachePressure)
	require.Len(t, coded, 1)
	assert.Equal(t, "WARN", coded[0]["level"])
	assert.Equal(t, "bgp-rs", coded[0]["plugin"])
	assert.NotEmpty(t, coded[0]["why"])
	assert.Equal(t, fixCachePressure, coded[0]["fix"])
	assert.Equal(t, ErrUpdateExpired.Error(), coded[0]["error"])

	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, reportSourceBGP, events[0].Source)
	assert.Equal(t, codeDoctorBGPCachePressure, events[0].Code)
	assert.Equal(t, "bgp-rs", events[0].Subject)
	assert.Equal(t, fixCachePressure, events[0].Fix)
}

// TestHandleAddressAddedReportsListenerReadyEmitFailure proves that a failed
// EventListenerReady emit in Reactor.handleAddressAdded emits a coded
// default-level WARN and raises on the report bus.
func TestHandleAddressAddedReportsListenerReadyEmitFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	lg, sink := newWarnCaptureLogger()
	t.Cleanup(swapReactorLogger(lg))

	emitErr := errors.New("event bus refused")
	localAddr := netip.MustParseAddr("192.0.2.55")
	peer := NewPeer(&PeerSettings{
		Address:      netip.MustParseAddr("192.0.2.56"),
		LocalAddress: localAddr,
		Connection:   ConnectionActive,
	})
	r := &Reactor{
		config:   &Config{},
		eventBus: failingEventBus{err: emitErr},
		peers: map[netip.AddrPort]*Peer{
			peer.Settings().PeerKey(): peer,
		},
	}

	assert.True(t, r.handleAddressAdded(localAddr))

	coded := closeRecordsWith(closeLogRecords(t, sink), "code", bgpserver.CodeDoctorBGPPluginEventsUndelivered)
	require.Len(t, coded, 1)
	assert.Equal(t, "WARN", coded[0]["level"])
	assert.Equal(t, localAddr.String(), coded[0]["address"])
	assert.NotEmpty(t, coded[0]["why"])
	assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, coded[0]["fix"])
	assert.Equal(t, emitErr.Error(), coded[0]["error"])

	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, reportSourceBGP, events[0].Source)
	assert.Equal(t, bgpserver.CodeDoctorBGPPluginEventsUndelivered, events[0].Code)
	assert.Equal(t, localAddr.String(), events[0].Subject)
	assert.Equal(t, bgpserver.PluginEventsUndeliveredFix, events[0].Fix)
}

// TestPublishBaseReportsMalformedAttributeIndex proves that an UPDATE payload
// whose attribute span index cannot be built in Session.publishBase emits a
// coded default-level WARN and raises on the report bus without copying wire
// bytes.
func TestPublishBaseReportsMalformedAttributeIndex(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	lg, sink := newWarnCaptureLogger()
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))

	settings := &PeerSettings{
		Address:  netip.MustParseAddr("203.0.113.77"),
		LocalAS:  65000,
		PeerAS:   65001,
		RouterID: 0x01020304,
	}
	session := NewSession(settings)
	// Truncated UPDATE payload fails ParseUpdateSections and wu.Attrs().
	wu := wireu.NewWireUpdate([]byte{0x00}, bgpctx.ContextID(0))

	got := session.publishBase(wu)
	assert.Same(t, wu, got)

	coded := closeRecordsWith(closeLogRecords(t, sink), "code", codeDoctorBGPMalformedFromPeer)
	require.Len(t, coded, 1)
	assert.Equal(t, "WARN", coded[0]["level"])
	assert.Equal(t, settings.Address.String(), coded[0]["peer"])
	assert.NotEmpty(t, coded[0]["why"])
	assert.Equal(t, fixMalformedFromPeer, coded[0]["fix"])
	assert.NotEmpty(t, coded[0]["error"])

	events := report.Errors(0)
	require.Len(t, events, 1)
	assert.Equal(t, reportSourceBGP, events[0].Source)
	assert.Equal(t, codeDoctorBGPMalformedFromPeer, events[0].Code)
	assert.Equal(t, settings.Address.String(), events[0].Subject)
	assert.Equal(t, fixMalformedFromPeer, events[0].Fix)
}
