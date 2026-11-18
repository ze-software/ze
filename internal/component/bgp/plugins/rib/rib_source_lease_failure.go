// Design: docs/architecture/plugin/rib-storage-design.md -- scoped source faults.
// Related: rib_advertise_diagnostic.go -- one runtime doctor aggregates faults.
package rib

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const sourceLeaseRetry = time.Second

func (s *ribSourceLeases) operationError(cause error, peer netip.Addr, receipt uint64, action rpc.SourceLeaseAction) error {
	var operation string
	switch action {
	case rpc.SourceLeaseAcquire:
		operation = "acquire"
	case rpc.SourceLeaseRetain:
		operation = "retain"
	case rpc.SourceLeaseRelease:
		operation = "release"
	case rpc.SourceLeaseUnspecified:
		panic("BUG: RIB issued an unspecified source lease action")
	default:
		panic("BUG: RIB issued an unknown source lease action")
	}
	problem := fmt.Sprintf("BGP routes received from %s could not %s source session %d", peer, operation, receipt)
	if _, ok := errors.AsType[*opererr.Error](cause); ok {
		return fmt.Errorf("%s: %w", problem, cause)
	}
	return opererr.Wrap(cause, problem,
		"This source's selected routes cannot be sent safely until its authorization settles; other sources remain available.", s.manager.advertiseRepair())
}

func (s *ribSourceLeases) setAdmission(peer uint64, err error) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	previous := s.admission[peer]
	changed := previous == nil || previous.Error() != err.Error()
	s.admission[peer] = err
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

func (s *ribSourceLeases) forgetAdmission(peer uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	_, changed := s.admission[peer]
	delete(s.admission, peer)
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

// failures contains current admission problems and failed final releases only.
// A successful unrelated source operation cannot erase a pending release.
func (s *ribSourceLeases) failures() map[uint64]error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	failures := make(map[uint64]error, len(s.admission))
	maps.Copy(failures, s.admission)
	var receipts []uint64
	for receipt, lease := range s.leases {
		if lease.failure != nil {
			receipts = append(receipts, receipt)
		}
	}
	slices.Sort(receipts)
	for _, receipt := range receipts {
		lease := s.leases[receipt]
		failures[lease.peerOwner] = errors.Join(failures[lease.peerOwner], lease.failure)
	}
	return failures
}

func (s *ribSourceLeases) doctorFailures() []rpc.DoctorCheckDiagnostic {
	failures := s.failures()
	if len(failures) == 0 {
		return nil
	}
	findings := make([]rpc.DoctorCheckDiagnostic, 0, len(failures))
	for _, failure := range failures {
		findings = append(findings, rpc.DoctorCheckDiagnostic{Code: diagnostic.CodeDoctorBGPSelectedOutput,
			Severity: "warning", Message: failure.Error(), Fix: s.manager.advertiseRepair()})
	}
	return findings
}

func sourceLeaseSubject(peer uint64) string {
	return fmt.Sprintf("received source owner %d", peer)
}

// reportFailures owns acknowledgment state only. It performs no operation under
// mu and no caller holds a peer, storage or election lock. A clear remains here
// only until the daemon acknowledges it; no historical source state is kept.
func (s *ribSourceLeases) reportFailures(ctx context.Context) bool {
	if s == nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return false
	}
	failures := s.failures()
	retry := false
	for peer, failure := range failures {
		message := failure.Error()
		if s.reported[peer] == message {
			continue
		}
		if time.Now().Before(s.reportAfter[peer]) {
			retry = true
			continue
		}
		fix := s.manager.advertiseRepair()
		logger().Warn("BGP source authorization has not settled", "peer", peer, "error", failure,
			"code", diagnostic.CodeDoctorBGPSelectedOutput, "fix", fix)
		work, cancel := context.WithTimeout(ctx, advertiseReportBudget)
		err := s.manager.plugin.RaiseIssue(work, sdk.Issue{Severity: rpc.ReportWarning,
			Code: diagnostic.CodeDoctorBGPSelectedOutput, Subject: sourceLeaseSubject(peer), Message: message, Fix: fix})
		cancel()
		if err != nil {
			s.reportAfter[peer] = time.Now().Add(rpc.ReportRaiseWindow)
			logger().Warn("BGP source authorization warning was not acknowledged", "peer", peer, "error", err,
				"code", diagnostic.CodeDoctorBGPSelectedOutput, "fix", fix)
			retry = true
			continue
		}
		s.reported[peer] = message
		delete(s.reportAfter, peer)
	}
	for peer := range s.reported {
		if failures[peer] != nil {
			continue
		}
		if time.Now().Before(s.reportAfter[peer]) {
			retry = true
			continue
		}
		work, cancel := context.WithTimeout(ctx, advertiseReportBudget)
		err := s.manager.plugin.ClearIssue(work, diagnostic.CodeDoctorBGPSelectedOutput, sourceLeaseSubject(peer))
		cancel()
		if err != nil {
			s.reportAfter[peer] = time.Now().Add(rpc.ReportRaiseWindow)
			logger().Warn("BGP source authorization warning could not be cleared", "peer", peer, "error", err,
				"code", diagnostic.CodeDoctorBGPSelectedOutput, "fix", s.manager.advertiseRepair())
			retry = true
			continue
		}
		delete(s.reported, peer)
		delete(s.reportAfter, peer)
	}
	for peer := range s.reportAfter {
		if failures[peer] != nil {
			continue
		}
		if s.reported[peer] == "" {
			delete(s.reportAfter, peer)
		}
	}
	return retry
}
