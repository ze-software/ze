// Design: docs/contributing/committing.md -- the commit secrets command
// Related: secrets.go -- the scanner and changecheck judge
// Related: register.go -- command and changecheck registration
package commitsecrets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "commit secrets"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb:    "check",
		Summary: "refuse secrets, keys, or local home paths in changes",
		Why:     "scan modified and untracked files against HEAD using the same rules `./le commit create` enforces, so leaks are caught before committing",
		Answer:  runCheck,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le commit secrets` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// Report is the structured answer of `./le commit secrets check`.
type Report struct {
	Checked  int       `json:"checked"`
	Findings []Finding `json:"findings,omitempty"`
}

// Text renders the report for a person.
func (r Report) Text() string {
	var tb textbuf.Buffer
	if len(r.Findings) == 0 {
		tb.Str("commit secrets: ok (").Int(int64(r.Checked)).Str(" file(s) checked)\n")
		return tb.String()
	}
	tb.Str(FormatFindings(r.Findings))
	return tb.String()
}

func runCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "cannot locate repository root", "checkout markers were not found", "run `./le commit secrets check` from inside a Ze checkout"))
		return nil, 2
	}
	report, err := CheckWorkingTree(root, os.Environ())
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "cannot scan working tree for secrets", "git status or file read failed", "inspect repository state with `git status` and re-run `./le commit secrets check`"))
		return nil, 2
	}
	if len(report.Findings) > 0 {
		return report, 1
	}
	return report, 0
}

// CheckWorkingTree scans every modified or untracked non-ignored file in root
// against HEAD using the same rules the commit gate enforces.
func CheckWorkingTree(root string, environ []string) (Report, error) {
	paths, err := dirtyPaths(root)
	if err != nil {
		return Report{}, err
	}
	if len(paths) == 0 {
		return Report{}, nil
	}
	change := changecheck.Change{
		Root:   root,
		Paths:  paths,
		Before: gitHeadSide(root),
		After:  diskSide(root),
	}
	findings, err := ScanChange(change, environ)
	if err != nil {
		return Report{}, err
	}
	return Report{Checked: len(paths), Findings: findings}, nil
}

func dirtyPaths(root string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", root, "status", "--porcelain", "-z", "--untracked-files=all") //nolint:gosec // git status over the resolved checkout root
	out, err := cmd.Output()
	if err != nil {
		return nil, opererr.Wrap(err, "cannot list uncommitted files in "+root, "git status failed", "run `git status` to inspect repository state")
	}
	var paths []string
	seen := make(map[string]bool)
	entries := bytes.Split(out, []byte{0})
	for idx := 0; idx < len(entries); idx++ {
		raw := entries[idx]
		if len(raw) < 4 {
			continue
		}
		status := string(raw[:2])
		rel := string(raw[3:])
		// Renames and copies emit the destination first, then the original path
		// in the next NUL-delimited entry; skip the old path.
		if strings.ContainsAny(status, "RC") {
			idx++
		}
		switch status {
		case " D", "D ", "DD":
			continue
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		paths = append(paths, rel)
	}
	slices.Sort(paths)
	return paths, nil
}

func gitHeadSide(root string) changecheck.Side {
	return func(rel string) (string, bool, error) {
		spec := "HEAD:" + rel
		cmd := exec.CommandContext(context.Background(), "git", "-C", root, "cat-file", "-e", spec) //nolint:gosec // git object check over a path from git status
		if err := cmd.Run(); err != nil {
			if _, exited := errors.AsType[*exec.ExitError](err); exited {
				return "", false, nil
			}
			return "", false, opererr.Wrap(err, "cannot check "+spec, "git cat-file failed", "verify git is installed and repository state is healthy")
		}
		show := exec.CommandContext(context.Background(), "git", "-C", root, "show", spec) //nolint:gosec // git show of an existing HEAD blob
		out, err := show.Output()
		if err != nil {
			return "", false, opererr.Wrap(err, "cannot read "+spec, "git show failed", "verify git object integrity with `git status`")
		}
		return string(out), true, nil
	}
}

func diskSide(root string) changecheck.Side {
	return func(rel string) (string, bool, error) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		if !info.Mode().IsRegular() {
			return "", false, nil
		}
		raw, err := os.ReadFile(full) //nolint:gosec // reading a working-tree path reported by git status under root
		if err != nil {
			return "", false, err
		}
		return string(raw), true, nil
	}
}
