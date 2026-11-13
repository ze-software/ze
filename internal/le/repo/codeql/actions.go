// Design: docs/architecture/testing/ci-workflows.md -- one callable CodeQL selector.
package codeql

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
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
}, leaction.Action{
	Verb:    "prepare",
	Does:    "create",
	Writes:  true,
	Summary: "prepare sentinels and record explicit cache refusal",
	Why:     "bind the later database check to exact authored and excluded cache source bytes",
	Parameters: []leaction.Parameter{
		{Keyword: "file", Kind: leaction.KindPath, Value: "manifest", Help: "a new proof manifest in an existing task-owned directory", Requirement: leaction.Required},
		{Keyword: "tags", Kind: leaction.KindString, Value: "tags", Help: "the actual Go build context for the explicit cache refusal", Requirement: leaction.Required},
	},
	AnswerArgs: prepareExtractionHere,
}, leaction.Action{
	Verb:    "check",
	Heavy:   true,
	Writes:  true,
	Summary: "check authored inclusion and cache exclusion",
	Why:     "a tracked package selection is not evidence of what CodeQL actually extracted",
	Parameters: []leaction.Parameter{
		{Keyword: "file", Kind: leaction.KindPath, Value: "manifest", Help: "the manifest saved by prepare", Requirement: leaction.Required},
		{Keyword: "database", Kind: leaction.KindPath, Value: "database", Help: "the finalized Go database reported by CodeQL analyze", Requirement: leaction.Required},
		{Keyword: "executable", Kind: leaction.KindPath, Value: "codeql", Help: "the bundled CodeQL executable reported by CodeQL init", Requirement: leaction.Required},
	},
	AnswerArgs: checkExtractionHere,
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

func prepareExtractionHere(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the CodeQL extraction prepare step found no Ze checkout",
			"it reads and records source files of the repository it runs in",
			"run the same ./le repo codeql prepare command from inside a Ze checkout"))
		return nil, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report, err := PrepareExtraction(ctx, root, arguments.One("file"), arguments.One("tags"))
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the CodeQL extraction proof could not be prepared",
			"the later database check is bound to the sentinels and the cache refusal this step records, so without them the extraction has no proof",
			"fix what the cause names, then run ./le repo codeql prepare file <new manifest in an existing directory> tags '<actual go build tags>' again"))
		return nil, 2
	}
	return report, 0
}

func checkExtractionHere(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the CodeQL extraction check found no Ze checkout",
			"it reads and records source files of the repository it runs in",
			"run the same ./le repo codeql check command from inside a Ze checkout"))
		return nil, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	report, err := CheckExtraction(ctx, root, arguments.One("file"), arguments.One("database"), arguments.One("executable"))
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the CodeQL extraction check could not judge the database",
			"a tracked package selection is not evidence of what CodeQL extracted, so only the database can show the authored sentinels and no cache source",
			"fix what the cause names, then run ./le repo codeql check file <manifest saved by prepare> database <finalized Go database> executable <bundled codeql executable> again"))
		return nil, 2
	}
	return report, 0
}
