//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- native client dependency.
// Related: iface/pppoe_client.go -- disabled entries are not reconciled.

package pppoeclient

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := pppkernel.OriginalPPPSupportDoctorCheck(clientOriginalPPPSupportScope)
	check.Name = "pppoe-client-original-ppp-support"
	probe := check.Check
	check.Check = func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		rows := probe(ctx)
		for index := range rows {
			rows[index].Path = "interface/pppoe-client"
		}
		return rows
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: register client original PPP support: " + err.Error())
	}
}

func clientOriginalPPPSupportScope(ctx diagnostic.DoctorCheckContext) (uint32, error) {
	if ctx.Tree == nil {
		return 0, nil
	}
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return 0, errors.New("PPPoE client doctor did not receive a parsed configuration tree")
	}
	interfaces := tree.GetContainer("interface")
	if interfaces == nil {
		return 0, nil
	}
	for _, entry := range interfaces.GetListOrdered("pppoe-client") {
		if _, disabled := entry.Value.Get("disable"); disabled {
			continue
		}
		return pppkernel.OriginalPPPSupportRequirements(false), nil
	}
	return 0, nil
}
