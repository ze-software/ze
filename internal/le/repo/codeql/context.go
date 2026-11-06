// Design: docs/architecture/testing/ci-workflows.md -- the traced build's actual Go context.
package codeql

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// BuildContext reads the context from the actual Go executable, including its
// tool and release tags. The caller MUST give the subsequent build the same
// environment and tags; the workflow's codeql_build function does so.
func BuildContext(ctx context.Context, root, tags string) (*build.Context, error) {
	return readBuildContext(ctx, root, tags, controlledEnvironment())
}

const contextTemplate = "{{context.GOOS}}\n{{context.GOARCH}}\n{{context.GOROOT}}\n{{context.GOPATH}}\n{{context.CgoEnabled}}\n{{context.UseAllFiles}}\n{{context.Compiler}}\n{{context.InstallSuffix}}\n{{join context.BuildTags \",\"}}\n{{join context.ToolTags \",\"}}\n{{join context.ReleaseTags \",\"}}"

func readBuildContext(ctx context.Context, root, tags string, environment []string) (*build.Context, error) {
	out, err := captureGo(ctx, root, environment, "list", "-tags", tags, "-f", contextTemplate, "unsafe")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(fields) != 11 {
		return nil, errors.New("go list context has an unsupported field boundary; refuse the CodeQL build")
	}
	cgo, err := strconv.ParseBool(fields[4])
	if err != nil {
		return nil, fmt.Errorf("decode Go context cgo: %w", err)
	}
	all, err := strconv.ParseBool(fields[5])
	if err != nil {
		return nil, fmt.Errorf("decode Go context all-files: %w", err)
	}
	return &build.Context{
		GOOS: fields[0], GOARCH: fields[1], GOROOT: fields[2], GOPATH: fields[3], Dir: root,
		CgoEnabled: cgo, UseAllFiles: all, Compiler: fields[6], InstallSuffix: fields[7],
		BuildTags: contextTags(fields[8]), ToolTags: contextTags(fields[9]), ReleaseTags: contextTags(fields[10]),
	}, nil
}

func contextTags(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func controlledEnvironment() []string {
	return append(os.Environ(), "GOENV=off", "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local")
}

func contextEnvironment(target *build.Context) []string {
	cgo := "0"
	if target.CgoEnabled {
		cgo = "1"
	}
	return append(controlledEnvironment(), "GOOS="+target.GOOS, "GOARCH="+target.GOARCH,
		"GOROOT="+target.GOROOT, "GOPATH="+target.GOPATH,
		"CGO_ENABLED="+cgo)
}

// validateBuildContext refuses contexts the actual Go command cannot reproduce.
// In particular, host tool/release tags cannot stand in for a target's tags.
func validateBuildContext(ctx context.Context, root string, target *build.Context) error {
	if target == nil {
		return errors.New("CodeQL needs a complete Go build context; call BuildContext first")
	}
	if target.UseAllFiles {
		return errors.New("CodeQL cannot trace a Go context that ignores build constraints")
	}
	if !physicalBuildContext(target) {
		return errors.New("CodeQL cannot trace a virtual Go build context")
	}
	actual, err := readBuildContext(ctx, root, strings.Join(target.BuildTags, ","), contextEnvironment(target))
	if err != nil {
		return err
	}
	if !sameBuildContext(actual, target) {
		return errors.New("CodeQL Go context differs from the actual Go command; obtain it with BuildContext")
	}
	return nil
}

func physicalBuildContext(target *build.Context) bool {
	return target.JoinPath == nil && target.SplitPathList == nil &&
		target.IsAbsPath == nil && target.IsDir == nil && target.HasSubdir == nil &&
		target.ReadDir == nil && target.OpenFile == nil
}

func sameBuildContext(actual, target *build.Context) bool {
	return actual.GOOS == target.GOOS && actual.GOARCH == target.GOARCH &&
		actual.GOROOT == target.GOROOT && actual.GOPATH == target.GOPATH &&
		actual.Dir == target.Dir && actual.CgoEnabled == target.CgoEnabled &&
		actual.Compiler == target.Compiler && actual.InstallSuffix == target.InstallSuffix &&
		slices.Equal(actual.BuildTags, target.BuildTags) &&
		slices.Equal(actual.ToolTags, target.ToolTags) &&
		slices.Equal(actual.ReleaseTags, target.ReleaseTags)
}

func captureGo(ctx context.Context, root string, environment []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // argv is constructed from the build context and tracked package directories.
	cmd.Dir = root
	cmd.Env = environment
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("go list: %w: %s", errors.Join(err, ctx.Err()), strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("go list: %w", errors.Join(err, ctx.Err()))
	}
	return out, nil
}
