// Design: docs/architecture/testing/interop.md -- the lab pin area, as one command
// Related: scan.go -- the pins this area reads out of the tree.
// Related: upstream.go -- what upstream answers for each pin.

package pin

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "lab pin"

// dockerfileKeyword names the Dockerfile whose source pin bump moves.
const dockerfileKeyword = "dockerfile"

// checkTimeout bounds one whole check: every registry, proxy and GitHub
// answer it waits for.
const checkTimeout = 5 * time.Minute

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Summary: "compare every lab peer pin with upstream", Why: "list every lab peer pin the tree holds (image tags, go install versions, git tags, source commits)" +
		" and compare each with upstream's latest stable release, over the network; a report, never a commit gate",
		Answer: runCheck},
	leaction.Action{Verb: "update", Writes: true,
		Summary: "move a source-built peer to upstream's latest commit",
		Why: "move a source-built peer's ZE_SOURCE_COMMIT and ZE_SOURCE_DATE to upstream's latest default-branch commit;" +
			" the next image build proves the source still builds and any Ze patch still applies",
		Parameters: []leaction.Parameter{{Keyword: dockerfileKeyword, Kind: leaction.KindPath, Help: "the Dockerfile whose source pin moves", Value: "path", Requirement: leaction.Required}},
		AnswerArgs: runBump},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le lab pin` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runCheck walks the tree, asks upstream about every pin and answers the
// rows. Exit 0 whatever is behind: the check reports and gates nothing; 2
// when the walk itself failed.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	pins, err := Scan(tree)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return Compare(ctx, newUpstream(httpFetcher{}), pins), 0
}

// runBump moves one Dockerfile's source pin to upstream's latest commit.
func runBump(args leaction.Arguments) (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	bumped, err := Bump(ctx, newUpstream(httpFetcher{}), tree, args.One(dockerfileKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return bumped, 0
}

// Bumped is what bump answers: the pin before and after.
type Bumped struct {
	Dockerfile string `json:"dockerfile"`
	Repository string `json:"repository"`
	From       string `json:"from"`
	To         string `json:"to"`
	Date       string `json:"date"`
	Changed    bool   `json:"changed"`
}

// Text renders the bump for a person.
func (b Bumped) Text() string {
	var tb textbuf.Buffer
	if !b.Changed {
		tb.Str(b.Dockerfile).Str(" already pins ").Str(b.Repository).Str(" at its latest commit ").Str(interoplab.ShortCommit(b.To)).Str(".\n")
		return tb.String()
	}
	tb.Str(b.Dockerfile).Str(": ").Str(b.Repository).Str(" ").Str(interoplab.ShortCommit(b.From)).Str(" -> ").
		Str(interoplab.ShortCommit(b.To)).Str(" (").Str(b.Date).Str(").\n").
		Str("Next: build the image to prove the new source still builds and any Ze patch still applies, then commit the Dockerfile.\n")
	return tb.String()
}

// Bump rewrites the ZE_SOURCE_COMMIT and ZE_SOURCE_DATE lines of the
// Dockerfile at path (from the tree root) to upstream's latest commit.
func Bump(ctx context.Context, up *upstream, tree, path string) (Bumped, error) {
	full := filepath.Join(tree, path)
	body, err := os.ReadFile(full) //nolint:gosec // the operator names a Dockerfile in their own checkout
	if err != nil {
		return Bumped{}, errors.New("cannot read " + path + ": " + err.Error() + ".\nFix: name a Dockerfile path from the repository root")
	}
	declared, found, err := interoplab.ParseSourcePin(path, string(body))
	if err != nil {
		return Bumped{}, err
	}
	if !found || declared.Commit == "" {
		return Bumped{}, errors.New(path + " pins no source commit (no " + interoplab.SourceRepositoryArg + " with " +
			interoplab.SourceCommitArg + "), so there is nothing to bump.\nFix: name the Dockerfile of a source-built peer, such as test/interop-rsvpte/Dockerfile.freertr")
	}
	latest, err := up.latestCommit(ctx, declared.Repository)
	if err != nil {
		return Bumped{}, err
	}
	bumped := Bumped{Dockerfile: path, Repository: declared.Repository, From: declared.Commit, To: latest.Commit,
		Date: latest.Date, Changed: latest.Commit != declared.Commit}
	if !bumped.Changed {
		return bumped, nil
	}
	lines := strings.Split(string(body), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ARG "+interoplab.SourceCommitArg+"=") {
			lines[index] = "ARG " + interoplab.SourceCommitArg + "=" + latest.Commit
		}
		if strings.HasPrefix(trimmed, "ARG "+interoplab.SourceDateArg+"=") {
			lines[index] = "ARG " + interoplab.SourceDateArg + "=" + latest.Date
		}
	}
	info, err := os.Stat(full)
	if err != nil {
		return Bumped{}, errors.New("cannot stat " + path + ": " + err.Error())
	}
	if err := os.WriteFile(full, []byte(strings.Join(lines, "\n")), info.Mode().Perm()); err != nil {
		return Bumped{}, errors.New("cannot write " + path + ": " + err.Error() + ".\nFix: check the file is writable")
	}
	return bumped, nil
}
