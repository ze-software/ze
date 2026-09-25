// Design: docs/architecture/testing/qemu-integration.md -- MOBIKE on the shipped runtime kernel
// Related: ../deployment/mobike.go -- guest action; ../interoplab/ipsec/mobike_netns_linux.go -- observations
package testqemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

func runIPsecMOBIKEHere(args leaction.Arguments) (answer any, code int) {
	if args.One(keywordKernel) == "" {
		leaction.ReportError(errors.New("MOBIKE proof requires kernel <vmlinuz-path>; the Alpine kernel is not a substitute"))
		return nil, 2
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	session, err := lepath.ResolveSession(root, true)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	artifacts, err := os.MkdirTemp(filepath.Join(root, session.Scratch), "ipsec-mobike-")
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	defer func() {
		if err := os.RemoveAll(artifacts); err != nil {
			leaction.ReportError(fmt.Errorf("remove MOBIKE build artifacts: %w", err))
			code = 1
		}
	}()
	relative, err := filepath.Rel(root, artifacts)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	guestArtifacts := filepath.Join(guestWorkspace, relative)
	command := shellQuote(filepath.Join(guestArtifacts, "le")) +
		" deployment ipsec-mobike-test daemon " + shellQuote(filepath.Join(guestArtifacts, "ze"))
	runArgs := leaction.Arguments{
		keywordCommand: {command},
		keywordKernel:  {args.One(keywordKernel)},
		"packages":     {"iproute2 iputils util-linux strongswan"},
	}
	if timeout := args.One("timeout"); timeout != "" {
		runArgs["timeout"] = []string{timeout}
	}
	options, err := parseRunArguments(runArgs)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	run := NewRun(root, options)
	if _, err := run.kernelPath(); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	ctx, cancel := guestContext()
	defer cancel()
	if err := buildMOBIKEGuests(ctx, root, artifacts); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := run.Execute(ctx)
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, runExitCode(&report)
}

// Only the artifacts executed inside Alpine get a target environment. The
// invoking le and kernel-builder host tool remain native host binaries.
func buildMOBIKEGuests(ctx context.Context, root, directory string) error {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return err
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return err
	}
	admission.Out = os.Stderr
	architecture := ArchAMD64
	if runtime.GOARCH == ArchARM64 {
		architecture = ArchARM64
	}
	environment := toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: architecture})
	for _, binary := range []struct{ name, base string }{
		{"ze", repofeaturetags.DaemonBase},
		{"le", repofeaturetags.LEBase},
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		tags, err := repofeaturetags.DaemonBuildTags(root, binary.base)
		if err != nil {
			return err
		}
		output := filepath.Join(directory, binary.name)
		argv := []string{"go", "build", tagsFlag, tags, "-ldflags", toolchain.LDFlags(), "-o", output, zeMainPackage}
		_, code := admission.Run("mobike-build-"+binary.name, argv, root, environment)
		if code != 0 {
			return fmt.Errorf("build MOBIKE guest %s for linux/%s exited %d", binary.name, architecture, code)
		}
		if err := runnableInGuest(output); err != nil {
			return fmt.Errorf("MOBIKE guest artifact %s: %w", binary.name, err)
		}
	}
	return nil
}
