// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr_egress.go -- LLGREgressFilter and the egress state it reads
// Related: register.go -- filter, code and doctor-check registration from init()

package gr

import (
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeGROutOfProcess is raised when bgp-gr is configured to run as a separate
// process. runGRPlugin refuses to start forked (gr.go), because the retention
// answer bgp-rr and bgp-rs read and the state LLGREgressFilter reads must live
// in the daemon's process, so the daemon does not start with this config.
const codeGROutOfProcess = "doctor-bgp-gr-out-of-process"

var grDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeGROutOfProcess,
		Title: "Graceful Restart plugin runs out of process",
		Description: "The bgp-gr plugin is configured with `run`, so it would run as a separate process, and it refuses to start that way: " +
			"the daemon does not start. " +
			"Its graceful-restart state must live in the daemon's process: the route reflector and route server ask it which routes " +
			"a restarting peer keeps (RFC 4724 Section 4.2), and the RFC 9494 LLGR egress filter reads the peer capabilities it records. " +
			"In a separate process neither answer reaches the daemon, so restarting peers' routes would be withdrawn. " +
			"Configure the plugin as `plugin { internal <name> { use bgp-gr; } }` to run it in the daemon.",
		Examples: []string{"ze doctor --json", "ze show diagnostic doctor-bgp-gr-out-of-process"},
	},
}

// grDoctorCheck describes the in-process check. register.go registers it.
var grDoctorCheck = diagnostic.DoctorCheck{
	Name:         "bgp-gr-in-process",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        745,
	Component:    grPluginName,
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeGROutOfProcess},
	Check:        checkGRInProcess,
}

// checkGRInProcess reports every bgp-gr plugin configured to run out of
// process.
//
// When the operator runs bgp-gr with `run` rather than `use`, the engine forks
// it (process.startExternal) and runGRPlugin refuses to start in the child
// (gr.go), which stops the daemon. An in-process bgp-gr beside it does not
// help: the forked copy still refuses. This check says so before the daemon is
// started, from the config alone.
func checkGRInProcess(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for i := range ctx.Plugins {
		p := ctx.Plugins[i]
		if p.Internal {
			continue
		}
		if !runTargetsPlugin(p.Run, grPluginName) {
			continue
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeGROutOfProcess,
			Severity: diagnostic.SeverityError,
			Message: tb.Reset().Str("plugin ").Str(p.Name).Str(": ").Str(grPluginName).
				Str(" runs as a separate process, which it refuses, so the daemon will not start:").
				Str(" its graceful-restart state must live in the daemon; replace the block with `internal ").
				Str(p.Name).Str(" { use ").Str(grPluginName).Str(" }`").String(),
		})
	}
	return diags
}

// runTargetsPlugin reports whether a `run` command line launches the named
// bundled plugin as a SEPARATE PROCESS, which is a different question from the
// one plugin.RegistryName answers. That function reads the identity the engine
// runs the plugin under, and it takes ResolvePlugin's `ze plugin <name>` form,
// where parts[0] must be exactly "ze". This check has to catch the arrangement
// whatever binary path spells it, because a path-qualified `/usr/bin/ze plugin
// bgp-gr` forks the engine out of the daemon exactly as `ze plugin bgp-gr`
// does, and runGRPlugin refuses to start either way. So it matches the
// `plugin <name>` verb pair anywhere in the command line.
// The "ze.<name>" spelling never reaches here: MarkInternalPlugin
// (config/loader.go) resolves it to an internal plugin.
func runTargetsPlugin(run, name string) bool {
	fields := strings.Fields(run)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "plugin" && fields[i+1] == name {
			return true
		}
	}
	return false
}
