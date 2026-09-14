//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
//
// The loaded-module list, read once for each doctor check that asks about a
// MODULE by name: the nf_tables module the nft firewall backend needs, the vfio
// modules a DPDK-bound NIC needs, the l2tp and pppoe modules. It is not a
// capability probe. A capability enrolled here (probe_linux.go) is asked whether
// it EXISTS, because a built-in feature appears in no module row; a check that
// arrives here asks about a loadable module the kernel has to carry by that
// name, which is what /proc/modules answers.
//
// Three packages read it (internal/component/doctor, internal/plugins/firewall/nft
// and internal/component/vpp), so it lives beside ProcPath rather than in any
// one of them: one parser, and one test override for the functional tests that
// stand in a module list.

package kernelcap

import (
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

// modulesFileEnv names the file the module list is read from in a functional
// test, in place of /proc/modules. It keeps the doctor spelling for the same
// reason procRootEnv does: the doctor tier's fixtures set it.
const modulesFileEnv = "ze.test.doctor.modules-file"

var _ = env.MustRegister(env.EnvEntry{
	Key:         modulesFileEnv,
	Type:        envTypeString,
	Description: "Override /proc/modules path for doctor functional tests",
	Private:     true,
})

// loadedModulesPath is the file LoadedModules reads: the test stub when
// ze.test.doctor.modules-file names one, else procfs.
func loadedModulesPath() string {
	if path := env.Get(modulesFileEnv); path != "" {
		return path
	}
	return ProcPath("modules")
}

// LoadedModules answers the set of loaded kernel modules by name, or nil when
// the module list cannot be read.
//
// The two answers are kept apart on purpose: an empty set is a kernel that has
// loaded nothing, and nil is a list this process could not read. A caller that
// folded them would report every module missing on a host that merely hides
// /proc/modules (docs/contributing/ze-go-style.md, "A zero value is never an
// answer").
func LoadedModules() map[string]bool {
	data, err := readFile(loadedModulesPath())
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		if sp := strings.IndexByte(line, ' '); sp > 0 {
			set[line[:sp]] = true
		}
	}
	return set
}
