// Design: docs/architecture/api/commands.md -- show doctor RPC handler, the daemon form of ze doctor (plugin self-containment)

package cmd

import (
	"context"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	perCheckTimeout        = 5 * time.Second
	maxDiagnosticsPerCheck = 64
)

// HandleDoctor is the RPC handler for ze-doctor:show-doctor, the daemon form
// of `ze doctor`. It runs the one producer `ze doctor` runs locally
// (doctor.Answer) on the daemon route, adding the checks that read this
// daemon's live state: the runtime phase of the doctor registry
// (doctor.RunRuntimeChecks) and the doctor checks of the plugins it runs. It
// maps the answer as the local-data bridge maps every local-data command
// (pluginserver.AnswerLocalData).
func HandleDoctor(cmdCtx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	runtime := func() []diagnostic.Diagnostic {
		return append(doctor.RunRuntimeChecks(), collectPluginDoctorChecks(cmdCtx)...)
	}
	answer := func(args command.ValidatedArgs) command.LocalAnswer {
		return doctor.Answer(args, runtime)
	}
	return pluginserver.AnswerLocalData("doctor", answer, validated)
}

// doctorCheckSource is the running-plugin set the daemon route asks for doctor
// checks: the declarations of the plugins it runs and the callback that runs
// one of them. *pluginserver.Server is the one implementation; a test holds a
// fixture.
type doctorCheckSource interface {
	DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration
	CallDoctorCheck(ctx context.Context, pluginName, checkName string) (*rpc.DoctorCheckOutput, error)
}

// collectPluginDoctorChecks asks every plugin this daemon runs for its doctor
// checks. A context with no server (a dispatcher built without one) holds no
// plugin, so it answers none.
//
// It calls every check a running plugin declared, whatever its phase: inside
// the daemon a plugin's live state is there to read, so a runtime check and a
// config-phase check are both answered here.
func collectPluginDoctorChecks(cmdCtx *pluginserver.CommandContext) []diagnostic.Diagnostic {
	if cmdCtx == nil || cmdCtx.Server == nil {
		return nil
	}
	// Skip the calling plugin to avoid deadlock: the caller is blocked
	// waiting for our response, so a callback to it would hang.
	var callerName string
	if cmdCtx.Process != nil {
		callerName = cmdCtx.Process.Name()
	}
	return collectDoctorChecksFrom(cmdCtx.Server, callerName)
}

// collectDoctorChecksFrom runs every doctor check the source's plugins
// declared, skipping callerName's, in declared order, and answers their
// findings with the fix each plugin named.
func collectDoctorChecksFrom(source doctorCheckSource, callerName string) []diagnostic.Diagnostic {
	plugins := source.DoctorCheckPlugins()
	if len(plugins) == 0 {
		return nil
	}

	platform, _ := host.DetectPlatform()

	type pendingCheck struct {
		plugin string
		check  plugin.DoctorCheckRegistration
	}
	var pending []pendingCheck
	for pluginName, checks := range plugins {
		if pluginName == callerName {
			continue
		}
		for _, check := range checks {
			if !platformMatches(check.Platforms, platform) {
				continue
			}
			pending = append(pending, pendingCheck{pluginName, check})
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].check.Order < pending[j].check.Order
	})

	var diags []diagnostic.Diagnostic
	for _, pc := range pending {
		ctx, cancel := context.WithTimeout(context.Background(), perCheckTimeout)
		out, err := source.CallDoctorCheck(ctx, pc.plugin, pc.check.Name)
		cancel()
		if err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-plugin-unreachable",
				Severity: diagnostic.SeverityWarning,
				Message:  "plugin " + pc.plugin + " doctor check " + pc.check.Name + " failed: " + err.Error(),
			})
			continue
		}
		count := len(out.Diagnostics)
		if count > maxDiagnosticsPerCheck {
			count = maxDiagnosticsPerCheck
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-plugin-excessive-diagnostics",
				Severity: diagnostic.SeverityWarning,
				Message:  "plugin " + pc.plugin + " check " + pc.check.Name + " returned too many diagnostics, truncated",
			})
		}
		for _, d := range out.Diagnostics[:count] {
			diag := doctor.PluginDiagnostic(d)
			diag.Severity = normalizeSeverity(d.Severity)
			diags = append(diags, diag)
		}
	}
	return diags
}

func platformMatches(platforms []string, info *host.PlatformInfo) bool {
	if len(platforms) == 0 {
		return true
	}
	for _, p := range platforms {
		if p == "any" {
			return true
		}
		if info != nil && p == info.Type.String() {
			return true
		}
	}
	return false
}

func normalizeSeverity(s string) diagnostic.Severity {
	switch diagnostic.Severity(s) {
	case diagnostic.SeverityError, diagnostic.SeverityWarning:
		return diagnostic.Severity(s)
	default:
		// Plugin severity strings form an open set; keep unknown findings as warnings.
		return diagnostic.SeverityWarning
	}
}
