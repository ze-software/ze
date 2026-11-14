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

// runDoctorChecks runs every check registered for one config phase, in
// registry order, then the checks a compiled-in plugin registered as a Go
// function (registry.DoctorCheckDef) for that phase.
func runDoctorChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
	return append(runRegisteredChecks(phase, ctx), runPluginRegistryChecks(phase, ctx)...)
}

// runRegisteredChecks runs every check the in-tree registry
// (internal/core/diagnostic) holds for one phase, in registry order.
func runRegisteredChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
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
	return diags
}

// RunRuntimeChecks runs every check the in-tree registry holds for the
// runtime phase and answers their findings.
//
// It runs no plugin check. A plugin's runtime check reads state that lives in
// the plugin's own process, so its one owner is the doctor callback the daemon
// route sends to the running plugin (collectDoctorChecksFrom in the show
// doctor handler), which reaches a forked plugin too; the plugin registry
// refuses a runtime registry.DoctorCheckDef, so no second route can run it.
//
// Only the daemon route MUST call it (HandleDoctor, the show doctor handler):
// a runtime check reads the live state its owner holds in the daemon, and in
// any other process that state is empty, so a check run there would read a box
// it never looked at as healthy. runChecks, the local route, never asks for
// this phase.
//
// A runtime check is handed the platform and judged, the tree the post-config
// phase of the same answer judged (runChecks), nil when that phase did not run.
// That tree is the saved configuration doctor read, never the running one: the
// daemon route loads the config file from the store. A runtime check uses it
// only to agree with a config-phase check that read the same file, and never
// reads it as what the daemon runs; it reads its owner's live state where it
// lives.
func RunRuntimeChecks(judged *config.Tree) []diagnostic.Diagnostic {
	// The answer this runs beside already reports a platform detection
	// failure (runChecks, through resolveDoctorPlatform), so its findings are
	// not repeated here; a nil platform still runs every check declared for
	// any platform.
	platform, _ := resolveDoctorPlatform()
	return runRegisteredChecks(diagnostic.DoctorPhaseRuntime, doctorCheckContext{Tree: judged, Platform: platform})
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
