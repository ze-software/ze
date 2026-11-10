package fixture

import (
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// pluginCode is the plugin check's code, held in an in-package constant.
const pluginCode = "doctor-fixture-plugin"

// sdkCheck is the declaration an SDK plugin sends at startup, with a literal
// code.
var sdkCheck = rpc.DoctorCheckDecl{Name: "fixture-sdk-check", Codes: []string{"doctor-fixture-sdk"}}

// sliceChecks declares a check as an elided element of a slice literal, the
// shape a package holding several checks uses.
var sliceChecks = []diagnostic.DoctorCheck{{
	Name:  "fixture-slice-check",
	Codes: []string{"doctor-fixture-slice"},
}}

// listCheck names its codes through another package's exported variable.
var listCheck = diagnostic.DoctorCheck{Name: "fixture-list-check", Codes: diagnostic.FixtureListCodes}

// capability is a kernel capability: kernelcap.MustRegister registers the
// check kernel-capability-fixture, which declares both codes.
var capability = kernelcap.Capability{
	Subsystem:   "fixture",
	CodeAbsent:  "doctor-fixture-absent",
	CodeUnknown: "doctor-fixture-unknown",
}

// runtimeCheck names its code through a field the source does not fix, so no
// code is read from it.
func runtimeCheck(code string) diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{Name: "fixture-runtime-check", Codes: []string{code}}
}

func init() {
	registry.Register(registry.Registration{
		Name: "fixture",
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:  "fixture-plugin-check",
			Codes: []string{pluginCode},
		}},
	})
}
