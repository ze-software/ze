// Design: ai/rules/repo-maintenance.md -- management hub reachability readiness check
// Related: client.go -- the managed client whose hub dependency this check guards
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck

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
	"strings"
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

// hubDoctorCheck is the readiness check registered from register.go. It reads
// the parsed config tree, so it depends on config-tree and runs post-config.
var hubDoctorCheck = diagnostic.DoctorCheck{
	Name:         "hub-unreachable",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        730,
	Component:    "managed",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{"doctor-hub-unreachable"},
	Check:        checkHubReachable,
}

// hubClientDoctorCheck reports a managed client whose configuration names no
// hub. Registered from register.go.
var hubClientDoctorCheck = diagnostic.DoctorCheck{
	Name:         "hub-client-missing",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        731,
	Component:    "managed",
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorHubClientMissing},
	Check:        checkHubClientDeclared,
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
	flag, err := ctx.Store.ReadFile(zefs.KeyInstanceManaged.Pattern)
	if err != nil {
		return nil // No managed flag to read: this check has nothing to judge.
	}
	hubCfg, err := config.ExtractHubConfig(tree)
	if err != nil {
		return nil // Config validation reports a hub block it cannot read.
	}
	return diagnoseHubClientDeclared(strings.TrimSpace(string(flag)) == "true", len(hubCfg.Clients))
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
		Code:     "doctor-hub-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "the configured management hub is not reachable",
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
