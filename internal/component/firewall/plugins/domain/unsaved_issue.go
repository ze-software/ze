// Design: docs/architecture/firewall/firewall-domain-group.md -- an unsaved answer, raised on the report bus
// Related: cache.go -- store.put, open and purge set and drop the unsaved records this mirrors
// Related: doctor.go -- unsavedAnswerMessage, the text the doctor finding carries too

package domain

import (
	"context"
	"hash/fnv"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// unsavedIssueTimeout bounds one report-issue request, so a daemon that does
// not answer holds a refresh for no longer.
const unsavedIssueTimeout = 5 * time.Second

// unsavedSubjectHashLen is the octets the subject of a cut name ends with: a
// '~' and the eight hex digits of the whole name's FNV-1a hash.
const unsavedSubjectHashLen = 1 + 8

// unsavedIssues is the part of *sdk.Plugin the store raises through. A forked
// plugin's own report bus is a copy nothing reads, so the raise goes to the
// daemon over the SDK (pkg/plugin/sdk/report.go); a test records into one.
type unsavedIssues interface {
	RaiseIssue(ctx context.Context, issue sdk.Issue) error
	ClearIssue(ctx context.Context, code, subject string) error
}

// syncIssue makes the bus agree with the unsaved record of one unit: a
// warning while the record is set, none once it is dropped. A raise is sent
// once per record, not per failed write, so a store that refuses every write
// stays inside the daemon's raise bound; a raise or clear the daemon did not
// take is retried at the unit's next write.
//
// It reads the record when it runs, under reportMu, so of two writes racing
// on one unit the later sync sees the final record and the bus ends equal to
// it. The caller MUST NOT hold mu.
func (s *store) syncIssue(key nameKey) {
	if s.issues == nil {
		return
	}
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	s.syncIssueLocked(key)
}

// syncAllIssues runs syncIssue over every unit that holds a record or a
// warning, for open and purge, which drop the records of many units at once.
// The caller MUST NOT hold mu.
func (s *store) syncAllIssues() {
	if s.issues == nil {
		return
	}
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	s.mu.RLock()
	units := make([]nameKey, 0, len(s.unsaved)+len(s.raised))
	for key := range s.unsaved {
		units = append(units, key)
	}
	s.mu.RUnlock()
	for key := range s.raised {
		units = append(units, key)
	}
	for _, key := range units {
		s.syncIssueLocked(key)
	}
}

// syncIssueLocked is syncIssue with reportMu held.
func (s *store) syncIssueLocked(key nameKey) {
	s.mu.RLock()
	failure, unsaved := s.unsaved[key]
	s.mu.RUnlock()
	_, raised := s.raised[key]
	if unsaved {
		if raised {
			return
		}
		if err := s.raiseUnsaved(unsavedUnit{key: key, failure: failure}); err != nil {
			logger().Warn("firewall-domain: the unsaved answer could not be raised on the report bus, so show warnings does not list it",
				"group", key.group, "name", key.name, "family", key.family.label, "error", err,
				"code", codeDomainGroupUnsavedAnswer, "fix", updateFix(key.group))
			return
		}
		s.raised[key] = struct{}{}
		return
	}
	if !raised {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), unsavedIssueTimeout)
	defer cancel()
	if err := s.issues.ClearIssue(ctx, codeDomainGroupUnsavedAnswer, unsavedIssueSubject(key)); err != nil {
		logger().Warn("firewall-domain: the answer was saved and its warning could not be cleared from the report bus, so show warnings still lists it",
			"group", key.group, "name", key.name, "family", key.family.label, "error", err,
			"code", codeDomainGroupUnsavedAnswer, "fix", updateFix(key.group))
		return
	}
	delete(s.raised, key)
}

// raiseUnsaved sends one unit's warning, with the doctor finding's text and
// fix.
func (s *store) raiseUnsaved(unit unsavedUnit) error {
	ctx, cancel := context.WithTimeout(context.Background(), unsavedIssueTimeout)
	defer cancel()
	return s.issues.RaiseIssue(ctx, sdk.Issue{
		Severity: rpc.ReportWarning,
		Code:     codeDomainGroupUnsavedAnswer,
		Subject:  unsavedIssueSubject(unit.key),
		Message:  cutAtRune(unsavedAnswerMessage(unit), rpc.ReportMessageMax),
		Fix:      updateFix(unit.key.group),
	})
}

// unsavedIssueSubject names one unit on the bus: group, family and name. A
// group holds 64 octets and a name 253, which together pass the bus's subject
// bound, so a subject that does not fit is cut and ends with a hash of the
// whole name: two long names of one group keep two subjects, and the clear of
// one never clears the other.
func unsavedIssueSubject(key nameKey) string {
	var tb textbuf.Buffer
	tb.Str(key.group).Byte(' ').Str(key.family.label).Byte(' ').Str(key.name)
	subject := tb.String()
	if len(subject) <= rpc.ReportSubjectMax {
		return subject
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key.name)) //nolint:errcheck // hash.Hash.Write never returns an error
	var cut textbuf.Buffer
	cut.Str(cutAtRune(subject, rpc.ReportSubjectMax-unsavedSubjectHashLen)).Byte('~').Hex(hash.Sum(nil))
	return cut.String()
}

// cutAtRune answers text cut to at most limit octets, at a UTF-8 rune
// boundary. A message carries a store error of any length, and the daemon
// refuses one past the bus's bound.
func cutAtRune(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}
