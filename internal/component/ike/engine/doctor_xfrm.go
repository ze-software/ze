// Design: plan/spec-ipsec-dataplane-inspection.md -- IPsec dataplane reachability check
// Related: register.go -- Registration.DoctorChecks declaration
// Related: doctor.go -- the vpn ipsec interface readiness check

package engine

import (
	"errors"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const envKeyIKEXFRMFail = "ze.test.ike.xfrm-fail"

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyIKEXFRMFail,
	Type:        "bool",
	Default:     "",
	Description: "Force the IPsec XFRM doctor probe to fail (test infrastructure)",
	Private:     true,
})

// errXFRMForced is what the test override reports. It names itself so an operator
// who finds it in a diagnostic knows the failure was injected.
var errXFRMForced = errors.New("forced failure (ze.test.ike.xfrm-fail)")

// xfrmProbe reports why the kernel XFRM dataplane did not answer, or nil when it
// did. It is a package-level var so a unit test drives both answers on a host
// whose own kernel never changes. The seam precedent is xfrmPolicyDel
// (internal/component/ike/dataplane/xfrm_linux.go).
var xfrmProbe = probeXFRMDataplane

// probeXFRMDataplane queries XFRM, honoring the test override first so a
// functional test reaches the diagnostic on a healthy kernel.
func probeXFRMDataplane() error {
	if coreenv.IsEnabled(envKeyIKEXFRMFail) {
		return errXFRMForced
	}
	return probeXFRM()
}

// checkXFRMReachable reports a configured IPsec that the host's XFRM dataplane
// cannot carry.
//
// The expectation comes from the CONFIG TREE, never from ActiveTable(). ze doctor
// runs offline against a config file, in a process where the engine never ran, so
// ActiveTable() is nil there and reading it would make this check silent in the
// one context it exists for.
//
// Warning rather than error: the probe also fails for a host that holds a working
// kernel and no CAP_NET_ADMIN. Both answers need operator action, and only one of
// them is a broken kernel, so the severity does not claim to tell them apart.
func checkXFRMReachable(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainerPath("vpn/ipsec") == nil {
		return nil
	}
	err := xfrmProbe()
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ipsec-xfrm-unavailable",
		Severity: "warning",
		Message: tb.Str("vpn ipsec is configured and the kernel XFRM dataplane did not answer: ").Err(err).
			Str("; ze installs every Child SA through XFRM, so tunnels will negotiate and carry no traffic").String(),
	}}
}
