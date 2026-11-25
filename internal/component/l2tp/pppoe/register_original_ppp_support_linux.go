//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- native AC dependency.
// Related: backend_gate.go -- actual configured backend and schema default.

package pppoe

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := pppkernel.OriginalPPPSupportDoctorCheck(pppoeOriginalPPPSupportScope)
	check.Name = "pppoe-original-ppp-support"
	probe := check.Check
	check.Check = func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		rows := probe(ctx)
		for index := range rows {
			rows[index].Path = configRoot
		}
		return rows
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: register PPPoE original PPP support: " + err.Error())
	}
}

func pppoeOriginalPPPSupportScope(ctx diagnostic.DoctorCheckContext) (uint32, error) {
	if ctx.Tree == nil {
		return 0, nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return 0, errors.New("PPPoE doctor did not receive a parsed configuration tree")
	}
	if !PPPoEInUse(tree) {
		return 0, nil
	}
	if len(tree.GetContainer(configRoot).GetListOrdered("interface")) == 0 {
		return 0, nil
	}
	backend, err := ConfiguredSessionBackendName(tree)
	if err != nil {
		return 0, err
	}
	if backend != "kernel" {
		return 0, nil
	}
	return pppkernel.OriginalPPPSupportRequirements(false), nil
}
