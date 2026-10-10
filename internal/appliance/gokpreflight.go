// Design: docs/architecture/appliance/gokrazy-build-pins.md -- the host go an
// image build compiles with, and the offline module cache it reads.
// Related: cmd_build.go -- buildOne refuses through checkGokHost before any
// long work; SetGokGoEnv sets the environment these checks assume.

package appliance

import (
	"context"
	"errors"
	"fmt"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// goVersionTimeout bounds `go env GOVERSION`, which reads no module and
// answers at once.
const goVersionTimeout = 30 * time.Second

// goDownloadURL is where an operator installs a newer go.
const goDownloadURL = "https://go.dev/dl/"

// checkGokHost refuses a host that cannot compile the image, before the
// runtime kernel resolves: a cold kernel cache builds for about 30 minutes,
// and gok only runs go after it. root is the ze source tree root, where the
// build runs. It checks the offline module cache, then the go on PATH.
func checkGokHost(root string) error {
	modcache := filepath.Join(root, "gokrazy", "modcache")
	if _, err := os.Stat(modcache); err != nil {
		return fmt.Errorf("gokrazy module cache not found at %s: the image build resolves every module from it, offline, "+
			"so it runs only from the ze source tree root (%w)\n"+
			"change to the root of the ze checkout, then run the build again", modcache, err)
	}
	return CheckGoToolchain(filepath.Join(root, "go.mod"))
}

// CheckGoToolchain refuses a host whose go cannot build the module at goMod:
// no go on PATH, or a go older than the module's go directive. The image build
// runs go with GOTOOLCHAIN=local and GOPROXY=off (SetGokGoEnv), so go cannot
// fetch a newer toolchain itself, and without this check an old go is found
// only after the kernel build, by gok. Callers MUST run it before resolving
// the runtime kernel.
func CheckGoToolchain(goMod string) error {
	required, err := goDirective(goMod)
	if err != nil {
		return err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("no go on PATH: the image build compiles ze and gokrazy with the host's go\n"+
			"install Go %s or newer from %s, and put its bin directory on PATH", required, goDownloadURL)
	}
	have, err := hostGoVersion(goBin)
	if err != nil {
		return err
	}
	if version.Compare(have, "go"+required) < 0 {
		return fmt.Errorf("the go on PATH (%s) is %s, older than go %s that %s requires: "+
			"the image build runs offline with GOTOOLCHAIN=local, so go cannot fetch a newer toolchain\n"+
			"install Go %s or newer from %s, and put it first on PATH", goBin, have, required, goMod, required, goDownloadURL)
	}
	return nil
}

// goDirective answers the go version the module at path requires.
func goDirective(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the ze checkout's own go.mod
	if err != nil {
		return "", fmt.Errorf("read %s: the image build runs from the ze source tree root (%w)\n"+
			"change to the root of the ze checkout, then run the build again", path, err)
	}
	file, err := modfile.ParseLax(path, data, nil)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if file.Go == nil {
		return "", fmt.Errorf("%s has no go directive, so the go version the image build needs is unknown", path)
	}
	return file.Go.Version, nil
}

// hostGoVersion answers `go env GOVERSION` for goBin, as go1.N.P. It runs with
// GOTOOLCHAIN=local, as the image build does, and from goBin's own directory,
// outside every module, so an old go answers with its version rather than
// refusing ze's go.mod.
func hostGoVersion(goBin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), goVersionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBin, "env", "GOVERSION")
	cmd.Dir = filepath.Dir(goBin)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("%s env GOVERSION failed (%w: %s), so the go version the image build would use is unknown\n"+
			"check the Go installation with: %s version", goBin, err, stderr, goBin)
	}
	have := strings.TrimSpace(string(out))
	if !version.IsValid(have) {
		return "", fmt.Errorf("%s reports version %q, which is not a release version, so Ze cannot tell whether it is new enough\n"+
			"install a Go release from %s, and put it first on PATH", goBin, have, goDownloadURL)
	}
	return have, nil
}
