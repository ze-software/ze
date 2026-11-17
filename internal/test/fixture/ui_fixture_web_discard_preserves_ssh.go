// Design: docs/architecture/web-interface.md -- browser discard preserves SSH intent.
// Related: plugin_fixture_session_editor.go -- real SSH editing and transcript checks.
// Related: ui_fixture_web_recovery_session_survives_commit.go -- authenticated HTTP client.

package fixture

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

const (
	webDiscardCookieHeader = "Cookie"
	webDiscardHTMXHeader   = "HX-Request"
	webDiscardCookiePrefix = "ze-session="
	webDiscardSaveCommand  = "save"
	webDiscardSavedNotice  = "Changes saved to draft"
	webDiscardSSHPending   = "3 pending changes"
	webDiscardBrowserHost  = "browser-discarded"
)

func init() {
	Register("ui/web-discard-preserves-ssh", uiDriver(webDiscardPreservesSSH))
}

// webDiscardPreservesSSH crosses both live daemon entry points under one
// account. Saving the surviving SSH change file and then opening a fresh editor
// proves durable intent, rather than trusting the first editor's old candidate.
func webDiscardPreservesSSH(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: web-discard-preserves-ssh <ssh-port> <config> <web-url>")
	}
	env, err := sessionEditorClientEnv(ctx, args[0], valueTestUser)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "ze", "config", "edit", args[1]) //nolint:gosec // fixture-selected arguments
	command.Env = env
	terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 30, Cols: 160})
	if err != nil {
		return err
	}
	defer terminal.Close() //nolint:errcheck // fixture teardown
	defer func() {
		_, _ = killSessionEditor(command, terminal) //nolint:errcheck // joins the owned SSH client on every path
	}()
	var transcript bytes.Buffer
	stage, cancel := context.WithTimeout(ctx, 5*time.Second)
	pending, err := sessionEditorWaitFor(stage, terminal, nil, &transcript, "\x1b[?1049h")
	cancel()
	if err != nil {
		return fmt.Errorf("SSH first frame: %w\n%s", err, transcript.String())
	}
	send := func(line, answer string) error {
		stage, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := terminal.WriteString(line + "\r"); err != nil {
			return err
		}
		pending, err = sessionEditorWaitFor(stage, terminal, pending, &transcript, answer)
		if err != nil {
			return fmt.Errorf("SSH %q: %w\n%s", line, err, transcript.String())
		}
		return nil
	}
	for _, step := range []struct{ line, answer string }{
		// Quoted input differs from cmdSet's normalized success message, so
		// the input echo cannot satisfy the completion barrier. TUI output
		// is differential and may omit a status prefix already on screen.
		{`set bgp router-id "10.0.0.88"`, "set bgp router-id 10.0.0.88"},
		{webDiscardSaveCommand, webDiscardSavedNotice},
		{`set system host "ssh-survivor"`, "set system host ssh-survivor"},
		{"copy bgp group original to ssh-copy", "Copied group original to ssh-copy"},
		{"show | changes", webDiscardSSHPending},
	} {
		if err := send(step.line, step.answer); err != nil {
			return err
		}
	}
	client, err := newWebRecoveryClient(ctx, args[2])
	if err != nil {
		return err
	}
	token := client.waitForLogin(valueTestUser, valueTestPassword)
	if token == nil {
		return fmt.Errorf("the SSH account could not log in through the browser")
	}
	headers := map[string]string{webDiscardCookieHeader: webDiscardCookiePrefix + *token, webDiscardHTMXHeader: valueTrue}
	response := client.post("/config/set/system/", []webRecoveryField{
		{name: fieldLeaf, value: "host"}, {name: fieldValue, value: webDiscardBrowserHost},
	}, headers)
	if !webRecoveryStatusIs(response.status, http.StatusOK) {
		return fmt.Errorf("stage browser edit: HTTP %s: %s", webRecoveryStatusText(response.status), response.body)
	}
	if err := webDiscardReviewCount(client, *token, 1); err != nil {
		return err
	}
	response = client.post("/config/discard/", nil, map[string]string{webDiscardCookieHeader: webDiscardCookiePrefix + *token})
	if !webRecoveryStatusIs(response.status, http.StatusSeeOther) {
		return fmt.Errorf("browser discard: HTTP %s: %s", webRecoveryStatusText(response.status), response.body)
	}
	if err := webDiscardReviewCount(client, *token, 0); err != nil {
		return err
	}
	// SaveDraft reads this owner's durable file. A deleted file makes this
	// save a no-op; it cannot recreate the old values from this client's tree.
	if err := send(webDiscardSaveCommand, webDiscardSavedNotice); err != nil {
		return err
	}
	reopen, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := sessionEditorRun(reopen, env, args[1], []sessionEditorStep{
		{verb: sessionEditorSend, text: "show | changes all"},
		{verb: sessionEditorWait, text: webDiscardSSHPending},
		{verb: sessionEditorSend, text: "show bgp | match router-id"},
		{verb: sessionEditorWait, text: "router-id 10.0.0.88"},
		{verb: sessionEditorSend, text: "show bgp | match ssh-copy"},
		{verb: sessionEditorWait, text: "group ssh-copy"},
		{verb: sessionEditorSend, text: "show system | match host"},
		{verb: sessionEditorWait, text: "host ssh-survivor"},
	}); err != nil {
		return fmt.Errorf("reopened SSH candidate lost durable intent: %w", err)
	}
	for _, check := range []struct {
		text string
		want bool
	}{{"baseline-host", true}, {"10.0.0.250", true}, {"ssh-survivor", false}, {"ssh-copy", false}, {webDiscardBrowserHost, false}} {
		if err := sessionEditorPoll(ctx, env, "show config running", check.text, check.want); err != nil {
			return fmt.Errorf("discard changed committed configuration: %w", err)
		}
	}
	return nil
}

func webDiscardReviewCount(client *webRecoveryClient, token string, count int) error {
	request, err := http.NewRequestWithContext(client.ctx, http.MethodGet, client.baseURL+"/config/diff", http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set(webDiscardCookieHeader, webDiscardCookiePrefix+token)
	httpClient, transport := webRecoveryHTTPClient()
	defer transport.CloseIdleConnections()
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close() //nolint:errcheck // the body is read below
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("browser review: HTTP %d: %s", response.StatusCode, body)
	}
	if !strings.Contains(string(body), fmt.Sprintf("Review changes (%d)", count)) {
		return fmt.Errorf("browser review does not hold %d changes: %s", count, body)
	}
	return nil
}
