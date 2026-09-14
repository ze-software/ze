// Design: docs/features/ai-first.md — Linux-specific readiness checks
//
// The checks here are the ones this component registers itself
// (doctor_checks.go): the kernel-module list, the machine id and the random
// seed. Every other Linux probe that lived here moved to the package that owns
// the dependency it reads (ai/patterns/registration.md, "Doctor Check
// Registry"): the interface checks to internal/component/iface, the VPP socket
// and version probes to internal/plugins/iface/vpp, DPDK to
// internal/component/vpp, the nexthop probe to internal/plugins/fib/kernel,
// nf_tables to internal/plugins/firewall/nft, /proc/stat to
// internal/component/telemetry/exporter, conntrack to
// internal/component/config/system, route netlink to
// internal/plugins/policyroute, CAP_SYS_TIME to internal/plugins/ntp and SMART
// to internal/component/storage.

//go:build linux

package doctor

import (
	"errors"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	backendVPP          = "vpp"
	doctorMachineIDEnv  = "ze.test.doctor.machine-id-path"
	doctorRandomSeedEnv = "ze.test.doctor.random-seed-path"

	machineIDPath = "/etc/machine-id"

	gokrazyRandomSeedPath = "/perm/random.seed"
	systemdRandomSeedPath = "/var/lib/systemd/random-seed"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorMachineIDEnv,
	Type:        envTypeString,
	Description: "Override /etc/machine-id path for doctor functional tests",
	Private:     true,
})

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorRandomSeedEnv,
	Type:        envTypeString,
	Description: "Override random-seed path for doctor functional tests",
	Private:     true,
})

// loadedKernelModules is the module-list reader checkKernelModules runs. It is
// a variable so a test can stand in a module list; nothing else assigns it.
// kernelcap holds the reader because the nft firewall backend and the VPP
// component ask the same question of the same file.
var loadedKernelModules = kernelcap.LoadedModules
var statPath = os.Stat
var readFilePath = os.ReadFile

func checkKernelModules(tree *config.Tree) []diagnostic.Diagnostic {
	var required []string
	hasIPsec := false
	l2tpRequired := false
	pppoeRequired := false

	if tree != nil {
		ifaceBlock := tree.GetContainer("interface")
		if ifaceBlock != nil {
			backend, _ := ifaceBlock.Get("backend")
			if backend == backendVPP {
				required = append(required, "vhost_net")
			}
		}

		if l2tp := tree.GetContainer("l2tp"); configEnabled(l2tp, true) {
			l2tpRequired = true
		}

		if pppoe := tree.GetContainer("pppoe"); configEnabled(pppoe, true) {
			pppoeRequired = true
		}

		// One predicate, three readers: this check, the capability gate and
		// extractIPsecListeners (owner decision 6, 2026-08-14). An empty
		// `vpn { ipsec { } }` installs no Security Association, so it warns
		// about no module and opens no listener.
		hasIPsec = kernelcap.IPsecInUse(tree)
	}

	if len(required) == 0 && !l2tpRequired && !pppoeRequired && !hasIPsec {
		return nil
	}

	loaded := loadedKernelModules()
	var diags []diagnostic.Diagnostic
	var tb textbuf.Buffer
	for _, mod := range required {
		if !loaded[mod] {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeDoctorModuleMissing,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("kernel module not loaded: ").Str(mod).String(),
			})
		}
	}

	if l2tpRequired && !loaded["l2tp_ppp"] && !loaded["pppol2tp"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorL2TPModule,
			Severity: diagnostic.SeverityError,
			Message:  "L2TP kernel module not loaded: l2tp_ppp or pppol2tp",
		})
	}

	if pppoeRequired && !loaded["pppoe"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorPPPoEModule,
			Severity: diagnostic.SeverityError,
			Message:  "PPPoE kernel module not loaded: pppoe",
		})
	}

	// The XFRM dataplane is NOT asked about here. It is an enrolled kernel
	// capability (internal/component/kernelcap), so one probe answers for
	// ze doctor, for the daemon's startup refusal and for `ze config validate`,
	// and the three cannot disagree. A module list could not answer it at all:
	// an appliance kernel builds XFRM in, so xfrm_user appears in no module row.

	if hasIPsec && !loaded["ip_tables"] && !loaded["nf_tables"] {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeDoctorModuleMissing,
			Severity: diagnostic.SeverityWarning,
			Message:  "IPsec: neither ip_tables nor nf_tables loaded (firewall marking may not work)",
		})
	}

	return diags
}

// sysClassNetDir is the directory the interface presence and state checks read.
func checkMachineID(platform *host.PlatformInfo, store storage.Storage) []diagnostic.Diagnostic {
	if platform == nil || (platform.Type != host.PlatformGokrazy && platform.Type != host.PlatformSystemd) {
		return nil
	}

	path := doctorMachineIDPath()
	data, err := readFilePath(path)
	if err == nil && strings.TrimSpace(string(data)) != "" {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if store != nil {
		if zefsData, zefsErr := store.ReadFile(zefs.KeyMachineID.Pattern); zefsErr == nil {
			if strings.TrimSpace(string(zefsData)) != "" {
				return nil
			}
		}
	}

	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     diagnostic.CodeDoctorMachineIDMissing,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("machine-id is missing or empty on ").Str(platform.Type.String()).String(),
		Path:     path,
		Expected: tb.Reset().Str("non-empty ").Str(path).Str(" or zefs meta/instance/machine-id").String(),
		Actual:   "missing or empty",
	}}
}

func doctorMachineIDPath() string {
	path := strings.TrimSpace(env.Get(doctorMachineIDEnv))
	if path != "" {
		return path
	}
	return machineIDPath
}
func checkRandomSeed(platform *host.PlatformInfo) []diagnostic.Diagnostic {
	if platform == nil {
		return nil
	}

	switch platform.Type {
	case host.PlatformGokrazy:
		path := randomSeedPath(gokrazyRandomSeedPath)
		if _, err := statPath(path); err == nil {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorRandomSeed,
			Severity: diagnostic.SeverityWarning,
			Message:  "gokrazy random seed not found at " + path + "; verify randomd is included in the gokrazy image",
			Path:     path,
			Expected: "randomd seed file",
			Actual:   "missing",
		}}

	case host.PlatformSystemd:
		path := randomSeedPath(systemdRandomSeedPath)
		if _, err := statPath(path); err == nil {
			return nil
		}
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorRandomSeed,
			Severity: diagnostic.SeverityWarning,
			Message:  "systemd random seed not found at " + path + "; systemd-random-seed.service may not be enabled",
			Path:     path,
			Expected: "systemd-random-seed.service seed file",
			Actual:   "missing",
		}}

	case host.PlatformPlainLinux:
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorRandomSeed,
			Severity: diagnostic.SeverityWarning,
			Message:  "non-systemd Linux without a known random-seed service; early-boot entropy may be insufficient for cryptographic operations",
		}}

	default:
		return nil
	}
}

func randomSeedPath(defaultPath string) string {
	if override := strings.TrimSpace(env.Get(doctorRandomSeedEnv)); override != "" {
		return override
	}
	return defaultPath
}
