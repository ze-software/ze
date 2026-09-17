// Design: docs/features/ai-first.md -- the readiness checks the doctor component owns
// Overview: doctor.go -- the runner, which now reaches these through the registry
// Related: registry.go -- runDoctorChecks and doctorTree
// Related: register.go -- the init() that installs every entry below
//
// A check belongs to the package that owns the runtime dependency it probes
// (ai/patterns/registration.md, "Doctor Check Registry"). The entries here are
// the ones with no narrower owner: the storage and platform substrate `ze
// doctor` stands on, and the cross-component coordination checks that would
// make one component import another if either side held them.
//
// Order reproduces the sequence the runner used to write out by hand. An order
// below the 700-1000 band that owner packages use runs before them; an order
// above it runs after. What the sequence guarantees is the PHASE -- the store is
// open, the platform is resolved, the config is loaded -- and order inside a
// phase decides only the order the diagnostics print in.

package doctor

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	// doctorOwnComponent names this component as the owner of a check that has
	// no narrower one.
	doctorOwnComponent = "doctor"

	// The dependencies the registry publishes for the checks below: the
	// filesystem `ze doctor` stands on, and the config tree the coordination
	// checks read.
	doctorDependencyFilesystem = "filesystem"
	doctorDependencyConfig     = "config"
)

// doctorOwnedChecks are the checks this component owns, in the order the runner
// ran them.
var doctorOwnedChecks = []diagnostic.DoctorCheck{{
	Name:         "store-integrity",
	Phase:        diagnostic.DoctorPhasePreConfig,
	Order:        100,
	Component:    doctorOwnComponent,
	Dependencies: []string{"storage"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorStoreIntegrity},
	Check:        doctorCheckStoreIntegrity,
}, {
	Name:         "machine-id",
	Phase:        diagnostic.DoctorPhasePreConfig,
	Order:        120,
	Component:    doctorOwnComponent,
	Dependencies: []string{"storage", doctorDependencyFilesystem},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorMachineIDMissing},
	Check:        doctorCheckMachineID,
}, {
	Name:         "random-seed",
	Phase:        diagnostic.DoctorPhasePreConfig,
	Order:        130,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyFilesystem},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorRandomSeed},
	Check:        doctorCheckRandomSeed,
}, {
	// The missing-config twin of kernel-modules. The runner called
	// checkKernelModules(nil) on the path where no config loaded, so the
	// operator still learns that a module the daemon needs is absent. A nil
	// tree is what the missing-config phase context already carries, so the
	// same function answers both, under two names because one registry entry
	// belongs to one phase.
	Name:         "kernel-modules-no-config",
	Phase:        diagnostic.DoctorPhaseMissingConfig,
	Order:        2000,
	Component:    doctorOwnComponent,
	Dependencies: []string{"kernel"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorModuleMissing, diagnostic.CodeDoctorL2TPModule, diagnostic.CodeDoctorPPPoEModule},
	Check:        doctorCheckKernelModules,
}, {
	Name:         "kernel-modules",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        150,
	Component:    doctorOwnComponent,
	Dependencies: []string{"kernel"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorModuleMissing, diagnostic.CodeDoctorL2TPModule, diagnostic.CodeDoctorPPPoEModule},
	Check:        doctorCheckKernelModules,
}, {
	Name:         "disk-space",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2020,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyFilesystem},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorDiskSpace},
	Check:        doctorCheckDiskSpace,
}, {
	Name:         "clock-skew",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2100,
	Component:    doctorOwnComponent,
	Dependencies: []string{"clock"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorClockSkew},
	Check:        doctorCheckClockSkew,
}, {
	// The four AS112 coordination checks read BGP config and AS112 config
	// together. Neither side may hold them: the as112 plugin must not read BGP
	// config and BGP must not spell AS112 (checks_as112_coordination.go names
	// the spec decision). This component is the neutral third home, so these
	// four are owned here rather than parked here.
	Name:         "as112-watchdog-withdraw",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2150,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyConfig},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorAS112WatchdogMissingWithdraw},
	Check:        doctorCheckAS112WatchdogWithdraw,
}, {
	Name:         "as112-global-origin-coordination",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2160,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyConfig},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorAS112GlobalOriginUncoordinated},
	Check:        doctorCheckAS112GlobalOriginCoordination,
}, {
	Name:         "as112-redistribute-origin-coordination",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2170,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyConfig},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorAS112RedistributeOriginUncoordinated},
	Check:        doctorCheckAS112RedistributeOriginCoordination,
}, {
	Name:         "as112-redistribute-not-imported",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2180,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyConfig},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorAS112RedistributeNotImported},
	Check:        doctorCheckAS112RedistributeNotImported,
}, {
	Name:         "writable-destinations",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2270,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyFilesystem},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorWriteDestination},
	Check:        doctorCheckWritableDestinations,
}, {
	// The judgement over the platform the runner resolved. The resolution
	// itself is the runner's own input (resolveDoctorPlatform,
	// checks_platform.go): the phase dispatch filters every check on it, so it
	// cannot be a check. Order 90 keeps its diagnostics ahead of
	// store-integrity, where the runner printed them.
	Name:         "platform",
	Phase:        diagnostic.DoctorPhasePreConfig,
	Order:        90,
	Component:    doctorOwnComponent,
	Dependencies: []string{"platform"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorPlatformUnknown, diagnostic.CodeDoctorPlatformPerm, diagnostic.CodeDoctorPlatformContainerRO},
	Check:        checkPlatform,
}, {
	// Every configured listen endpoint, from every service, probed for a bind.
	// The listener inventory is the schema's and the probe is this component's,
	// so no one service owns a check over all of them. Order 2015 sits between
	// the SSH host key check (2010) and disk-space (2020), where the runner ran
	// it.
	Name:         "listeners",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2015,
	Component:    doctorOwnComponent,
	Dependencies: []string{"network"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes: []string{
		diagnostic.CodeDoctorListenUnavailable,
		diagnostic.CodeDoctorBGPListen,
		diagnostic.CodeDoctorBFDPort,
		diagnostic.CodeDoctorIPsecListen,
		diagnostic.CodeDoctorTFTPListen,
		diagnostic.CodeDoctorImageListen,
		diagnostic.CodeDoctorNTPListen,
	},
	Check: doctorCheckListeners,
}, {
	// The config-delivery audit reads the plugin registry and the schema
	// registry together (checks_config_claims.go names why neither side owns
	// it). Order 2310 sits after the S.M.A.R.T. check (2300) and before the RIR
	// delegation sources (2320), where the runner ran it.
	Name:         "config-claims",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2310,
	Component:    doctorOwnComponent,
	Dependencies: []string{doctorDependencyConfig, "plugin-registry"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorConfigClaimsUnavailable, diagnostic.CodeDoctorConfigRootUnclaimed},
	Check:        doctorCheckConfigClaims,
}}

// registerDoctorOwnedChecks installs every entry above. A refusal is a
// programmer error in the table beside it -- a duplicate name, a phase that
// does not exist, a code without the doctor prefix -- and none of them can be
// reached from a config or a peer, so it stops the process rather than leaving
// `ze doctor` quietly short of a check.
func registerDoctorOwnedChecks() {
	for i := range doctorOwnedChecks {
		if err := diagnostic.RegisterDoctorCheck(doctorOwnedChecks[i]); err != nil {
			panic("BUG: doctor check registration refused: " + err.Error())
		}
	}
}

func doctorCheckStoreIntegrity(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Store == nil {
		return nil // The open diagnostic already names absence or unsafe permissions.
	}
	return checkStoreIntegrity(ctx.ConfigDir)
}

func doctorCheckMachineID(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkMachineID(ctx.Platform, ctx.Store)
}

func doctorCheckRandomSeed(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkRandomSeed(ctx.Platform)
}

func doctorCheckKernelModules(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkKernelModules(doctorTree(ctx))
}

func doctorCheckDiskSpace(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkDiskSpace()
}

func doctorCheckClockSkew(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkClockSkew()
}

func doctorCheckAS112WatchdogWithdraw(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkAS112WatchdogWithdraw(doctorTree(ctx))
}

func doctorCheckAS112GlobalOriginCoordination(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkAS112GlobalOriginCoordination(doctorTree(ctx))
}

func doctorCheckAS112RedistributeOriginCoordination(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkAS112RedistributeOriginCoordination(doctorTree(ctx))
}

func doctorCheckAS112RedistributeNotImported(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkAS112RedistributeNotImported(doctorTree(ctx))
}

func doctorCheckWritableDestinations(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkWritableDestinations(doctorTree(ctx), ctx.Platform)
}

func doctorCheckListeners(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkListeners(doctorTree(ctx))
}

func doctorCheckConfigClaims(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkConfigClaims(doctorTree(ctx))
}
