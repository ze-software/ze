// Design: plan/learned/1105-vpp-host-tuning.md -- host-side hugepage reservation via the
// appliance kernel cmdline. This file is the single kernel-argument assembly
// seam for the appliance image build. spec-vpp-host-tuning emits the hugepage
// arguments here; spec-vpp-isolated-cpus adds its isolcpus argument to the same
// seam (both specs "touch the cmdline assembly once"). Keep new arguments as
// pure functions of the appliance config so they stay unit-testable without a
// gok build.
//
// Instance preparation (copying the gokrazy instance under project tmp/ so the
// build never runs from a tracked path) lives in the instance subpackage, which
// cmd/ze-gok also imports.

package appliance

import (
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// resolveBuildParentDir returns the gokrazy parent dir to build from and a
// cleanup func (always non-nil, safe to defer). The returned dir is always a
// prepared copy under the project tmp/ carrying the checked-in builddir, with
// any hugepage kernel arguments patched into its ze/config.json; the checked-in
// gokrazy dir is never built from and never written to.
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
	prepared, cleanup, err := instance.Prepare(parentDir, instance.Options{ExtraKernelArgs: extraArgs})
	if err != nil {
		return "", noop, fmt.Errorf("prepare gokrazy instance: %w", err)
	}
	return prepared, cleanup, nil
}
