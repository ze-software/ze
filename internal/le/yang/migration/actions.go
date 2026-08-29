// Design: plan/spec-le-is-a-ze-binary.md -- three native YANG migration actions
package yangmigration

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const area = "yang migration"

var actions = leaction.New(area,
	leaction.Action{
		Verb:       "commands-to-plugins",
		Why:        "move owned *-cmd.yang modules and their schema tests from components to plugin YANG directories",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: keywordApply}},
		AnswerArgs: runCommandsToPluginsHere,
	},
	leaction.Action{
		Verb:   "path-refactor",
		Why:    "rewrite one removed, renamed, or moved YANG path across YANG, Go, .ci, and .et syntax",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "operation", Value: "remove|rename|move"},
			{Keyword: "target", Value: "segment"},
			{Keyword: "replacement", Value: "segment"},
			{Keyword: "under", Value: valuePath},
			{Keyword: "source", Value: valuePath},
			{Keyword: "destination", Value: valuePath},
			{Keyword: "list-nodes", Value: "comma-list"},
			{Keyword: keywordApply},
		},
		AnswerArgs: runPathRefactorHere,
	},
	leaction.Action{
		Verb:       "schema-to-yang",
		Why:        "rename YANG-bearing schema directories and update Go syntax and documentation paths",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: keywordApply}},
		AnswerArgs: runSchemaToYangHere,
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Gates is empty because these operator refactors have no Make target.

// Subs is the one-line help surface derived from the action table.
func Subs() string { return actions.Subs() }

// Answer dispatches the yang-migration action grammar.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCommandsToPluginsHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := commandsToPlugins(root, args.Has(keywordApply))
	return actionResult(report, err)
}

func runSchemaToYangHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := schemaToYang(root, args.Has(keywordApply))
	return actionResult(report, err)
}

func runPathRefactorHere(args leaction.Arguments) (any, int) {
	operation, err := operationFromArguments(args)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := refactorPaths(root, operation, args.Has(keywordApply))
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Refused() {
		return report, 1
	}
	if !report.Changed() && len(report.Manual) == 0 {
		return report, 1
	}
	return report, 0
}

func actionResult(report Report, err error) (any, int) {
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Refused() {
		return report, 1
	}
	return report, 0
}

func operationFromArguments(args leaction.Arguments) (pathOperation, error) {
	kind := PathOperationKind(args["operation"])
	listNodes := defaultListNodes()
	if value := args["list-nodes"]; value != "" {
		listNodes = make(map[string]bool)
		for name := range strings.SplitSeq(value, ",") {
			if name == "" {
				return pathOperation{}, fmt.Errorf("list-nodes contains an empty name")
			}
			listNodes[name] = true
		}
	}
	op := pathOperation{
		Kind:        kind,
		Target:      args["target"],
		Replacement: args["replacement"],
		Under:       args["under"],
		Source:      args["source"],
		Destination: args["destination"],
		ListNodes:   listNodes,
	}
	if err := op.Validate(); err != nil {
		return pathOperation{}, err
	}
	return op, nil
}
