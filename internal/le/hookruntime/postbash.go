// Design: docs/architecture/core-design.md -- native post-command hook policy
// Related: postwrite.go -- postFormatGo, the format and lint step a Write or an Edit gets

package hookruntime

import (
	"bytes"
	stdcontext "context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/gitenv"
)

// postBashWindow is how recently a Go file must have changed for the check
// after a Bash call to judge it. The hook payload carries no start time for the
// command, so the window stands in for "changed by this command": a write
// earlier than the window in a longer command is missed, and another session's
// write inside it is reported too, which the message says.
const postBashWindow = 2 * time.Minute

// postBashBudget bounds the git status and the gofmt run together. The hook's
// own timeout in .claude/settings.json is 10s.
const postBashBudget = 5 * time.Second

// ze point: quality/linting/fix-lint-issues-never-disable-a-linter
// postBashGofmt names the Go files a shell command just changed that gofmt
// would reshape. A Write or an Edit is formatted by postFormatGo; a sed, a
// heredoc or a scripted rename reaches the tree without it, which is how a
// misformatted line survived a rename batch until a scoped lint found it. It
// reports and never rewrites: the window cannot tell this session's write from
// another's, and another session's file is not this one's to change.
//
// Only a file the command names, by its path or by a directory holding it,
// blocks. The window alone blocked every session for whichever file another
// session left unformatted, and put that file in each one's context after
// every command. A file the command does not name is said at code 0, which
// the person watching sees and the agent does not.
func postBashGofmt(ctx context) *verdict {
	timeout, cancel := stdcontext.WithTimeout(stdcontext.Background(), postBashBudget)
	defer cancel()

	changed, err := recentGoFiles(timeout, ctx.root, time.Now())
	if err != nil {
		return gofmtNotRun("the changed Go files could not be listed: " + err.Error())
	}
	if len(changed) == 0 {
		return nil
	}
	binary, err := exec.LookPath("gofmt")
	if err != nil {
		return gofmtNotRun("gofmt is not on PATH")
	}
	command := exec.CommandContext(timeout, binary, append([]string{"-l", "--"}, changed...)...) //nolint:gosec // gofmt from PATH, -l only, over paths git status listed
	command.Dir = ctx.root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	named := make([]string, 0, 4)
	unnamed := 0
	for _, path := range strings.Fields(string(output)) {
		if commandNames(stringInput(ctx.input, "command"), path) {
			named = append(named, path)
			continue
		}
		unnamed++
	}
	lines := make([]string, 0, len(named)+4)
	if len(named) != 0 {
		lines = append(lines, yellow+"⚠ gofmt: "+strconv.Itoa(len(named))+" Go file(s) this command names are not formatted"+reset)
		for _, path := range named {
			lines = append(lines, "  "+path)
		}
		lines = append(lines, "  Fix: gofmt -w <path>")
	}
	if err != nil {
		lines = append(lines, yellow+"⚠ gofmt could not read every changed file, so those were NOT checked: "+err.Error()+reset)
		for line := range strings.SplitSeq(stderr.String(), "\n") {
			if line != "" && len(lines) < len(named)+6 {
				lines = append(lines, "  "+dim+line+reset)
			}
		}
	}
	if len(lines) != 0 {
		return &verdict{2, strings.Join(lines, "\n")}
	}
	if unnamed != 0 {
		return &verdict{0, dim + "gofmt: " + strconv.Itoa(unnamed) + " unformatted Go file(s) changed in the last " + postBashWindow.String() + " that this command does not name" + reset}
	}
	return nil
}

// commandNames answers whether a shell command names path, a slash path
// relative to the checkout, either whole or through one of its parent
// directories. A command that writes a file names it or a directory holding
// it; `./` and a trailing slash are spelling, so the bare form is searched.
func commandNames(command, path string) bool {
	for dir := path; dir != "." && dir != "/" && dir != ""; dir = filepath.ToSlash(filepath.Dir(dir)) {
		if strings.Contains(command, dir) {
			return true
		}
	}
	return false
}

// gofmtNotRun is the verdict for a format check that could not judge the tree.
// It blocks for the reason lintNotRun does: only a blocking post-tool verdict
// reaches the agent.
func gofmtNotRun(why string) *verdict {
	return &verdict{2, yellow + "⚠ gofmt did NOT check this command's Go edits: " + why + reset}
}

// recentGoFiles answers the uncommitted Go files under root, tracked or not,
// whose content changed within postBashWindow of now, as paths relative to
// root. vendor/ and generated source are left out: neither is written by hand,
// and postFormatGo leaves generated source as its generator wrote it too.
func recentGoFiles(timeout stdcontext.Context, root string, now time.Time) ([]string, error) {
	command := exec.CommandContext(timeout, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	command.Dir = root
	// The deadline SIGKILLs git, and a status killed while it refreshes the index
	// leaves an orphaned .git/index.lock that blocks every session's git writes.
	command.Env = gitenv.ReadOnly(os.Environ())
	status, err := command.Output()
	if err != nil {
		return nil, err
	}
	entries := strings.Split(string(status), "\x00")
	recent := make([]string, 0, 8)
	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if len(entry) < 4 {
			continue
		}
		// A rename or a copy carries its source path as the next entry.
		if entry[0] == 'R' || entry[0] == 'C' {
			index++
		}
		path := entry[3:]
		if !strings.HasSuffix(path, ".go") || strings.HasPrefix(path, "vendor/") {
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		info, statErr := os.Stat(full)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, statErr
		}
		if now.Sub(info.ModTime()) > postBashWindow || !info.Mode().IsRegular() {
			continue
		}
		body, readErr := os.ReadFile(full) //nolint:gosec // a path git status listed under the checkout
		if readErr != nil {
			return nil, readErr
		}
		if generatedMarkerRE.Match(body) {
			continue
		}
		recent = append(recent, path)
	}
	return recent, nil
}
