// Design: docs/architecture/testing/ci-workflows.md -- one callable CodeQL selector.
package codeql

import (
	"context"
	"errors"
	"strings"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

var actions = leaction.New(area, leaction.Action{
	Verb:    "packages",
	Does:    "list",
	Summary: "list tracked packages after cache dependency admission",
	Why:     "the exact roots of the next traced build, derived from tracked Go in its actual tag context; errors never widen to ./...",
	Parameters: []leaction.Parameter{
		{Keyword: "tags", Kind: leaction.KindString, Value: "tags", Help: "the actual traced go build tag argument", Requirement: leaction.Required},
	},
	AnswerArgs: packagesHere,
})

// Actions exposes the same table the native dispatcher uses.
func Actions() leaction.List { return actions.Actions() }

// Subs derives command help from that table.
func Subs() string { return actions.Subs() }

// Answer dispatches the registered CodeQL action.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// PackageReport carries explicit package arguments to the shared renderer.
type PackageReport struct {
	Packages []string `json:"packages"`
}

// Text gives the workflow one explicit package directory per line.
func (report PackageReport) Text() string {
	return strings.Join(report.Packages, "\n") + "\n"
}

func packagesHere(arguments leaction.Arguments) (any, int) {
	if !arguments.Has("tags") {
		leaction.ReportError(errors.New("CodeQL packages requires tags '<actual go build tags>'"))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	target, err := BuildContext(ctx, root, arguments.One("tags"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	packages, err := Select(ctx, root, target)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return PackageReport{Packages: packages}, 0
}
