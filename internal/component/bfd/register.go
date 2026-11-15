package bfd

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	bfdyang "github.com/ze-software/ze/internal/component/bfd/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// pluginName is the name this plugin registers under, and configRoot is the
// top-level YANG container it owns. The two hold the same text and name two
// different things: a rename of one does not rename the other.
const (
	pluginName = "bfd"
	configRoot = api.ConfigRoot
)

// bfdDiagnosticCodes describes the codes the BFD engine raises on the report
// bus, for `ze show diagnostic`. They are registered here rather than in
// diagnostic.builtinCodes, so removing the bfd plugin removes its codes, and
// before their event-code registration in init, which requires them.
var bfdDiagnosticCodes = []diagnostic.CodeMeta{{
	Code:  engine.AuthFailedCode,
	Title: "BFD packets from a peer fail authentication",
	Description: "A BFD session drops the Control packets of its peer because they fail authentication " +
		"(RFC 5880 Section 6.7): the digest, password, key ID or sequence number does not match. " +
		"A session whose packets all fail cannot come Up, and an Up session goes Down when its " +
		"Detection Time passes. The peer runs another auth type, key-id or secret than the bfd " +
		"profile the session uses, or someone sends packets under the session's discriminator. " +
		"Each dropped packet is counted in ze_bfd_auth_failures_total. Make auth type, key-id and " +
		"secret of that bfd profile match the peer's, on both routers. The warning clears when a " +
		"packet verifies again, when the session's authentication changes, when the session is removed, " +
		"or when no packet has failed for two minutes.",
	Examples: []string{"ze show warnings", "ze show bfd sessions", "ze show diagnostic " + engine.AuthFailedCode},
}, {
	Code:  engine.SeqUnsavedCode,
	Title: "BFD could not save a session's authentication sequence number",
	Description: "A BFD session using meticulous keyed authentication saves the sequence number of the last " +
		"packet it sent to the state store, so that after a restart ze resumes from it (RFC 5880 Section 6.7.3). " +
		"The store refused that write, or the session's authentication closed with its last write refused. " +
		"After ze restarts, the session sends from an older sequence number, and the peer drops its packets " +
		"until the number passes the one it last accepted, so the session stays Down for that time. Remedy: run " +
		"ze doctor, which reports why the state store cannot be written (permissions, integrity, disk space).",
	Examples: []string{"ze doctor", "ze show errors", "ze show diagnostic " + engine.SeqUnsavedCode},
}, {
	Code:  engine.SendFailedCode,
	Title: "BFD could not send a session's packets",
	Description: "The transport refused a BFD packet of a session: a Control packet, an Echo packet, or the " +
		"return of an Echo packet the peer sent. It refuses a single-hop packet whose interface no longer " +
		"resolves or whose peer is on none of that interface's subnets (RFC 5881 Section 6), and passes on " +
		"what the kernel refuses. A refused Control packet makes the peer take the session Down when its " +
		"Detection Time passes; a refused Echo packet makes this router take it Down with Echo Function " +
		"Failed; a refused return makes the peer's echo function fail. Each refused packet is counted in " +
		"ze_bfd_send_failures_total by packet kind. Remedy: for a single-hop session run ze show interface " +
		"name <interface> detail and give the interface an address on the peer's subnet; otherwise run " +
		"ze show bfd session address <peer>. The warning clears when a packet of the same kind is sent, " +
		"when the session is removed, or when no packet of that kind has failed for two minutes.",
	Examples: []string{"ze show warnings", "ze show bfd sessions", "ze show diagnostic " + engine.SendFailedCode},
}}

func init() {
	// A BGP peer or a static next-hop that names a profile asks this at
	// commit (api.CheckProfile), so a typo is refused where it was typed.
	api.SetProfileChecker(checkClientProfile)
	// A client opting into BFD asks this at commit (api.CheckOptIns), so BFD
	// cannot be removed or disabled from under a protected client.
	api.SetEnabledChecker(checkClientsEnabled)
	// The engine raises engine.AuthFailedCode, engine.SeqUnsavedCode and
	// engine.SendFailedCode on the report bus. Each is described first,
	// because RegisterEventCode refuses a code `ze show diagnostic` cannot
	// describe.
	for _, meta := range bfdDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: register bfd diagnostic code: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: engine.AuthFailedCode}); err != nil {
		panic("BUG: register bfd auth-failed event code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: engine.SeqUnsavedCode}); err != nil {
		panic("BUG: register bfd auth-seq-unsaved event code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: engine.SendFailedCode}); err != nil {
		panic("BUG: register bfd send-failed event code: " + err.Error())
	}

	reg := registry.Registration{
		Name:                    pluginName,
		Description:             "Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)",
		Features:                "yang",
		RFCs:                    []string{"5880", "5881", "5882", "5883"},
		ConfigRoots:             []string{configRoot},
		StartAfter:              []string{"interface"},
		YANG:                    bfdyang.ZeBFDConfYANG,
		InProcessConfigVerifier: verifyBFDConfig,
		RunEngine:               runBFDPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			useLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r metrics.Registry) {
			bindMetricsRegistry(r)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			useLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: registration failed: %v\n", err)
		os.Exit(1)
	}
}
