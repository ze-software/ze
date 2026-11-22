//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- namespace FWD default dependency.
// Related: dataplane/xfrm_forward_default_linux.go -- live, read-only host and ownership check.

package engine

import (
	"errors"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func answerForwardDefault(_ *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	dp := dataplane.Get()
	_, usesDefault := forwardDefaultInstaller(dp)
	if !usesDefault {
		return nil
	}
	checker, ok := dp.(dataplane.ForwardDefaultChecker)
	if !ok {
		return forwardDefaultFinding(opererr.New("Ze's dataplane cannot inspect its IPsec forwarding-default claim",
			"A backend using the namespace default must expose its live check. This is a Ze defect; collect the installed version when reporting it.", "ze show version"))
	}
	if err := checker.CheckForwardDefault(); err != nil {
		failure, ok := errors.AsType[*opererr.Error](err)
		if !ok {
			failure = opererr.Wrap(err, "Ze cannot inspect the forwarding default its IPsec policies need",
				"The current forwarding disposition is unknown, so forwarded traffic may meet weaker protection than "+
					"the IPsec configuration states. A restart reads the namespace default again and claims it for the "+
					"configuration, and names the cause if the read fails again.", spdInstallFixRestart())
		}
		return forwardDefaultFinding(failure)
	}
	return nil
}

// forwardDefaultFinding is the doctor finding for a forwarding-default failure, at the
// severity its log line and report-bus event carry (logForwardDefaultRefusal): error,
// because the namespace holds a forwarding disposition Ze did not decide.
func forwardDefaultFinding(failure *opererr.Error) []rpc.DoctorCheckDiagnostic {
	message := failure.Problem() + " " + failure.Why()
	if cause := failure.Unwrap(); cause != nil {
		message += " (" + cause.Error() + ")"
	}
	return []rpc.DoctorCheckDiagnostic{{Code: diagnostic.CodeDoctorIPsecForwardDefault,
		Severity: severityError, Message: message, Fix: failure.Fix()}}
}
