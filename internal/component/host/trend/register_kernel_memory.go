// Design: docs/architecture/host/observability.md -- the kernel-memory trend rule
// Related: trend.go -- the rule registry this file registers into
// Related: register.go -- ruleCheck, the doctor check this rule registers

package trend

import (
	"errors"
	"io/fs"
	"time"

	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/stats"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The kernel-memory rule watches the kernel memory that cannot be reclaimed
// (SUnreclaim), the floor a kernel memory leak raises. Its limit is that level
// plus the memory still available, which is how far it can grow before the
// box runs out. The figures are the spec's built-in defaults, with no config
// leaf, unmeasured until a lab run measures them.
const (
	kernelMemoryRuleName  = "kernel-memory"
	kernelMemoryCheckName = "host-trend-kernel-memory"
	// CodeKernelMemoryGrowth is the code of the kernel-memory rule's warning.
	CodeKernelMemoryGrowth = "doctor-host-kernel-memory-growth"

	kernelMemorySamplePeriod = 60 * time.Second
	kernelMemoryBucketPeriod = time.Hour
	kernelMemoryBucketCount  = 168 // 7 days of hourly buckets
	kernelMemoryRises        = 6   // 6 consecutive rising hours compare 7 buckets
	kernelMemoryHorizon      = 7 * 24 * time.Hour
	kernelMemoryRiseShare    = 0.01 // the rise must reach 1% of total memory
	kernelMemoryConfirmAfter = 3
	kernelMemoryClearAfter   = 3
)

// kernelMemorySource names what the reader reads when its error names no path.
const kernelMemorySource = "/proc/meminfo"

func init() {
	registerRule(rule{
		name:         kernelMemoryRuleName,
		code:         CodeKernelMemoryGrowth,
		unit:         UnitBytes,
		samplePeriod: kernelMemorySamplePeriod,
		history: stats.LevelParams{
			BucketPeriod:  kernelMemoryBucketPeriod,
			BucketCount:   kernelMemoryBucketCount,
			GrowthRises:   kernelMemoryRises,
			GrowthHorizon: kernelMemoryHorizon,
		},
		confirmAfter: kernelMemoryConfirmAfter,
		clearAfter:   kernelMemoryClearAfter,
		read:         readKernelMemory,
		describe:     describeKernelMemory,
		fix:          trendFix,
	})
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        CodeKernelMemoryGrowth,
		Title:       "Kernel memory that cannot be reclaimed keeps growing",
		Description: "The kernel memory that cannot be reclaimed (SUnreclaim in /proc/meminfo) rose every hour for six hours, by at least 1% of the box's memory, and at that rate the box runs out of memory within seven days. That is the pattern of a kernel memory leak: a kernel update fixes it, and a reboot frees the memory until it grows again. The warning clears after the growth stops for three hours.",
		Examples:    []string{trendFix, "show host memory"},
		HostVerdict: true,
	}); err != nil {
		panic("BUG: host trend kernel-memory code registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeKernelMemoryGrowth}); err != nil {
		panic("BUG: host trend kernel-memory event registration: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: kernelMemoryCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: trendCheckOrder,
		Component: trendComponent, Dependencies: []string{trendDependency},
		Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes:     []string{CodeKernelMemoryGrowth},
		Check:     ruleCheck(kernelMemoryRuleName),
	}); err != nil {
		panic("BUG: host trend kernel-memory doctor registration: " + err.Error())
	}
}

// readKernelMemory reads SUnreclaim, MemAvailable and MemTotal through the
// host's meminfo parser, under the detector's root.
func readKernelMemory(d *host.Detector) reading {
	memory, err := d.DetectKernelMemory()
	if err != nil {
		return unreadableFromPathError(err, kernelMemorySource)
	}
	level := float64(memory.UnreclaimableBytes)
	limit := level + float64(memory.AvailableBytes)
	riseMin := float64(memory.TotalBytes) * kernelMemoryRiseShare
	sample, err := newLevelReading(level, limit, riseMin)
	if err != nil {
		return unreadableReading{source: kernelMemorySource, err: err}
	}
	return sample
}

// unreadableFromPathError turns a reader's error into an unreadable reading
// naming the path the error carries, or fallback when it carries none.
func unreadableFromPathError(err error, fallback string) reading {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return unreadableReading{source: pathErr.Path, err: pathErr.Err}
	}
	return unreadableReading{source: fallback, err: err}
}

// describeKernelMemory words the kernel-memory warning: the growth, its span,
// the projected time to exhaustion, and what the pattern means.
func describeKernelMemory(f finding) string {
	var tb textbuf.Buffer
	tb.Str("Kernel memory that cannot be reclaimed grew by ").Str(humanBytes(f.rise)).
		Str(" over the last ").Str(humanDuration(f.span)).Str(" and is still growing. ")
	if f.projected {
		tb.Str("At this rate the box runs out of memory in about ").Str(humanDuration(f.exhaustion)).Str(". ")
	} else {
		tb.Str("It has already reached the memory the box has left, so the box is about to run out of memory. ")
	}
	tb.Str("This is the pattern of a kernel memory leak: a kernel update fixes it, " +
		"and a reboot frees the memory until it grows again.")
	return tb.String()
}
