package daemonroute

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/redact"
)

// keyAnswer is the answer shape of a daemon key command, as the local `ze
// data rm` and `ze data write` read it.
type keyAnswer struct {
	Key string `json:"key"`
}

// VALIDATES: an answer the daemon gives that is not JSON is read as no answer,
// never as success, with the command to run by hand as the fix.
// PREVENTS: a removal printed as done when nothing says it ran.
func TestSendToDaemonUnreadableAnswerIsNoAnswer(t *testing.T) {
	ask := func(_, _ string) ([]byte, error) { return []byte("removed, probably"), nil }
	var answer keyAnswer
	err := Send(ask, "/var/lib/ze/database", "delete data key meta/x", &answer)
	unanswered, ok := errors.AsType[*Unanswered](err)
	if !ok {
		t.Fatalf("err = %v, want a *Unanswered", err)
	}
	if want := `ze cli --user <name> -c "delete data key meta/x"`; unanswered.Fix != want {
		t.Fatalf("fix = %q, want %q", unanswered.Fix, want)
	}
}

// TestDaemonRouteTextsHoldNoContent proves a line that carries a content never
// reaches a text the local route shows the operator.
//
// GOAL: when the daemon gives no usable answer to `update data key <k>
// content <v>`, the refusal printed, its why and its fix hold the placeholder
// and not the value.
// METHOD: answer the line with bytes that are not JSON, build the refusal the
// local command prints, and read every part of it; read CommandFix alone.
// VALIDATES: AC-8 (a refusal that echoes an argument): Send, CommandFix,
// UnansweredRefusal.
// PREVENTS: the base64 of a credential printed on the terminal and into any
// log of stderr.
func TestDaemonRouteTextsHoldNoContent(t *testing.T) {
	const content = "c2VjcmV0LWNyZWRlbnRpYWw="
	line := "update data key meta/test/state content " + content
	ask := func(_, _ string) ([]byte, error) { return []byte("written, probably"), nil }
	var answer keyAnswer
	err := Send(ask, "/var/lib/ze/database", line, &answer)
	unanswered, ok := errors.AsType[*Unanswered](err)
	if !ok {
		t.Fatalf("err = %v, want a *Unanswered", err)
	}
	refusal := UnansweredRefusal("ze data write", storage.ErrBusy, unanswered).Error()
	for name, text := range map[string]string{"why": unanswered.Why, "fix": unanswered.Fix, "refusal": refusal} {
		if strings.Contains(text, content) {
			t.Errorf("%s holds the content: %q", name, text)
		}
	}
	if !strings.Contains(refusal, "content "+redact.Placeholder) {
		t.Errorf("refusal = %q, want the placeholder after content", refusal)
	}
	want := `ze cli --user <name> -c "update data key meta/test/state content ` + redact.Placeholder + `"`
	if fix := CommandFix(line); fix != want {
		t.Errorf("CommandFix = %q, want %q", fix, want)
	}
}
