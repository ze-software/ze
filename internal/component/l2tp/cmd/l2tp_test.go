package cmd

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config"
	l2tppkg "github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
	outgoingCall         func(remote, called string) (l2tppkg.OutgoingCallResult, error)
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
func (f *fakeService) EnableRawCapture()                                                {}
func (f *fakeService) DisableRawCapture()                                               {}
func (f *fakeService) RawCaptureSnapshot(_ int) []l2tppkg.RawCaptureEntry               { return nil }
func (f *fakeService) RecordDisconnect(_ uint16, _, _ string, _ uint32)                 {}
func (f *fakeService) PlaceOutgoingCall(remote, called string) (l2tppkg.OutgoingCallResult, error) {
	if f.outgoingCall != nil {
		return f.outgoingCall(remote, called)
	}
	return l2tppkg.OutgoingCallResult{Remote: remote, Called: called}, nil
}

// publishFake wires fake into the service locator and returns a
// deferred unpublish.
func publishFake(t *testing.T, svc l2tppkg.Service) {
	t.Helper()
	l2tppkg.PublishService(svc)
	t.Cleanup(func() { l2tppkg.PublishService(nil) })
}

// responseString marshals Response.Data to a JSON string for assertion.
func responseString(t *testing.T, r *plugin.Response) string {
	t.Helper()
	require.NotNil(t, r.Data, "expected non-nil Response.Data")
	b, err := json.Marshal(r.Data)
	require.NoError(t, err, "marshal Response.Data")
	return string(b)
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

	resp, err := handleSummary(nil, commandtest.Args())
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

	resp, err := handleSummary(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "not running")
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

	resp, err := handleTunnel(nil, commandtest.Args("100"))
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

	resp, err := handleTunnel(nil, commandtest.Args("999"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "999")
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

	resp, err := handleTunnelTeardown(nil, commandtest.Args("42"))
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

	resp, err := handleTunnelTeardown(nil, commandtest.Args("999"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "tunnel not found")
}

// VALIDATES: AC-15 tunnel teardown-all reports the count.
func TestHandleTunnelTeardownAllReportsCount(t *testing.T) {
	publishFake(t, &fakeService{teardownAllTunnelsN: 5})

	resp, err := handleTunnelTeardownAll(nil, commandtest.Args())
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

	resp, err := handleConfig(nil, commandtest.Args())
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, "<set>", got["shared-secret"])
	require.Equal(t, "chap-md5", got["auth-method"])
	require.Equal(t, false, got["allow-no-auth"])
	require.Equal(t, float64(60), got["hello-interval"])
}

// TestConfigJSONMRUAndAllowList reads the real subsystem's effective parameters
// through the command handler, including absent-leaf defaults and written order.
func TestConfigJSONMRUAndAllowList(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		mru    float64
		method string
		allow  []any
	}{
		{"default CHAP", "", 1500, "chap-md5", []any{"chap-md5"}},
		{"default MSCHAP", "auth-method ms-chap-v2;", 1500, "ms-chap-v2", []any{"ms-chap-v2"}},
		{"default PAP", "auth-method pap;", 1500, "pap", []any{"chap-md5", "pap"}},
		{"no auth", "auth-method none; allow-no-auth true;", 1500, "none", []any{}},
		{"explicit ordered policy", "mru 1492; auth-method pap; authentication { allow [ pap ms-chap-v2 chap-md5 ]; }",
			1492, "pap", []any{"pap", "ms-chap-v2", "chap-md5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loaded, err := config.LoadConfig("l2tp { enabled true; "+tc.body+" }", "policy.conf", nil)
			if err != nil {
				t.Fatal(err)
			}
			params, err := l2tppkg.ExtractParameters(loaded.Tree)
			if err != nil {
				t.Fatal(err)
			}
			publishFake(t, l2tppkg.NewSubsystem(params))
			resp, err := handleConfig(nil, commandtest.Args())
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(responseString(t, resp)), &got); err != nil {
				t.Fatal(err)
			}
			if got["mru"] != tc.mru || got["auth-method"] != tc.method {
				t.Fatalf("effective config = %v, want MRU %v method %s", got, tc.mru, tc.method)
			}
			allow, ok := got["authentication-allow"].([]any)
			if !ok {
				t.Fatalf("effective allow list = %#v, want JSON array", got["authentication-allow"])
			}
			if len(allow) != len(tc.allow) {
				t.Fatalf("allow = %v, want %v", allow, tc.allow)
			}
			for i := range allow {
				if allow[i] != tc.allow[i] {
					t.Fatalf("allow = %v, want %v", allow, tc.allow)
				}
			}
		})
	}
}

// TestSessionJSONMRUAndAuthMethod checks detail and list command publication of
// measured values different from the configured policy, including unknown LCP.
func TestSessionJSONMRUAndAuthMethod(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mru    uint16
		method string
	}{
		{"negotiated PAP fallback", 1400, "pap"},
		{"negotiated CHAP", 1492, "chap-md5"},
		{"negotiated no-auth", 1500, "none"},
		{"not negotiated or LCP down", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ss := l2tppkg.SessionSnapshot{LocalSID: 42, MRU: tc.mru, AuthMethod: tc.method}
			publishFake(t, &fakeService{
				config:        l2tppkg.ConfigSnapshot{MRU: 1492, AuthMethod: "chap-md5", AuthAllow: []string{"chap-md5", "pap"}},
				snapshot:      l2tppkg.Snapshot{Tunnels: []l2tppkg.TunnelSnapshot{{Sessions: []l2tppkg.SessionSnapshot{ss}}}},
				lookupSession: func(id uint16) (l2tppkg.SessionSnapshot, bool) { return ss, id == ss.LocalSID },
			})
			detail, err := handleSession(nil, commandtest.Args("42"))
			if err != nil {
				t.Fatal(err)
			}
			list, err := handleSessions(nil, commandtest.Args())
			if err != nil {
				t.Fatal(err)
			}
			var detailData map[string]any
			var listData []map[string]any
			if err := json.Unmarshal([]byte(responseString(t, detail)), &detailData); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(responseString(t, list)), &listData); err != nil {
				t.Fatal(err)
			}
			if len(listData) != 1 {
				t.Fatalf("session list = %v, want one session", listData)
			}
			for _, got := range []map[string]any{detailData, listData[0]} {
				if _, ok := got["mru"]; !ok {
					t.Fatal("session output omits mru")
				}
				if _, ok := got["auth-method"]; !ok {
					t.Fatal("session output omits auth-method")
				}
				if tc.mru == 0 {
					if got["mru"] != nil || got["auth-method"] != nil {
						t.Fatalf("unknown negotiation = %v, want null MRU/method", got)
					}
					continue
				}
				if got["mru"] != float64(tc.mru) || got["auth-method"] != tc.method {
					t.Fatalf("session output = %v, want measured MRU %d method %s", got, tc.mru, tc.method)
				}
			}
		})
	}
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

// outgoingCtx builds a CommandContext with the two typed selectors the
// outgoing-call grammar captures.
func outgoingCtx(remote, called string) *pluginserver.CommandContext {
	return &pluginserver.CommandContext{Selectors: map[string]string{"remote": remote, "called": called}}
}

// VALIDATES: AC-4 -- outgoing-call handler returns the established session
// identifiers on success.
func TestHandleOutgoingCall_Success(t *testing.T) {
	publishFake(t, &fakeService{
		outgoingCall: func(remote, called string) (l2tppkg.OutgoingCallResult, error) {
			require.Equal(t, "lns1", remote)
			require.Equal(t, "5551234", called)
			return l2tppkg.OutgoingCallResult{
				Remote: remote, Called: called, Established: true, LocalSID: 11, RemoteSID: 22,
			}, nil
		},
	})
	resp, err := handleOutgoingCall(outgoingCtx("lns1", "5551234"), commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, true, got["established"])
	require.Equal(t, float64(11), got["local-sid"])
	require.Equal(t, float64(22), got["remote-sid"])
}

// VALIDATES: AC-4 -- a failed call surfaces StatusError plus the failure
// cause and RFC 2661 Result Code (tie-breaker loss / auth reject visibility).
func TestHandleOutgoingCall_FailureSurfacesResultCode(t *testing.T) {
	publishFake(t, &fakeService{
		outgoingCall: func(remote, called string) (l2tppkg.OutgoingCallResult, error) {
			return l2tppkg.OutgoingCallResult{Remote: remote, Called: called, ResultCode: 4},
				errors.New("tunnel authentication rejected")
		},
	})
	resp, err := handleOutgoingCall(outgoingCtx("lns1", "555"), commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "failed")
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(responseString(t, resp)), &got))
	require.Equal(t, false, got["established"])
	require.Equal(t, float64(4), got["result-code"])
}

// VALIDATES: AC-4 -- a missing selector is rejected with a clear error.
func TestHandleOutgoingCall_MissingArgs(t *testing.T) {
	publishFake(t, &fakeService{})
	resp, err := handleOutgoingCall(outgoingCtx("", "555"), commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "remote")

	resp, err = handleOutgoingCall(outgoingCtx("lns1", ""), commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "called")
}
