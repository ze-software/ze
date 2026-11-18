//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- L2TP native dependency.
// Related: kernelcap_linux.go -- actual L2TP enabled predicate.

package l2tp

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := pppkernel.OriginalPPPSupportDoctorCheck(l2tpOriginalPPPSupportScope)
	check.Name = "l2tp-original-ppp-support"
	check.Check = l2tpOriginalPPPSupportPath(check.Check)
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: register L2TP original PPP support: " + err.Error())
	}
}

func l2tpOriginalPPPSupportScope(ctx diagnostic.DoctorCheckContext) (uint32, error) {
	if ctx.Tree == nil {
		return 0, nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return 0, errors.New("L2TP doctor did not receive a parsed configuration tree")
	}
	if !L2TPInUse(tree) {
		return 0, nil
	}
	block := tree.GetContainer("l2tp")
	if len(block.GetListOrdered("relay")) > 0 {
		return pppkernel.OriginalPPPSupportRequirements(true), nil
	}
	return pppkernel.OriginalPPPSupportRequirements(len(block.GetListOrdered("relay-realm")) > 0), nil
}

func l2tpOriginalPPPSupportPath(check diagnostic.DoctorCheckFunc) diagnostic.DoctorCheckFunc {
	return func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		rows := check(ctx)
		for index := range rows {
			rows[index].Path = "l2tp"
		}
		return rows
	}
}
