// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: register_reload_dataplane.go -- registers the check at the post-config phase
// Related: check_lifecycle_local.go -- autoloadSet, the selection boot makes
// Related: ../registry/dataplane_writer.go -- DataPlaneWriter, the refusal it repeats

package doctor

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkDataPlaneWriter reports the refusal boot and reload give when the
// config selects a plugin whose routes need a data-plane writer
// (Registration.NeedsDataPlane: static, bgp-epe and any later producer) and
// no plugin programs the data plane the config selects, or this build
// compiled that writer out. It keys on the registration, never on a list of
// producers, and asks registry.DataPlaneWriter, the judgement the daemon
// makes (Server.appendDataPlaneWriter).
func checkDataPlaneWriter(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	// A context holding no *config.Tree, and a nil one, are both no parsed
	// config: one fact, one guard.
	tree, _ := ctx.Tree.(*config.Tree)
	if tree == nil {
		return []diagnostic.Diagnostic{{
			Code:     codeDoctorPluginNoDataPlaneWriter,
			Severity: diagnostic.SeverityWarning,
			Message:  "the config was not parsed, so whether its routes have a data-plane writer was not checked",
			Fix:      "ze config validate",
		}}
	}
	var configured []string
	for i := range ctx.Plugins {
		configured = append(configured, zeplugin.RegistryNames(ctx.Plugins[i])...)
	}
	return dataPlaneWriterRows(autoloadSet(tree, ctx.Plugins), configured, server.ConfigDataPlane(tree.ToMap()))
}

// dataPlaneWriterRows judges one selection through registry.JudgeDataPlaneWriter,
// the one judgement boot and reload share: the first NeedsDataPlane producer it
// holds needs a writer for dataPlane unless a plugin that ProgramsFIB is
// selected or configured (an explicit `fib { ... }` block, bus endpoint, or
// `--plugin` entry).
func dataPlaneWriterRows(selected, configured []string, dataPlane string) []diagnostic.Diagnostic {
	chosen := make([]string, 0, len(selected)+len(configured))
	chosen = append(chosen, selected...)
	chosen = append(chosen, configured...)
	_, err := registry.JudgeDataPlaneWriter(selected, chosen, dataPlane)
	if err == nil {
		return nil
	}
	producerIndex := slices.IndexFunc(selected, registry.NeedsDataPlane)
	producer := selected[producerIndex]
	var message textbuf.Buffer
	message.Str("this config loads ").Str(producer).Str(", whose routes reach forwarding only through a data-plane writer, ").
		Str("so ze refuses to start or reload it: ").Err(err)
	fix := pluginSetupFixDefault
	if refusal, isOperator := errors.AsType[*opererr.Error](err); isOperator {
		fix = refusal.Fix()
	}
	return []diagnostic.Diagnostic{{
		Code:     codeDoctorPluginNoDataPlaneWriter,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix,
	}}
}
