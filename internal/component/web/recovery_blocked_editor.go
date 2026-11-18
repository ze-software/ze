// Design: docs/architecture/web-interface.md -- failed refresh fences admitted requests.
// Related: handler_config_recovery.go -- replaces the editor before retiring its cache entry.
package web

import (
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
)

// recoveryBlockedEditor is an unavailable candidate held only by requests that
// were admitted before recovery retired their session. It holds no old editor,
// tree, storage or ownership. New requests create a fresh editor instead.
//
// Error-returning operations refuse with reason. Text reads display that reason;
// a nil tree and empty identity/change collections mean this retired editor owns
// no candidate. These are deliberate absence guards, never a readable empty
// configuration. The setters cannot reactivate it. Safe for concurrent use.
// Its zero value is invalid; callers MUST use blockedRecoveryEditor.
type recoveryBlockedEditor struct {
	reason error
}

var _ contract.Editor = recoveryBlockedEditor{}

// blockedRecoveryEditor fences admitted requests after a failed refresh. The
// caller MUST supply the non-nil refusal returned for that failed refresh.
func blockedRecoveryEditor(reason error) contract.Editor {
	return recoveryBlockedEditor{reason: reason}
}

func (e recoveryBlockedEditor) ExecuteConfigCommand(contract.ConfigCommandRequest) (contract.ConfigCommandResult, error) {
	return contract.ConfigCommandResult{}, e.reason
}

func (recoveryBlockedEditor) SetSession(contract.EditSession) {}

func (e recoveryBlockedEditor) ResumeSession() error { return e.reason }

func (recoveryBlockedEditor) SessionID() string { return "" }

func (e recoveryBlockedEditor) CreateEntry([]string) error { return e.reason }

func (e recoveryBlockedEditor) SetValue([]string, string, string) error { return e.reason }

func (e recoveryBlockedEditor) DeleteValue([]string, string) error { return e.reason }

func (e recoveryBlockedEditor) DeleteByPath([]string) error { return e.reason }

func (e recoveryBlockedEditor) RenameListEntry([]string, string, string, []string) error {
	return e.reason
}

func (e recoveryBlockedEditor) CopyListEntry([]string, string, string, []string) error {
	return e.reason
}

func (e recoveryBlockedEditor) ApplyActivation([]string, bool) (string, error) {
	return "", e.reason
}

func (e recoveryBlockedEditor) CommitSession() (*contract.CommitResult, error) {
	return nil, e.reason
}

func (e recoveryBlockedEditor) CommitSessionCandidate(time.Time) (*contract.CommitResult, string, error) {
	return nil, "", e.reason
}

func (e recoveryBlockedEditor) CommitSessionForce() (*contract.CommitResult, error) {
	return nil, e.reason
}

func (e recoveryBlockedEditor) CommitSessionCandidateForce(time.Time) (*contract.CommitResult, string, error) {
	return nil, "", e.reason
}

func (e recoveryBlockedEditor) MarkCommittedContent(string) error { return e.reason }

func (e recoveryBlockedEditor) VerifySession() error { return e.reason }

func (e recoveryBlockedEditor) RefreshCommittedView() error { return e.reason }

func (e recoveryBlockedEditor) PendingDiscardNotice() (string, int64) { return e.reason.Error(), 0 }

func (e recoveryBlockedEditor) AckDiscardNotice(int64) error { return e.reason }

func (e recoveryBlockedEditor) OriginalContent() string { return e.reason.Error() }

func (e recoveryBlockedEditor) WorkingContent() string { return e.reason.Error() }

func (e recoveryBlockedEditor) Discard() error { return e.reason }

func (e recoveryBlockedEditor) DiscardSessionPath([]string) error { return e.reason }

func (e recoveryBlockedEditor) DisconnectSession(string) error { return e.reason }

func (e recoveryBlockedEditor) Diff() string { return e.reason.Error() }
func (e recoveryBlockedEditor) PendingReview([]string) (string, error) { return "", e.reason }

func (e recoveryBlockedEditor) SaveDraft() error { return e.reason }

func (recoveryBlockedEditor) SetPreCommitValidate(func(string) error) {}

func (e recoveryBlockedEditor) ListBackups() ([]contract.BackupInfo, error) { return nil, e.reason }

func (e recoveryBlockedEditor) Rollback(string) error { return e.reason }

func (recoveryBlockedEditor) Tree() any { return nil }

func (e recoveryBlockedEditor) ContentAtPath([]string) string { return e.reason.Error() }

func (e recoveryBlockedEditor) DisplayContentAtPath([]string) string { return e.reason.Error() }

func (e recoveryBlockedEditor) OriginalContentAtPath([]string) string { return e.reason.Error() }

func (recoveryBlockedEditor) Dirty() bool { return false }

func (recoveryBlockedEditor) SessionChanges(string) []contract.SessionChange { return nil }

func (recoveryBlockedEditor) PendingChanges(string) []contract.PendingChange { return nil }

func (e recoveryBlockedEditor) PendingDiscardChanges(string, string) ([]contract.PendingChange, error) {
	return nil, e.reason
}

func (recoveryBlockedEditor) ActiveSessions() []string { return nil }
