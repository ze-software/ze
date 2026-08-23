package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// registerClaimingPlugin registers a throwaway plugin that claims role, and
// unregisters it when the test ends.
func registerClaimingPlugin(t *testing.T, name, role string) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "claims " + role,
		Claims:      []string{role},
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
	}))
}

// TestAdvertiseClaimsCoversUnstartedClaimant is the engine half of the ordering
// guarantee.
//
// VALIDATES: advertiseClaims answers for a claimant that has been committed to
// a startup phase but has NOT yet handshaked. That is the case that matters:
// bgp-adj-rib-in is configured in an earlier dependency tier than bgp-rs (which
// declares it as an OptionalDependency, so TopologicalTiers orders the
// dependency first), so bgp-rs has not sent its own registration RPC when
// bgp-adj-rib-in reaches Stage 2. Sourcing the claim from the static
// registration is what makes the answer available that early.
//
// PREVENTS: a regression to resolving ownership from a runtime message, which
// cannot be ready in time and leaves the first peer racing (the duplicate
// announce in test/plugin/llgr-readvertise-multipeer).
func TestAdvertiseClaimsCoversUnstartedClaimant(t *testing.T) {
	const role = "test-exclusive-role"
	registerClaimingPlugin(t, "test-claimant", role)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	assert.Empty(t, s.advertiseClaims("test-stander-down"),
		"a plugin that is not in the startup set claims nothing")

	// runPluginPhase marks every plugin of a phase loaded before the tier
	// handshake begins, so this is the state at the standing-down plugin's
	// Stage 2 -- claimant committed, not yet started.
	s.markPluginLoaded("test-claimant")
	s.markPluginLoaded("test-stander-down")

	assert.Equal(t, []string{role}, s.advertiseClaims("test-stander-down"),
		"the claim must be advertised before the claimant has handshaked")
}

// TestDeliverConfigCarriesClaimToPlugin closes the one link of the ordering
// guarantee that nothing else owned.
//
// VALIDATES: the engine's own Stage-2 delivery puts the claimed roles on the
// configure message. That is what makes peer-up replay ownership resolved
// before the first session can establish: Stage 2 is inside the sequential
// startup handshake, so it completes before the plugin sends Stage-5 ready
// (TestSharedStartupDriverSinkDispatch pins that stage order), the phase
// completes only when every plugin has, and signalStartupComplete then calls
// SignalPluginStartupComplete -> StartPeers in straight-line code. There is no
// window.
//
// The racing alternative is absent here rather than out-run: no ProcessManager
// is registered, so sendPostStartupToAll -- the detached, deliberately un-awaited
// fan-out that used to carry this decision -- sends nothing at all. The claim
// arrives anyway, which is the property. Nothing here depends on that callback
// winning a race, because it never runs.
//
// PREVENTS: dropping the claims argument at the SendConfigure call site in
// deliverConfigRPC, or moving the decision back onto the post-startup fan-out.
// Either leaves bgp-adj-rib-in self-replaying alongside bgp-rs and announces a
// route twice to the first peer (test/plugin/rfc7606-relay-one-field,
// test/plugin/llgr-readvertise-multipeer). Every other test in the chain --
// advertiseClaims below, TestRPCConfigureCarriesClaims (plugin/ipc), the SDK's
// claims_test.go, bgp-adj-rib-in's TestReplayOwnerDedupe -- stays green through
// that edit: each owns one link and none owned the join.
//
// an earlier draft of this same test (uncommitted, this session)
// also asserted a sequence number recorded at the configure callback was lower
// than one recorded in a mock reactor's SignalPluginStartupComplete. That
// assertion could not fail: the test itself calls deliverConfigRPC before
// signalStartupComplete, so it pinned the test's own call order, not the
// product's (ai/rules/interop-and-goal-validation.md, vacuity traps). The
// ordering it was reaching for is owned by TestSharedStartupDriverSinkDispatch
// (DeliverConfig precedes OnReady) plus straight-line code, as stated above.
func TestDeliverConfigCarriesClaimToPlugin(t *testing.T) {
	const role = "test-exclusive-role"
	registerClaimingPlugin(t, "test-claimant", role)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engineConn, pluginMux := newDriverPipe(t)

	s := &Server{
		config:   &ServerConfig{},
		registry: plugin.NewPluginRegistry(),
		ctx:      ctx,
		loadedPlugins: map[string]bool{
			"test-claimant":     true,
			"test-stander-down": true,
		},
	}

	proc := process.NewProcess(plugin.PluginConfig{Name: "test-stander-down"})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(engineConn)

	// Plugin side: answer the Stage-2 configure callback, reporting what it carried.
	seen := make(chan []string, 1)
	go func() {
		var req *rpc.Request
		select {
		case req = <-pluginMux.Requests():
		case <-ctx.Done():
			return
		}
		var input rpc.ConfigureInput
		if err := json.Unmarshal(req.Params, &input); err != nil {
			return
		}
		seen <- input.Claims
		_ = pluginMux.SendOK(ctx, req.ID) //nolint:errcheck // test plugin side
	}()

	require.NoError(t, s.deliverConfigRPC(ctx, proc))

	select {
	case got := <-seen:
		// The engine's own delivery carries the claim -- not a later message,
		// and not a payload this test fabricated.
		assert.Equal(t, []string{role}, got,
			"deliverConfigRPC must put the claimed roles on the Stage-2 configure callback")
	case <-ctx.Done():
		t.Fatal("plugin never received the Stage-2 configure callback")
	}
}

// TestAdvertiseClaimsExcludesSelf pins that a claim never stands its own
// claimant down.
//
// VALIDATES: a claim means "I take this role over from another plugin", so the
// claimant must not be told its own role is claimed.
// PREVENTS: a claiming plugin disabling the very behavior it claimed.
func TestAdvertiseClaimsExcludesSelf(t *testing.T) {
	const role = "test-self-role"
	registerClaimingPlugin(t, "test-self-claimant", role)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	s.markPluginLoaded("test-self-claimant")

	assert.Empty(t, s.advertiseClaims("test-self-claimant"),
		"a claimant must not be told its own role is claimed")
	assert.Equal(t, []string{role}, s.advertiseClaims("someone-else"))
}

// TestAdvertiseClaimsFromExplicitConfig covers the claimant that is explicitly
// configured rather than auto-loaded, and so is known from the config before
// any phase runs.
//
// VALIDATES: an explicitly configured claimant is in the prospective set.
// PREVENTS: a phase-ordering hole where the standing-down plugin auto-loads in
// an earlier phase than the explicitly configured claimant.
func TestAdvertiseClaimsFromExplicitConfig(t *testing.T) {
	const role = "test-configured-role"
	registerClaimingPlugin(t, "test-configured-claimant", role)

	s := &Server{
		config: &ServerConfig{
			Plugins: []plugin.PluginConfig{{Name: "test-configured-claimant"}},
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	assert.Equal(t, []string{role}, s.advertiseClaims("test-stander-down"))
}

// TestAdvertiseClaimsResolvesRenamedImplementation covers the operator spelling
// the real tests use: `plugin { internal rs { use bgp-rs } }`.
//
// VALIDATES: a claim declared by the IMPLEMENTATION (registry name "bgp-rs") is
// advertised even though the plugin runs under the operator's chosen process
// name ("rs"), and the claimant is recorded under the process name so the
// backing check can find its process.
// PREVENTS: two bugs at once -- the claim silently never being advertised (so
// bgp-adj-rib-in keeps self-replaying and the duplicate announce returns), and
// verifyAdvertisedClaims declaring a perfectly live claimant "unbacked" and
// aborting a healthy daemon. test/plugin/rfc7606-relay-one-field.ci uses exactly
// this spelling.
func TestAdvertiseClaimsResolvesRenamedImplementation(t *testing.T) {
	const role = "test-renamed-role"
	registerClaimingPlugin(t, "bgp-renamed-claimant", role)

	s := &Server{
		config: &ServerConfig{
			Plugins: []plugin.PluginConfig{
				{Name: "shortname", Internal: true, Run: "bgp-renamed-claimant"},
			},
		},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	assert.Equal(t, []string{role}, s.advertiseClaims("test-stander-down"),
		"a claim declared by the implementation must survive the operator's rename")

	// The claimant must be recorded under the PROCESS name, which is what
	// ProcessManager.GetProcess keys on.
	s.advertisedClaimsMu.Lock()
	claimants := s.advertisedClaims[role]
	s.advertisedClaimsMu.Unlock()
	assert.True(t, claimants["shortname"],
		"the claimant must be recorded under its process name, not its registry name")

	assert.Empty(t, s.advertiseClaims("shortname"),
		"the claimant must not be told its own role is claimed, under its process name")
}

// TestUnadvertisedClaimsNeverFailStartup pins that the fail-closed check is
// inert for every daemon that has no claiming plugin -- the overwhelmingly
// common case.
//
// VALIDATES: verifyAdvertisedClaims leaves startupErr alone when nothing was
// advertised.
// PREVENTS: turning an unrelated plugin startup failure into a daemon abort.
func TestUnadvertisedClaimsNeverFailStartup(t *testing.T) {
	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}

	assert.Empty(t, s.unbackedClaims())
	s.verifyAdvertisedClaims()
	assert.NoError(t, s.startupErr)
}

// TestUnbackedClaimIsDetectedAndReportedNotFatal pins both halves of the
// unbacked-claim contract.
//
// VALIDATES: when the engine told a plugin a role was claimed but the claimant
// never reached Running, unbackedClaims names the role (so verifyAdvertisedClaims
// can say so), and verifyAdvertisedClaims does NOT turn that into a startup
// failure.
//
// PREVENTS: two opposite regressions.
//   - Silence: advertising a claim at Stage 2 is a promise about a plugin that
//     has not started yet, and the plugin that received it has already stood its
//     own default down. If the claimant then fails, nobody performs the role and
//     nothing else in the daemon would mention it.
//   - Over-reaction: making this fatal was tried on 2026-07-25 and is
//     disproportionate. A claimant reaches Running only if its whole startup
//     phase succeeded, so ANY unrelated plugin failure in that phase -- an
//     unknown plugin name in the operator's config, a plugin dying at Stage 1 --
//     left the claimant short of Running. Aborting on that killed daemons that
//     previously survived and turned 25 unrelated functional tests red
//     (bgp-redistribute-*, fib-*, forward-*). A missing replay in an
//     already-degraded daemon is a smaller failure than no daemon at all.
func TestUnbackedClaimIsDetectedAndReportedNotFatal(t *testing.T) {
	const role = "test-unbacked-role"
	registerClaimingPlugin(t, "test-absent-claimant", role)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	s.markPluginLoaded("test-absent-claimant")

	// The standing-down plugin is configured and told the role is claimed.
	require.Equal(t, []string{role}, s.advertiseClaims("test-stander-down"))

	// The claimant never produced a running process, so the claim is unbacked.
	assert.Equal(t, []string{role}, s.unbackedClaims())

	s.verifyAdvertisedClaims()
	assert.NoError(t, s.startupErr,
		"an unbacked claim must be reported, never turned into a daemon abort")
}

// TestVerifyAdvertisedClaimsLeavesStartupErrUntouched pins that the claim check
// neither creates nor clears a startup verdict.
//
// VALIDATES: an existing startupErr is preserved verbatim across
// verifyAdvertisedClaims.
// PREVENTS: the claim check masking the real, more specific startup failure that
// a phase already recorded -- which is also the failure that explains why the
// claimant is missing in the first place.
func TestVerifyAdvertisedClaimsLeavesStartupErrUntouched(t *testing.T) {
	const role = "test-first-error-role"
	registerClaimingPlugin(t, "test-first-error-claimant", role)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	s.markPluginLoaded("test-first-error-claimant")
	require.NotEmpty(t, s.advertiseClaims("test-stander-down"))
	require.NotEmpty(t, s.unbackedClaims(), "precondition: the claim is unbacked")

	original := assert.AnError
	s.startupErr = original

	s.verifyAdvertisedClaims()
	assert.Same(t, original, s.startupErr, "the first failure is the useful one")
}

// TestVerifyAdvertisedClaimsSpeaks pins the ONLY observable effect this guard
// has: the ERROR it logs.
//
// VALIDATES: an unbacked claim is reported, naming the role, via the log.
// PREVENTS: the log body being dropped or downgraded in a refactor. Every other
// test in this file asserts on unbackedClaims() -- a different function -- and on
// startupErr, which this never touches, so deleting the loop body left the suite
// green while the guard fell silent. A guard that cannot deny MUST speak
// (ai/rules/evidence.md); an untested log is not speech.
func TestVerifyAdvertisedClaimsSpeaks(t *testing.T) {
	orig := logger
	t.Cleanup(func() { logger = orig })

	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger = func() *slog.Logger { return captured }

	s := &Server{}
	s.recordAdvertisedClaim("bgp-peer-up-replay", "bgp-rs")
	s.verifyAdvertisedClaims()

	out := buf.String()
	if !strings.Contains(out, "bgp-peer-up-replay") {
		t.Fatalf("the role must be named in the log, got %q", out)
	}
	if !strings.Contains(out, "level=ERROR") {
		t.Fatalf("an unbacked claim must be reported at ERROR, got %q", out)
	}

	// The backed case must stay silent, or the ERROR is noise operators learn to
	// ignore. With no advertisement there is nothing to report.
	buf.Reset()
	quiet := &Server{}
	quiet.verifyAdvertisedClaims()
	if buf.Len() != 0 {
		t.Fatalf("no advertisement must produce no output, got %q", buf.String())
	}
}

// TestUnheldRolesRetractsAClaimForAPeerTheClaimantIsNotFed pins the per-event
// retraction of a daemon-wide claim.
//
// VALIDATES: UnheldRoles names an advertised role when no process in the
// delivery set holds it, and names nothing when one of them does.
// PREVENTS: a peer whose config attaches bgp-adj-rib-in but not bgp-rs being
// served by nobody -- adj-rib-in stands down for the daemon-wide claim while
// bgp-rs, taking no delivery of that peer's state, never replays it and never
// makes it a forward target (rs/server_forward.go selectForwardTargets).
func TestUnheldRolesRetractsAClaimForAPeerTheClaimantIsNotFed(t *testing.T) {
	const role = "bgp-peer-up-replay"

	owner := process.NewProcess(plugin.PluginConfig{Name: "bgp-rs"})
	stoodDown := process.NewProcess(plugin.PluginConfig{Name: "bgp-adj-rib-in"})

	t.Run("nothing advertised retracts nothing", func(t *testing.T) {
		s := &Server{}
		assert.Empty(t, s.UnheldRoles([]*process.Process{stoodDown}),
			"a daemon where no plugin claims a role has nothing to retract")
	})

	t.Run("claimant fed this peer holds the role", func(t *testing.T) {
		s := &Server{}
		s.recordAdvertisedClaim(role, "bgp-rs")
		assert.Empty(t, s.UnheldRoles([]*process.Process{stoodDown, owner}),
			"the claimant takes delivery, so the Stage-2 promise holds for this peer")
	})

	t.Run("claimant not fed this peer holds nothing", func(t *testing.T) {
		s := &Server{}
		s.recordAdvertisedClaim(role, "bgp-rs")
		assert.Equal(t, []string{role}, s.UnheldRoles([]*process.Process{stoodDown}),
			"the claimant takes no delivery, so the plugin that stood down must act")
	})

	t.Run("an empty delivery set holds nothing", func(t *testing.T) {
		s := &Server{}
		s.recordAdvertisedClaim(role, "bgp-rs")
		assert.Equal(t, []string{role}, s.UnheldRoles(nil),
			"an event delivered to nobody is held by nobody")
	})

	t.Run("one claimant of several is enough", func(t *testing.T) {
		s := &Server{}
		s.recordAdvertisedClaim(role, "bgp-rs")
		s.recordAdvertisedClaim(role, "rs-under-another-name")
		second := process.NewProcess(plugin.PluginConfig{Name: "rs-under-another-name"})
		assert.Empty(t, s.UnheldRoles([]*process.Process{second}),
			"the role has a holder here, whichever claimant it is")
	})

	t.Run("every advertised role is judged on its own", func(t *testing.T) {
		s := &Server{}
		s.recordAdvertisedClaim(role, "bgp-rs")
		s.recordAdvertisedClaim("some-other-role", "bgp-adj-rib-in")
		assert.Equal(t, []string{role}, s.UnheldRoles([]*process.Process{stoodDown}),
			"the role whose claimant IS fed must not be retracted with the one that is not")
	})
}

// TestUnheldRolesOverTheRealDeliverySet drives the retraction from the operator
// input that produces it: a peer's `attach process` blocks.
//
// VALIDATES: the set UnheldRoles judges is the set the event is delivered to,
// so a peer that attaches the stood-down plugin and not the claimant retracts
// the role, and one that attaches both does not.
// PREVENTS: the retraction being computed over the registry, the process table,
// or the delivery graph alone. Each of those answers the same for every peer,
// which is the daemon-wide answer the claim already gives.
func TestUnheldRolesOverTheRealDeliverySet(t *testing.T) {
	const (
		role      = "bgp-peer-up-replay"
		owner     = "bgp-rs"
		stoodDown = "bgp-adj-rib-in"
		bothPeer  = "10.0.0.1"
		alonePeer = "10.0.0.2"
	)

	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	s.recordAdvertisedClaim(role, owner)

	ns, _, stateET := graphIDs(t)
	ownerProc := process.NewProcess(plugin.PluginConfig{Name: owner})
	stoodDownProc := process.NewProcess(plugin.PluginConfig{Name: stoodDown})
	for _, proc := range []*process.Process{ownerProc, stoodDownProc} {
		s.Subscriptions().Add(proc, &Subscription{
			Namespace: ns,
			EventType: stateET,
			Direction: events.DirUnspecified,
		})
	}

	s.UpdateDeliveryGraph(graphNS, []DeliveryPeer{
		{
			Addr: bothPeer,
			Bindings: []plugin.PeerProcessBinding{
				{PluginName: owner, ReceiveAll: true},
				{PluginName: stoodDown, ReceiveAll: true},
			},
		},
		{
			Addr:     alonePeer,
			Bindings: []plugin.PeerProcessBinding{{PluginName: stoodDown, ReceiveAll: true}},
		},
	})

	unheldFor := func(peerAddr string) []string {
		procs := s.PeerScopedProcs(ns, stateET, events.DirUnspecified, peerAddr, "")
		require.NotEmpty(t, procs, "the fixture must feed %s something", peerAddr)
		return s.UnheldRoles(procs)
	}

	assert.Empty(t, unheldFor(bothPeer),
		"the peer attaches the claimant, so the claim holds for it")
	assert.Equal(t, []string{role}, unheldFor(alonePeer),
		"the peer attaches only the plugin that stood down, so nothing here holds the role")
}
