// Design: docs/architecture/testing/verify-freshness-scope.md -- verification status command
package verifystatus

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/verifyengine"
)

const name = "verify-status"

// Action is one closed command form supported by verify-status.
type Action struct {
	Action string `json:"action"`
	Usage  string `json:"usage"`
}

// Actions is the structured command inventory.
type Actions struct {
	Actions []Action `json:"actions"`
}

// Answer runs the native replacement for verify-status.sh.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return actions(), 0
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	switch args[0] {
	case "write":
		return write(root, args[1:])
	case "check":
		return check(root, args[1:])
	case showAction:
		if len(args) != 1 {
			refuse(args[1])
			return nil, 2
		}
		certificate, readErr := verifyengine.ReadCertificate(root)
		if os.IsNotExist(readErr) {
			return verifyengine.Freshness{Reason: "no status file at " + verifyengine.StatusPath}, 1
		}
		if readErr != nil {
			leaction.ReportError(readErr)
			return nil, 1
		}
		return certificate, 0
	case treeHashAction:
		if len(args) != 1 {
			refuse(args[1])
			return nil, 2
		}
		return TreeHash{TreeHash: job.TreeHash(root)}, 0
	default:
		refuse(args[0])
		return nil, 2
	}
}

// The read-only verbs this command accepts.
const (
	showAction     = "show"
	treeHashAction = "tree-hash"
)

// TreeHash is the structured tree-hash answer.
type TreeHash struct {
	TreeHash string `json:"tree-hash"`
}

// Text preserves the script's one-line tree hash output.
func (h TreeHash) Text() string { return h.TreeHash + "\n" }

func write(root string, args []string) (any, int) {
	if len(args) < 2 {
		refuse("write requires exit-code <code>")
		return nil, 2
	}
	if args[0] != "exit-code" {
		refuse(args[0])
		return nil, 2
	}
	code, err := strconv.Atoi(args[1])
	if err != nil {
		refuse(fmt.Sprintf("exit-code %q is not an integer", args[1]))
		return nil, 2
	}
	mode := "ze-verify"
	if len(args) != 2 {
		if len(args) != 4 {
			refuse("write accepts only mode <name> after the exit code")
			return nil, 2
		}
		if args[2] != "mode" {
			refuse(args[2])
			return nil, 2
		}
		mode = args[3]
	}
	start := job.SnapshotTree(root)
	certificate, err := verifyengine.WriteCertificate(root, verifyengine.WriteRequest{
		Exit: code, Mode: mode, Skipped: verifyengine.SkippedSuites(), Start: start,
	})
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return certificate, 0
}

func check(root string, args []string) (any, int) {
	paths := make([]string, 0, len(args)/2)
	for len(args) != 0 {
		if len(args) < 2 {
			refuse("path requires a value")
			return nil, 2
		}
		if args[0] != "path" {
			refuse(args[0])
			return nil, 2
		}
		paths = append(paths, args[1])
		args = args[2:]
	}
	freshness := verifyengine.CheckCertificate(root, paths)
	if freshness.Fresh {
		return freshness, 0
	}
	return freshness, 1
}

func actions() Actions {
	return Actions{Actions: []Action{
		{Action: "write", Usage: "write exit-code <code> [mode <name>]"},
		{Action: "check", Usage: "check [path <path> ...]"},
		{Action: showAction, Usage: showAction},
		{Action: treeHashAction, Usage: treeHashAction},
	}}
}

func refuse(message string) {
	leaction.ReportError(errors.New("verify-status: " + message))
}
