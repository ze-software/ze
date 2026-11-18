package all

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// BenchmarkSchemaBuildStages times, and counts the allocations of, each stage
// of one build of the process's module set and of its config schema, over
// every module this composition root registers, one line per stage. A change
// to one stage is judged on that stage's line.
//
//	load              yang.StageLoad: every embedded and registered module text parsed into a Loader
//	process           Loader.StageProcess: goyang's Modules.Process (imports, types, entry trees)
//	check-extensions  Resolve's checkExtensions, through Resolved.RecheckExtensions
//	check-patterns    Resolve's checkPatterns, the process-wide pattern cache emptied first
//	check-structure   Resolve's checkStructure
//	check-references  Resolve's checkReferences (leafref paths)
//	resolve           the whole of Loader.Resolve, the cross-check of the five lines above it
//	lower             config.SchemaOf: the resolved set lowered to config.Schema
//
// Method: each line's input is prepared outside the timed region. The process
// and resolve lines load a fresh Loader each iteration with the timer stopped,
// because both run once per Loader; resolve also empties the pattern cache, as
// a process's first Resolve finds it. The checks and the lowering read one
// Resolved, which none of them writes to: the benchmark's own, resolved from
// StageLoad's Loader, never a joined set, whose compiled-in modules the
// checks would skip (the rechecks refuse one, ErrRecheckJoinedSet). Every line reports the module set it
// measured (modules: the resolved module names; texts: the registered module
// texts), so two runs are compared over one set only: the set depends on the
// build tags. The RFC 7950 grammar the structure check builds on first use
// (sync.OnceValue) is built once per process, by the Resolve that prepares
// the input, so it is in no line, though a start pays it once.
//
// Allocation figures are valid on any host. A time figure is recorded only
// from a host of known performance (ai/META-DECISIONS.md, "Test systems and
// labs").
//
// VALIDATES: AC-19 of spec-config-yang-compiled-at-build-time, the staged
// measure of today's schema build.
func BenchmarkSchemaBuildStages(b *testing.B) {
	resolved := resolveCompiledIn(b)
	modules := float64(len(resolved.ModuleNames()))
	texts := float64(len(configyang.Modules()))
	b.Logf("module set: %.0f resolved modules from %.0f registered module texts", modules, texts)
	reportModuleSet := func(b *testing.B) {
		b.Helper()
		b.ReportMetric(modules, "modules")
		b.ReportMetric(texts, "texts")
	}

	b.Run("load", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := configyang.StageLoad(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("process", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			loader := stageLoad(b)
			b.StartTimer()
			if err := loader.StageProcess(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("check-extensions", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := resolved.RecheckExtensions(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("check-patterns", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			configyang.ForgetCompiledPatterns()
			b.StartTimer()
			if err := resolved.RecheckPatterns(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("check-structure", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := resolved.RecheckStructure(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("check-references", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := resolved.RecheckReferences(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("resolve", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			loader := stageLoad(b)
			configyang.ForgetCompiledPatterns()
			b.StartTimer()
			if _, err := loader.Resolve(); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
	b.Run("lower", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := config.SchemaOf(resolved); err != nil {
				b.Fatal(err)
			}
		}
		reportModuleSet(b)
	})
}

// resolveCompiledIn answers a Resolved of its own over every registered
// module, through the real Resolve, for the stages that read one. It also
// resolves the shared set DefaultLoader answers, which config.SchemaOf asks
// for on each call: resolved here, that is a cache read inside the lower line
// rather than a whole build in its first iteration. The Resolved answered is
// not the shared one, so SchemaOf lowers it on every call instead of answering
// YANGSchema's cached build.
func resolveCompiledIn(tb testing.TB) *configyang.Resolved {
	tb.Helper()
	if _, err := configyang.DefaultLoader(); err != nil {
		tb.Fatalf("resolve the shared module set: %v", err)
	}
	resolved, err := stageLoad(tb).Resolve()
	if err != nil {
		tb.Fatalf("resolve the module set: %v", err)
	}
	return resolved
}

// stageLoad answers a Loader holding every embedded and registered module,
// not yet resolved.
func stageLoad(tb testing.TB) *configyang.Loader {
	tb.Helper()
	loader, err := configyang.StageLoad()
	if err != nil {
		tb.Fatalf("load the module texts: %v", err)
	}
	return loader
}
