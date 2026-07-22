// Design: plan/learned/1105-vpp-host-tuning.md -- host-side hugepage reservation via the
// appliance kernel cmdline. This file is the single kernel-argument assembly
// seam for the appliance image build. spec-vpp-host-tuning emits the hugepage
// arguments here; spec-vpp-isolated-cpus adds its isolcpus argument to the same
// seam (both specs "touch the cmdline assembly once"). Keep new arguments as
// pure functions of the appliance config so they stay unit-testable without a
// gok build.

package appliance

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	// buildDirName is the gokrazy per-instance directory holding the pinned build
	// modules. gok resolves it relative to the instance dir it chdirs into
	// (vendor/github.com/gokrazy/tools/packer/gotool.go BuildDir), so a derived
	// instance must carry its own copy or every pin is lost.
	buildDirName = "builddir"

	// gokrazyInstance is the gokrazy instance name under the parent dir; it names
	// both gokrazy/<instance>/ and the `gok -i` argument.
	gokrazyInstance = "ze"
)

// hugepageKernelArgs returns the kernel cmdline tokens that reserve hugepages at
// boot for the given image config, or nil when no reservation is configured.
// Order is deterministic: default_hugepagesz, then hugepagesz, then hugepages
// (the kernel needs the size directives before the count). The page count is
// derived from Size / PageSize.
func hugepageKernelArgs(img ImageConfig) ([]string, error) {
	if img.Hugepages == nil {
		return nil, nil
	}
	pageBytes, err := img.Hugepages.pageSizeBytes()
	if err != nil {
		return nil, err
	}
	token, _ := hugepageToken(pageBytes) // guaranteed supported by pageSizeBytes
	count, err := img.Hugepages.pageCount()
	if err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	return []string{
		tb.Reset().Str("default_hugepagesz=").Str(token).String(),
		tb.Reset().Str("hugepagesz=").Str(token).String(),
		tb.Reset().Str("hugepages=").Int(count).String(),
	}, nil
}

// deriveInstanceConfigJSON reads a gokrazy instance config.json and returns a
// copy with extraArgs appended to KernelExtraArgs, preserving every other field
// (including fields this build does not understand -- R-4). The checked-in
// config.json is never mutated; the caller writes the result to a derived copy.
func deriveInstanceConfigJSON(src []byte, extraArgs []string) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return nil, fmt.Errorf("parse instance config: %w", err)
	}
	var base []string
	if raw, ok := obj["KernelExtraArgs"]; ok {
		if err := json.Unmarshal(raw, &base); err != nil {
			return nil, fmt.Errorf("parse KernelExtraArgs: %w", err)
		}
	}
	args := make([]string, 0, len(base)+len(extraArgs))
	args = append(args, base...)
	args = append(args, extraArgs...)
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("encode KernelExtraArgs: %w", err)
	}
	obj["KernelExtraArgs"] = encodedArgs
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("encode instance config: %w", err)
	}
	return out, nil
}

// resolveBuildParentDir returns the gokrazy parent dir to build from and a
// cleanup func (always non-nil, safe to defer). When the image config requests a
// hugepage reservation, the returned dir is a temporary derived parent whose
// ze/config.json carries the extra kernel arguments and cleanup removes it;
// otherwise it is the checked-in gokrazy dir with a no-op cleanup (AC-8).
func resolveBuildParentDir(cfg *applianceConfig) (string, func(), error) {
	noop := func() {}
	parentDir, err := filepath.Abs("gokrazy")
	if err != nil {
		return "", noop, err
	}
	extraArgs, err := hugepageKernelArgs(cfg.Image)
	if err != nil {
		return "", noop, fmt.Errorf("prepare hugepage kernel args: %w", err)
	}
	if len(extraArgs) == 0 {
		return parentDir, noop, nil
	}
	derived, cleanup, err := materializeDerivedParent(parentDir, extraArgs)
	if err != nil {
		return "", noop, fmt.Errorf("prepare hugepage kernel args: %w", err)
	}
	return derived, cleanup, nil
}

// materializeDerivedParent builds a derived gokrazy parent dir under the project
// tmp/ whose <instance>/config.json is patched with extraArgs. Sibling entries of
// the source instance dir are symlinked in; builddir is COPIED, with its
// filesystem-path replace directives rewritten to absolute paths so they still
// resolve from the new depth. Returns the derived parent dir and a cleanup func;
// the checked-in source dir is never modified.
//
// The builddir must travel with the instance. gok looks for it relative to the
// instance dir it chdirs into, and when it is absent gok synthesizes an empty
// module and resolves every package over the network with `go get`
// (vendor/github.com/gokrazy/tools/packer/gotool.go getPkg/getIncomplete),
// discarding the pins in gokrazy/ze/builddir/*/go.mod. That is not theoretical:
// derived builds on 2026-07-18 and 2026-07-20 pulled github.com/rtr7/kernel at
// two versions NEWER than the pinned one into gokrazy/modcache, and re-fetched
// ze itself from the proxy at a fresh pseudo-version each build.
func materializeDerivedParent(srcParent string, extraArgs []string) (string, func(), error) {
	srcInstance := filepath.Join(srcParent, gokrazyInstance)
	srcData, err := os.ReadFile(filepath.Join(srcInstance, "config.json")) //nolint:gosec // trusted checked-in gokrazy instance dir
	if err != nil {
		return "", nil, fmt.Errorf("read instance config: %w", err)
	}
	patched, err := deriveInstanceConfigJSON(srcData, extraArgs)
	if err != nil {
		return "", nil, err
	}

	// Project tmp/, never the system temp dir (ai/rules/testing.md). srcParent is
	// <repo>/gokrazy, so its parent is the repo root whatever the caller's cwd is.
	tmpRoot := filepath.Join(filepath.Dir(srcParent), "tmp")
	if err := os.MkdirAll(tmpRoot, 0o750); err != nil {
		return "", nil, fmt.Errorf("create project tmp dir %s: %w", tmpRoot, err)
	}
	tmpParent, err := os.MkdirTemp(tmpRoot, "appliance-build-")
	if err != nil {
		return "", nil, fmt.Errorf("create derived parent dir under %s: %w", tmpRoot, err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpParent) }

	tmpInstance := filepath.Join(tmpParent, gokrazyInstance)
	if err := os.MkdirAll(tmpInstance, 0o750); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create derived instance dir: %w", err)
	}

	entries, err := os.ReadDir(srcInstance)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read instance dir: %w", err)
	}
	for _, e := range entries {
		if e.Name() == "config.json" {
			continue
		}
		absTarget, err := filepath.Abs(filepath.Join(srcInstance, e.Name()))
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("resolve %s: %w", e.Name(), err)
		}
		if e.Name() == buildDirName {
			if err := copyBuildDir(absTarget, filepath.Join(tmpInstance, e.Name())); err != nil {
				cleanup()
				return "", nil, err
			}
			continue
		}
		if err := os.Symlink(absTarget, filepath.Join(tmpInstance, e.Name())); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("symlink %s: %w", e.Name(), err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpInstance, "config.json"), patched, 0o644); err != nil { //nolint:gosec // build config, not secrets
		cleanup()
		return "", nil, fmt.Errorf("write patched instance config: %w", err)
	}
	return tmpParent, cleanup, nil
}

// copyBuildDir copies a gokrazy builddir tree, rewriting each go.mod's
// filesystem-path replace directives to absolute paths (the checked-in ze module
// replaces itself with a relative path six levels up, which no longer points at
// the repo root once the instance moves). Copying rather than symlinking keeps
// the derived build from sharing, and racing on, the checked-in builddir.
//
// Fails closed: a builddir that yields no go.mod would leave gok resolving every
// package over the network against unpinned upstream versions, so that is an
// error naming the directory rather than a quiet fallback.
func copyBuildDir(src, dst string) error {
	modules := 0
	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("copy builddir: %s: unsupported file type %s, expected a regular file", path, d.Type())
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // trusted checked-in builddir
		if readErr != nil {
			return fmt.Errorf("copy builddir: read %s: %w", path, readErr)
		}
		if d.Name() == "go.mod" {
			data, err = absolutizeReplaces(path, data)
			if err != nil {
				return err
			}
			modules++
		}
		if writeErr := os.WriteFile(target, data, 0o600); writeErr != nil {
			return fmt.Errorf("copy builddir: write %s: %w", target, writeErr)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if modules == 0 {
		return fmt.Errorf("copy builddir %s: no go.mod found, so gok would discard every pin and resolve packages with `go get`", src)
	}
	return nil
}

// absolutizeReplaces rewrites every filesystem-path replace in a go.mod to an
// absolute path resolved against the ORIGINAL go.mod's directory, so the module
// keeps resolving to the same target after it is copied to a different depth.
// Version replaces and module-path replaces are left untouched.
func absolutizeReplaces(goModPath string, data []byte) ([]byte, error) {
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", goModPath, err)
	}
	type rewrite struct{ oldPath, oldVers, newPath string }
	var todo []rewrite
	dir := filepath.Dir(goModPath)
	for _, r := range f.Replace {
		if !modfile.IsDirectoryPath(r.New.Path) || filepath.IsAbs(r.New.Path) {
			continue
		}
		abs, absErr := filepath.Abs(filepath.Join(dir, r.New.Path))
		if absErr != nil {
			return nil, fmt.Errorf("resolve replace target %s in %s: %w", r.New.Path, goModPath, absErr)
		}
		todo = append(todo, rewrite{oldPath: r.Old.Path, oldVers: r.Old.Version, newPath: abs})
	}
	if len(todo) == 0 {
		return data, nil
	}
	for _, w := range todo {
		if err := f.AddReplace(w.oldPath, w.oldVers, w.newPath, ""); err != nil {
			return nil, fmt.Errorf("rewrite replace %s in %s: %w", w.oldPath, goModPath, err)
		}
	}
	f.Cleanup()
	out, err := f.Format()
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", goModPath, err)
	}
	return out, nil
}
