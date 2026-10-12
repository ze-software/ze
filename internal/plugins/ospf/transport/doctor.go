// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- raw socket readiness doctor check

package transport

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

var rawSocketProbe = checkRawSocket

func checkOSPFRawSocket(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("ospf") == nil {
		return nil
	}
	err := rawSocketProbe()
	if err == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-ospf-raw-socket",
		Severity: diagnostic.SeverityWarning,
		Message:  err.Error(),
	}}
}
