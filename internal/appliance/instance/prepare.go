// Design: plan/learned/1254-gokrazy-derived-parent-discards-pins.md -- preparing a
// relocated gokrazy instance that keeps the checked-in build pins. Extracted from
// internal/appliance/kernelargs.go so cmd/ze-gok can prepare an instance without
// importing the whole appliance package (spec-gokrazy-builddir-tmp, D-1c).

// Package instance prepares a relocated copy of the checked-in gokrazy instance
// so an image build never runs from, or writes to, a tracked path.
//
// gok chdirs into the instance directory before packing
// (vendor/github.com/gokrazy/tools/internal/gok/overwrite.go), so everything it
// resolves by relative path is instance-relative. A relocated instance must
// therefore carry its own builddir, and the filesystem-path replace directives
// inside that builddir must be rewritten for the new depth.
package instance

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

const (
	// buildDirName is the gokrazy per-instance directory holding the pinned build
	// modules. gok resolves it relative to the instance dir it chdirs into
	// (vendor/github.com/gokrazy/tools/packer/gotool.go BuildDir), so a prepared
	// instance must carry its own copy or every pin is lost.
	buildDirName = "builddir"

	// Name is the gokrazy instance name under the parent dir; it names both
	// gokrazy/<instance>/ and the `gok -i` argument.
	Name = "ze"

	// GoModName is the module manifest filename, exported so tests that walk a
	// builddir agree with the copier on what a module looks like.
	GoModName = "go.mod"

	// KernelModule is the module supplying the appliance kernel. An out-of-tree
	// kernel is selected by replacing it in the prepared copy.
	KernelModule = "github.com/rtr7/kernel"
)

// Options controls how an instance is prepared. The zero value prepares a
// faithful copy: the pinned kernel, and no extra kernel arguments.
type Options struct {
	// ExtraKernelArgs are appended to the instance's KernelExtraArgs.
	ExtraKernelArgs []string

	// KernelPackage, when set, is a filesystem path to an out-of-tree kernel
	// package that replaces KernelModule in the prepared copy only.
	//
	// Deliberately an explicit parameter rather than a probe for a well-known
	// directory: a probe would silently give every later build a custom kernel
	// for as long as that directory existed, which is the trap the previous
	// tracked-go.mod approach fell into (it needed `make ze-kernel-clean` to
	// undo, and nothing enforced that).
	KernelPackage string
}

// deriveConfigJSON reads a gokrazy instance config.json and returns a copy with
// extraArgs appended to KernelExtraArgs, preserving every other field (including
// fields this build does not understand). Passing no extraArgs is the common
// case and leaves the argument list unchanged. The checked-in config.json is
// never mutated; the caller writes the result to the prepared copy.
func deriveConfigJSON(src []byte, extraArgs []string) ([]byte, error) {
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

// Prepare builds a gokrazy parent dir under the project tmp/ whose
// <instance>/config.json is patched with extraArgs (pass nil for none). Sibling
// entries of the source instance dir are symlinked in; builddir is COPIED, with
// its filesystem-path replace directives rewritten to absolute paths so they
// still resolve from the new depth. Returns the prepared parent dir and a
// cleanup func; the checked-in source dir is never modified.
//
// The caller MUST call cleanup when the build is done. cleanup is nil only when
// Prepare returns an error.
//
// The builddir must travel with the instance. gok looks for it relative to the
// instance dir it chdirs into, and when it is absent gok synthesizes an empty
// module and resolves every package over the network with `go get`
// (vendor/github.com/gokrazy/tools/packer/gotool.go getPkg/getIncomplete),
// discarding the pins in gokrazy/ze/builddir/*/go.mod. That is not theoretical:
// derived builds on 2026-07-18 and 2026-07-20 pulled github.com/rtr7/kernel at
// two versions NEWER than the pinned one into gokrazy/modcache, and re-fetched
// ze itself from the proxy at a fresh pseudo-version each build.
func Prepare(srcParent string, opts Options) (string, func(), error) {
	srcInstance := filepath.Join(srcParent, Name)
	srcData, err := os.ReadFile(filepath.Join(srcInstance, "config.json")) //nolint:gosec // trusted checked-in gokrazy instance dir
	if err != nil {
		return "", nil, fmt.Errorf("read instance config: %w", err)
	}
	patched, err := deriveConfigJSON(srcData, opts.ExtraKernelArgs)
	if err != nil {
		return "", nil, err
	}

	// Resolve the kernel package before anything is created, so a typo fails
	// before a build starts rather than after it has silently used the pin.
	kernelPkg := ""
	if opts.KernelPackage != "" {
		kernelPkg, err = filepath.Abs(opts.KernelPackage)
		if err != nil {
			return "", nil, fmt.Errorf("resolve kernel package %s: %w", opts.KernelPackage, err)
		}
		if _, statErr := os.Stat(kernelPkg); statErr != nil {
			return "", nil, fmt.Errorf("kernel package %s: %w", kernelPkg, statErr)
		}
	}

	// Project tmp/, never the system temp dir (ai/rules/testing.md). srcParent is
	// <repo>/gokrazy, so its parent is the repo root whatever the caller's cwd is.
	tmpRoot := filepath.Join(filepath.Dir(srcParent), "tmp")
	if err := os.MkdirAll(tmpRoot, 0o750); err != nil {
		return "", nil, fmt.Errorf("create project tmp dir %s: %w", tmpRoot, err)
	}
	tmpParent, err := os.MkdirTemp(tmpRoot, "appliance-build-")
	if err != nil {
		return "", nil, fmt.Errorf("create prepared parent dir under %s: %w", tmpRoot, err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpParent) }

	tmpInstance := filepath.Join(tmpParent, Name)
	if err := os.MkdirAll(tmpInstance, 0o750); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create prepared instance dir: %w", err)
	}

	entries, err := os.ReadDir(srcInstance)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read instance dir: %w", err)
	}

	// Fail closed on a source instance with no builddir at all. copyBuildDir
	// guards an EMPTY builddir, but an absent one would simply never be copied,
	// and gok would then synthesize modules and resolve everything over the
	// network. That is the exact silent failure this package exists to prevent,
	// so it must not depend on the directory happening to be there.
	hasBuildDir := false
	for _, e := range entries {
		if e.Name() == buildDirName {
			hasBuildDir = true
			break
		}
	}
	if !hasBuildDir {
		cleanup()
		return "", nil, fmt.Errorf("instance %s has no %s, so gok would discard every pin and resolve packages with `go get`", srcInstance, buildDirName)
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

	if kernelPkg != "" {
		if err := replaceKernel(filepath.Join(tmpInstance, buildDirName), kernelPkg); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return tmpParent, cleanup, nil
}

// replaceKernel points KernelModule at an out-of-tree kernel package inside the
// PREPARED builddir. The tracked module is never touched, so a custom-kernel
// build leaves the working tree clean and cannot leak into a later build.
//
// Fails closed: if the module that pins the kernel is not found, the request is
// an error rather than a build that quietly uses the pinned kernel while the
// operator believes they are testing their own.
func replaceKernel(buildDir, pkg string) error {
	modPath := filepath.Join(buildDir, filepath.FromSlash(KernelModule), GoModName)
	data, err := os.ReadFile(modPath) //nolint:gosec // path derived from the prepared builddir
	if err != nil {
		return fmt.Errorf("locate the %s module to point at kernel package %s: %w", KernelModule, pkg, err)
	}
	f, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", modPath, err)
	}
	if err := f.AddReplace(KernelModule, "", pkg, ""); err != nil {
		return fmt.Errorf("replace %s with %s: %w", KernelModule, pkg, err)
	}
	f.Cleanup()
	out, err := f.Format()
	if err != nil {
		return fmt.Errorf("format %s: %w", modPath, err)
	}
	if err := os.WriteFile(modPath, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", modPath, err)
	}
	return nil
}

// copyBuildDir copies a gokrazy builddir tree, rewriting each go.mod's
// filesystem-path replace directives to absolute paths (the checked-in ze module
// replaces itself with a relative path six levels up, which no longer points at
// the repo root once the instance moves). Copying rather than symlinking keeps
// the prepared build from sharing, and racing on, the checked-in builddir.
//
// Fails closed: a builddir that yields no go.mod would leave gok resolving every
// package over the network against unpinned upstream versions, so that is an
// error naming the directory rather than a quiet fallback.
func copyBuildDir(src, dst string) error {
	// Resolve a symlinked builddir to its target. WalkDir does not follow
	// symlinks, so without this a caller that symlinks its builddir (rather than
	// duplicating it) would hit the "unsupported file type" branch below on the
	// root itself. Relative replaces are still resolved against each go.mod's own
	// path, so the rewritten targets are unaffected.
	if resolved, err := filepath.EvalSymlinks(src); err == nil {
		src = resolved
	}

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
		if d.Name() == GoModName {
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
