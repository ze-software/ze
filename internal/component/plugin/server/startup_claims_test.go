package server

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
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
