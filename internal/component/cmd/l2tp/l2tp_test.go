package l2tp

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tppkg "codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// fakeService is a minimal l2tppkg.Service for unit-testing handlers
// without a live subsystem. Every method returns the field values the
// test populated.
type fakeService struct {
	snapshot             l2tppkg.Snapshot
	listeners            []l2tppkg.ListenerSnapshot
	config               l2tppkg.ConfigSnapshot
	lookupTunnel         func(uint16) (l2tppkg.TunnelSnapshot, bool)
	lookupSession        func(uint16) (l2tppkg.SessionSnapshot, bool)
	teardownTunnelErr    error
	teardownSessionErr   error
	teardownAllTunnelsN  int
	teardownAllSessionsN int
}

func (f *fakeService) Snapshot() l2tppkg.Snapshot              { return f.snapshot }
func (f *fakeService) Listeners() []l2tppkg.ListenerSnapshot   { return f.listeners }
func (f *fakeService) EffectiveConfig() l2tppkg.ConfigSnapshot { return f.config }
func (f *fakeService) TeardownTunnel(_ uint16) error           { return f.teardownTunnelErr }
func (f *fakeService) TeardownSession(_ uint16) error          { return f.teardownSessionErr }
func (f *fakeService) TeardownAllTunnels() int                 { return f.teardownAllTunnelsN }
func (f *fakeService) TeardownAllSessions() int                { return f.teardownAllSessionsN }
func (f *fakeService) LookupTunnel(id uint16) (l2tppkg.TunnelSnapshot, bool) {
	if f.lookupTunnel != nil {
		return f.lookupTunnel(id)
	}
	return l2tppkg.TunnelSnapshot{}, false
}
func (f *fakeService) LookupSession(id uint16) (l2tppkg.SessionSnapshot, bool) {
	if f.lookupSession != nil {
		return f.lookupSession(id)
	}
	return l2tppkg.SessionSnapshot{}, false
}
func (f *fakeService) SessionEvents(_ uint16) []l2tppkg.ObserverEvent                   { return nil }
func (f *fakeService) LoginSamples(_ string) []l2tppkg.CQMBucket                        { return nil }
func (f *fakeService) SessionSummaries() []l2tppkg.SessionSummary                       { return nil }
func (f *fakeService) LoginSummaries() []l2tppkg.LoginSummary                           { return nil }
func (f *fakeService) EchoState(_ string) *l2tppkg.LoginEchoState                       { return nil }
func (f *fakeService) ReliableStats(_ uint16) *l2tppkg.ReliableStats                    { return nil }
func (f *fakeService) TunnelFSMHistory(_ uint16) []l2tppkg.FSMTransition                { return nil }
func (f *fakeService) SessionFSMHistory(_ uint16) []l2tppkg.FSMTransition               { return nil }
func (f *fakeService) CaptureSnapshot(_ int, _ uint16, _ string) []l2tppkg.CaptureEntry { return nil }
func (f *fakeService) RecordDisconnect(_ uint16, _, _ string, _ uint32)                 {}

// publishFake wires fake into the service locator and returns a
// deferred unpublish.
func publishFake(t *testing.T, svc l2tppkg.Service) {
	t.Helper()
	l2tppkg.PublishService(svc)
	t.Cleanup(func() { l2tppkg.PublishService(nil) })
}

// responseString type-asserts Response.Data to string. Fails the
// test when Data is any other type.
func responseString(t *testing.T, r *plugin.Response) string {
	t.Helper()
	s, ok := r.Data.(string)
	require.True(t, ok, "expected Response.Data to be string, got %T", r.Data)
	return s
}

// VALIDATES: AC-6 summary handler returns JSON with aggregate counts.
func TestHandleSummaryReturnsAggregate(t *testing.T) {
	snap := l2tppkg.Snapshot{
		TunnelCount:  3,
		SessionCount: 7,
		CapturedAt:   time.Unix(1700000000, 0).UTC(),
	}
	publishFake(t, &fakeService{
		snapshot:  snap,
		listeners: []l2tppkg.ListenerSnapshot{{Addr: netip.MustParseAddrPort("0.0.0.0:1701")}},
	})

	resp, err := handleSummary(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, float64(3), got["tunnel-count"])
	require.Equal(t, float64(7), got["session-count"])
	require.Equal(t, float64(1), got["listener-count"])
}

// VALIDATES: handlers return StatusError + "subsystem not running"
// when no Service has been published.
func TestHandlerSubsystemDownReturnsStatusError(t *testing.T) {
	// Nothing published. LookupService returns nil.
	l2tppkg.PublishService(nil)

	resp, err := handleSummary(nil, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Data, "not running")
}

// VALIDATES: AC-8 handler returns detail JSON for a known tunnel.
func TestHandleTunnelReturnsDetail(t *testing.T) {
	publishFake(t, &fakeService{
		lookupTunnel: func(tid uint16) (l2tppkg.TunnelSnapshot, bool) {
			if tid == 100 {
				return l2tppkg.TunnelSnapshot{
					LocalTID:     100,
					RemoteTID:    200,
					PeerAddr:     netip.MustParseAddrPort("10.0.0.1:1701"),
					State:        "established",
					PeerFraming:  0x00000003,
					PeerHostName: "peer.example.net",
				}, true
			}
			return l2tppkg.TunnelSnapshot{}, false
		},
	})

	resp, err := handleTunnel(nil, []string{"100"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, float64(100), got["local-tid"])
	require.Equal(t, "peer.example.net", got["peer-hostname"])
	require.Equal(t, "async+sync", got["peer-framing"])
}

// VALIDATES: AC-18 unknown tunnel ID returns a StatusError naming the ID.
func TestHandleTunnelUnknownIDErrors(t *testing.T) {
	publishFake(t, &fakeService{})

	resp, err := handleTunnel(nil, []string{"999"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Data, "999")
}

// VALIDATES: Positional arg parser rejects missing and zero IDs.
func TestParseIDArgRejectsInvalid(t *testing.T) {
	_, err := parseIDArg(nil, "tunnel-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing tunnel-id")

	_, err = parseIDArg([]string{"0"}, "session-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved by RFC 2661")

	_, err = parseIDArg([]string{"abc"}, "tunnel-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid tunnel-id")

	n, err := parseIDArg([]string{"-v", "42"}, "tunnel-id")
	require.NoError(t, err)
	require.Equal(t, uint16(42), n)
}

// VALIDATES: AC-14 tunnel teardown returns a JSON status on success.
func TestHandleTunnelTeardownSuccess(t *testing.T) {
	publishFake(t, &fakeService{})

	resp, err := handleTunnelTeardown(nil, []string{"42"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, "tunnel-teardown", got["action"])
	require.Equal(t, float64(42), got["tunnel-id"])
}

// VALIDATES: Teardown of unknown tunnel surfaces the wrapped error.
func TestHandleTunnelTeardownUnknownID(t *testing.T) {
	publishFake(t, &fakeService{
		teardownTunnelErr: errors.New("l2tp: tunnel not found: local-tid=999"),
	})

	resp, err := handleTunnelTeardown(nil, []string{"999"})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Data, "tunnel not found")
}

// VALIDATES: AC-15 tunnel teardown-all reports the count.
func TestHandleTunnelTeardownAllReportsCount(t *testing.T) {
	publishFake(t, &fakeService{teardownAllTunnelsN: 5})

	resp, err := handleTunnelTeardownAll(nil, nil)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, float64(5), got["tunnels-cleared"])
}

// VALIDATES: AC-13 config handler redacts the shared secret.
func TestHandleConfigRedactsSecret(t *testing.T) {
	publishFake(t, &fakeService{
		config: l2tppkg.ConfigSnapshot{
			Enabled:       true,
			AuthMethod:    "chap-md5",
			AllowNoAuth:   false,
			HelloInterval: 60 * time.Second,
			SharedSecret:  "<set>",
			ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("0.0.0.0:1701")},
		},
	})

	resp, err := handleConfig(nil, nil)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, "<set>", got["shared-secret"])
	require.Equal(t, "chap-md5", got["auth-method"])
	require.Equal(t, false, got["allow-no-auth"])
	require.Equal(t, float64(60), got["hello-interval"])
}

func TestParseKeywordArgs_ReasonAndCause(t *testing.T) {
	args := []string{"42", "reason", "maintenance", "window", "cause", "6"}
	actor, reason, cause := parseKeywordArgs(args)
	require.Equal(t, "", actor)
	require.Equal(t, "maintenance window", reason)
	require.Equal(t, uint32(6), cause)
}

func TestParseKeywordArgs_ReasonOnly(t *testing.T) {
	args := []string{"42", "reason", "scheduled"}
	actor, reason, cause := parseKeywordArgs(args)
	require.Equal(t, "", actor)
	require.Equal(t, "scheduled", reason)
	require.Equal(t, uint32(0), cause)
}

func TestParseKeywordArgs_Empty(t *testing.T) {
	args := []string{"42"}
	actor, reason, cause := parseKeywordArgs(args)
	require.Equal(t, "", actor)
	require.Equal(t, "", reason)
	require.Equal(t, uint32(0), cause)
}

func TestParseKeywordArgs_ActorReasonCause(t *testing.T) {
	args := []string{"42", "actor", "web", "reason", "maintenance", "cause", "6"}
	actor, reason, cause := parseKeywordArgs(args)
	require.Equal(t, "web", actor)
	require.Equal(t, "maintenance", reason)
	require.Equal(t, uint32(6), cause)
}
