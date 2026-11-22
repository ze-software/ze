// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- MOBIKE over the shared NAT-T socket
// Related: apply.go -- repointSessions, which restarts a MOBIKE peer whose reopened pair lacks the shared socket

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
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorIPsecMOBIKESocketNotShared}); err != nil {
		panic("BUG: ike: MOBIKE shared-socket event code registration failed: " + err.Error())
	}
}

// mobikeSocketNotSharedFailure is the operator error for a MOBIKE peer that repointSessions
// restarts because the pair reopened at host did not take the shared port-4500 socket.
// openPair hands every pair that one socket only while shareNATTFn answers true, and a
// kept session holds it, so the answer changed between the session's open and this
// reopen.
func mobikeSocketNotSharedFailure(peer, host string) *opererr.Error {
	return opererr.New(
		"Ze restarted IPsec peer "+peer+" instead of moving its SA with MOBIKE: the sockets it reopened at "+host+
			" do not hold the port-4500 socket the peer's SA sends from",
		"The peer's tunnel carries no traffic until it renegotiates from the new address. Ze shares one port-4500 "+
			"socket between addresses only while the dataplane can migrate a tunnel and the ze.test.ike.port test "+
			"override is unset, and that answer changed after this peer's session opened. Unset ze.test.ike.port if "+
			"it is set, then restart Ze so every socket follows one decision",
		spdInstallFixRestart())
}

// reportMobikeSocketNotShared logs failure at Error under its doctor code and raises it
// once on the daemon's report bus through raiser, as an error event the error ring
// keeps. raiser is nil where no plugin connection exists (a unit test), and the coded
// log line is then the record.
func reportMobikeSocketNotShared(log *slog.Logger, failure *opererr.Error, raiser espFormIssueRaiser, peer string) {
	log.LogAttrs(context.Background(), slog.LevelError, "ike: "+failure.Problem(),
		failure.LogAttrs(slog.String("code", diagnostic.CodeDoctorIPsecMOBIKESocketNotShared))...)
	// refuse-by-default: guards repointSessions -- a test state holds no report connection.
	if raiser == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := raiser.RaiseIssue(ctx, sdk.Issue{Severity: rpc.ReportError,
		Code: diagnostic.CodeDoctorIPsecMOBIKESocketNotShared, Subject: "peer-" + peer,
		Message: failure.Problem() + " " + failure.Why(), Fix: failure.Fix()}); err != nil {
		log.Warn("ike: could not record a MOBIKE peer restart on the report bus; the error log keeps it",
			"code", diagnostic.CodeDoctorIPsecMOBIKESocketNotShared, "peer", peer, "error", err,
			"fix", "ze show log recent level error")
	}
}
