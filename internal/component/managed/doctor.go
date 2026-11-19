// Design: ai/rules/repo-maintenance.md -- management hub reachability readiness check
// Related: client.go -- the managed client whose hub dependency this check guards
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
// Related: baseline.go -- ManagedFlag, ActiveVersion, BaselineVersion and Diverged, the one reader of each fact the checks judge

// The managed component owns the plugin/hub/client dependency (a node's outbound
// connection to a management hub), so it owns this doctor check
// (ai/rules/repo-maintenance.md "Components that are not plugins"). It mirrors the
// radius component doctor pattern. The check is a STATELESS config-tree
// reachability probe, not a read of live connection state: `ze doctor` runs as a
// separate process from the daemon (DoctorCheckContext carries only the parsed
// config, not runtime state), so it reads the hub address from the config tree
// via config.ExtractHubConfig -- the same source cmd/ze uses at startup
// (extractManagedClientConfig) -- and probes it.
package managed

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/pkg/zefs"
)

// hubProbeTimeout bounds the per-hub TCP reachability probe. A var so tests can
// shrink it; not a config surface.
var hubProbeTimeout = 3 * time.Second

// hubProbe is a test seam over hubReachable so doctor_test.go can drive the
// diagnostic logic without real network I/O (radiusAdminProbe convention).
var hubProbe = hubReachable

// doctorComponent is the component every check of this package reports under,
// and doctorNeedsConfigTree the dependency each one declares: all three read
// the configuration tree.
const (
	doctorComponent       = "managed"
	doctorNeedsConfigTree = "config-tree"
	// codeHubUnreachable is the code hubDoctorCheck declares. A hub exchange
	// that fails on a live connection logs under it too.
	codeHubUnreachable = "doctor-hub-unreachable"
)

// hubDoctorCheck is the readiness check registered from register.go. It reads
// the parsed config tree, so it depends on config-tree and runs post-config.
var hubDoctorCheck = diagnostic.DoctorCheck{
	Name:         "hub-unreachable",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        730,
	Component:    doctorComponent,
	Dependencies: []string{doctorNeedsConfigTree},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeHubUnreachable},
	Check:        checkHubReachable,
}

// hubClientDoctorCheck reports a managed client whose configuration names no
// hub. Registered from register.go.
var hubClientDoctorCheck = diagnostic.DoctorCheck{
	Name:         "hub-client-missing",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        731,
	Component:    doctorComponent,
	Dependencies: []string{doctorNeedsConfigTree},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorHubClientMissing},
	Check:        checkHubClientDeclared,
}

// managedDivergedDoctorCheck reports a managed client whose configuration is
// not the last one its hub gave it. Registered from register.go. It reads the
// store alone, which answers the same in the operator's process and in the
// daemon's, so it is a post-config check and reports with the daemon running
// or stopped.
var managedDivergedDoctorCheck = diagnostic.DoctorCheck{
	Name:         "managed-diverged",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        732,
	Component:    doctorComponent,
	Dependencies: []string{doctorNeedsConfigTree},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorManagedDiverged},
	Check:        checkManagedDiverged,
}

// managedDivergedFix names the two ways out of a hold: give the hub the
// configuration this device runs, or leave the fleet, return to the hub's
// configuration and rejoin. Each ends at a fetch the hub answers "current",
// which is what records a new baseline. client is the name the device's hub
// client block gives it, which names its configuration on the hub
// (client-<name>.conf); empty when the configuration names no client, and the
// fix then shows the placeholder.
func managedDivergedFix(client string) string {
	if client == "" {
		client = "<name>"
	}
	return "to keep this device's configuration, on the hub: ze data import client-" + client + ".conf; " +
		"to discard it, on this device: request fleet disable, roll back to the version the hub sent, request fleet enable"
}

// checkManagedDiverged warns when this node is a managed client and the
// configuration it runs differs from its baseline. The managed client then
// holds: it applies nothing its hub sends.
//
// A node that is not managed reports nothing, whatever its store holds:
// outside a fleet a local change is not a problem. A flag that cannot be read
// reports nothing either, as in checkHubClientDeclared. A managed node whose
// baseline or configuration cannot be read is reported: its client applies
// nothing while that lasts (refuseUncompared), so "cannot compare" is never
// shown as "not diverged".
func checkManagedDiverged(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		return nil
	}
	managed, err := ManagedFlag(ctx.Store)
	if err != nil {
		return nil // The managed flag cannot be read: this check has nothing to judge.
	}
	if !managed {
		return nil
	}
	content, stored, err := activeConfig(ctx.Store)
	if err != nil {
		return diagnoseManagedUncompared("the configuration of this device", err)
	}
	baseline, err := BaselineVersion(ctx.Store)
	if err != nil {
		return diagnoseManagedUncompared("the baseline ("+zefs.KeyManagedBaseVersion.Pattern+")", err)
	}
	var active ConfigVersion
	if stored {
		active = versionOf(content)
	}
	return diagnoseManagedDiverged(active, baseline, clientNameOf(content))
}

// clientNameOf answers the name the hub client block of content gives this
// device, or empty when content names none or its block cannot be read: the
// name only fills the fix, which then shows the placeholder.
func clientNameOf(content []byte) string {
	endpoint, err := DeclaredEndpoint(content)
	if err != nil {
		return "" // No client name to show: the fix keeps its placeholder.
	}
	return endpoint.Name
}

// diagnoseManagedDiverged is checkManagedDiverged on what a managed node's
// store answered. The text says the configuration differs and never that it
// was edited: a recovery or a confirmed-commit revert changes it with no
// operator involved.
// client is the device's name on its hub, empty when unknown.
func diagnoseManagedDiverged(active, baseline ConfigVersion, client string) []diagnostic.Diagnostic {
	if !Diverged(active, baseline) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     diagnostic.CodeDoctorManagedDiverged,
		Severity: diagnostic.SeverityWarning,
		Message: "the configuration this managed client runs (version " + active.Hash() + ") differs from the last one its hub sent (version " +
			baseline.Hash() + "), so it holds: it applies nothing the hub sends",
		Help: "the hold ends when the hub holds the same configuration as this device; show fleet status reports both hashes",
		Fix:  managedDivergedFix(client),
	}}
}

// diagnoseManagedUncompared reports a managed node whose store could not
// answer what, one of the two values the hold compares.
func diagnoseManagedUncompared(what string, cause error) []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{{
		Code:     diagnostic.CodeDoctorManagedDiverged,
		Severity: diagnostic.SeverityWarning,
		Message: what + " could not be read from the store, so this managed client cannot compare it with the last configuration its hub sent " +
			"and applies nothing the hub sends: " + cause.Error(),
		Fix: fixCheckStore,
	}}
}

// hubClientMissingFix is what brings a managed client with no hub block back
// under its hub: provision it again, so its next start fetches the config the
// hub serves, once that config holds the block.
const hubClientMissingFix = "ze init --managed --force"

// checkHubClientDeclared warns when this node is a managed client and its
// configuration has no plugin/hub/client block. Such a node connects to no
// hub and is never told of a change, and the running client says so only
// once, in its log, when it applies that configuration.
//
// It answers nothing when it cannot look: no store, no managed flag it can
// read, or a hub block config validation refuses. Each of those is another
// check's finding, and "no hub block" is claimed only from a tree that was
// read.
func checkHubClientDeclared(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if ctx.Store == nil {
		return nil
	}
	managed, err := ManagedFlag(ctx.Store)
	if err != nil {
		return nil // The managed flag cannot be read: this check has nothing to judge.
	}
	hubCfg, err := config.ExtractHubConfig(tree)
	if err != nil {
		return nil // Config validation reports a hub block it cannot read.
	}
	return diagnoseHubClientDeclared(managed, len(hubCfg.Clients))
}

// diagnoseHubClientDeclared is checkHubClientDeclared on what it read.
func diagnoseHubClientDeclared(managed bool, clientBlocks int) []diagnostic.Diagnostic {
	if !managed {
		return nil
	}
	if clientBlocks > 0 {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     diagnostic.CodeDoctorHubClientMissing,
		Severity: diagnostic.SeverityWarning,
		Message: "this node is a managed client and its configuration has no plugin hub client block, " +
			"so it follows no hub and receives no configuration change",
		Help: "add the client block to this node's configuration on the hub, then provision the node again so that it fetches it",
		Fix:  hubClientMissingFix,
	}}
}

// checkHubReachable warns when this node is configured as a managed client
// (plugin/hub/client block) but none of its hubs answers a TCP probe, so an
// operator sees the degraded hub connection via `ze doctor` instead of only in
// the daemon logs (startup-resilience AC-5). No-op when no hub client is
// configured; a malformed hub block is left to config validation, not flagged
// here as unreachable.
func checkHubReachable(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	hubCfg, err := config.ExtractHubConfig(tree)
	if err != nil {
		return nil
	}
	return diagnoseHubReachability(hubCfg.Clients)
}

// diagnoseHubReachability probes the management hub the daemon will connect to
// and warns when it is unreachable. No-op when the node is not a managed client
// (no client blocks).
//
// The daemon connects to the FIRST client block only
// (cmd/ze/ze_core_start.go extractManagedClientConfig -> hubCfg.Clients[0]), so
// that hub's reachability is what determines whether managed config sync will
// work. Probing "any hub reachable" would report healthy while the daemon's
// actual hub is down, hiding the dead server this check exists to surface (R-2).
func diagnoseHubReachability(clients []plugin.HubClientConfig) []diagnostic.Diagnostic {
	if len(clients) == 0 {
		return nil
	}
	primary := clients[0]
	if hubProbe(primary.Address(), primary.SourceAddress) {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeHubUnreachable,
		Severity: diagnostic.SeverityWarning,
		Message:  "the configured management hub is not reachable",
		Fix: "start the hub at " + primary.Address() + ", or correct the host and port of this device's " +
			"plugin hub client block in its configuration on the hub; ze doctor on the hub reports a managed listener that did not bind",
	}}
}

// hubReachable reports whether a TCP connection to the hub address can be
// established within hubProbeTimeout. A bounded dial only: TLS and auth are the
// running daemon's job; this probe answers the reachability question the
// startup-resilience invariant cares about (an unreachable peer must not be
// silent). SourceAddress is honored to match the daemon's dial path.
func hubReachable(address, sourceAddress string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), hubProbeTimeout)
	defer cancel()
	dialer := &network.RealDialer{}
	if err := dialer.SetSourceAddress(sourceAddress); err != nil {
		return false
	}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	if cerr := conn.Close(); cerr != nil {
		logger().Debug("hub reachability probe: close failed", "address", address, "error", cerr)
	}
	return true
}
