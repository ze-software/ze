// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- forwarding-default restoration events.
// Related: register.go -- SDK remains available through policy cleanup when the connection survives.

package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorIPsecForwardDefault}); err != nil {
		panic("BUG: ike: forwarding-default event code registration failed: " + err.Error())
	}
}

func logForwardDefaultRefusal(log *slog.Logger, failure *opererr.Error) {
	log.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorIPsecForwardDefault))...)
}

// One issue covers a removal pass, even if several final entries remained. The
// SDK is the same report-issue boundary in both process modes. After a daemon
// disconnect the request can fail; the coded log remains the evidence then.
func raiseForwardDefaultRefusal(log *slog.Logger, failure *opererr.Error, raisers []espFormIssueRaiser) {
	// refuse-by-default: guards runEngine -- direct installer tests have no live plugin or daemon report connection.
	if len(raisers) == 0 || raisers[0] == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	message := failure.Problem() + " " + failure.Why()
	if cause := failure.Unwrap(); cause != nil {
		message += " (" + cause.Error() + ")"
	}
	if err := raisers[0].RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportError,
		Code: diagnostic.CodeDoctorIPsecForwardDefault, Subject: "namespace-forward-default",
		Message: message, Fix: failure.Fix()}); err != nil {
		logForwardDefaultRefusal(log, opererr.Wrap(err, "Ze could not record its IPsec forwarding-default failure on the report bus",
			"The daemon connection may already be closed during shutdown. The forwarding-default failure remains in the warning log; the command below lists it.",
			"ze show log recent level warn"))
	}
}
