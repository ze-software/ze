// Design: docs/research/l2tpv2-ze-integration.md -- PPP IPv6 carrier runtime diagnostics

package ppp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codePPPIPv6Service = "doctor-ppp-ipv6-service"
	ipv6FailureFix     = "ze show errors"
	ipv6FailureWhy     = "The IPv6 control operation failed; the affected message or replacement service was not delivered. Inspect the retained carrier, subscriber-message or backend cause and repair it before retrying IPv6 service."
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codePPPIPv6Service, Title: "PPP IPv6 service operation failed",
		Description: ipv6FailureWhy + " This is event history, not a claim that the current PPP session is down or that resource cleanup failed.",
		Examples:    []string{ipv6FailureFix, "ze show diagnostic " + codePPPIPv6Service},
	}); err != nil {
		panic("BUG: PPP IPv6 diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codePPPIPv6Service}); err != nil {
		panic("BUG: PPP IPv6 event registration failed: " + err.Error())
	}
}

func reportIPv6Failure(logger *slog.Logger, ifname, operation string, cause error) {
	failure := opererr.Wrap(cause, "PPP IPv6 "+operation+" failed on interface "+ifname, ipv6FailureWhy, ipv6FailureFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codePPPIPv6Service), slog.String("interface", ifname))...)
	report.RaiseErrorFix("ppp", codePPPIPv6Service, diagnosticSubject(ifname+"/"+operation),
		report.FitMessage(failure.Problem()+". "+failure.Why()+": "+cause.Error()), failure.Fix(),
		map[string]any{reportFieldInterface: ifname, reportFieldOperation: operation, reportFieldError: cause.Error()})
}

// The exact human subject stays in the message and detail. A long backend name
// must not make the bounded bus reject an otherwise valid diagnostic event.
func diagnosticSubject(subject string) string {
	if len(subject) <= report.SubjectMax && subject != "" {
		return subject
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(subject)))
}

// A closed receive lane is normal only after its owning context has ended.
// Unwrap ordinary single-cause errors only: a composite may also contain a
// backend failure, and must remain visible with its complete joined cause.
func normalIPv6CarrierStop(ctx context.Context, err error) bool {
	if ctx.Err() == nil {
		return false
	}
	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(err) {
		err = cause
	}
	// A join is not a single-cause closure, even when one child is a normal
	// stop sentinel: another child may retain the backend failure.
	if _, composite := err.(interface{ Unwrap() []error }); composite {
		return false
	}
	return errors.Is(err, ctx.Err()) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}
