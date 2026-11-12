// actions.go exposes the native netlab render check (`check`) and golden rewrite
// (`update`) through leaction.
package testnetlab

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "test netlab"

type actionRunner func() (any, int)

func actionTable(runCheck, runUpdate actionRunner, additional ...leaction.Action) leaction.Area {
	base := []leaction.Action{
		leaction.Action{Verb: "check", Summary: "render netlab topologies and compare goldens", Why: "render contrib/netlab with the installed netlab, compare every node with its golden file," +
			" and prove each golden is accepted by ze config validate",
			Answer: runCheck},
		leaction.Action{
			Verb:    "update",
			Summary: "rewrite the netlab goldens from a real render",
			Why:     "rewrite contrib/netlab/golden from a real netlab render, then validate every rewritten file",
			Writes:  true,
			Answer:  runUpdate,
		},
	}
	return leaction.New(area, append(base, additional...)...)
}

var actions = actionTable(runCheckHere, runUpdateHere,
	leaction.Action{
		Verb: "package", Does: "build", Summary: "export a self-contained netlab topology", Writes: true,
		Why:        "copy the topology, local daemon definition and templates into a new directory, ready for netlab up without a source checkout",
		Parameters: []leaction.Parameter{{Keyword: "directory", Kind: leaction.KindPath, Value: "path", Requirement: leaction.Required, Help: "create this new lab directory"}},
		AnswerArgs: packageHere,
	},
	leaction.Action{
		Verb: "run", Summary: "prove live protocols, routes and traffic, then stop", Writes: true,
		Why: "build a local Ze lab image and pinned upstream tools, run the reference topology on the Ze Docker host, assert every declared adjacency and route plus loopback traffic, and tear down the lab",
		Parameters: []leaction.Parameter{
			{Keyword: "runtime", Kind: leaction.KindEnum, Value: "runtime", Requirement: leaction.Optional, Help: "run tools in a container or this Docker host namespace", Values: []string{"container", "host"}, ValueHelp: map[string]string{"container": "build and run the tool container on the Ze Docker host", "host": "run the packaged lab inside the prepared tool container"}},
			{Keyword: "directory", Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional, Help: "prepared lab directory for runtime host"},
		},
		AnswerArgs: runLiveHere,
	},
)

// Actions answers the command surface as structured data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line action hint shown by command help.
func Subs() string { return actions.Subs() }

// Answer is the le test netlab command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) { return runHere(false) }

func runUpdateHere() (any, int) { return runHere(true) }

func runHere(update bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		verb := "check"
		if update {
			verb = "update"
		}
		leaction.ReportError(opererr.Wrap(err, "netlab "+verb+" cannot find the Ze checkout",
			"The contributed topology and templates cannot be located. Change to the Ze checkout containing go.mod and feature-gates.txt before running this command.",
			"./le test netlab "+verb))
		return nil, 1
	}
	report, code := newChecker(root).Run(update)
	if diagnostics := report.errorText(); diagnostics != "" {
		fmt.Fprint(os.Stderr, diagnostics) //nolint:errcheck // CLI diagnostic output
	}
	return report, code
}
