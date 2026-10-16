// Design: docs/architecture/diagnostics/crash-capture.md -- crash capture readiness
// Overview: crashes.go -- the offline `show crashes` fallback that reports it
// Overview: doctor.go -- the three registered checks that read the same answer
// Related: internal/core/crashlog/kernel.go -- where configured and armed are computed
//
// Readiness is read through one call on every surface. The daemon RPC, the
// offline fallback that runs when the daemon has died, and `ze doctor` all reach
// the same intent and the same machine probes, so an operator gets one answer
// wherever they ask.

package crashes

import (
	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Readiness answers configured versus armed for kernel crash capture, reading
// the operator's intent from the config at configPath. An empty path reads the
// instance's own config.
//
// A config that cannot be read is reported as an unreadable config rather than
// as crash capture being off: those are different facts, and only one of them is
// something the operator chose.
func Readiness(configPath string) crashlog.Readiness {
	intent, err := system.LoadCrashDumpIntent(configPath)
	return readinessOf(intent, err)
}

// readinessOf answers readiness for an intent, or names why the intent could
// not be read, which is a different fact from crash capture being off.
func readinessOf(intent crashlog.Intent, err error) crashlog.Readiness {
	if err != nil {
		readiness := crashlog.CrashReadiness(crashlog.Intent{})
		readiness.Reason = err.Error()
		return readiness
	}
	return crashlog.CrashReadiness(intent)
}

// harvestAtDaemonStart is the crashes boot effect (register.go): it harvests,
// and warns when no crash directory is writable, because a crash report would
// then be lost.
//
// It runs at daemon start and only there. The record is written by the kernel
// during the fault and read here on the next healthy boot, so there is nothing
// to re-run on a config reload: the reservation itself is a boot argument, and
// a commit cannot change what the running kernel already reserved.
func harvestAtDaemonStart(configName string, configData []byte) {
	readiness := HarvestAtBoot(configName, configData)
	if !readiness.DirectoryWritable {
		slogutil.Logger(componentName).Warn("no crash directory is writable; a crash report would be lost")
	}
}

// HarvestAtBoot moves any kernel crash record the previous boot left into the
// crash directory, then reports readiness and publishes both gauges.
//
// It runs once, from the daemon start path through harvestAtDaemonStart, and
// it is the only caller that
// writes: every other surface reads. The return is the readiness the caller
// logs, so a box that is configured and not armed says so in its startup log as
// well as in `show crashes`. configData is the config text the daemon started
// on and configName the source it read that text from, named in a parse error.
func HarvestAtBoot(configName string, configData []byte) crashlog.Readiness {
	log := slogutil.Logger("crashes")

	result := crashlog.HarvestKernelCrashes()
	for _, name := range result.Written {
		log.Warn("kernel crash record recovered from the previous boot", "artifact", name)
	}
	if result.Retained > 0 {
		log.Warn("kernel crash records kept for the next boot", "records", result.Retained, "reason", result.Reason)
	}

	// The intent comes from the text the daemon started on, never from a
	// second read of configName: a "-" source is a stdin already consumed.
	readiness := readinessOf(system.ParseCrashDumpIntent(configName, configData))
	publishReadiness(readiness)
	if readiness.Configured && !readiness.Armed {
		log.Warn("kernel crash capture is configured but not armed", "reason", readiness.Reason)
	}
	return readiness
}

// crashMetrics are the two gauges a fleet alerts on. Artifacts present says
// something faulted; armed says whether the next fault would be recorded at all,
// which is the state that fails silently.
type crashMetrics struct {
	artifacts metrics.Gauge
	armed     metrics.Gauge
}

var boundMetrics *crashMetrics

// bindMetrics is called through registry.InjectPluginMetrics, which defers it
// until a registry exists. It runs once, before any surface reads the gauges.
func bindMetrics(reg metrics.Registry) {
	boundMetrics = &crashMetrics{
		artifacts: reg.Gauge("ze_crashes_artifacts", "Crash reports currently stored, of both kinds."),
		armed:     reg.Gauge("ze_crashes_kernel_capture_armed", "1 when the running kernel carries the crash reservation, 0 otherwise."),
	}
}

// publishReadiness writes the current state to the gauges. It is a no-op until
// the metrics registry exists, which is the normal state of a build with
// telemetry compiled out.
func publishReadiness(readiness crashlog.Readiness) {
	if boundMetrics == nil {
		return
	}
	boundMetrics.artifacts.Set(float64(len(crashlog.ListCrashes())))
	if readiness.Armed {
		boundMetrics.armed.Set(1)
		return
	}
	boundMetrics.armed.Set(0)
}
