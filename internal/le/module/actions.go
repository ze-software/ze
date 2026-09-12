// Design: docs/architecture/core-design.md -- module migration workflows
package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var actions = leaction.New(area,
	leaction.Action{
		Verb:   "move",
		Why:    "preview or atomically relocate an internal package tree, rewrite imports, refresh plugin discovery, and prove generated registrations are preserved",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "source", Value: "name-or-internal-path", Requirement: leaction.Required},
			// A component moving to plugins, or back, derives its destination
			// from the source tier. Every other source needs the keyword, and
			// moveDestination is what refuses the move without it.
			{Keyword: "destination", Value: "tier-or-internal-path", Requirement: leaction.Optional},
			{Keyword: "apply"},
			{Keyword: "allow-rpc-drop"},
		},
		AnswerArgs: answerMove,
	},
	leaction.Action{
		Verb:   "rename",
		Why:    "preview or atomically rename the repository Go module and every tracked textual and on-disk spelling without corrupting generated protobuf descriptors",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "to", Value: "module", Requirement: leaction.Required},
			{Keyword: "from", Value: "module", Requirement: leaction.Optional},
			{Keyword: "repository", Value: "path", Requirement: leaction.Optional},
			{Keyword: "limit", Value: "rows", Requirement: leaction.Optional},
			{Keyword: "apply"},
			{Keyword: "no-goimports"},
			{Keyword: "no-reseal"},
		},
		AnswerArgs: answerRename,
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint rendered by help.
func Subs() string { return actions.Subs() }

// Answer is the `le module` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func answerMove(args leaction.Arguments) (any, int) {
	source := args.One("source")
	if source == "" {
		leaction.ReportError(fmt.Errorf("module move requires source <name-or-internal-path>"))
		return nil, 2
	}
	root, err := checkoutRoot("")
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := Move(root, MoveOptions{
		Source: source, Destination: args.One("destination"), Apply: args.Has("apply"),
		AllowRPCDrop: args.Has("allow-rpc-drop"),
	})
	if err != nil {
		leaction.ReportError(err)
		if report.Code == 0 {
			report.Code = 2
		}
	}
	return &report, report.Code
}

func answerRename(args leaction.Arguments) (any, int) {
	to := args.One("to")
	if to == "" {
		leaction.ReportError(fmt.Errorf("module rename requires to <module>"))
		return nil, 2
	}
	root, err := checkoutRoot(args.One("repository"))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	limit := 15
	if args.Has("limit") {
		raw := args.One("limit")
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 0 {
			leaction.ReportError(fmt.Errorf("limit requires a non-negative integer, got %q", raw))
			return nil, 2
		}
	}
	report, err := Rename(root, RenameOptions{
		Old: args.One("from"), New: to, Apply: args.Has("apply"), Limit: limit,
		NoGoimports: args.Has("no-goimports"), NoReseal: args.Has("no-reseal"),
	})
	if err != nil {
		leaction.ReportError(err)
		if report.Code == 0 {
			report.Code = 2
		}
	}
	return &report, report.Code
}

func checkoutRoot(explicit string) (string, error) {
	root := explicit
	var err error
	if root == "" {
		root, err = lepath.Root()
		if err != nil {
			return "", err
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("read repository root %s: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository root is not a directory: %s", root)
	}
	return root, nil
}
