// Design: ai/rules/repo-maintenance.md -- management hub reachability readiness check
// Related: client.go -- the managed client whose hub dependency this check guards
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
// Related: baseline.go -- ManagedFlag, ActiveVersion, BaselineVersion and Diverged, the one reader of each fact the checks judge
// Related: ../plugin/server/managed_held.go -- HeldClients and ReadHeldRecord, the hub's copies the hub-side check reports

// The managed component owns the bus/hub/client dependency (a node's outbound
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
	"errors"
	"io/fs"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/pkg/fleet"
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
	Codes:        []string{codeDoctorHubClientMissing},
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
	Codes:        []string{CodeDoctorManagedDiverged},
	Check:        checkManagedDiverged,
}

// hubDivergedDoctorCheck reports, on a hub, each managed client whose
// configuration the hub keeps a copy of because the client holds
// (pluginserver.HeldRecord). Registered from register.go. It reads the store
// records and the serving blocks of the configuration tree, which answer the
// same in the operator's process and in the daemon's, so it is a post-config
// check: a record outlives a hub restart, and it is reported with the hub
// running or stopped.
var hubDivergedDoctorCheck = diagnostic.DoctorCheck{
	Name:         "managed-client-diverged",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        733,
	Component:    doctorComponent,
	Dependencies: []string{doctorNeedsConfigTree},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeDoctorManagedClientDiverged},
	Check:        checkHubClientsDiverged,
}

// checkHubClientsDiverged warns once per record this hub keeps: a declared
// client's record is a hold the hub's operator can resolve, and the record of
// a client no serving block declares is orphaned, because that client cannot
// connect to be resolved.
//
// It answers nothing when it cannot look at the serving blocks: no tree, no
// store, or a hub block config validation refuses, which is that check's
// finding. A record list or a record that cannot be read is reported: it may
// hold the only copy of a client's configuration.
func checkHubClientsDiverged(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return nil
	}
	if tree == nil {
		return nil
	}
	if ctx.Store == nil {
		return nil
	}
	hubCfg, err := config.ExtractHubConfig(tree)
	if err != nil {
		return nil // Config validation reports a hub block it cannot read.
	}
	clients, err := pluginserver.HeldClients(ctx.Store)
	if err != nil {
		return []diagnostic.Diagnostic{diagnoseHeldUnread("the list of client configurations this hub keeps a copy of", err)}
	}
	declared := servedClientNames(hubCfg)
	var diags []diagnostic.Diagnostic
	for _, name := range clients {
		record, held, err := pluginserver.ReadHeldRecord(ctx.Store, name)
		if err != nil {
			diags = append(diags, diagnoseHeldUnread("the copy of the configuration client "+name+" holds", err))
			continue
		}
		if !held {
			continue // Removed since the list was read: nothing is held now.
		}
		if !declared[name] {
			diags = append(diags, diagnoseHeldOrphan(name, record))
			continue
		}
		hubPhrase, hubVersion := hubVersionOf(ctx.Store, name)
		diags = append(diags, diagnoseHeldClient(name, record, hubPhrase, hubVersion))
	}
	return diags
}

// servedClientNames answers the names the serving blocks of hubCfg declare,
// the clients the managed server admits (startManagedServer reads the same
// blocks).
func servedClientNames(hubCfg plugin.HubConfig) map[string]bool {
	names := make(map[string]bool)
	for _, server := range hubCfg.Servers {
		for name := range server.Clients {
			names[name] = true
		}
	}
	return names
}

// hubVersionOf words what the hub serves client name, and answers its hash as
// the fetch hashes it (startManagedServer's ReadConfig reads the same file):
// the hub hash a revert names. The hash is empty when the hub serves no
// configuration or it could not be read.
func hubVersionOf(store storage.Storage, name string) (phrase, version string) {
	data, err := store.ReadFile(pluginserver.ClientConfigKey(name))
	if err == nil {
		version = fleet.VersionHash(data)
		return "this hub serves it version " + version, version
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "this hub serves it no configuration", ""
	}
	return "the configuration this hub serves it could not be read (" + err.Error() + ")", ""
}

// heldTimePhrase words when a record's version was edited on the client; the
// zero time is a client whose store recorded none.
func heldTimePhrase(at time.Time) string {
	if at.IsZero() {
		return "at a time the client did not record"
	}
	return "at " + at.UTC().Format(time.RFC3339)
}

// diagnoseHeldClient reports the record of a declared client: the client
// holds, and the hub's operator resolves it with the three hub commands over
// the record's version and hubVersion, the hash of what the hub serves it
// (empty when it serves none). hub words that configuration.
func diagnoseHeldClient(name string, record pluginserver.HeldRecord, hub, hubVersion string) diagnostic.Diagnostic {
	help := "show fleet divergence name " + name + " lists what differs; adopt makes the client's configuration the hub's, revert makes the client take the hub's"
	if unchecked := record.UncheckedPlugins(); len(unchecked) > 0 {
		help = "the copy holds sections of external plugins this hub did not start (" + strings.Join(unchecked, ", ") +
			"), so the difference cannot be shown; adopt and revert still work"
	}
	return diagnostic.Diagnostic{
		Code:     codeDoctorManagedClientDiverged,
		Severity: diagnostic.SeverityWarning,
		Message: "the managed client " + name + " holds: it runs version " + record.Version() + ", edited " + heldTimePhrase(record.EditedAt()) +
			", and " + hub + ", so it applies nothing this hub sends; this hub keeps a copy of its configuration, received at " +
			record.ReceivedAt().UTC().Format(time.RFC3339),
		Help: help,
		Fix:  divergenceCommandsFix(name, record.Version(), placeholderOr(hubVersion, placeholderHubVersion)),
	}
}

// diagnoseHeldOrphan reports the record of a client no serving block
// declares. The client cannot connect, so neither a fetch nor an ack ends the
// hold, and adopt and revert refuse an undeclared client. The record stays:
// it is the only copy of what the client runs.
func diagnoseHeldOrphan(name string, record pluginserver.HeldRecord) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     codeDoctorManagedClientDiverged,
		Severity: diagnostic.SeverityWarning,
		Message: "this hub keeps a copy of the configuration client " + name + " holds (version " + record.Version() +
			"), and no bus hub server block declares " + name + ", so the copy is orphaned: the client cannot connect and the hold cannot be resolved",
		Help: "declaring the client again lets it connect, and the hold is then resolved from this hub; the copy is the only one this hub keeps of what the client runs",
		Fix: "declare client " + name + " again in a bus hub server block, or remove the copy with the hub stopped: ze data rm " +
			pluginserver.HeldRecordKey(name),
	}
}

// diagnoseHeldUnread reports what of the hub's copies could not be read from
// the store.
func diagnoseHeldUnread(what string, cause error) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     codeDoctorManagedClientDiverged,
		Severity: diagnostic.SeverityWarning,
		Message:  what + " could not be read from the store, so whether a managed client holds is unknown: " + cause.Error(),
		Fix:      fixCheckStore,
	}
}

// managedDivergedFix names the hub commands that end a hold: show what
// differs, then keep this device's configuration (adopt) or discard it
// (revert). Both name version, the version the device runs, which is the
// version of the copy the hub keeps once the device sent it (config-held).
// The revert also names the hash of what the hub serves the device, which
// only the hub's show answers, so the device's fix carries a placeholder for
// it. client is the name the device's hub client block gives it; empty when
// the configuration names no client, and the fix then shows the placeholder.
func managedDivergedFix(client, version string) string {
	return divergenceCommandsFix(placeholderOr(client, "<name>"), placeholderOr(version, "<hash>"), placeholderHubVersion)
}

// placeholderHubVersion stands for the hub hash of a revert in a fix that
// cannot know it: the show's hub-version field answers it.
const placeholderHubVersion = "<hub-version>"

// divergenceCommandsFix is the fix of a hold the hub keeps a copy of, on the
// device and on the hub alike: the three hub commands over client, the copy's
// version and hubVersion, the hash of the configuration the hub serves client.
func divergenceCommandsFix(client, version, hubVersion string) string {
	return "on the hub, see what differs: show fleet divergence name " + client +
		"; to keep this device's configuration: update fleet divergence name " + client + " adopt version " + version +
		"; to discard it: request fleet divergence name " + client + " revert version " + version + " hub-version " + hubVersion
}

// managedDivergedManualFix names the two ways out of a hold the hub keeps no
// copy of, because the configuration is larger than a copy may be
// (fleet.HeldConfigMax) or the copy did not reach the hub: give the hub the
// configuration this device runs, or leave the fleet, return to the hub's
// configuration and rejoin. Each ends at a fetch the hub answers "current",
// which is what records a new baseline. client is as for managedDivergedFix.
func managedDivergedManualFix(client string) string {
	client = placeholderOr(client, "<name>")
	return "to keep this device's configuration, on the hub: ze data import client-" + client + ".conf; " +
		"to discard it, on this device: request fleet disable, roll back to the version the hub sent, request fleet enable"
}

// placeholderOr answers value, or placeholder when value is empty: a fix
// whose name is unknown shows where the operator puts it.
func placeholderOr(value, placeholder string) string {
	if value == "" {
		return placeholder
	}
	return value
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
	return diagnoseManagedDiverged(active, baseline, clientNameOf(content), len(content) > fleet.HeldConfigMax)
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
// client is the device's name on its hub, empty when unknown. oversize says
// the configuration is larger than a copy the hub takes, so the hub cannot
// resolve the hold and the fix names the manual routes.
func diagnoseManagedDiverged(active, baseline ConfigVersion, client string, oversize bool) []diagnostic.Diagnostic {
	if !Diverged(active, baseline) {
		return nil
	}
	fix := managedDivergedFix(client, active.Hash())
	if oversize {
		fix = managedDivergedManualFix(client)
	}
	return []diagnostic.Diagnostic{{
		Code:     CodeDoctorManagedDiverged,
		Severity: diagnostic.SeverityWarning,
		Message: "the configuration this managed client runs (version " + active.Hash() + ") differs from the last one its hub sent (version " +
			baseline.Hash() + "), so it holds: it applies nothing the hub sends",
		Help: "the hold ends when the hub holds the same configuration as this device, or when the hub's operator discards this device's change; " +
			"show fleet status reports both hashes and the version of the copy the hub keeps (held-by-hub)",
		Fix: fix,
	}}
}

// diagnoseManagedUncompared reports a managed node whose store could not
// answer what, one of the two values the hold compares.
func diagnoseManagedUncompared(what string, cause error) []diagnostic.Diagnostic {
	return []diagnostic.Diagnostic{{
		Code:     CodeDoctorManagedDiverged,
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
// configuration has no bus/hub/client block. Such a node connects to no
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
		Code:     codeDoctorHubClientMissing,
		Severity: diagnostic.SeverityWarning,
		Message: "this node is a managed client and its configuration has no bus hub client block, " +
			"so it follows no hub and receives no configuration change",
		Help: "add the client block to this node's configuration on the hub, then provision the node again so that it fetches it",
		Fix:  hubClientMissingFix,
	}}
}

// checkHubReachable warns when this node is configured as a managed client
// (bus/hub/client block) but none of its hubs answers a TCP probe, so an
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
			"bus hub client block in its configuration on the hub; ze doctor on the hub reports a managed listener that did not bind",
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
