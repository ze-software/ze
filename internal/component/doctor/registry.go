// Design: docs/features/ai-first.md -- doctor check registration
// Overview: doctor.go -- readiness check runner
// Related: check_plugins.go -- the out-of-process plugin check bridge
//
// The doctor component holds no registry of its own. Every check it runs is
// registered with internal/core/diagnostic, which is the registry an owner
// package outside this component can reach (ai/patterns/registration.md,
// "Doctor Check Registry"). This file carries only the runner-side context and
// the phase dispatch that reads that registry.

package doctor

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// doctorCheckPlatformAny matches every platform. It is the doctor-side spelling
// of diagnostic.DoctorPlatformAny, which the out-of-process plugin bridge
// compares against as a plain string.
const doctorCheckPlatformAny = diagnostic.DoctorPlatformAny

type doctorCheckPhase = diagnostic.DoctorCheckPhase

const (
	doctorCheckPhasePreConfig     = diagnostic.DoctorPhasePreConfig
	doctorCheckPhaseMissingConfig = diagnostic.DoctorPhaseMissingConfig
	doctorCheckPhasePostConfig    = diagnostic.DoctorPhasePostConfig
)

// doctorCheckContext is the runner's own typed view of the check context. The
// registry types Tree as any so internal/core/diagnostic needs no config
// import; the runner knows the concrete type and converts once, here.
type doctorCheckContext struct {
	Tree      *config.Tree
	ConfigDir string
	Plugins   []zeplugin.PluginConfig
	Store     storage.Storage
	Platform  *host.PlatformInfo
}

// runDoctorChecks runs every check registered for one phase, in registry order,
// then the checks an out-of-process plugin declared.
func runDoctorChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
	registered := diagnostic.DoctorChecksForPhase(phase)

	exportedCtx := diagnostic.DoctorCheckContext{
		Tree:      ctx.Tree,
		ConfigDir: ctx.ConfigDir,
		Plugins:   ctx.Plugins,
		Store:     ctx.Store,
		Platform:  ctx.Platform,
	}

	var diags []diagnostic.Diagnostic
	for i := range registered {
		if !diagnostic.DoctorCheckSupportsPlatform(registered[i], ctx.Platform) {
			continue
		}
		diags = append(diags, registered[i].Check(exportedCtx)...)
	}

	return append(diags, runPluginRegistryChecks(phase, ctx)...)
}

// RunRuntimeChecks runs every check registered for the runtime phase, the
// in-tree registry's then the plugin registry's, and answers their findings.
//
// Only the daemon route MUST call it (HandleDoctor, the show doctor handler):
// a runtime check reads the live state its owner holds in the daemon, and in
// any other process that state is empty, so a check run there would read a box
// it never looked at as healthy. runChecks, the local route, never asks for
// this phase.
//
// A runtime check is handed the platform alone. It reads its owner's live
// state, never a config tree: the daemon route loads the config file from the
// store, not the running configuration, so a tree here would be a second,
// possibly different, answer to what is running.
func RunRuntimeChecks() []diagnostic.Diagnostic {
	// The answer this runs beside already reports a platform detection
	// failure (runChecks, through resolveDoctorPlatform), so its findings are
	// not repeated here; a nil platform still runs every check declared for
	// any platform.
	platform, _ := resolveDoctorPlatform()
	return runDoctorChecks(diagnostic.DoctorPhaseRuntime, doctorCheckContext{Platform: platform})
}

// doctorTree answers the config tree a registered check reads.
//
// A nil Tree is a legitimate value: the missing-config phase runs with no
// config loaded, and the checks that run there are written for it. A Tree that
// holds something other than *config.Tree can only come from a runner defect,
// so it panics rather than returning a nil tree the check would read as "no
// config" (docs/contributing/ze-go-style.md, "A zero value is never an answer").
func doctorTree(ctx diagnostic.DoctorCheckContext) *config.Tree {
	if ctx.Tree == nil {
		return nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		panic("BUG: doctor check context carries a tree that is not *config.Tree")
	}
	return tree
}
