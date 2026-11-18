// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// Related: loader.go — resolveDefault and Resolve, whose stages this file runs one at a time
// Related: pattern.go — the process-wide pattern cache ForgetCompiledPatterns empties
//
// The staged build of the process's module set. BenchmarkSchemaBuildStages
// (internal/component/plugin/all, where every module is registered) times and
// counts the allocations of each stage alone, so a change to one stage is
// judged on that stage's line. Each function here runs the stage function
// Resolve or resolveDefault runs, never a copy of its body, and
// TestStagesRunEveryStageOfResolve refuses a call Resolve or resolveDefault
// gains on their Loader, or of a package function, without this file calling
// it; a stage added inside one of those functions is measured on that
// function's line. The rechecks judge a set Resolve built from StageLoad's
// Loader, and refuse a joined set (ErrRecheckJoinedSet). Nothing here changes
// what Resolve does or costs.
package yang

import (
	"errors"
	"fmt"
)

// StageLoad loads every embedded and registered module into a new Loader: the
// load resolveDefault performs before it resolves, which is the first stage of
// a build of the process's module set. Unlike resolveDefault, which resolves
// what loaded and joins the parse errors to the resolve's, it answers no
// Loader once any module fails to parse: a stage measured over a module set
// that lacks a module measures another set.
func StageLoad() (*Loader, error) {
	l := NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("YANG LoadEmbedded: %w", err)
	}
	if err := l.LoadRegistered(); err != nil {
		return nil, err
	}
	return l, nil
}

// StageProcess runs the first stage of Resolve on l alone: it marks l
// resolved, as Resolve does, and runs goyang's import and type resolution
// (Modules.Process). Ze's checks do not run, so l then holds a module set no
// reader may take, and like Resolve it runs once: afterwards, successful or
// not, every load, every Resolve and every StageProcess on l returns
// ErrLoaderResolved. The caller discards l; the checks are measured by the
// Recheck methods of a Resolved.
func (l *Loader) StageProcess() error {
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
	l.modules.resolved = true
	return l.process()
}

// ErrRecheckJoinedSet marks a recheck of the extension, pattern or structure
// check asked of a Resolved the join built (Join, through reloaded and
// markJudged). Those three checks skip every module the join marked judged,
// the compiled-in ones, so over such a set they would judge, and cost, the
// external modules alone while answering as if they had judged the whole set.
// The recheck refuses rather than measure a smaller set under the same name.
var ErrRecheckJoinedSet = errors.New("YANG recheck asked of a joined module set")

// RecheckExtensions runs Resolve's extension check (checkExtensions) again
// over r's module set, and answers its verdict. No check writes to the module
// set, so the set it reads is the one Resolve's own check read, just after
// goyang's Process; for a Resolved that Resolve built from StageLoad's Loader
// the verdict is nil. On a joined Resolved it answers ErrRecheckJoinedSet and
// runs nothing.
func (r *Resolved) RecheckExtensions() error {
	l, err := r.recheckLoader()
	if err != nil {
		return err
	}
	return l.checkExtensions()
}

// RecheckPatterns runs Resolve's pattern check (checkPatterns) again over r's
// module set, answers its verdict, and drops the patterns it compiled: r
// already holds them. compilePattern keeps every pattern it compiled for the
// life of the process, so a caller measuring what the first Resolve of a
// process pays calls ForgetCompiledPatterns before each run. On a joined
// Resolved it answers ErrRecheckJoinedSet and runs nothing.
func (r *Resolved) RecheckPatterns() error {
	l, err := r.recheckLoader()
	if err != nil {
		return err
	}
	_, err = l.checkPatterns()
	return err
}

// RecheckStructure runs Resolve's structure check (checkStructure) again over
// r's module set, and answers its verdict. On a joined Resolved it answers
// ErrRecheckJoinedSet and runs nothing.
func (r *Resolved) RecheckStructure() error {
	l, err := r.recheckLoader()
	if err != nil {
		return err
	}
	return l.checkStructure()
}

// RecheckReferences runs Resolve's leafref check (checkReferences) again over
// r, the Resolved Resolve builds once the other checks passed, and answers its
// verdict. checkReferences judges every module of the set, a joined one too,
// so it runs on any Resolved.
func (r *Resolved) RecheckReferences() error {
	return checkReferences(r)
}

// recheckLoader answers a Loader over r's module set, for the checks that are
// Loader methods, or ErrRecheckJoinedSet when the join marked modules of the
// set judged: those checks would skip them. The set is resolved, so this
// Loader refuses every load and every Resolve (ErrLoaderResolved), and it
// never leaves this file.
func (r *Resolved) recheckLoader() (*Loader, error) {
	m := r.set()
	if len(m.judged) > 0 {
		return nil, fmt.Errorf("%w: the check skips the %d modules the join marked judged; "+
			"recheck the Resolved that Loader.Resolve answers for StageLoad's Loader", ErrRecheckJoinedSet, len(m.judged))
	}
	return &Loader{modules: m}, nil
}

// ForgetCompiledPatterns empties compilePattern's process-wide cache, so the
// next pattern check compiles every pattern again, as the first Resolve of a
// process does. It changes no answer, only what the next compile costs. A
// running daemon has no reason to call it: every value validated after it
// compiles its pattern again.
func ForgetCompiledPatterns() {
	patternCache.Clear()
}
