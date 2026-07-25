// Design: docs/architecture/api/commands.md -- show doctor RPC handler (plugin self-containment)

package cmd

import (
	"context"
	"sort"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	perCheckTimeout        = 5 * time.Second
	maxDiagnosticsPerCheck = 64
)

// HandleShowDoctor is the RPC handler for ze-show:doctor.
// Registration is deferred until the central show.go entry is removed.
func HandleShowDoctor(cmdCtx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	var configPath string
	if len(args) > 0 {
		configPath = args[0]
	}

	diags := diagnostic.RunDoctorChecks(configPath)
	if diags == nil {
		diags = []diagnostic.Diagnostic{}
	}

	if cmdCtx.Server != nil {
		pluginDiags := collectPluginDoctorChecks(cmdCtx)
		diags = append(diags, pluginDiags...)
	}

	ready := true
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityError {
			ready = false
			break
		}
	}

	result := diagnostic.NewDoctorResult(ready, diags)
	return &plugin.Response{Status: plugin.StatusDone, Data: result}, nil
}

func collectPluginDoctorChecks(cmdCtx *pluginserver.CommandContext) []diagnostic.Diagnostic {
	plugins := cmdCtx.Server.DoctorCheckPlugins()
	if len(plugins) == 0 {
		return nil
	}

	// Skip the calling plugin to avoid deadlock: the caller is blocked
	// waiting for our response, so a callback to it would hang.
	var callerName string
	if cmdCtx.Process != nil {
		callerName = cmdCtx.Process.Name()
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
		out, err := cmdCtx.Server.CallDoctorCheck(ctx, pc.plugin, pc.check.Name)
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
			sev := normalizeSeverity(d.Severity)
			diags = append(diags, diagnostic.Diagnostic{
				Code:     d.Code,
				Severity: sev,
				Message:  d.Message,
			})
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
		return diagnostic.SeverityWarning
	}
}
