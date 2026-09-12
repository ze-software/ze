// Design: docs/architecture/testing/verify-freshness-scope.md -- verification status command
package verifystatus

import (
	"errors"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

const name = "verify status"

// actions is this area's whole command surface. The table is what the manifest
// publishes and what `le verify status <verb> --help` renders. A reader learns
// the grammar and no verb runs, so the certificate stays as it was.
//
// check declares `path` Repeat. An operator scopes one check to as many paths
// as they name, and the body reads every one with Values.
var actions = leaction.New(name,
	leaction.Action{
		Verb:   "write",
		Why:    "record the verdict of a verification run as this checkout's certificate",
		Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "exit-code", Value: "code", Requirement: leaction.Required},
			{Keyword: "mode", Value: "name", Requirement: leaction.Optional},
		},
		AnswerArgs: write,
	},
	leaction.Action{
		Verb: "check",
		Why:  "report whether the recorded PASS still covers the tree, or the paths named",
		Parameters: []leaction.Parameter{
			{Keyword: "path", Value: "path", Requirement: leaction.Optional, Repeat: true},
		},
		AnswerArgs: check,
	},
	leaction.Action{
		Verb:   "show",
		Why:    "print the certificate the last verification wrote",
		Answer: show,
	},
	leaction.Action{
		Verb:   treeHashAction,
		Why:    "print the hash of the tracked tree as it stands now",
		Answer: treeHash,
	},
)

// treeHashAction is spelled once because the verb carries a hyphen, and a
// second spelling of it drifts in silence.
const treeHashAction = "tree-hash"

// defaultMode is the mode a write records when the caller names none. It is
// what this command has written since it replaced verify-status.sh.
const defaultMode = "ze-verify"

// Actions answers the command surface as data, for the manifest and for the
// help the dispatcher renders. It calls no handler.
func Actions() leaction.List { return actions.Actions() }

// Subs answers the action hint command help renders under this area.
func Subs() string { return actions.Subs() }

// Answer runs the native replacement for verify-status.sh.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// TreeHash is the structured tree-hash answer.
type TreeHash struct {
	TreeHash string `json:"tree-hash"`
}

// Text preserves the script's one-line tree hash output.
func (h TreeHash) Text() string { return h.TreeHash + "\n" }

// treeRoot answers the checkout every verb reads and writes under, and the
// code to answer when there is none. A tool that cannot find the tree says so
// and answers 1. That is the code for verification that did not run, which is
// a different fact from a grammar the area refused.
func treeRoot() (string, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return "", 1
	}
	return root, 0
}

// write records the verdict of one verification run. exit-code is declared
// Required and refused here, because the table PUBLISHES requiredness and each
// action's own body enforces it.
func write(args leaction.Arguments) (any, int) {
	if !args.Has("exit-code") {
		refuse("write requires exit-code <code>")
		return nil, 2
	}
	exit, err := strconv.Atoi(args.One("exit-code"))
	if err != nil {
		var tb textbuf.Buffer
		refuse(tb.Str("exit-code ").Quoted(args.One("exit-code")).Str(" is not an integer").String())
		return nil, 2
	}
	root, failed := treeRoot()
	if failed != 0 {
		return nil, failed
	}

	mode := defaultMode
	if args.Has("mode") {
		mode = args.One("mode")
	}
	start := job.SnapshotTree(root)
	certificate, err := verifyengine.WriteCertificate(root, verifyengine.WriteRequest{
		Exit: exit, Mode: mode, Skipped: verifyengine.SkippedSuites(), Start: start,
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return certificate, 0
}

// check answers whether the recorded PASS still covers the tree. A stale
// verdict answers 1, which is the code every caller of this gate reads.
func check(args leaction.Arguments) (any, int) {
	root, failed := treeRoot()
	if failed != 0 {
		return nil, failed
	}

	// Values rather than One: `path` is declared Repeat, so an invocation
	// naming several paths scopes the check to all of them.
	freshness := verifyengine.CheckCertificate(root, args.Values("path"))
	if freshness.Fresh {
		return freshness, 0
	}
	return freshness, 1
}

// show prints the certificate the last verification wrote. A checkout that was
// never verified holds none, which is a verdict rather than a failure to read.
func show() (any, int) {
	root, failed := treeRoot()
	if failed != 0 {
		return nil, failed
	}

	certificate, err := verifyengine.ReadCertificate(root)
	if os.IsNotExist(err) {
		return verifyengine.Freshness{Reason: "no status file at " + verifyengine.StatusPath}, 1
	}
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return certificate, 0
}

// treeHash prints the hash a certificate is compared against.
func treeHash() (any, int) {
	root, failed := treeRoot()
	if failed != 0 {
		return nil, failed
	}
	return TreeHash{TreeHash: job.TreeHash(root)}, 0
}

// refuse writes one failure line naming the command a reader typed.
func refuse(message string) {
	leaction.ReportError(errors.New(name + ": " + message))
}
