// Design: docs/architecture/testing/ci-format.md -- the release-candidate gate
// Detail: report.go -- the payload this run answers
// Related: actions.go -- the area table that reaches this run
//
// Package verifyevidence runs the verify gate the way a release candidate must be
// judged: over a CLONE of the committed tree, inside a container, on a machine
// that carries none of the developer's own state. Nothing in the working tree
// can make it pass, which is the whole of what it is for.
//
// Three facts are established before anything starts, and each is a refusal
// rather than a warning. The external commands must exist, because a missing
// docker otherwise arrives as an unreadable failure from somewhere inside the
// run. The worktree must be clean, because a dirty tree means the thing being
// judged is not the thing that would be released. The Docker daemon's kernel
// must carry every feature Ze enrolls, because the gate runs Ze's functional
// tests inside the container on that kernel.
//
// The work inside the container is a bash program rather than a sequence of
// steps this package drives. The container is a golang image that carries no
// le, so driving it from here means cross-compiling a second le for whatever
// the platform flag names, built out of the tree that has not been judged yet.
// The program is DATA here (ContainerScript), which is what lets a test read
// it.
package verifyevidence

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/le/tail"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	"github.com/ze-software/ze/internal/test/runner"
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
	DefaultImage    = "golang:1.27"
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
// internal/le/verify/evidence/evidence.go handed it to bash.
//
// It installs what the gate needs, clones the read-only mount into a writable
// path, checks that the CLONE is clean too, and runs the gate there.
// Golangci-lint and Staticcheck are built from the clone's vendor tree after
// that check, so each is the release go.mod requires and links the x/tools
// go.mod requires; an @version install names a second copy of the version,
// which drifted (golangci-lint v2.13.1 against go.mod's v2.14.0), and links its
// own older x/tools, which cannot read the toolchain's export data. The clone
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
apt-get install -y --no-install-recommends build-essential git iputils-ping iproute2 iptables nftables util-linux
export PATH="/go/bin:/usr/local/go/bin:$PATH"

git config --global --add safe.directory /host
git clone --no-local /host /work/src
cd /work/src
if [ -n "$(git status --porcelain)" ]; then
    echo "error: cloned source is not clean" >&2
    git status --short >&2
    exit 1
fi
CGO_ENABLED=0 go install -mod=vendor github.com/golangci/golangci-lint/v2/cmd/golangci-lint
CGO_ENABLED=0 go install -mod=vendor honnef.co/go/tools/cmd/staticcheck

./le verify current
`

// requiredCommands are the programs the run cannot proceed without. Both are
// checked before either is used, so an operator missing both learns it in one
// run rather than in two.
var requiredCommands = [...]string{"docker", "git"}

// requiredCommandNames answers the programs the run needs on PATH. A test drives
// every one of them from this list rather than from a copy of it.
func requiredCommandNames() []string { return requiredCommands[:] }

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
// Look, Ask, Docker and Kernel are parameters rather than calls because the
// run's whole behavior is which container it starts, and a test that reached
// the real docker would take an hour and need a network. NewRunner fills them
// with the production ones; a test hands Docker interoplab.NewFixtureDocker.
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
	// Docker runs the container (Docker.RunOneShot): labeled, named, removed
	// by name when the run's context ends or its bound passes.
	Docker *interoplab.Docker
	// Stdin and Progress are the container's input and where its output goes.
	// The output is progress, not the answer: production sends it to stderr,
	// so `le verify evidence release-candidate | json` still answers one JSON
	// document on stdout.
	Stdin    io.Reader
	Progress io.Writer
	// Kernel refuses a Docker daemon whose kernel lacks a feature Ze enrolls.
	Kernel func() error
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
		Docker:   interoplab.NewDocker(),
		Stdin:    os.Stdin,
		Progress: os.Stderr,
		Kernel:   dockerKernel(tree),
	}
}

// kernelZeRel is where the kernel check's linux ze is staged, relative to the
// tree. It sits under tmp/, which git ignores, so staging it leaves the tree
// as clean as the dirty-tree check found it.
const kernelZeRel = "tmp/verify-evidence/ze-linux"

// kernelCheckTimeout bounds the kernel check: one cross-compile of ze, whose
// own bound is five minutes, then the release query and the probe container.
const kernelCheckTimeout = 10 * time.Minute

// dockerKernel answers the production Kernel: it cross-compiles a linux ze for
// the daemon's architecture into the tree and runs the check every Docker run
// that runs Ze makes (interoplab.DockerKernel) with it.
func dockerKernel(tree string) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), kernelCheckTimeout)
		defer cancel()

		docker := interoplab.NewDocker()
		ze := interoplab.LabBinary{Name: "ze", Base: repofeaturetags.DaemonBase, Output: kernelZeRel}
		if err := interoplab.StageBinaries(tree, false, ze)(ctx, docker); err != nil {
			return err
		}
		return interoplab.DockerKernel(filepath.Join(tree, filepath.FromSlash(kernelZeRel)))(ctx, docker)
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
// A missing command, a dirty tree and a refused Docker kernel are errors:
// nothing was started, and the operator has something to fix before anything
// can be. A container that ran
// and failed is NOT an error. It is the verdict this command exists to deliver,
// and it travels in the report's own code.
//
// ctx is the run's root: when it ends, RunOneShot removes the container and
// Run answers the interruption as an error, never as a verdict.
func (r *Runner) Run(ctx context.Context) (Report, error) {
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

	// The gate inside the container runs Ze's functional tests on the Docker
	// daemon's kernel, so that kernel must carry every feature Ze enrolls
	// before the container starts (owner D-4: a wrong kernel "should not be
	// possible - fail").
	if err := r.Kernel(); err != nil {
		return report, err
	}

	container, err := r.container()
	if err != nil {
		return report, err
	}
	stderrTail := tail.New(stderrTailOctets)
	container.Stderr = io.MultiWriter(r.Progress, stderrTail)
	result, err := r.Docker.RunOneShot(ctx, container)
	// An ended root or the bound stopped the run: the container was removed
	// (or its removal failure is joined in), and no gate verdict exists.
	if errors.Is(err, context.Canceled) {
		return report, err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return report, err
	}
	if result.ExitCode == 0 {
		if err != nil {
			return report, err
		}
		report.Passed = true
		return report, nil
	}
	report.Code = result.ExitCode
	if refusal := deviceRefusal(result.ExitCode, stderrTail.String()); refusal != nil {
		return report, refusal
	}
	return report, nil
}

// deviceRefusal answers the operator's fix when Docker refused to start the
// container because the Docker host lacks a device node the container is
// given (Runner.container, runner.ContainerDevicePaths), and nil for any other
// failure. Docker answers that case only with "error gathering device
// information" and the path; the gate never ran, so an exit code alone would
// read as a failed gate and name nothing to do.
func deviceRefusal(code int, stderrTail string) error {
	for line := range strings.SplitSeq(stderrTail, "\n") {
		if !strings.Contains(line, "error gathering device information") {
			continue
		}
		for _, path := range runner.ContainerDevicePaths() {
			if !strings.Contains(line, path) {
				continue
			}
			module, known := runner.DeviceHostModule(path)
			if !known {
				continue
			}
			var tb textbuf.Buffer
			tb.Str("the release-candidate container did not start (docker exit ").Int(int64(code)).Str("): ").
				Str(strings.TrimSpace(line)).Byte('\n').
				Str("The Docker host has no ").Str(path).
				Str(", and the evidence container never loads kernel modules.\n").
				Str("Load it on the Docker host, then run the gate again:\n").
				Str("  sudo modprobe ").Str(module)
			return errors.New(tb.String())
		}
	}
	return nil
}

// evidenceContainerTimeout bounds the release-candidate container (owner D-1,
// 2026-10-19). The gate takes 25 to 53 minutes on this hardware and the
// container adds a toolchain install, so four hours never ends a normal run:
// it only stops a wedged daemon, which RunOneShot then removes by name.
const evidenceContainerTimeout = 4 * time.Hour

// container answers the one-shot the gate runs in. RunOneShot adds `run --rm`,
// the name and the owner labels; the grants and mounts keep the order the
// shell script wrote them in.
//
// The container never runs --privileged (spec-lab-containers-least-privilege,
// site 6). The gate inside it runs every test in the tree. A test that needs
// a capability declares it as `option=needs-linux:caps=`, so the container
// holds every capability that vocabulary names (runner.ContainerCapsDockerNames)
// and every device node a production path reached by a test opens
// (runner.ContainerDevicePaths), and Docker's default seccomp and AppArmor
// profiles stay on. A token added to either table reaches this list with no
// edit here. SYS_TIME is the one capability left out on purpose: a test
// that sets the clock from a container sets the Docker host's clock, so
// storage-ntp-restart.ci skips here through its own caps gate. Docker's
// default capability set already holds NET_BIND_SERVICE, so its --cap-add is
// redundant and harmless; it is kept because it comes from the table.
func (r *Runner) container() (interoplab.OneShotContainer, error) {
	capabilities, err := runner.ContainerCapsDockerNames()
	if err != nil {
		return interoplab.OneShotContainer{}, err
	}
	devices := runner.ContainerDevicePaths()

	var tb textbuf.Buffer
	mount := tb.Str(r.Tree).Str(":/host:ro").String()

	args := make([]string, 0, 2*len(capabilities)+2*len(devices)+10)
	for _, capability := range capabilities {
		args = append(args, "--cap-add", capability)
	}
	for _, device := range devices {
		args = append(args, "--device", device)
	}
	args = append(args,
		"--platform", r.Platform,
		"-v", mount,
		// The three named volumes are caches. They survive between runs, which
		// is what keeps a second release-candidate run minutes rather than the
		// better part of an hour, and none can carry the developer's source:
		// each is a toolchain or package cache the container fills. Removing
		// the container never touches them.
		"-v", "ze-gomod-cache:/go/pkg/mod",
		"-v", "ze-gobuild-cache:/root/.cache/go-build",
		"-v", "ze-apt-cache:/var/cache/apt",
	)
	return interoplab.OneShotContainer{
		Image:     r.Image,
		Arguments: args,
		Command:   []string{"bash", "-lc", ContainerScript},
		Timeout:   evidenceContainerTimeout,
		Stdin:     r.Stdin,
		Stdout:    r.Progress,
	}, nil
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

// stderrTailOctets bounds what Run keeps of docker's stderr. Docker's
// refusal to start a container is one line written just before it exits, so
// the last few kilobytes always hold it, and a 50-minute gate's output is
// never held whole.
const stderrTailOctets = 4096
