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
	"testing"

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
