package reactor

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
)

// These tests close the L25 gap: TestMetricNames_MatchRegistration proves the
// counters are REGISTERED; these prove real event paths INCREMENT them. Each
// drives the actual producer function (not a re-implementation) through a spy
// registry and asserts the exact delta, so a producer that stopped incrementing
// would fail here rather than pass a name-only check.

// TestReloadIncrementsConfigReloadCounter verifies a successful Reload() bumps
// ze_config_reloads_total via the production wiring (SetMetricsRegistry -> Start
// -> initReactorMetrics -> Reload).
//
// VALIDATES: reactorAPIAdapter.Reload success path increments configReloads
// (reactor_api.go:395), once per reload.
// PREVENTS: the reload counter being registered but never advanced, so a
// "reloads/sec" dashboard silently reads flat.
func TestReloadIncrementsConfigReloadCounter(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	reg := newSpyRegistry()
	cfg := &Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true}
	r := New(cfg)
	r.SetReloadFunc(simpleReloadFunc)
	r.SetMetricsRegistry(reg)
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.Reload())

	c := reg.counter("ze_config_reloads_total")
	require.NotNil(t, c, "ze_config_reloads_total must be registered")
	assert.Equal(t, 1.0, c.Value(), "one successful reload must increment the counter once")

	// A second reload increments again: proves a monotonic counter, not a one-shot set.
	require.NoError(t, adapter.Reload())
	assert.Equal(t, 2.0, c.Value(), "second reload must advance the counter to 2")
}

// TestReloadParseErrorIncrementsErrorCounter verifies a failed parse bumps
// ze_config_reload_errors_total{error_type="parse"} and leaves the success
// counter untouched.
//
// VALIDATES: reactorAPIAdapter.Reload parse-error path increments
// configReloadErrors.With("parse") (reactor_api.go:383).
// PREVENTS: reload failures being invisible in metrics (error counter never
// moves), and a failed reload wrongly counting as a success.
func TestReloadParseErrorIncrementsErrorCounter(t *testing.T) {
	failingReloadFunc := func(string) ([]*PeerSettings, error) {
		return nil, os.ErrInvalid
	}

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	reg := newSpyRegistry()
	cfg := &Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true}
	r := New(cfg)
	r.SetReloadFunc(failingReloadFunc)
	r.SetMetricsRegistry(reg)
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	require.Error(t, adapter.Reload(), "reload must fail when the parse function errors")

	cv := reg.counterVec("ze_config_reload_errors_total")
	require.NotNil(t, cv, "ze_config_reload_errors_total must be registered")
	parse := cv.get("parse")
	require.NotNil(t, parse, `error_type="parse" series must exist after a parse failure`)
	assert.Equal(t, 1.0, parse.Value(), "a parse-error reload must increment the parse error counter")

	// The success counter must stay at zero on the failure path.
	if c := reg.counter("ze_config_reloads_total"); c != nil {
		assert.Equal(t, 0.0, c.Value(), "a failed reload must not increment the success counter")
	}
}

// TestPeerEventsIncrementChurnCounters verifies the per-peer RIB/session churn
// counters advance when the real Peer producers run: a received UPDATE bumps
// ze_peer_messages_received_total{type="update"}, and FSM transitions bump the
// established/flap/transition counters.
//
// VALIDATES: Peer.IncrUpdatesReceived (peer_stats.go:134) and
// updatePeerStateMetric (peer_stats.go:361-367).
// PREVENTS: churn counters registered but never incremented, so UPDATE volume
// and session flaps read as zero on the dashboard.
func TestPeerEventsIncrementChurnCounters(t *testing.T) {
	reg := newSpyRegistry()
	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		peers:           make(map[netip.AddrPort]*Peer),
		rmetrics:        initReactorMetrics(reg, "test", "1.2.3.4", "65000"),
		clock:           clock.RealClock{},
	}

	settings := NewPeerSettings(netip.MustParseAddr("198.51.100.7"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.SetReactor(r)
	label := peer.peerAddrLabel()

	// RIB churn: each received UPDATE increments the update-typed message counter.
	for range 3 {
		peer.IncrUpdatesReceived()
	}
	recv := reg.counterVec("ze_peer_messages_received_total")
	require.NotNil(t, recv, "ze_peer_messages_received_total must be registered")
	updates := recv.get(label, "update")
	require.NotNil(t, updates, "the update-type series must exist after IncrUpdatesReceived")
	assert.Equal(t, 3.0, updates.Value(), "three received UPDATEs must count as three")

	// Session churn: Active -> Established records an establishment + transition.
	peer.updatePeerStateMetric(PeerStateActive, PeerStateEstablished)
	est := reg.counterVec("ze_peer_sessions_established_total").get(label)
	require.NotNil(t, est, "reaching Established must create the established series")
	assert.Equal(t, 1.0, est.Value(), "reaching Established must increment the established counter")
	trans := reg.counterVec("ze_peer_state_transitions_total").get(label, "active", "established")
	require.NotNil(t, trans, "the active->established transition series must exist")
	assert.Equal(t, 1.0, trans.Value())

	// Session churn: Established -> Active records a flap.
	peer.updatePeerStateMetric(PeerStateEstablished, PeerStateActive)
	flaps := reg.counterVec("ze_peer_session_flaps_total").get(label)
	require.NotNil(t, flaps, "dropping from Established must create the flap series")
	assert.Equal(t, 1.0, flaps.Value(), "one drop from Established must count as one flap")
}

// TestSessionReadIncrementsWireBytesCounter verifies the wire-layer byte counter
// advances as the session actually reads framed messages off the connection.
// It replays the OPEN + KEEPALIVE handshake through the real read path and
// asserts the counter equals the summed message lengths.
//
// VALIDATES: Session.readAndProcessMessage increments wireBytesRecv by hdr.Length
// per message (session_read.go:129).
// PREVENTS: ze_wire_bytes_received_total registered but flat, hiding real
// per-peer ingress volume.
func TestSessionReadIncrementsWireBytesCounter(t *testing.T) {
	reg := newSpyRegistry()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	session.prefixMetrics = initReactorMetrics(reg, "test", "1.2.3.4", "65000")
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup
	t.Cleanup(func() { server.Close() }) //nolint:errcheck // test cleanup

	_ = acceptWithReader(t, session, server, client)
	require.NotNil(t, session.bufReader, "bufReader must be set after Accept")

	// Peer OPEN -> OpenConfirm (session's own KEEPALIVE reply is drained by the goroutine).
	peerOpen := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302}
	openBytes := message.PackTo(peerOpen, nil)
	go func() {
		if _, err := client.Write(openBytes); err != nil {
			return
		}
		buf := make([]byte, 4096)
		if _, err := client.Read(buf); err != nil {
			return
		}
	}()
	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Peer KEEPALIVE -> Established.
	keepaliveBytes := message.PackTo(message.NewKeepalive(), nil)
	go func() {
		if _, err := client.Write(keepaliveBytes); err != nil {
			return
		}
	}()
	require.NoError(t, session.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, session.State())

	// wireBytesRecv accumulates hdr.Length per message: OPEN + KEEPALIVE.
	recv := reg.counterVec("ze_wire_bytes_received_total")
	require.NotNil(t, recv, "ze_wire_bytes_received_total must be registered")
	bytesForPeer := recv.get(session.addrLabel)
	require.NotNil(t, bytesForPeer, "per-peer wire byte series must exist after reads")
	assert.Equal(t, float64(len(openBytes)+len(keepaliveBytes)), bytesForPeer.Value(),
		"wire bytes received must equal the OPEN + KEEPALIVE message lengths")
}
