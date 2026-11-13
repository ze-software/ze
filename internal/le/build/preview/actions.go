// Design: docs/contributing/preview-artifacts.md -- the preview command surface.
package buildpreview

import (
	"context"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "build preview"

var actions = leaction.New(area,
	leaction.Action{Verb: "run", Heavy: true, Summary: "prepare identified Linux previews and optional images", Why: "Build both Linux architectures and optionally export local Docker images. The checksummed bundle remains unpublished.", Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "output", Kind: leaction.KindPath, Value: "directory", Help: "new directory for the preview bundle", Requirement: leaction.Required},
			{Keyword: "images", Kind: leaction.KindSwitch, Help: "also build and export native Docker images"},
		}, AnswerArgs: func(args leaction.Arguments) (any, int) {
			root, err := lepath.Root()
			if err != nil {
				leaction.ReportError(opererr.Wrap(err, "preview preparation cannot find the Ze checkout",
					"Compiler inputs cannot be located. Change to the Ze checkout containing go.mod and feature-gates.txt, then run this command there.",
					previewRunCommand(args.One("output"), args.Has("images"))))
				return nil, 1
			}
			return runPreview(context.Background(), root, args.One("output"), args.Has("images"), realDependencies())
		}},
	leaction.Action{Verb: "check", Summary: "check a prepared preview bundle", Why: "Check file hashes, both binary targets, feature inventory and archive contents without executing foreign binaries.",
		Parameters: []leaction.Parameter{{Keyword: "path", Kind: leaction.KindPath, Value: "directory", Help: "directory containing the prepared bundle", Requirement: leaction.Required}},
		AnswerArgs: func(args leaction.Arguments) (any, int) {
			return checkPreview(args.One("path"), inspectBinary)
		}},
)

// Actions describes the registered grammar.
func Actions() leaction.List { return actions.Actions() }

// Subs is the same grammar rendered for help.
func Subs() string { return actions.Subs() }

// Answer dispatches the closed preview grammar.
func Answer(args []string) (any, int) { return actions.Answer(args) }
