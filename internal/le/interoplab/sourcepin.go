// Design: docs/architecture/testing/interop.md -- source-built peers and the monthly freshness rule
// Related: docker.go -- Docker.Build, the one place every lab image is built, calls RequireFreshSource.
//
// A peer that publishes no stable release (tac_plus-ng, freeRtr) is built from
// source, and the owner's standing decisions build it only from source less than one month
// old. The Dockerfile that builds the peer is the declaration: it names the
// upstream repository and either the pinned commit with its commit date, or the
// named build context that holds a local checkout. No list of peers exists
// beside those Dockerfiles, so a new source-built peer joins the rule by
// carrying the build arguments below.

package interoplab

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gitenv"
)

// The build arguments a source-built peer's Dockerfile declares. They are
// ordinary Docker build arguments, so the build itself reads the same values
// this file judges and the two cannot disagree.
const (
	// SourceRepositoryArg names the upstream GitHub repository, owner/name.
	SourceRepositoryArg = "ZE_SOURCE_REPOSITORY"
	// SourceCommitArg names the pinned upstream commit, as a full hex id.
	SourceCommitArg = "ZE_SOURCE_COMMIT"
	// SourceDateArg is the pinned commit's committer date, RFC 3339.
	SourceDateArg = "ZE_SOURCE_DATE"
	// SourceCheckoutArg names the build context that holds a local checkout
	// the build follows instead of a pin.
	SourceCheckoutArg = "ZE_SOURCE_CHECKOUT"
)

// SourceBumpCommand is the command that moves a pin to upstream's latest
// commit; the refusal of a stale pin prints it with the Dockerfile path.
const SourceBumpCommand = "./le lab pin update dockerfile "

// sourceGitTimeout bounds one git command against a local checkout. A fetch
// over a slow link is the longest of them.
const sourceGitTimeout = 3 * time.Minute

// SourcePin is what one Dockerfile declares about the source it builds.
// Exactly one of Commit and Checkout is set on a pin ParseSourcePin answers.
type SourcePin struct {
	Dockerfile string    `json:"dockerfile"`
	Repository string    `json:"repository"`
	Commit     string    `json:"commit,omitempty"`
	Date       time.Time `json:"date,omitzero"`
	Checkout   string    `json:"checkout,omitempty"`
}

// ParseSourcePin reads the source declaration out of a Dockerfile body. It
// answers false when the body declares no repository, which is every image
// that is not built from a source tree, and an error when the declaration is
// incomplete, because a half-declared pin would build with no freshness check.
func ParseSourcePin(dockerfile, body string) (SourcePin, bool, error) {
	pin := SourcePin{Dockerfile: dockerfile}
	var date string
	for raw := range strings.SplitSeq(body, "\n") {
		name, value, found := buildArgument(raw)
		if !found {
			continue
		}
		switch name {
		case SourceRepositoryArg:
			pin.Repository = value
		case SourceCommitArg:
			pin.Commit = value
		case SourceDateArg:
			date = value
		case SourceCheckoutArg:
			pin.Checkout = value
		}
	}
	if pin.Repository == "" {
		return pin, false, nil
	}
	if pin.Checkout != "" {
		return pin, true, nil
	}
	if pin.Commit == "" {
		return pin, true, pinError(dockerfile, "declares "+SourceRepositoryArg+" with neither "+SourceCommitArg+" nor "+SourceCheckoutArg)
	}
	if date == "" {
		return pin, true, pinError(dockerfile, "pins "+SourceCommitArg+" with no "+SourceDateArg)
	}
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return pin, true, pinError(dockerfile, SourceDateArg+" "+date+" is not an RFC 3339 date")
	}
	pin.Date = parsed
	return pin, true, nil
}

// buildArgument answers the name and default of one `ARG NAME=value` line.
func buildArgument(raw string) (string, string, bool) {
	words := strings.Fields(raw)
	if len(words) != 2 {
		return "", "", false
	}
	if !strings.EqualFold(words[0], "ARG") {
		return "", "", false
	}
	return strings.Cut(words[1], "=")
}

// pinError names the Dockerfile and what its declaration lacks.
func pinError(dockerfile, problem string) error {
	var tb textbuf.Buffer
	tb.Str(dockerfile).Str(": the source pin ").Str(problem).
		Str(". A source-built peer is built only from source less than one month old, so the build cannot judge it.\n").
		Str("Fix: declare ").Str(SourceRepositoryArg).Str(", ").Str(SourceCommitArg).Str(" and ").Str(SourceDateArg).
		Str(" together, or ").Str(SourceRepositoryArg).Str(" with ").Str(SourceCheckoutArg).
		Str(" (docs/architecture/testing/interop.md, \"Source-built peers\")")
	return errors.New(tb.String())
}

// Stale reports whether source committed at date is more than one calendar
// month old at now.
func Stale(date, now time.Time) bool {
	return now.After(date.AddDate(0, 1, 0))
}

// AgeDays is the whole days between date and now, rounded down.
func AgeDays(date, now time.Time) int64 {
	return int64(now.Sub(date) / (24 * time.Hour))
}

// staleRefusal is the refusal of a pinned commit more than a month old.
func (p SourcePin) staleRefusal(now time.Time) error {
	var tb textbuf.Buffer
	tb.Str(p.Dockerfile).Str(" builds ").Str(p.Repository).Str(" at commit ").Str(ShortCommit(p.Commit)).
		Str(", committed ").Str(p.Date.UTC().Format(time.DateOnly)).Str(", ").Int(AgeDays(p.Date, now)).
		Str(" days ago. A source-built peer is built only from source less than one month old, so this build is refused.\n").
		Str("Fix: ").Str(SourceBumpCommand).Str(p.Dockerfile)
	return errors.New(tb.String())
}

// ShortCommit is the twelve-character form a reader matches against git log.
func ShortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

// RequireFreshSource refuses a build whose Dockerfile pins source more than
// one month old, and brings a followed local checkout up to its upstream when
// its HEAD is that old. A Dockerfile that declares no source answers nil.
func (d *Docker) RequireFreshSource(ctx context.Context, build ImageBuild, now time.Time) error {
	body, err := os.ReadFile(build.Dockerfile)
	if errors.Is(err, fs.ErrNotExist) {
		// docker build reads the same path and refuses a missing Dockerfile
		// in its own words, so no image is built from an unjudged file.
		return nil
	}
	if err != nil {
		return errors.New(build.Dockerfile + ": the Dockerfile cannot be read, so its source pin cannot be judged: " + err.Error() +
			".\nFix: check that the path exists from the directory the build runs in")
	}
	pin, declared, err := ParseSourcePin(build.Dockerfile, string(body))
	if err != nil {
		return err
	}
	if !declared {
		return nil
	}
	if pin.Checkout == "" {
		if Stale(pin.Date, now) {
			return pin.staleRefusal(now)
		}
		return nil
	}
	for _, named := range build.NamedContexts {
		if named.Name == pin.Checkout {
			return d.freshenCheckout(ctx, pin, named.Path, now)
		}
	}
	return pinError(build.Dockerfile, "follows the build context "+pin.Checkout+", which this build does not pass")
}

// freshenCheckout fast-forwards the checkout at path to its upstream when its
// HEAD commit is more than a month old. It never touches a checkout holding
// uncommitted changes, and refuses the build instead, naming the fix.
func (d *Docker) freshenCheckout(ctx context.Context, pin SourcePin, path string, now time.Time) error {
	notCheckout := "check that " + path + " is a git checkout of " + pin.Repository
	head, err := d.git(ctx, path, "log", "-1", "--format=%cI", "HEAD")
	if err != nil {
		return checkoutRefusal(pin, path, "its HEAD commit date cannot be read: "+err.Error(), notCheckout)
	}
	committed, err := time.Parse(time.RFC3339, strings.TrimSpace(head))
	if err != nil {
		return checkoutRefusal(pin, path, "git answered the HEAD date "+strings.TrimSpace(head), notCheckout)
	}
	if !Stale(committed, now) {
		return nil
	}

	var tb textbuf.Buffer
	tb.Str("its HEAD was committed ").Str(committed.UTC().Format(time.DateOnly)).Str(", ").
		Int(AgeDays(committed, now)).Str(" days ago, more than one month")
	age := tb.String()
	pull := "git -C " + path + " pull --ff-only"
	status, err := d.git(ctx, path, "status", "--porcelain")
	if err != nil {
		return checkoutRefusal(pin, path, age+", and git status failed: "+err.Error(), "repair the checkout, then run: "+pull)
	}
	if strings.TrimSpace(status) != "" {
		return checkoutRefusal(pin, path, age+", and it holds uncommitted changes, so it is not updated for you",
			"commit or set aside the changes in "+path+", then run: "+pull)
	}
	if _, err := d.git(ctx, path, "fetch", "--quiet"); err != nil {
		return checkoutRefusal(pin, path, age+", and git fetch failed: "+err.Error(), "check the network and the remote, then run: "+pull)
	}
	if _, err := d.git(ctx, path, "merge", "--ff-only", "@{upstream}"); err != nil {
		return checkoutRefusal(pin, path, age+", and it cannot fast-forward to its upstream: "+err.Error(),
			"give the branch an upstream it has not diverged from, then run: "+pull)
	}
	return nil
}

// git runs one git command in the checkout at path and answers its stdout.
// It goes to the process runner directly: git is not a Docker command, so the
// Docker endpoint selection that run applies does not bind it.
func (d *Docker) git(ctx context.Context, path string, arguments ...string) (string, error) {
	argv := append([]string{"git", "-C", path}, arguments...)
	// The timeout SIGKILLs git, and a status killed while it refreshes the index
	// leaves an orphaned .git/index.lock that blocks every session's git writes.
	result, err := d.runner.Run(ctx, ProcessCommand{Arguments: argv, Timeout: sourceGitTimeout, Environment: gitenv.ReadOnly(os.Environ())})
	if err != nil {
		return result.Stdout, errors.New(strings.Join(argv, " ") + ": " + err.Error() + " " + strings.TrimSpace(result.Stderr))
	}
	if result.ExitCode != 0 {
		return result.Stdout, errors.New(strings.Join(argv, " ") + " failed: " + strings.TrimSpace(result.Stderr))
	}
	return result.Stdout, nil
}

// checkoutRefusal names the checkout, why it cannot be built, and the fix.
func checkoutRefusal(pin SourcePin, path, why, fix string) error {
	var tb textbuf.Buffer
	tb.Str(pin.Dockerfile).Str(" builds ").Str(pin.Repository).Str(" from the local checkout ").Str(path).
		Str(", and ").Str(why).Str(".\nFix: ").Str(fix)
	return errors.New(tb.String())
}
