// Design: docs/guide/developer-setup.md -- the setup area as one command
//
// actions.go contains the ported Python setup area. The dispatch, help line,
// and two refusals are in internal/le/leaction. Only the TABLE remains here.
//
// SEVEN ACTIONS REPLACE TWO FLAGS, THE CLAUDE SERVER BOOTSTRAP, THE
// GENERATED-PROTO PIPELINE, AND THE LOCAL WEB SERVER. The first three actions
// represent the effective invocations of the old setup tool. The script
// declared --check and --no-vendor. These flags have four combinations, but
// only three can differ. The --no-vendor flag never reaches a check run because
// check mode returns before the vendoring step.
//
//	install          probe, install what is missing, then synchronize vendor/
//	check            probe only, change nothing, and fail on a required tool
//	tools            install as above and do not change vendor/
//	claude-server    provision the pinned Ubuntu development server
//	proto-generate   regenerate both protobuf Go files
//	proto-json-tags  apply explicit proto json_name options to generated Go
//	web-test         initialize an isolated config and run the local web server

// THE BARE COMMAND RUNS INSTALL. This is what `./le setup` has always done and
// what every document that names it means. `le functional` made the same
// choice for the same reason. This area has one long run, so it must not show a
// list to the operator who starts that run.

package setup

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "setup"

// The verbs, named once because each is spelled in the table, in the dispatch
// below and in the tests.
const (
	installVerb       = "install"
	checkVerb         = "check"
	toolsVerb         = "tools"
	claudeServerVerb  = "claude-server"
	protoGenerateVerb = "proto-generate"
	protoJSONTagsVerb = "proto-json-tags"
	webTestVerb       = "web-test"
)

// actions is the whole command surface.
//
// No action carries a Gate: people and native owners invoke the setup entry
// points directly.
var actions = leaction.New(area,
	leaction.Action{
		Verb:   installVerb,
		Writes: true,
		Why:    "install every tool a Ze dev or test workflow needs, then bring vendor/ in step with go.mod",
		Answer: func() (any, int) { return run(false, true) },
	},
	leaction.Action{
		Verb:   checkVerb,
		Why:    "probe only; change nothing, and fail when a required tool is missing",
		Answer: func() (any, int) { return run(true, true) },
	},
	leaction.Action{
		Verb:   toolsVerb,
		Writes: true,
		Why:    "install as `install` does, and leave vendor/ alone",
		Answer: func() (any, int) { return run(false, false) },
	},
	leaction.Action{
		Verb:   claudeServerVerb,
		Writes: true,
		Why:    "provision the pinned Ubuntu Claude development server",
		Parameters: []leaction.Parameter{
			{Keyword: "user", Value: "username"},
			{Keyword: "ssh-key-dir", Value: "path"},
		},
		AnswerArgs: runClaudeServer,
	},
	leaction.Action{
		Verb:   protoGenerateVerb,
		Writes: true,
		Why:    "regenerate the checked-in protobuf Go files with pinned native plugins",
		Answer: runProtoGenerate,
	},
	leaction.Action{
		Verb:   protoJSONTagsVerb,
		Writes: true,
		Why:    "apply explicit proto json_name options to generated Go tags",
		Answer: runProtoJSONTags,
	},
	leaction.Action{
		Verb:   webTestVerb,
		Writes: true,
		Why:    "initialize an isolated config and run the local web server on port 3443",
		Answer: runWebTestServer,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le setup` command. A bare command is the install run.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return run(false, true)
	}
	return actions.Answer(args)
}

// run runs every action in this area. The two options that each action passes
// are the only differences.
//
// Use 2 instead of 1 when the checkout cannot be found. No probe ran in this
// case. The caller can distinguish it from a run that found a missing tool.
func run(check, vendor bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	setup := &Setup{Root: root, Check: check, Vendor: vendor}
	return setup.Run()
}
