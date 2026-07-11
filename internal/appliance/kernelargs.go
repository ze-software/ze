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
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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
	derived, cleanup, err := materializeDerivedParent(parentDir, "ze", extraArgs)
	if err != nil {
		return "", noop, fmt.Errorf("prepare hugepage kernel args: %w", err)
	}
	return derived, cleanup, nil
}

// materializeDerivedParent builds a temporary gokrazy parent dir whose
// <instance>/config.json is patched with extraArgs, while every other entry of
// the source instance dir is symlinked in. builddir is deliberately excluded so
// gok rebuilds it cold in the temp dir rather than sharing (and racing on) the
// checked-in builddir. Returns the temp parent dir and a cleanup func; the
// checked-in source dir is never modified.
func materializeDerivedParent(srcParent, instance string, extraArgs []string) (string, func(), error) {
	srcInstance := filepath.Join(srcParent, instance)
	srcData, err := os.ReadFile(filepath.Join(srcInstance, "config.json")) //nolint:gosec // trusted checked-in gokrazy instance dir
	if err != nil {
		return "", nil, fmt.Errorf("read instance config: %w", err)
	}
	patched, err := deriveInstanceConfigJSON(srcData, extraArgs)
	if err != nil {
		return "", nil, err
	}

	tmpParent, err := os.MkdirTemp("", "ze-appliance-build-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp parent dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpParent) }

	tmpInstance := filepath.Join(tmpParent, instance)
	if err := os.MkdirAll(tmpInstance, 0o750); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create temp instance dir: %w", err)
	}

	entries, err := os.ReadDir(srcInstance)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read instance dir: %w", err)
	}
	for _, e := range entries {
		if e.Name() == "config.json" || e.Name() == "builddir" {
			continue
		}
		absTarget, err := filepath.Abs(filepath.Join(srcInstance, e.Name()))
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("resolve %s: %w", e.Name(), err)
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
