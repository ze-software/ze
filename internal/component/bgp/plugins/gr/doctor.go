// Design: docs/architecture/core-design.md -- LLGR egress filter
// RFC: rfc/short/rfc9494.md -- Long-Lived Graceful Restart readvertisement
// Overview: gr_egress.go -- LLGREgressFilter and the egress state it reads
// Related: register.go -- filter, code and doctor-check registration from init()

package gr

import (
	"strings"

	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeGROutOfProcess is raised when bgp-gr is configured to run as a separate
// process. LLGREgressFilter lives in the daemon and reads state that only
// RunGRPlugin stores, so an out-of-process engine leaves the filter blind
// (gr_egress.go, egressState).
const codeGROutOfProcess = "doctor-bgp-gr-out-of-process"

var grDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeGROutOfProcess,
		Title: "Graceful Restart plugin runs out of process",
		Description: "The bgp-gr plugin is configured with `run`, so it runs as a separate process. " +
			"The RFC 9494 LLGR egress filter runs in the daemon and reads the peer capability state that only the plugin engine stores, " +
			"so in this arrangement it has no state at all. It then treats every neighbor as LLGR-incapable: " +
			"stale routes are withdrawn from external peers and depreferenced toward internal peers, for the life of the process, " +
			"including peers that negotiated LLGR. " +
			"Configure the plugin as `plugin { internal <name> { use bgp-gr; } }` to run it in the daemon.",
		Examples: []string{"ze doctor --json", "ze explain doctor-bgp-gr-out-of-process"},
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

// checkGRInProcess reports a bgp-gr plugin configured to run out of process.
//
// The LLGR egress filter is registered from init() (register.go), so it is live
// in every daemon that links the BGP build group, while setEgressState is called
// only from RunGRPlugin's OnConfigure callback (gr.go). When the operator runs
// bgp-gr with `run` rather than `use`, the engine forks it
// (process.startExternal) and RunGRPlugin executes in the CHILD, so the daemon's
// filter answers every destination from a nil state. Failing closed is correct
// with no state loaded, but it is the wrong answer for a peer that did negotiate
// LLGR, and the filter's own WARN speaks only once the first stale route reaches
// it. This check says so before that happens.
func checkGRInProcess(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	// One in-process bgp-gr is enough, so the whole plugin list is judged before
	// any single `run` line. setEgressState stores into a package-level pointer
	// (gr_egress.go), so a daemon that loads the engine once answers for every
	// destination however many other copies run as children. Reading each `run`
	// line on its own reported that arrangement as blind, which it is not.
	for i := range ctx.Plugins {
		if internalIsPlugin(ctx.Plugins[i], grPluginName) {
			return nil
		}
	}

	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for i := range ctx.Plugins {
		p := ctx.Plugins[i]
		if p.Internal || !runTargetsPlugin(p.Run, grPluginName) {
			continue
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeGROutOfProcess,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("plugin ").Str(p.Name).Str(": ").Str(grPluginName).
				Str(" runs as a separate process, so the LLGR egress filter in the daemon has no peer capability state").
				Str(" and withdraws or depreferences every stale route; use `use ").Str(grPluginName).
				Str("` to run it in the daemon").String(),
		})
	}
	return diags
}

// internalIsPlugin reports whether an in-process plugin entry is the named
// bundled plugin. Which registry row a process config names is answered once,
// by plugin.RegistryName, so `use bgp-gr`, `run ze.bgp-gr` and a bare
// `internal bgp-gr { }` all name the same engine here.
func internalIsPlugin(p zeplugin.PluginConfig, name string) bool {
	return p.Internal && zeplugin.RegistryName(p) == name
}

// runTargetsPlugin reports whether a `run` command line launches the named
// bundled plugin as a SEPARATE PROCESS, which is a different question from the
// one plugin.RegistryName answers. That function reads the identity the engine
// runs the plugin under, and it takes ResolvePlugin's `ze plugin <name>` form,
// where parts[0] must be exactly "ze". This check has to catch the arrangement
// whatever binary path spells it, because a path-qualified `/usr/bin/ze plugin
// bgp-gr` forks the engine out of the daemon exactly as `ze plugin bgp-gr`
// does, and the LLGR egress filter is blind either way. So it matches the
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
