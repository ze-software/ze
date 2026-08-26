// Design: docs/architecture/testing/ci-format.md -- the release-candidate gate
// Detail: report.go -- the payload this run answers
// Related: actions.go -- the area table that reaches this run
//
// Package evidence runs the verify gate the way a release candidate must be
// judged: over a CLONE of the committed tree, inside a container, on a machine
// that carries none of the developer's own state. Nothing in the working tree
// can make it pass, which is the whole of what it is for.
//
// Two facts are established before anything starts, and each is a refusal
// rather than a warning. The external commands must exist, because a missing
// docker otherwise arrives as an unreadable failure from somewhere inside the
// run. The worktree must be clean, because a dirty tree means the thing being
// judged is not the thing that would be released.
//
// The work inside the container is a bash program rather than a sequence of
// steps this package drives. The container is a golang image that carries no
// le, so driving it from here means cross-compiling a second le for whatever
// the platform flag names, built out of the tree that has not been judged yet.
// The program is DATA here (ContainerScript), which is what lets a test read
// it.
package evidence

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ImageKey and PlatformKey are the dot-notation spellings of
// ZE_CLEAN_VERIFY_IMAGE and ZE_CLEAN_VERIFY_PLATFORM. env.Get matches
// case-insensitively and treats a dot and an underscore as the same character,
// so these keys read the variables the shell script read.
const (
	ImageKey    = "ze.clean.verify.image"
	PlatformKey = "ze.clean.verify.platform"
)

// The image and platform the run uses when the operator names neither.
//
// The image is pinned to a Go release rather than to latest: a release
// candidate judged on a toolchain nobody chose is evidence about that
// toolchain. The platform is named rather than left to the daemon, because an
// arm64 host would otherwise judge amd64 code on an emulator or not at all.
const (
	DefaultImage    = "golang:1.26"
	DefaultPlatform = "linux/amd64"
)

var imageEntry = env.MustRegister(env.EnvEntry{
	Key:         ImageKey,
	Type:        "string",
	Default:     DefaultImage,
	Description: "the container image the release-candidate verify run uses",
	// Private keeps the key out of `ze env list`. It is le's variable, and an
	// operator reading an appliance has no container to point it at.
	Private: true,
})

var platformEntry = env.MustRegister(env.EnvEntry{
	Key:         PlatformKey,
	Type:        "string",
	Default:     DefaultPlatform,
	Description: "the container platform the release-candidate verify run uses",
	Private:     true,
})

// ContainerScript is the program the container runs, exactly as
// scripts/evidence/effective-verify.sh handed it to bash.
//
// It installs what the gate needs, clones the read-only mount into a writable
// path, checks that the CLONE is clean too, and runs the gate there. The clone
// is what makes the evidence about the commit rather than about the mount: a
// build in place would write into the developer's tree, and a gate reading its
// own artifacts is not a gate.
//
// The firewall and web suites are skipped because neither can run in this
// container: the firewall suite needs the host's own nftables, and the web
// suite needs a browser.
const ContainerScript = `
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

rm -f /etc/apt/apt.conf.d/docker-clean
apt-get update
apt-get install -y --no-install-recommends build-essential curl git iputils-ping iproute2 iptables nftables python3 python3-venv util-linux
if ! command -v uv >/dev/null 2>&1; then
    curl -LsSf https://astral.sh/uv/install.sh | sh
fi
export PATH="/go/bin:/usr/local/go/bin:$HOME/.local/bin:$PATH"
CGO_ENABLED=0 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
CGO_ENABLED=0 go install honnef.co/go/tools/cmd/staticcheck@2026.1

git config --global --add safe.directory /host
git clone --no-local /host /work/src
cd /work/src
if [ -n "$(git status --porcelain)" ]; then
    echo "error: cloned source is not clean" >&2
    git status --short >&2
    exit 1
fi

ZE_SKIP_SUITES=firewall,web ZE_SUITE_TIMEOUT=1200s make ze-precommit-verify
`

// requiredCommands are the programs the run cannot proceed without. Both are
// checked before either is used, so an operator missing both learns it in one
// run rather than in two.
var requiredCommands = [...]string{"docker", "git"}

// RequiredCommands answers the programs the run needs on PATH. A test drives
// every one of them from this list rather than from a copy of it.
func RequiredCommands() []string { return requiredCommands[:] }

// gitTimeout bounds one git query. `git status` over a checkout is local work
// with no network and no lock this process waits behind, so a minute means git
// is stuck rather than slow.
const gitTimeout = time.Minute

// ErrDirtyTree says the worktree carries changes, so what would be judged is
// not what would be released.
var ErrDirtyTree = errors.New("clean release-candidate evidence requires a clean git worktree")

// Runner is one release-candidate run, and it holds every effect the run has
// outside its own process.
//
// Look, Ask and Start are parameters rather than calls because the run's whole
// behavior is which container it starts, and a test that reached the real
// docker would take an hour and need a network. NewRunner fills them with the
// production three.
//
// Safe for concurrent use: a Runner is built for one run and read by one
// goroutine.
type Runner struct {
	// Tree is the checkout to judge. It is mounted read-only into the
	// container and cloned there.
	Tree string
	// Image and Platform name the container the gate runs in.
	Image    string
	Platform string

	// Look reports an external command that is not on PATH.
	Look func(name string) error
	// Ask runs one git query about the tree and answers its standard output.
	Ask func(args ...string) (string, error)
	// Start runs docker to completion, with the container's output going to
	// the operator's terminal, and answers its exit status.
	Start func(args ...string) int
}

// NewRunner answers the runner the command uses: the real PATH, the real git
// and the real docker, over tree.
func NewRunner(tree string) *Runner {
	return &Runner{
		Tree:     tree,
		Image:    setting(imageEntry.Key, DefaultImage),
		Platform: setting(platformEntry.Key, DefaultPlatform),
		Look:     lookPath,
		Ask:      askGit(tree),
		Start:    startDocker,
	}
}

// setting answers what the operator named for key, or fallback when they named
// nothing. env.Get answers the empty string for an unset variable rather than
// the registered default, so the default is applied at the one place that reads
// it.
func setting(key, fallback string) string {
	if named := env.Get(key); named != "" {
		return named
	}
	return fallback
}

// Run judges the tree and answers what happened.
//
// A missing command and a dirty tree are errors: nothing was started, and the
// operator has something to fix before anything can be. A container that ran
// and failed is NOT an error. It is the verdict this command exists to deliver,
// and it travels in the report's own code.
func (r *Runner) Run() (Report, error) {
	report := Report{Image: r.Image, Platform: r.Platform, Tree: r.Tree}

	for _, name := range requiredCommands {
		if err := r.Look(name); err != nil {
			var tb textbuf.Buffer
			return report, errors.New(tb.Str("missing required command: ").Str(name).String())
		}
	}

	status, err := r.Ask("-C", r.Tree, "status", "--porcelain")
	if err != nil {
		return report, err
	}
	report.Dirty = statusLines(status)
	if len(report.Dirty) > 0 {
		return report, ErrDirtyTree
	}

	report.Code = r.Start(r.dockerArgs()...)
	report.Passed = report.Code == 0
	return report, nil
}

// dockerArgs answers the invocation, in the order the shell script wrote it.
// The order is preserved because it is what a reader compares against the
// script for as long as both exist.
func (r *Runner) dockerArgs() []string {
	var tb textbuf.Buffer
	mount := tb.Str(r.Tree).Str(":/host:ro").String()

	return []string{
		"run", "--rm",
		"--privileged",
		"--platform", r.Platform,
		"-v", mount,
		// The four named volumes are caches. They survive between runs, which
		// is what keeps a second release-candidate run minutes rather than the
		// better part of an hour, and none of them can carry the developer's
		// source: each is a toolchain or package cache the container fills.
		"-v", "ze-gomod-cache:/go/pkg/mod",
		"-v", "ze-gobuild-cache:/root/.cache/go-build",
		"-v", "ze-apt-cache:/var/cache/apt",
		"-v", "ze-uv-cache:/root/.local/bin",
		r.Image,
		"bash", "-lc", ContainerScript,
	}
}

// statusLines answers one entry per line git wrote, with the trailing newline
// dropped and nothing else changed. git's own two-column prefix is kept: it
// says whether a path is modified, staged or untracked, which is what an
// operator acts on.
func statusLines(status string) []string {
	trimmed := strings.TrimRight(status, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// lookPath reports a command that is not on PATH.
func lookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

// askGit answers the production Ask: it runs git in the tree and answers its
// standard output.
//
// The query is bounded by git itself rather than by a timeout. `git status`
// over a checkout is local work with no network and no lock this process waits
// behind, and a timeout would turn a slow filesystem into a report that the
// tree is clean.
func askGit(tree string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // a build tool queries the checkout it was pointed at
		cmd.Dir = tree
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		return string(out), err
	}
}

// startDocker answers the production Start: it runs the container with the
// operator's terminal attached, so the gate's output arrives while it runs
// rather than in one block at the end.
//
// The run is unbounded in time, deliberately. The gate it starts takes between
// 25 and 53 minutes on this hardware, the container adds a toolchain install to
// that, and a bound short enough to catch a hang would kill a normal run.
func startDocker(args ...string) int {
	// Background rather than a deadline: the gate this starts takes between 25
	// and 53 minutes on this hardware, the container adds a toolchain install
	// to that, and a bound short enough to catch a hang would kill a normal
	// run. The operator's own interrupt is the stop path.
	cmd := exec.CommandContext(context.Background(), "docker", args...) //nolint:gosec // the argv is built by dockerArgs, never by an operator
	cmd.Stdin = os.Stdin
	// The container's own output is progress, not the answer. It goes to
	// stderr so that `le evidence release-candidate-check | json` still
	// answers one JSON document on stdout.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode()
		}
		var tb textbuf.Buffer
		os.Stderr.WriteString(tb.Str("error: docker run: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI output
		return 1
	}
	return 0
}
