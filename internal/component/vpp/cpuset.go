// Design: docs/architecture/vpp-host-tuning.md -- the host CPU inventory VPP
// worker placement draws from, read behind an overridable sysfs root.
// Detail: startupconf.go -- resolveWorkerCores turns this inventory into the
// corelist-workers line.
// Related: config.go -- CPUSettings, whose validate method below refuses a CPU
// config the inventory contradicts.
//
// A VPP worker busy-polls. A worker that shares a CPU with the Linux scheduler
// therefore competes with every other runnable task on that CPU, which is the
// reason the kernel offers isolcpus and the reason Ze reads the isolated set
// here instead of counting cores up from main-core.

package vpp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/cpulist"
	"github.com/ze-software/ze/internal/core/env"
)

const (
	// cpuSysfsRoot is the kernel's CPU topology directory. Both files Ze reads
	// under it, online and isolated, are plain CPU lists such as "0-3,7".
	cpuSysfsRoot = "/sys/devices/system/cpu"
	// cpuRootEnv replaces cpuSysfsRoot when it is set, so a functional test
	// runs the CPU checks against fixture files instead of the machine it
	// executes on.
	cpuRootEnv = "ze.test.vpp.cpu.root"
	// envTypeString is the env.EnvEntry Type every override key in this package
	// declares. Named once so the three registrations cannot drift.
	envTypeString = "string"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         cpuRootEnv,
	Type:        envTypeString,
	Description: "Override the /sys/devices/system/cpu root the VPP CPU checks read (functional tests)",
	Private:     true,
})

// errNoOnlineCPUs reports an inventory that parsed but held no CPU, which no
// running kernel produces and a truncated fixture does. It is an error rather
// than an empty set, so a bad read never reads as a real answer.
var errNoOnlineCPUs = errors.New("host reports no online CPUs")

// CPUInventory holds the host CPU facts VPP worker placement needs. Online is
// the set of CPUs the kernel reports usable, and Isolated is the subset the
// kernel keeps its scheduler off.
//
// IsolationKnown is a separate field on purpose. "The kernel isolated no CPU"
// and "this host could not tell us which CPUs are isolated" are different
// answers, and one empty slice cannot carry both. A caller that reads the
// second as the first pins workers to CPUs Linux still schedules on and reports
// nothing (ai/rules/principles.md).
type CPUInventory struct {
	Online         []uint8
	Isolated       []uint8
	IsolationKnown bool
}

// hostCPUInventory reads the host's online and isolated CPU sets.
//
// An unreadable online file is an error: without it Ze cannot say whether a
// requested core exists, and answering "it does not" from a failed read would
// refuse a valid config for the wrong reason. An unreadable isolated file is
// not an error, because isolation governs placement quality rather than
// validity; it leaves IsolationKnown false and the vpp-cpu-isolation doctor
// check reports it.
func hostCPUInventory() (CPUInventory, error) {
	root := strings.TrimSpace(env.Get(cpuRootEnv))
	if root == "" {
		root = cpuSysfsRoot
	}

	online, err := readCoreListFile(filepath.Join(root, "online"))
	if err != nil {
		return CPUInventory{}, fmt.Errorf("read online CPUs from %s: %w", root, err)
	}

	if len(online) == 0 {
		return CPUInventory{}, fmt.Errorf("read online CPUs from %s: %w", root, errNoOnlineCPUs)
	}

	inv := CPUInventory{Online: online}
	isolated, isolatedErr := readCoreListFile(filepath.Join(root, "isolated"))
	if isolatedErr != nil {
		// Isolation governs placement quality, not config validity, so an
		// unreadable file leaves IsolationKnown false and the
		// vpp-cpu-isolation doctor check reports it to the operator. The
		// unknown is carried in the struct rather than in this error, which is
		// what lets a caller tell it from an isolated set that is really empty.
		return inv, nil //nolint:nilerr // IsolationKnown carries the failure to the caller
	}
	inv.Isolated = isolated
	inv.IsolationKnown = true
	return inv, nil
}

// readCoreListFile reads one sysfs CPU-list file. An empty file is a valid
// empty set, which is what /sys/devices/system/cpu/isolated holds on a host
// booted without isolcpus.
func readCoreListFile(path string) ([]uint8, error) {
	data, err := os.ReadFile(path) //nolint:gosec // sysfs path, or a fixture root a test declared
	if err != nil {
		return nil, err
	}
	ids, err := cpulist.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return ids, nil
}

// hasCore reports whether id is one of the inventory's online CPUs.
func (inv CPUInventory) hasCore(id uint8) bool {
	return slices.Contains(inv.Online, id)
}

// hasIsolatedCPUs reports whether the host both answered the isolation question
// and named at least one isolated CPU. It is one name for the two facts that
// decide where worker cores come from, so the two callers cannot drift apart on
// what "there is an isolated set" means.
func (inv CPUInventory) hasIsolatedCPUs() bool {
	if !inv.IsolationKnown {
		return false
	}
	return len(inv.Isolated) > 0
}

// workerPool returns the CPUs available to VPP workers, ascending, with
// mainCore removed because the main thread already owns it.
//
// The pool is the isolated set when the kernel isolated CPUs, and the online
// set otherwise. Drawing from the isolated set is the point of this feature: a
// busy-polling worker on a CPU the scheduler still uses competes with every
// other runnable task there.
func (inv CPUInventory) workerPool(mainCore uint8) []uint8 {
	source := inv.Online
	if inv.hasIsolatedCPUs() {
		source = inv.Isolated
	}
	pool := make([]uint8, 0, len(source))
	for _, id := range source {
		if id == mainCore {
			continue
		}
		pool = append(pool, id)
	}
	return pool
}

// errCPUWorkersAndCores refuses a cpu container that carries both a worker
// count and an explicit worker core list. They are two answers to one question,
// and Ze does not pick between them.
var errCPUWorkersAndCores = errors.New("vpp cpu: set workers or worker-cores, not both")

// validate refuses a CPU configuration the host contradicts.
//
// The host is read only when the operator asked Ze to place VPP threads. With
// every cpu placement leaf absent there is nothing to check against the host,
// and Ze behaves as it did before it consulted the isolated set.
//
// An unreadable inventory is refused rather than passed. Ze cannot then say
// whether the requested cores exist, and accepting the config would leave the
// operator believing a placement was checked when nothing checked it.
func (cpu *CPUSettings) validate() error {
	if cpu.MainCore == nil && cpu.Workers == nil && len(cpu.WorkerCores) == 0 {
		return nil
	}

	if cpu.Workers != nil && len(cpu.WorkerCores) > 0 {
		return errCPUWorkersAndCores
	}

	inv, err := hostCPUInventory()
	if err != nil {
		return fmt.Errorf("vpp cpu: %w", err)
	}
	return cpu.validateAgainst(inv)
}

// validateAgainst is validate's half that takes the host as an argument, so a
// test states the host it is testing against instead of reading the machine it
// runs on.
func (cpu *CPUSettings) validateAgainst(inv CPUInventory) error {
	if cpu.MainCore != nil && !inv.hasCore(*cpu.MainCore) {
		return fmt.Errorf("vpp cpu main-core: core %d is not online on this host (online %s)",
			*cpu.MainCore, cpulist.Format(inv.Online))
	}

	for _, core := range cpu.WorkerCores {
		if cpu.MainCore != nil && core == *cpu.MainCore {
			return fmt.Errorf("vpp cpu worker-cores: core %d is also main-core; the main thread and a worker cannot share a CPU", core)
		}
		if !inv.hasCore(core) {
			return fmt.Errorf("vpp cpu worker-cores: core %d is not online on this host (online %s)",
				core, cpulist.Format(inv.Online))
		}
	}

	// The same function startup.conf generation calls, so a config that
	// validates is a config Ze can write a core list for.
	if _, err := resolveWorkerCores(cpu, inv); err != nil {
		return fmt.Errorf("vpp cpu %w", err)
	}
	return nil
}

// resolveWorkerCores returns the CPU ids VPP pins its workers to, ascending.
//
// An explicit worker-cores list wins, because the operator named the CPUs. A
// worker count is drawn from the CPUs the kernel isolated, which is the point
// of this file: a busy-polling worker on a CPU the scheduler still uses
// competes with every other runnable task there. When the kernel isolated
// nothing, or could not be asked, the cores are the contiguous block after
// main-core, which is the placement Ze emitted before it read the isolated set,
// and the vpp-cpu-isolation doctor check reports that placement to the
// operator.
func resolveWorkerCores(cpu *CPUSettings, inv CPUInventory) ([]uint8, error) {
	if len(cpu.WorkerCores) > 0 {
		// The settings' own slice, for reading. Its cores are checked against
		// the host by validateAgainst, which every path into startup.conf
		// generation runs first (VPPManager.Run, vpp.go).
		return cpu.WorkerCores, nil
	}
	if cpu.Workers == nil || *cpu.Workers == 0 {
		return nil, nil
	}

	count := int(*cpu.Workers)
	var mainCore uint8
	if cpu.MainCore != nil {
		mainCore = *cpu.MainCore
	}

	if !inv.hasIsolatedCPUs() {
		return contiguousWorkerCores(mainCore, count, inv)
	}

	pool := inv.workerPool(mainCore)
	if count > len(pool) {
		return nil, fmt.Errorf("workers: %d requested, host offers %d for workers (isolated %s, main-core %d excluded)",
			count, len(pool), cpulist.Format(inv.Isolated), mainCore)
	}
	return pool[:count], nil
}

// contiguousWorkerCores returns count cores starting one after mainCore. It is
// the placement Ze used before it consulted the kernel, kept for a host that
// isolated no CPU: the block after main-core keeps the workers off CPU 0, where
// Linux places most of its interrupt work.
func contiguousWorkerCores(mainCore uint8, count int, inv CPUInventory) ([]uint8, error) {
	cores := make([]uint8, 0, count)
	for offset := 1; offset <= count; offset++ {
		id := int(mainCore) + offset
		if id > cpulist.IDMax || !inv.hasCore(uint8(id)) {
			return nil, fmt.Errorf("workers: %d requested after main-core %d, but core %d is not online on this host (online %s)",
				count, mainCore, id, cpulist.Format(inv.Online))
		}
		cores = append(cores, uint8(id))
	}
	return cores, nil
}
