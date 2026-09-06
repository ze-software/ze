//go:build linux

// Design: docs/architecture/vpp-host-tuning.md -- ze doctor check for the CPU
// isolation VPP workers depend on. Reads the CPU inventory through the same
// overridable root cpuset.go defines, so it is testable against fixtures.
// Registered from the vpp component via register_linux.go.
//
// Config validation refuses a placement the host cannot hold. This check
// reports a placement the host CAN hold but should not: workers on CPUs the
// Linux scheduler still owns, a kernel that would not say which CPUs are
// isolated, and an isolated set so wide that no CPU is left for Linux.

package vpp

import (
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/cpulist"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const doctorVPPCPUIsolationCode = "doctor-vpp-cpu-isolation"

// vppCPUIsolationDoctorCheck is the doctor check registered from register_linux.go.
func vppCPUIsolationDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "vpp-cpu-isolation",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        821,
		Component:    componentVPP,
		Dependencies: []string{"kernel"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorVPPCPUIsolationCode},
		Check:        checkVPPCPUIsolation,
	}
}

// checkVPPCPUIsolation is the registered doctor check. It is silent when VPP is
// disabled or when no CPU placement leaf is set, because VPP then places its
// own threads and there is no Ze decision to report on.
func checkVPPCPUIsolation(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	cpu, ok := cpuSettingsFromTree(tree)
	if !ok {
		return nil
	}
	inv, err := hostCPUInventory()
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     doctorVPPCPUIsolationCode,
			Severity: diagnostic.SeverityError,
			Message:  "VPP CPU isolation: cannot read the host CPU inventory: " + err.Error(),
		}}
	}
	return evaluateVPPCPUIsolation(&cpu, inv)
}

// cpuSettingsFromTree reads the vpp/cpu placement leaves from the config tree.
// ok is false when VPP is absent, disabled, or places no thread, and also when
// a leaf does not parse: config validation reports that, and a second voice
// saying it here would not help the operator.
func cpuSettingsFromTree(tree *config.Tree) (CPUSettings, bool) {
	vpp := tree.GetContainer(componentVPP)
	if vpp == nil {
		return CPUSettings{}, false
	}
	if enabled, _ := vpp.Get("enabled"); enabled != yangTrue {
		return CPUSettings{}, false
	}
	container := vpp.GetContainer("cpu")
	if container == nil {
		return CPUSettings{}, false
	}

	var cpu CPUSettings
	if v, found := container.Get("main-core"); found && v != "" {
		id, err := cpulist.ParseID(v)
		if err != nil {
			return CPUSettings{}, false
		}
		cpu.MainCore = &id
	}
	if v, found := container.Get("workers"); found && v != "" {
		// A count, not a core id, so it is parsed here rather than by
		// cpulist.ParseID. The uint8 bound is the same and the meaning is not.
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return CPUSettings{}, false
		}
		count := uint8(n)
		cpu.Workers = &count
	}
	if v, found := container.Get("worker-cores"); found && v != "" {
		cores, err := cpulist.Parse(v)
		if err != nil {
			return CPUSettings{}, false
		}
		cpu.WorkerCores = cores
	}

	if cpu.MainCore == nil && cpu.Workers == nil && len(cpu.WorkerCores) == 0 {
		return CPUSettings{}, false
	}
	return cpu, true
}

// isolatedContains reports whether id is one of the isolated CPUs. It answers
// false when isolation is unknown, so every caller checks IsolationKnown first.
func (inv CPUInventory) isolatedContains(id uint8) bool {
	return slices.Contains(inv.Isolated, id)
}

// nonIsolatedCount returns how many online CPUs the Linux scheduler still owns.
// ok is false when isolation is unknown, because a count of zero would then be
// read as "the control plane has no CPU" on a host that never said so.
func (inv CPUInventory) nonIsolatedCount() (count int, ok bool) {
	if !inv.IsolationKnown {
		return 0, false
	}
	for _, id := range inv.Online {
		if !inv.isolatedContains(id) {
			count++
		}
	}
	return count, true
}

// evaluateVPPCPUIsolation reports what the host says about the placement Ze
// would write into startup.conf.
func evaluateVPPCPUIsolation(cpu *CPUSettings, inv CPUInventory) []diagnostic.Diagnostic {
	var tb textbuf.Buffer

	if !inv.IsolationKnown {
		return []diagnostic.Diagnostic{{
			Code:     doctorVPPCPUIsolationCode,
			Severity: diagnostic.SeverityWarning,
			Message: "VPP CPU isolation: this host does not report which CPUs are isolated, so Ze cannot tell whether " +
				"the VPP workers share their CPUs with the Linux scheduler",
		}}
	}

	cores, err := resolveWorkerCores(cpu, inv)
	if err != nil {
		// Config validation refuses this and names the same reason, so the
		// check stays quiet rather than reporting it twice.
		return nil
	}

	var diags []diagnostic.Diagnostic

	var shared []uint8
	for _, core := range cores {
		if !inv.isolatedContains(core) {
			shared = append(shared, core)
		}
	}
	if len(shared) > 0 {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     doctorVPPCPUIsolationCode,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("VPP CPU isolation: worker core ").Str(cpulist.Format(shared)).
				Str(" is not isolated, so the Linux scheduler still places work on it; add isolcpus=").
				Str(cpulist.Format(cores)).Str(" to the boot cmdline, or set image.isolated-cpus on the appliance").
				String(),
		})
	}

	if left, known := inv.nonIsolatedCount(); known && left == 0 {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     doctorVPPCPUIsolationCode,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("VPP CPU isolation: every online CPU is isolated (").
				Str(cpulist.Format(inv.Online)).Str("), so the Linux control plane has no CPU of its own").String(),
		})
	}

	return diags
}
