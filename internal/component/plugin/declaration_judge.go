// Design: docs/architecture/plugin/plugin-system.md -- the Stage 1 check table and its judge
// Related: declarations.go -- queriedDeclarationRow, the first reader of the judge
// Related: server/stage1_checks.go -- the table that fills this seam

package plugin

import (
	"errors"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DeclarationVerdict is what one Stage 1 check says about one declaration.
//
// Err is nil when the declaration passes the check. A non-nil Err carries the
// text the daemon answers the plugin with, so a reader that prints it prints
// the daemon's own reason.
type DeclarationVerdict struct {
	// Check is the kebab-case name of the check.
	Check string
	// Section is the section of docs/plugin-development/protocol.md the check
	// enforces.
	Section string
	// KeepsPlugin says what the daemon does when the check fails. False means
	// the daemon refuses the plugin at Stage 1. True means the daemon keeps
	// the plugin, logs a warning and refuses only what the check names.
	KeepsPlugin bool
	// Err is the refusal, or nil.
	Err error
}

// UnjudgedCheck names one Stage 1 check that needs daemon state: the plugins
// the daemon loaded, its registries, or the operator's config. Nobody can
// judge it for a program in isolation, and Reason says which state it needs.
type UnjudgedCheck struct {
	Check   string
	Section string
	Reason  string
}

// DeclarationJudgement is the whole answer of the judge for one declaration:
// one verdict for each check it ran, in the daemon's order, and the checks it
// could not run.
type DeclarationJudgement struct {
	Verdicts []DeclarationVerdict
	Unjudged []UnjudgedCheck
}

// Refusal answers the reason the daemon refuses the plugin at Stage 1, or nil
// when it starts the plugin. The verdicts are in the daemon's order, so the
// first failed one is the reason the daemon gives.
//
// A failed check that keeps the plugin is not a refusal of the plugin, so it
// is not answered here.
func (j DeclarationJudgement) Refusal() error {
	for i := range j.Verdicts {
		verdict := &j.Verdicts[i]
		if verdict.KeepsPlugin {
			continue
		}
		if verdict.Err != nil {
			return verdict.Err
		}
	}
	return nil
}

// DeclarationJudge judges a Stage 1 declaration the way the daemon does, for
// the plugin named name, with no daemon running.
//
// It is a seam rather than a call because the dependency runs the other way:
// `internal/component/plugin/server` holds the check table and already imports
// this package, so this package cannot import it back. The server registers
// its judge from an init().
type DeclarationJudge func(name string, declaration *rpc.DeclareRegistrationInput) DeclarationJudgement

var (
	declarationJudgeMu sync.RWMutex
	declarationJudge   DeclarationJudge
)

// ErrNoDeclarationJudge is what JudgeDeclaration answers in a binary that
// links no Stage 1 check table.
//
// An empty seam is a refusal, never a pass: a binary that cannot judge says
// so, and a caller MUST NOT report the declaration as accepted
// (`ai/rules/principles.md`).
var ErrNoDeclarationJudge = errors.New("this binary links no Stage 1 check table " +
	"(internal/component/plugin/server), so the declaration cannot be judged here")

// SetDeclarationJudge registers the function that judges a Stage 1
// declaration. The plugin server calls it once, from an init().
//
// Safe for concurrent use.
func SetDeclarationJudge(judge DeclarationJudge) {
	if judge == nil {
		return
	}
	declarationJudgeMu.Lock()
	defer declarationJudgeMu.Unlock()
	declarationJudge = judge
}

// JudgeDeclaration runs the registered judge over declaration, for the plugin
// named name. It answers ErrNoDeclarationJudge when no judge is registered.
//
// Safe for concurrent use.
func JudgeDeclaration(name string, declaration *rpc.DeclareRegistrationInput) (DeclarationJudgement, error) {
	declarationJudgeMu.RLock()
	judge := declarationJudge
	declarationJudgeMu.RUnlock()

	if judge == nil {
		return DeclarationJudgement{}, ErrNoDeclarationJudge
	}
	return judge(name, declaration), nil
}
