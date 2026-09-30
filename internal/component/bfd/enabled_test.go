// Design: bfd.go -- the bfd > enabled master switch and pluginService.EnsureSession
//
// The goal is the operator's reading of bfd > enabled: false means no BFD, and
// the method is the path a BGP peer takes. Each case starts from the JSON the
// config tree delivers for the operator's bfd container, applies it the way the
// plugin lifecycle applies it, and then asks for a session the way the reactor
// asks for one (peer_bfd.go calls Service.EnsureSession).
package bfd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// peerRequest is the request a BGP peer with a bfd container produces.
var peerRequest = api.SessionRequest{
	Peer:  netip.MustParseAddr("192.0.2.1"),
	Local: netip.MustParseAddr("192.0.2.2"),
	Mode:  api.SingleHop,
}

// serviceForConfig applies the operator's bfd container and returns the Service
// the BGP reactor reaches.
//
// The single-hop loop is pre-populated so that a build which ignores the switch
// hands out a session rather than failing to bind a socket: the case must fail
// on the switch and on nothing else.
func serviceForConfig(t *testing.T, data string) *pluginService {
	t.Helper()

	cfg, err := parseSections([]sdk.ConfigSection{{Root: configRoot, Data: data}})
	if err != nil {
		t.Fatalf("parseSections: %v", err)
	}

	state := newRuntimeState()
	runtimeStateGuard.Lock()
	err = state.applyConfig(cfg)
	runtimeStateGuard.Unlock()
	if err != nil {
		t.Fatalf("applyConfig: %v", err)
	}

	lb, _ := transport.Pair(api.SingleHop, peerRequest.Local, peerRequest.Peer)
	state.loops[loopKey{vrf: api.DefaultVRF, mode: api.SingleHop}] = engine.NewLoop(lb, clock.RealClock{})
	mb, _ := transport.Pair(api.MultiHop, peerRequest.Local, peerRequest.Peer)
	state.loops[loopKey{vrf: api.DefaultVRF, mode: api.MultiHop}] = engine.NewLoop(mb, clock.RealClock{})

	return &pluginService{state: state}
}

// TestDisabledBFDRefusesTheSessionABGPPeerAsksFor covers false. The YANG leaf
// calls itself the master switch for the plugin, so a runtime client is refused
// exactly as a pinned session is stopped.
func TestDisabledBFDRefusesTheSessionABGPPeerAsksFor(t *testing.T) {
	svc := serviceForConfig(t, `{"bfd":{"enabled":"false"}}`)

	handle, err := svc.EnsureSession(peerRequest)
	if handle != nil {
		t.Fatalf("EnsureSession returned a handle while bfd > enabled is false")
	}
	if !errors.Is(err, errBfdDisabled) {
		t.Fatalf("EnsureSession error = %v, want %v", err, errBfdDisabled)
	}
}

// TestEnabledBFDGivesTheSessionABGPPeerAsksFor covers true, which is the other
// polarity of the same switch. Without it a build that refused every request
// would pass the case above.
func TestEnabledBFDGivesTheSessionABGPPeerAsksFor(t *testing.T) {
	svc := serviceForConfig(t, `{"bfd":{"enabled":"true"}}`)

	handle, err := svc.EnsureSession(peerRequest)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if handle == nil {
		t.Fatal("EnsureSession returned no handle while bfd > enabled is true")
	}
	if relErr := svc.ReleaseSession(handle); relErr != nil {
		t.Fatalf("ReleaseSession: %v", relErr)
	}
}

// TestUnconfiguredBFDSaysSoRatherThanAnsweringDisabled separates the two
// silences an absent configuration and a disabled plugin would otherwise share.
// A nil config is a Ze defect rather than an operator choice, and the operator
// reading the reactor's log needs to be told which one happened.
func TestUnconfiguredBFDSaysSoRatherThanAnsweringDisabled(t *testing.T) {
	svc := &pluginService{state: newRuntimeState()}

	handle, err := svc.EnsureSession(peerRequest)
	if handle != nil {
		t.Fatal("EnsureSession returned a handle before any configuration was applied")
	}
	if !errors.Is(err, errBfdNotConfigured) {
		t.Fatalf("EnsureSession error = %v, want %v", err, errBfdNotConfigured)
	}
}

// profileConfigJSON defines profile "fast" at values no default produces: a
// detect multiplier of 5 and a 300ms receive interval.
const profileConfigJSON = `{"bfd":{"enabled":"true","profile":{"fast":{` +
	`"detect-multiplier":"5","desired-min-tx-us":"300000","required-min-rx-us":"300000"}}}}`

// TestProfileNamedByAClientSetsTheSessionTimers covers the path a BGP peer's
// `connection bfd { profile fast; }` and a static next-hop take: the client
// names a profile and carries no timers, and EnsureSession resolves it. The
// method reads the session back through Snapshot, the table `show bfd sessions`
// prints. Before the profile was resolved there, the session ran its whole life
// at the slow-start one second and the default multiplier of 3, so its
// Detection Time was 3s where the operator configured 1.5s.
func TestProfileNamedByAClientSetsTheSessionTimers(t *testing.T) {
	svc := serviceForConfig(t, profileConfigJSON)

	req := peerRequest
	req.Profile = "fast"
	handle, err := svc.EnsureSession(req)
	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	t.Cleanup(func() {
		if relErr := svc.ReleaseSession(handle); relErr != nil {
			t.Errorf("ReleaseSession: %v", relErr)
		}
	})

	sessions := svc.Snapshot()
	if len(sessions) != 1 {
		t.Fatalf("Snapshot has %d sessions, want 1", len(sessions))
	}
	got := sessions[0]
	if got.DetectMult != 5 {
		t.Errorf("DetectMult = %d, want 5 from profile fast", got.DetectMult)
	}
	// RFC 5880 Section 6.8.4: detect_time = remote_detect_mult *
	// max(local RequiredMinRx, remote DesiredMinTx). No packet has arrived,
	// so the local multiplier and the 300ms local RequiredMinRx decide it.
	if got.DetectionInterval != 1500*time.Millisecond {
		t.Errorf("DetectionInterval = %s, want 1.5s from profile fast", got.DetectionInterval)
	}
	if got.Profile != "fast" {
		t.Errorf("Profile = %q, want fast", got.Profile)
	}
}

// TestUndefinedProfileRefusesTheSession is the negative polarity: a name the
// config does not define gets no session, rather than a session at timers the
// operator never configured.
func TestUndefinedProfileRefusesTheSession(t *testing.T) {
	svc := serviceForConfig(t, profileConfigJSON)

	req := peerRequest
	req.Profile = "missing"
	handle, err := svc.EnsureSession(req)
	if handle != nil {
		t.Fatal("EnsureSession returned a handle for an undefined profile")
	}
	if err == nil || !strings.Contains(err.Error(), `profile "missing" is not defined`) {
		t.Fatalf("EnsureSession error = %v, want the undefined-profile refusal", err)
	}
}

// modeProfileConfigJSON defines one passive profile and one echo profile, so a
// client request names a profile its hop mode forbids.
const modeProfileConfigJSON = `{"bfd":{"enabled":"true","profile":{` +
	`"quiet":{"passive":"true"},` +
	`"echoing":{"echo":{"desired-min-echo-tx-us":"50000"}}}}}`

// ensureProfileSession asks for a session in mode that names profile, the way a
// BGP peer's `connection bfd { profile ... }` or a static next-hop asks, and
// releases any handle it gets at cleanup.
func ensureProfileSession(t *testing.T, mode api.HopMode, profile string) (api.SessionHandle, error) {
	t.Helper()
	svc := serviceForConfig(t, modeProfileConfigJSON)
	req := peerRequest
	req.Mode = mode
	req.Profile = profile
	handle, err := svc.EnsureSession(req)
	if handle != nil {
		t.Cleanup(func() {
			if relErr := svc.ReleaseSession(handle); relErr != nil {
				t.Errorf("ReleaseSession: %v", relErr)
			}
		})
	}
	return handle, err
}

// RFC requirement: RFC5881-3-1 negative -- both sides of a single-hop session
// MUST take the Active role, so pluginService.EnsureSession refuses a client
// request (a BGP peer, a static next-hop) that names a passive profile in
// single-hop mode. Before the check, resolveProfile copied passive into the
// request and the engine created a Passive single-hop session.
func TestRFC5881ClientSingleHopPassiveProfileRefused(t *testing.T) {
	handle, err := ensureProfileSession(t, api.SingleHop, "quiet")
	if handle != nil {
		t.Fatal("EnsureSession returned a Passive single-hop session; RFC 5881 Section 3 requires the Active role")
	}
	if err == nil || !strings.Contains(err.Error(), `profile "quiet" sets passive`) {
		t.Fatalf("EnsureSession error = %v, want the single-hop passive refusal", err)
	}
}

// RFC requirement: RFC5881-3-1 positive -- the refusal is scoped to single hop:
// the same passive profile on a multi-hop client request gets a session that
// takes the Passive role (RFC 5883 Section 4.3 permits it), so a check that
// refused every passive profile would fail here.
func TestRFC5881ClientMultiHopPassiveProfileAccepted(t *testing.T) {
	handle, err := ensureProfileSession(t, api.MultiHop, "quiet")
	if err != nil {
		t.Fatalf("EnsureSession refused a passive multi-hop request: %v", err)
	}
	if handle == nil {
		t.Fatal("EnsureSession returned no handle for a passive multi-hop request")
	}
}

// RFC requirement: RFC5883-3-1 negative -- the Echo function MUST NOT be used
// over multiple hops, so pluginService.EnsureSession refuses a multi-hop client
// request that names an echo profile. Before the check, resolveProfile copied
// the echo interval into the request and the session advertised a non-zero
// Required Min Echo RX Interval on a multi-hop path.
func TestRFC5883ClientMultiHopEchoProfileRefused(t *testing.T) {
	handle, err := ensureProfileSession(t, api.MultiHop, "echoing")
	if handle != nil {
		t.Fatal("EnsureSession returned a multi-hop session with echo; RFC 5883 Section 3 forbids it")
	}
	if err == nil || !strings.Contains(err.Error(), `profile "echoing" enables echo`) {
		t.Fatalf("EnsureSession error = %v, want the multi-hop echo refusal", err)
	}
}

// RFC requirement: RFC5883-3-1 positive -- the echo refusal is scoped to multi
// hop: the same echo profile on a single-hop client request gets a session, as
// RFC 5881 permits Echo over a single hop.
func TestRFC5883ClientSingleHopEchoProfileAccepted(t *testing.T) {
	handle, err := ensureProfileSession(t, api.SingleHop, "echoing")
	if err != nil {
		t.Fatalf("EnsureSession refused a single-hop echo request: %v", err)
	}
	if handle == nil {
		t.Fatal("EnsureSession returned no handle for a single-hop echo request")
	}
}

// TestCommitProfileCheckMatchesSessionStart covers api.CheckProfile, the seam a
// BGP peer and a static next-hop call at commit. It answers from the bfd
// plugin's own parser and resolveProfile, so each case below is refused at
// commit for exactly the reason EnsureSession would refuse it at session start.
func TestCommitProfileCheckMatchesSessionStart(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		profile string
		mode    api.HopMode
		refusal string
	}{
		{"defined profile", modeProfileConfigJSON, "echoing", api.SingleHop, ""},
		{"passive multi-hop", modeProfileConfigJSON, "quiet", api.MultiHop, ""},
		{"undefined profile", modeProfileConfigJSON, "typo", api.SingleHop, `profile "typo" is not defined`},
		{"no bfd section", "", "quiet", api.MultiHop, "the config has no bfd section"},
		{"passive single-hop", modeProfileConfigJSON, "quiet", api.SingleHop, `profile "quiet" sets passive`},
		{"echo multi-hop", modeProfileConfigJSON, "echoing", api.MultiHop, `profile "echoing" enables echo`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := api.CheckProfile(tc.data, tc.profile, tc.mode)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("CheckProfile refused an acceptable profile: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("CheckProfile error = %v, want %q", err, tc.refusal)
			}
		})
	}
}
