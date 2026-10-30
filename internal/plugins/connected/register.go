// Design: docs/architecture/core-design.md -- plugin registration

package connected

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	connectedyang "github.com/ze-software/ze/internal/plugins/connected/yang"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	// The filter attachment points this plugin's config holds, with the route
	// fields their routes carry and a filter there may set (the route field
	// vocabulary, internal/component/policy/fields.go).
	policy.MustRegisterAttachment(pluginName, "connected import",
		[]policy.Field{policy.FieldPrefix, policy.FieldFamily, policy.FieldProtocol, policy.FieldInterface},
		[]policy.Field{policy.FieldDistance, policy.FieldTag})
	// The import chain is compiled into the Loc-RIB gate by the policy plugin.
	policy.MustRegisterRIBImport(policy.RIBImport{Attachment: "connected import", Root: pluginName, Scope: policy.RIBScopeSource, Protocol: pluginName})

	// Register the redistribute source at init (not only in runConnectedPlugin)
	// so `import connected` resolves during `ze config validate`, which imports
	// plugins but does not start their engines. sync.Once keeps the run-time call
	// idempotent.
	registerConnectedSources()

	reg := registry.Registration{
		Name:        pluginName,
		Description: "Connected routes: redistribute directly connected interface prefixes",
		Features:    "yang",
		YANG:        connectedyang.ZeConnectedConfYANG,
		ConfigRoots: []string{pluginName},
		// StartAfter, never a load: connected answers the BGP redistribute
		// orchestrator's late-join replay request, and a build or config without
		// BGP simply never asks (./le plugin coupling check).
		StartAfter: []string{"redistribute-orchestrator"},
		// policy is a hard dependency of every protocol with a filter attachment:
		// the filter instances its chains name live under the policy root.
		Dependencies: []string{policy.PluginName},
		RunEngine:    runConnectedPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "connected: registration failed: %v\n", err)
		os.Exit(1)
	}
}
