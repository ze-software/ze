// Design: docs/architecture/testing/interop.md -- the daemon every proof drives
// Related: deployment.go -- the container and the collector these builds feed
// Related: l2tp.go -- the L2TP proof that builds a daemon
// Related: vppiface.go -- the VPP proof that builds one
//
// daemonbuild.go cross-compiles the daemon that a proof points at a peer. Every
// deployment proof needs one, built the same way. The build is stated ONCE here.
// Two proofs with two copies can drift until one carries a feature that the
// other does not.
//
// The tags are DERIVED rather than written down. A literal list creates a second
// record of one fact, and a record that nothing compares can drift. ze_l2tp
// became a gate on 2026-07-24, and the evidence scripts continued to build a
// daemon with no L2TP for a month (letools/featuretags/daemontags.go).

package deployment

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/featuretags"
)

// buildTimeout bounds the cross-compile. It allows a cold-cache build on a slow
// machine to finish. It reports a build stuck on a network module rather than
// wait for it without a bound.
const buildTimeout = 20 * time.Minute

// daemonRel answers the path of the cross-compiled daemon relative to the tree.
// The path is relative because the container sees the tree at /src, while the
// build sees it at the checkout path. One path must be converted to the other.
//
// The name contains the architecture. A checkout used from two machines
// therefore does not hand one machine the other machine's binary.
func daemonRel(goarch string) string {
	var tb textbuf.Buffer
	return filepath.Join("tmp", "evidence", "bin", tb.Str("ze-linux-").Str(goarch).String())
}

// daemonBuildArgs answers the cross-compilation, with every feature gate the
// manifest declares.
func daemonBuildArgs(tree, goarch string) ([]string, error) {
	return daemonBuildArgsTo(tree, daemonRel(goarch))
}

// daemonBuildArgsTo answers the command arguments that build the daemon into
// outRel with every feature gate the manifest declares.
//
// The output path is a parameter because a proof that runs the daemon on THIS
// machine names it after itself. A proof that runs it in a container names it
// after the architecture it was cross-compiled for. The tags are the same in
// both cases. They are derived here, so neither proof can carry its own list.
func daemonBuildArgsTo(tree, outRel string) ([]string, error) {
	tags, err := featuretags.DaemonBuildTags(tree, featuretags.DaemonBase)
	if err != nil {
		return nil, err
	}
	return []string{"build", "-tags", tags, "-o", filepath.Join(tree, outRel), "./cmd/ze"}, nil
}

// errBuildFailed is the error a caller receives when the toolchain refuses the
// build. It is the sentence the Python originals printed, kept word for word.
// The toolchain has already written its diagnosis to the progress stream. This
// sentence identifies the step of the proof that stopped.
var errBuildFailed = errors.New("go build ./cmd/ze failed")

// buildDaemon cross-compiles the daemon into the tree's scratch directory.
//
// CGO is off, because the daemon runs in a container whose libc is not the
// build host's. GOCACHE is set inside the checkout only when the operator named
// none, which keeps a proof that cross-compiles the whole daemon from paying
// for it twice.
func buildDaemon(tree, goarch string, progress io.Writer) error {
	var tb textbuf.Buffer
	cross := []string{"GOOS=linux", tb.Str("GOARCH=").Str(goarch).String()}
	return runDaemonBuild(tree, daemonRel(goarch), cross, progress)
}

// buildHostDaemon compiles the daemon for THIS machine into outRel.
//
// The build sets no GOOS and no GOARCH. A proof that runs ze beside a peer daemon
// on the host uses the binary it just built. A cross-compiled binary would not
// execute. This is the one difference from the container proofs, so the two
// builds have separate names rather than a shared flag.
func buildHostDaemon(tree, outRel string, progress io.Writer) error {
	return runDaemonBuild(tree, outRel, nil, progress)
}

// runDaemonBuild compiles the daemon into outRel with extra in the environment.
//
// Both callers disable CGO. The container proof runs the daemon on a libc that
// is not the build host's. The host proof gains nothing from a dynamic link. The
// function sets GOCACHE inside the checkout only when the operator named none.
// This keeps a proof that compiles the whole daemon from paying for it twice.
func runDaemonBuild(tree, outRel string, extra []string, progress io.Writer) error {
	argv, err := daemonBuildArgsTo(tree, outRel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(tree, outRel)), 0o750); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	build := exec.CommandContext(ctx, "go", argv...) //nolint:gosec // the argv is derived from the manifest, never from an operator
	build.Dir = tree
	build.Stdout = progress
	build.Stderr = progress
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	build.Env = append(build.Env, extra...)
	if os.Getenv("GOCACHE") == "" {
		var tb textbuf.Buffer
		build.Env = append(build.Env, tb.Str("GOCACHE=").Str(filepath.Join(tree, "tmp", "go-cache")).String())
	}

	if err := build.Run(); err != nil {
		return errBuildFailed
	}
	return nil
}
