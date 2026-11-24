// Design: docs/architecture/core-design.md -- plugin registration

package connected

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/diagnostic"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
	connectedyang "github.com/ze-software/ze/internal/plugins/connected/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func connectedSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: connectedevents.Namespace + ":" + redistevents.EventType},
		{Message: ifaceevents.Namespace + ":" + ifaceevents.EventAddressReplayRequest},
		{Message: sysribevents.Namespace + ":" + sysribevents.EventRouteInstall},
	}
}

func connectedReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: ifaceevents.Namespace + ":" + ifaceevents.EventAddrAdded},
		{Message: ifaceevents.Namespace + ":" + ifaceevents.EventAddrRemoved},
		{Message: redistevents.ReplayNamespace + ":" + redistevents.ReplayRequestEventType},
	}
}

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorPluginPanic}); err != nil {
		panic("BUG: connected replay diagnostic registration refused: " + err.Error())
	}
	// The filter attachment points this plugin's config holds, with the route
	// fields their routes carry and a filter there may set (the route field
	// vocabulary, internal/component/policy/fields.go).
	policy.MustRegisterRouteAttachment(pluginName, "connected import",
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
		Sends:        connectedSends(),
		Receives:     connectedReceives(),
		RunEngine:    runConnectedPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		panic("BUG: connected: registration failed: " + err.Error())
	}
}
