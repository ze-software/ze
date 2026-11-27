// Design: docs/architecture/api/commands.md -- show doctor RPC handler, the daemon form of ze doctor (plugin self-containment)

package cmd

import (
	"cmp"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	perCheckTimeout        = 5 * time.Second
	doctorQueueWait        = 5 * time.Second
	maxDiagnosticsPerCheck = 64
)

// HandleDoctor is the RPC handler for ze-doctor:show-doctor, the daemon form
// of `ze doctor`. It runs the one producer `ze doctor` runs locally
// (doctor.Answer) on the daemon route, adding the checks that read this
// daemon's live state: the runtime phase of the in-tree doctor registry
// (doctor.RunRuntimeChecks) and the doctor checks of the plugins it runs. It
// maps the answer as the local-data bridge maps every local-data command
// (pluginserver.AnswerLocalData).
func HandleDoctor(cmdCtx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	source, callerName := doctorCheckSourceOf(cmdCtx)
	return answerDaemonDoctor(source, callerName, validated)
}

// answerDaemonDoctor answers show doctor with source as the running-plugin
// set. Each check runs once per answer, through one route: an in-tree check
// through doctor.Answer or doctor.RunRuntimeChecks by its phase, a compiled-in
// plugin's registry.DoctorCheckDef through doctor.Answer (config phases only:
// the plugin registry refuses a runtime one), and a check a running plugin
// declared at Stage 1 through its doctor callback here (the server refuses a
// declared name the plugin's registry entry already holds). A nil source holds
// no plugin.
func answerDaemonDoctor(source doctorCheckSource, callerName string, validated command.ValidatedArgs) (*plugin.Response, error) {
	runtime := func(judged *config.Tree) []diagnostic.Diagnostic {
		if source == nil {
			return doctor.RunRuntimeChecks(judged, nil)
		}
		// The plugin checks run first so the runtime phase knows which codes
		// they answered for (recent-events leaves those warnings to them);
		// the in-tree findings still print first.
		plugins := collectDoctorChecksFrom(source, callerName)
		return append(doctor.RunRuntimeChecks(judged, plugins.codes), plugins.diagnostics...)
	}
	answer := func(args command.ValidatedArgs) command.LocalAnswer {
		return doctor.Answer(args, runtime)
	}
	return pluginserver.AnswerLocalData("doctor", answer, validated)
}

// doctorCheckSource is the running-plugin set the daemon route asks for doctor
// checks: the declarations of the plugins it runs and the batch runner that
// executes them concurrently across plugin generations. *pluginserver.Server is
// the one production implementation; tests hold a fixture.
type doctorCheckSource interface {
	DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration
	RunDoctorChecks(requests []pluginserver.DoctorCheckPluginRequest, checkTimeout, queueWait time.Duration) []pluginserver.DoctorCheckPluginResult
}

// doctorCheckSourceOf answers the running-plugin set of a command context and
// the name of the plugin that sent the command. A context with no server (a
// dispatcher built without one) holds no plugin, so it answers a nil source.
//
// The calling plugin is skipped to avoid deadlock: the caller is blocked
// waiting for our response, so a callback to it would hang.
func doctorCheckSourceOf(cmdCtx *pluginserver.CommandContext) (doctorCheckSource, string) {
	if cmdCtx == nil || cmdCtx.Server == nil {
		return nil, ""
	}
	var callerName string
	if cmdCtx.Process != nil {
		callerName = cmdCtx.Process.Name()
	}
	return cmdCtx.Server, callerName
}

// pluginCheckAnswer is what the running plugins' doctor checks answered:
// their findings, and the codes declared by each check that answered. A check
// whose callback failed is reported as unreachable and adds no code, because
// it reported no state.
type pluginCheckAnswer struct {
	diagnostics []diagnostic.Diagnostic
	codes       map[string]bool
}

// collectDoctorChecksFrom runs every doctor check the source's plugins
// declared, skipping callerName's, and answers their findings with the fix
// each plugin named and the codes of the checks that answered.
//
// It calls every check a running plugin declared, whatever its phase: inside
// the daemon a plugin's live state is there to read, so a runtime check and a
// config-phase check are both answered here, and this is the only route that
// runs a declared check.
func collectDoctorChecksFrom(source doctorCheckSource, callerName string) pluginCheckAnswer {
	return collectDoctorChecksWithTimeouts(source, callerName, perCheckTimeout, doctorQueueWait)
}

// collectDoctorChecksWithTimeouts is collectDoctorChecksFrom with explicit
// check and queue-wait bounds so tests can exercise concurrency and timeouts
// without mutating package globals.
func collectDoctorChecksWithTimeouts(source doctorCheckSource, callerName string, checkTimeout, queueWait time.Duration) pluginCheckAnswer {
	plugins := source.DoctorCheckPlugins()
	if len(plugins) == 0 {
		return pluginCheckAnswer{}
	}

	platform, _ := host.DetectPlatform()

	type checkKey struct {
		plugin string
		check  string
	}
	meta := make(map[checkKey]plugin.DoctorCheckRegistration)
	requests := make([]pluginserver.DoctorCheckPluginRequest, 0, len(plugins))
	for pluginName, checks := range plugins {
		if pluginName == callerName {
			continue
		}
		var matched []plugin.DoctorCheckRegistration
		for _, check := range checks {
			if !platformMatches(check.Platforms, platform) {
				continue
			}
			matched = append(matched, check)
		}
		if len(matched) == 0 {
			continue
		}
		slices.SortFunc(matched, func(a, b plugin.DoctorCheckRegistration) int {
			if c := cmp.Compare(a.Order, b.Order); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		})
		names := make([]string, len(matched))
		for i, check := range matched {
			names[i] = check.Name
			meta[checkKey{plugin: pluginName, check: check.Name}] = check
		}
		requests = append(requests, pluginserver.DoctorCheckPluginRequest{
			Plugin: pluginName,
			Checks: names,
		})
	}
	if len(requests) == 0 {
		return pluginCheckAnswer{}
	}
	slices.SortFunc(requests, func(a, b pluginserver.DoctorCheckPluginRequest) int {
		return cmp.Compare(a.Plugin, b.Plugin)
	})

	results := source.RunDoctorChecks(requests, checkTimeout, queueWait)

	type completedCheck struct {
		plugin string
		check  plugin.DoctorCheckRegistration
		output *rpc.DoctorCheckOutput
		err    error
	}
	var completed []completedCheck
	for _, res := range results {
		for _, outcome := range res.Outcomes {
			reg := meta[checkKey{plugin: res.Plugin, check: outcome.Check}]
			if reg.Name == "" {
				reg.Name = outcome.Check
			}
			completed = append(completed, completedCheck{
				plugin: res.Plugin,
				check:  reg,
				output: outcome.Output,
				err:    outcome.Err,
			})
		}
	}
	slices.SortFunc(completed, func(a, b completedCheck) int {
		if c := cmp.Compare(a.check.Order, b.check.Order); c != 0 {
			return c
		}
		if c := cmp.Compare(a.plugin, b.plugin); c != 0 {
			return c
		}
		return cmp.Compare(a.check.Name, b.check.Name)
	})

	var answer pluginCheckAnswer
	for _, entry := range completed {
		if entry.err != nil {
			answer.diagnostics = append(answer.diagnostics, diagnostic.Diagnostic{
				Code:     "doctor-plugin-unreachable",
				Severity: diagnostic.SeverityWarning,
				Message:  "plugin " + entry.plugin + " doctor check " + entry.check.Name + " failed: " + entry.err.Error(),
				Fix:      "ze request plugin restart " + entry.plugin,
			})
			continue
		}
		answer.addCodes(entry.check.Codes)
		if entry.output == nil {
			continue
		}
		count := len(entry.output.Diagnostics)
		if count > maxDiagnosticsPerCheck {
			count = maxDiagnosticsPerCheck
			answer.diagnostics = append(answer.diagnostics, diagnostic.Diagnostic{
				Code:     "doctor-plugin-excessive-diagnostics",
				Severity: diagnostic.SeverityWarning,
				Message:  "plugin " + entry.plugin + " check " + entry.check.Name + " returned too many diagnostics, truncated",
				Fix:      "ze show log recent component " + pluginserver.LogSubsystem,
			})
		}
		for _, d := range entry.output.Diagnostics[:count] {
			diag := doctor.PluginDiagnostic(d)
			diag.Severity = normalizeSeverity(d.Severity)
			answer.diagnostics = append(answer.diagnostics, diag)
		}
	}
	return answer
}

// addCodes records the codes a check that answered declares.
func (a *pluginCheckAnswer) addCodes(codes []string) {
	if a.codes == nil {
		a.codes = make(map[string]bool, len(codes))
	}
	for _, code := range codes {
		a.codes[code] = true
	}
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
	case diagnostic.SeverityError, diagnostic.SeverityWarning, diagnostic.SeverityInfo:
		return diagnostic.Severity(s)
	default:
		// Plugin severity strings form an open set; keep unknown findings as warnings.
		return diagnostic.SeverityWarning
	}
}
