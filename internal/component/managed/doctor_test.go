package managed

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

// restoreHubProbe captures and restores the hub probe seam so an override does
// not leak into other tests. Tests using it MUST NOT call t.Parallel().
func restoreHubProbe(t *testing.T) {
	t.Helper()
	orig := hubProbe
	t.Cleanup(func() { hubProbe = orig })
}

// TestHubDoctorCheckUnreachable verifies the managed-hub doctor diagnostic:
// no client block is a no-op, an unreachable primary hub warns
// doctor-hub-unreachable, a reachable primary clears the warning, and -- because
// the daemon connects only to the FIRST client block -- a reachable secondary
// does NOT mask an unreachable primary.
//
// VALIDATES: startup-resilience AC-5 - an unreachable management hub is surfaced
// via doctor, not left silent (the only touchpoint that was log-only).
// PREVENTS: the check firing when no hub client is configured; failing to warn
// when the daemon's hub (Clients[0]) is down; or a reachable spare hub producing
// a false-healthy result (the daemon ignores Clients[1:]).
func TestHubDoctorCheckUnreachable(t *testing.T) {
	restoreHubProbe(t)

	clients := []plugin.HubClientConfig{
		{Name: "hub-a", Host: "203.0.113.9", Port: 8443},
		{Name: "hub-b", Host: "203.0.113.10", Port: 8443},
	}

	t.Run("no client configured is a no-op", func(t *testing.T) {
		hubProbe = func(string, string) bool {
			t.Fatal("probe must not run when no hub client is configured")
			return false
		}
		assert.Nil(t, diagnoseHubReachability(nil))
	})

	t.Run("primary hub unreachable warns", func(t *testing.T) {
		var probed []string
		hubProbe = func(addr, _ string) bool {
			probed = append(probed, addr)
			return false
		}
		diags := diagnoseHubReachability(clients)
		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-hub-unreachable", diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Equal(t, []string{"203.0.113.9:8443"}, probed,
			"only the daemon-selected (first) hub should be probed")
	})

	t.Run("primary hub reachable is clean", func(t *testing.T) {
		hubProbe = func(addr, _ string) bool { return addr == "203.0.113.9:8443" }
		assert.Nil(t, diagnoseHubReachability(clients))
	})

	t.Run("reachable secondary does not mask a down primary", func(t *testing.T) {
		// The daemon connects to Clients[0] (203.0.113.9). A reachable spare
		// (203.0.113.10) must NOT clear the warning: the daemon's hub is down.
		hubProbe = func(addr, _ string) bool { return addr == "203.0.113.10:8443" }
		diags := diagnoseHubReachability(clients)
		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-hub-unreachable", diags[0].Code)
	})
}

// TestCheckHubReachableNonTreeContext verifies the check is defensive against a
// missing or wrongly-typed config tree (returns no diagnostics rather than
// panicking).
//
// VALIDATES: startup-resilience - the doctor check never turns a missing tree
// into a false unreachable warning.
func TestCheckHubReachableNonTreeContext(t *testing.T) {
	assert.Nil(t, checkHubReachable(diagnostic.DoctorCheckContext{Tree: nil}))
	assert.Nil(t, checkHubReachable(diagnostic.DoctorCheckContext{Tree: "not a tree"}))
}

// TestHubReachableProbe exercises the real (non-seam) TCP probe: a live listener
// reads as reachable, a closed port reads as unreachable, both promptly.
//
// VALIDATES: startup-resilience AC-5 - hubReachable is a bounded TCP probe.
// PREVENTS: a probe that reports reachable on a refused connection.
func TestHubReachableProbe(t *testing.T) {
	origTimeout := hubProbeTimeout
	t.Cleanup(func() { hubProbeTimeout = origTimeout })
	hubProbeTimeout = 2 * time.Second

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	reachableAddr := ln.Addr().String()
	assert.True(t, hubReachable(reachableAddr, ""), "live listener should be reachable")

	// Close the listener; the same address now refuses connections promptly.
	require.NoError(t, ln.Close())
	assert.False(t, hubReachable(reachableAddr, ""), "closed port should be unreachable")
}

// TestHubClientDoctorCheck: a managed client whose configuration names no hub
// is reported, with the command that brings it back. A node that is not
// managed, and a managed client that names its hub, are not.
//
// VALIDATES: a client that applied a pushed config with no hub client block,
// and so stopped following its hub, is visible in `ze doctor`.
// PREVENTS: that state being one log line at the moment it happened and
// nothing afterwards.
func TestHubClientDoctorCheck(t *testing.T) {
	t.Run("managed with no hub block warns", func(t *testing.T) {
		diags := diagnoseHubClientDeclared(true, 0)
		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorHubClientMissing, diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Equal(t, hubClientMissingFix, diags[0].Fix)
	})
	t.Run("managed with a hub block is quiet", func(t *testing.T) {
		assert.Nil(t, diagnoseHubClientDeclared(true, 1))
	})
	t.Run("not managed is quiet", func(t *testing.T) {
		assert.Nil(t, diagnoseHubClientDeclared(false, 0))
	})
	t.Run("nothing to read is quiet", func(t *testing.T) {
		assert.Nil(t, checkHubClientDeclared(diagnostic.DoctorCheckContext{Tree: nil}))
		assert.Nil(t, checkHubClientDeclared(diagnostic.DoctorCheckContext{Tree: "not a tree"}))
	})
}

// divergedDoctorStore creates a device store that holds holdDeviceConfig as
// its configuration, the hash of earlier as its baseline when earlier is not
// empty, and flag as its managed flag when flag is not empty.
func divergedDoctorStore(t *testing.T, flag, earlier string) storage.Storage {
	t.Helper()
	store := holdDevice(t, holdDeviceConfig, earlier)
	if flag != "" {
		require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(flag)))
	}
	return store
}

// TestDivergedDoctorCheck proves when the managed-diverged check reports: it
// runs the registered check function over real stores of a managed client.
//
// VALIDATES: AC-8. A managed client whose configuration is not the last one
// its hub sent is reported as a warning under doctor-managed-diverged, with
// both hashes and the fix; one whose configuration is the baseline, and one
// with no baseline (D-6), report nothing.
// PREVENTS: a device that holds with nothing in `ze doctor` to say so, and a
// finding that says the configuration was edited when a recovery changed it.
func TestDivergedDoctorCheck(t *testing.T) {
	t.Run("managed and differing warns", func(t *testing.T) {
		store := divergedDoctorStore(t, managedFlagTrue, holdEarlierConfig)

		diags := managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store})

		require.Len(t, diags, 1)
		assert.Equal(t, CodeDoctorManagedDiverged, diags[0].Code)
		assert.Contains(t, managedDivergedDoctorCheck.Codes, diags[0].Code, "the check reports a code it does not declare")
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Equal(t, managedDivergedFix("", versionOf([]byte(holdDeviceConfig)).Hash()), diags[0].Fix,
			"a configuration that names no hub client keeps the placeholder")
		assert.Contains(t, diags[0].Message, "differs from the last one its hub sent")
		assert.Contains(t, diags[0].Message, versionOf([]byte(holdDeviceConfig)).Hash())
		assert.Contains(t, diags[0].Message, versionOf([]byte(holdEarlierConfig)).Hash())
		assert.NotContains(t, diags[0].Message, "edit")
	})
	t.Run("managed and equal to the baseline is quiet", func(t *testing.T) {
		store := divergedDoctorStore(t, managedFlagTrue, holdDeviceConfig)
		assert.Nil(t, managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store}))
	})
	t.Run("managed with no baseline is quiet", func(t *testing.T) {
		store := divergedDoctorStore(t, managedFlagTrue, "")
		assert.Nil(t, managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store}))
	})
	t.Run("no store is quiet", func(t *testing.T) {
		assert.Nil(t, managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{}))
	})
}

// TestDivergedDoctorFixNamesTheClient proves that the fix of the hold names
// the device's own configuration on its hub: the device runs a configuration
// whose hub client block is named edge-01, which differs from its baseline.
//
// VALIDATES: the fix names edge-01, the name the hub knows the client by, in
// each hub command (spec-fleet-7 E-12), and no placeholder.
// PREVENTS: a fix the operator must edit before it runs.
func TestDivergedDoctorFixNamesTheClient(t *testing.T) {
	store := holdDevice(t, fleetStatusConfig, holdEarlierConfig)
	require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(managedFlagTrue)))

	diags := managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store})

	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Fix, "show fleet divergence name edge-01;")
	assert.NotContains(t, diags[0].Fix, "<name>")
}

// TestDivergedFixNamesHubCommands proves the device's fix for a hold names
// the hub commands that end it, over the client name and the version the
// device runs, and that a configuration too large for the hub's copy names
// the manual routes instead.
//
// VALIDATES: spec-fleet-7 AC-21 and E-12: show, adopt and revert, each with
// the client and the active version; over fleet.HeldConfigMax, the import on
// the hub and the leave and rejoin on the device.
// PREVENTS: a fix that names commands the hub cannot answer for a hold it
// keeps no copy of, and one that names only the manual routes when the hub
// can resolve the hold.
// MUTATION: return managedDivergedManualFix from managedDivergedFix and the
// first case fails; drop the oversize branch of diagnoseManagedDiverged and
// the second fails.
func TestDivergedFixNamesHubCommands(t *testing.T) {
	t.Run("the hub keeps a copy", func(t *testing.T) {
		store := holdDevice(t, fleetStatusConfig, holdEarlierConfig)
		require.NoError(t, store.WriteKey(zefs.KeyInstanceManaged.Pattern, []byte(managedFlagTrue)))
		active := versionOf([]byte(fleetStatusConfig)).Hash()

		diags := managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store})

		require.Len(t, diags, 1)
		assert.Contains(t, diags[0].Fix, "show fleet divergence name edge-01")
		assert.Contains(t, diags[0].Fix, "update fleet divergence name edge-01 adopt version "+active)
		assert.Contains(t, diags[0].Fix, "request fleet divergence name edge-01 revert version "+active+" hub-version "+placeholderHubVersion)
		assert.NotContains(t, diags[0].Fix, "ze data import")
	})
	t.Run("too large for a copy", func(t *testing.T) {
		active := versionOf([]byte("active"))
		baseline := versionOf([]byte("baseline"))

		diags := diagnoseManagedDiverged(active, baseline, "edge-01", true)

		require.Len(t, diags, 1)
		assert.Equal(t, managedDivergedManualFix("edge-01"), diags[0].Fix)
		assert.Contains(t, diags[0].Fix, "ze data import client-edge-01.conf;")
		assert.NotContains(t, diags[0].Fix, "fleet divergence")
	})
}

// hubServingTree is a hub configuration whose one serving block declares
// each of clients.
func hubServingTree(t *testing.T, clients ...string) *config.Tree {
	t.Helper()
	text := "plugin {\n\thub {\n\t\tserver fleet {\n\t\t\tip 127.0.0.1;\n\t\t\tport 1790;\n" +
		"\t\t\tsecret \"fleet-server-secret-of-at-least-32ch\";\n"
	for _, name := range clients {
		text += "\t\t\tclient " + name + " {\n\t\t\t\tsecret \"" + name + "-secret-of-at-least-32-chars\";\n\t\t\t}\n"
	}
	text += "\t\t}\n\t}\n}\n"
	tree, err := config.ParseTreeForValidation(text)
	require.NoError(t, err)
	return tree
}

// heldHubStore is a hub store that serves name hubConfig and keeps a record
// of deviceConfig for it, as the config-held arm writes one.
func heldHubStore(t *testing.T, name, hubConfig, deviceConfig string) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	require.NoError(t, store.WriteFile(pluginserver.ClientConfigKey(name), []byte(hubConfig), 0o600))
	record, err := json.Marshal(map[string]any{
		"version":     fleet.VersionHash([]byte(deviceConfig)),
		"config":      []byte(deviceConfig),
		"edited-at":   time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC),
		"received-at": time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(pluginserver.HeldRecordKey(name), record))
	return store
}

// TestHubDivergedCheckDeclared proves the hub-side check over a real store
// record and a parsed serving block: a declared client's record is a warning
// naming the client, both versions and the three hub commands, and a hub with
// no record reports nothing.
//
// VALIDATES: spec-fleet-7 AC-20 and E-10: doctor-managed-client-diverged,
// read from the store, so it reports with the hub running or stopped; after
// adopt or revert removed the record, nothing for that client.
// PREVENTS: a holding client the hub keeps a copy of with nothing in ze doctor
// to say so once the refused-config row is gone.
// MUTATION: return nil from checkHubClientsDiverged and this fails.
func TestHubDivergedCheckDeclared(t *testing.T) {
	const hubConfig = "system { host hub-served; }\n"
	const deviceConfig = "system { host device-edited; }\n"
	store := heldHubStore(t, "edge-01", hubConfig, deviceConfig)
	device := fleet.VersionHash([]byte(deviceConfig))
	hub := fleet.VersionHash([]byte(hubConfig))

	diags := hubDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store, Tree: hubServingTree(t, "edge-01")})

	require.Len(t, diags, 1)
	assert.Equal(t, codeDoctorManagedClientDiverged, diags[0].Code)
	assert.Contains(t, hubDivergedDoctorCheck.Codes, diags[0].Code, "the check reports a code it does not declare")
	assert.Equal(t, diagnostic.DoctorPhasePostConfig, hubDivergedDoctorCheck.Phase)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "edge-01")
	assert.Contains(t, diags[0].Message, device)
	assert.Contains(t, diags[0].Message, hub)
	assert.NotContains(t, diags[0].Message, "device-edited", "the finding carries no configuration text")
	assert.Equal(t, divergenceCommandsFix("edge-01", device, hub), diags[0].Fix)
	assert.Contains(t, diags[0].Fix, "show fleet divergence name edge-01")
	assert.Contains(t, diags[0].Fix, "update fleet divergence name edge-01 adopt version "+device)
	assert.Contains(t, diags[0].Fix, "request fleet divergence name edge-01 revert version "+device+" hub-version "+hub)

	require.NoError(t, store.RemoveKey(pluginserver.HeldRecordKey("edge-01")))
	assert.Empty(t, hubDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store, Tree: hubServingTree(t, "edge-01")}),
		"a hub whose record was removed reports nothing for the client")
}

// TestHubDivergedCheckOrphan proves a record whose client no serving block
// declares is reported orphaned, with the fix that declares it again or
// removes the record with the hub stopped, and that a record that does not
// decode is reported rather than read as none.
//
// VALIDATES: spec-fleet-7 AC-19 and A-9: the serving blocks are read from the
// tree with config.ExtractHubConfig, the extraction the managed server is
// built from.
// PREVENTS: an orphaned record shown as a hold an operator could resolve from
// the hub, and the only copy of a client's configuration hidden by a record
// the check could not decode.
// MUTATION: drop the declared test of checkHubClientsDiverged and the first
// case fails; read a ReadHeldRecord error as no record and the second fails.
func TestHubDivergedCheckOrphan(t *testing.T) {
	t.Run("undeclared client", func(t *testing.T) {
		store := heldHubStore(t, "edge-01", "system { host hub-served; }\n", "system { host device-edited; }\n")

		diags := hubDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store, Tree: hubServingTree(t, "edge-02")})

		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorManagedClientDiverged, diags[0].Code)
		assert.Contains(t, diags[0].Message, "orphaned")
		assert.Contains(t, diags[0].Message, "edge-01")
		assert.Contains(t, diags[0].Fix, "declare client edge-01 again")
		assert.True(t, strings.HasSuffix(diags[0].Fix, "ze data rm meta/config/client-edge-01.conf/held"), diags[0].Fix)
	})
	t.Run("record that does not decode", func(t *testing.T) {
		store := heldHubStore(t, "edge-01", "system { host hub-served; }\n", "system { host device-edited; }\n")
		require.NoError(t, store.WriteKey(pluginserver.HeldRecordKey("edge-01"), []byte("{not json")))

		diags := hubDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store, Tree: hubServingTree(t, "edge-01")})

		require.Len(t, diags, 1)
		assert.Equal(t, codeDoctorManagedClientDiverged, diags[0].Code)
		assert.Contains(t, diags[0].Message, "could not be read")
		assert.Equal(t, fixCheckStore, diags[0].Fix)
	})
}

// TestDivergedDoctorSilentWhenUnmanaged proves the check is gated on the
// managed flag itself: it runs over stores whose configuration differs from a
// recorded baseline, with the flag false and with no flag.
//
// VALIDATES: AC-8 and AC-22. Outside a fleet a local change is not a problem.
// PREVENTS: a fleet warning on a device that left its fleet with `request
// fleet disable` and was then changed, which is what leaving is for.
func TestDivergedDoctorSilentWhenUnmanaged(t *testing.T) {
	for _, flag := range []string{"false", ""} {
		store := divergedDoctorStore(t, flag, holdEarlierConfig)
		assert.Nil(t, managedDivergedDoctorCheck.Check(diagnostic.DoctorCheckContext{Store: store}), "managed flag %q", flag)
	}
}

// TestDivergedDoctorReportsWhatItCannotCompare proves that a managed client
// whose store could not answer is reported, by building the finding from a
// read failure.
//
// VALIDATES: "cannot compare" is a warning under doctor-managed-diverged that
// carries the cause and the command that checks the store.
// PREVENTS: a store that cannot be read shown as a device that is in sync
// while its client applies nothing.
func TestDivergedDoctorReportsWhatItCannotCompare(t *testing.T) {
	diags := diagnoseManagedUncompared("the baseline", errors.New("the key did not decode"))

	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorManagedDiverged, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Equal(t, fixCheckStore, diags[0].Fix)
	assert.Contains(t, diags[0].Message, "the key did not decode")
}
