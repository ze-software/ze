// Design: docs/architecture/vpp-host-tuning.md -- host-side hugepage reservation via the
// appliance kernel cmdline. This file is the single kernel-argument assembly
// seam for the appliance image build. spec-vpp-host-tuning emits the hugepage
// arguments here; the CPU isolation this file also emits is the second consumer
// of the same seam (both specs "touch the cmdline assembly once"). Keep new
// arguments as pure functions of the appliance config so they stay
// unit-testable without a gok build.
//
// Instance preparation (copying the gokrazy instance under project tmp/ so the
// build never runs from a tracked path) lives in the instance subpackage, which
// cmd/ze-gok also imports.

package appliance

import (
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/appliance/instance"
	"github.com/ze-software/ze/internal/core/cpulist"
	"github.com/ze-software/ze/internal/core/crashlog"
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

// isolatedCPUKernelArgs returns the kernel cmdline tokens that keep the Linux
// scheduler off the configured CPUs, or nil when no isolation is configured.
//
// Three arguments, not one. isolcpus removes the CPUs from the scheduler's
// domains, nohz_full stops the periodic timer tick on them, and rcu_nocbs moves
// their RCU callbacks to a housekeeping CPU. A VPP worker busy-polls, so a tick
// or a callback on its CPU is a stall in the packet path, and isolcpus alone
// leaves both in place.
//
// The list is validated by applianceConfig.validateIsolatedCPUs, which parses
// it with the same grammar, so it is re-parsed here only to render it back in
// canonical form.
func isolatedCPUKernelArgs(img ImageConfig) ([]string, error) {
	if img.IsolatedCPUs == "" {
		return nil, nil
	}
	ids, err := cpulist.Parse(img.IsolatedCPUs)
	if err != nil {
		return nil, fmt.Errorf("image.isolated-cpus %q: %w", img.IsolatedCPUs, err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("image.isolated-cpus %q: %w", img.IsolatedCPUs, cpulist.ErrEmpty)
	}
	list := cpulist.Format(ids)
	var tb textbuf.Buffer
	return []string{
		tb.Reset().Str("isolcpus=").Str(list).String(),
		tb.Reset().Str("nohz_full=").Str(list).String(),
		tb.Reset().Str("rcu_nocbs=").Str(list).String(),
	}, nil
}

// crashDumpKernelArgs returns the kernel cmdline tokens that reserve a region
// for kernel crash capture at boot, or nil when no reservation is configured.
//
// Two arguments, and both are required. reserve_mem carves a NAMED region of
// the requested size out of RAM, and ramoops.mem_name binds the pstore ramoops
// backend to the region with that name. Either one alone captures nothing: a
// region nothing binds to is RAM taken from the operator for no record, and a
// binding with no region has nowhere to write.
//
// The region is named rather than addressed. Size-named reservation has been in
// the kernel since 6.12, and internal/appliance/kernel.version pins a kernel
// above that floor, so the physical address that used to make ramoops awkward on
// x86 is not needed and the tokens read like the hugepage tokens beside them.
func crashDumpKernelArgs(img ImageConfig) ([]string, error) {
	if img.CrashDump == nil {
		return nil, nil
	}
	megabytes, err := img.CrashDump.reserveMegabytes()
	if err != nil {
		return nil, err
	}
	var tb textbuf.Buffer
	return []string{
		tb.Reset().Str("reserve_mem=").Uint16(megabytes).Str("M:4096:").Str(crashlog.ReserveRegionName).String(),
		tb.Reset().Str("ramoops.mem_name=").Str(crashlog.ReserveRegionName).String(),
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
	isolationArgs, err := isolatedCPUKernelArgs(cfg.Image)
	if err != nil {
		return "", noop, fmt.Errorf("prepare CPU isolation kernel args: %w", err)
	}
	extraArgs = append(extraArgs, isolationArgs...)
	crashArgs, err := crashDumpKernelArgs(cfg.Image)
	if err != nil {
		return "", noop, fmt.Errorf("prepare crash reservation kernel args: %w", err)
	}
	extraArgs = append(extraArgs, crashArgs...)
	prepared, cleanup, err := instance.Prepare(parentDir, instance.Options{ExtraKernelArgs: extraArgs})
	if err != nil {
		return "", noop, fmt.Errorf("prepare gokrazy instance: %w", err)
	}
	return prepared, cleanup, nil
}
