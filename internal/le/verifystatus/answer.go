// Design: docs/architecture/testing/verify-freshness-scope.md -- verification status command
package verifystatus

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lejob"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/verify"
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
	case "show":
		if len(args) != 1 {
			return refuse(args[1]), 2
		}
		certificate, readErr := verify.ReadCertificate(root)
		if os.IsNotExist(readErr) {
			return verify.Freshness{Reason: "no status file at " + verify.StatusPath}, 1
		}
		if readErr != nil {
			leaction.ReportError(readErr)
			return nil, 1
		}
		return certificate, 0
	case "tree-hash":
		if len(args) != 1 {
			return refuse(args[1]), 2
		}
		return TreeHash{TreeHash: lejob.TreeHash(root)}, 0
	default:
		return refuse(args[0]), 2
	}
}

// TreeHash is the structured tree-hash answer.
type TreeHash struct {
	TreeHash string `json:"tree-hash"`
}

// Text preserves the script's one-line tree hash output.
func (h TreeHash) Text() string { return h.TreeHash + "\n" }

func write(root string, args []string) (any, int) {
	if len(args) < 2 {
		return refuse("write requires exit-code <code>"), 2
	}
	if args[0] != "exit-code" {
		return refuse(args[0]), 2
	}
	code, err := strconv.Atoi(args[1])
	if err != nil {
		return refuse(fmt.Sprintf("exit-code %q is not an integer", args[1])), 2
	}
	mode := "ze-verify"
	if len(args) != 2 {
		if len(args) != 4 {
			return refuse("write accepts only mode <name> after the exit code"), 2
		}
		if args[2] != "mode" {
			return refuse(args[2]), 2
		}
		mode = args[3]
	}
	start := lejob.SnapshotTree(root)
	certificate, err := verify.WriteCertificate(root, verify.WriteRequest{
		Exit: code, Mode: mode, Skipped: verify.SkippedSuites(), Start: start,
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
			return refuse("path requires a value"), 2
		}
		if args[0] != "path" {
			return refuse(args[0]), 2
		}
		paths = append(paths, args[1])
		args = args[2:]
	}
	freshness := verify.CheckCertificate(root, paths)
	if freshness.Fresh {
		return freshness, 0
	}
	return freshness, 1
}

func actions() Actions {
	return Actions{Actions: []Action{
		{Action: "write", Usage: "write exit-code <code> [mode <name>]"},
		{Action: "check", Usage: "check [path <path> ...]"},
		{Action: "show", Usage: "show"},
		{Action: "tree-hash", Usage: "tree-hash"},
	}}
}

func refuse(message string) any {
	leaction.ReportError(errors.New("verify-status: " + message))
	return nil
}
