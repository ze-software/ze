// Design: docs/architecture/web-interface.md -- Applied commit feedback and SSE
//go:build integration

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestBrowserCommitAdvisorySurvivesDelayedConfigChange isolates the consumer
// before the graphical HTTP fix: actual runCommit text is returned by the
// existing notification OOB renderer, then the real EventSource receives the
// same commit's held banner. No replacement JavaScript renders either answer.
// MUTATION: clear notification-bar on every config-change; the advisory is lost
// after the browser has positively shown it, while the event still arrives.
func TestBrowserCommitAdvisorySurvivesDelayedConfigChange(t *testing.T) {
	checkBrowserCommitAdvisoryAfterSSE(t, false)
}

// TestBrowserGraphicalCommitWarningEndToEnd repeats the same observed sequence
// through HandleConfigCommitWithAuthorizerAndAudit. This proves the graphical
// producer uses the consumer behavior proved independently above.
func TestBrowserGraphicalCommitWarningEndToEnd(t *testing.T) {
	checkBrowserCommitAdvisoryAfterSSE(t, true)
}

func checkBrowserCommitAdvisoryAfterSSE(t *testing.T, graphicalHandler bool) {
	t.Helper()
	browser, broker, release := newCommitNotificationBrowser(t, graphicalHandler)
	browser.command("click", "#commit-review-btn")
	browser.wait(`!!document.querySelector('#diff-modal.open')`, "open graphical review")
	browser.command("click", "#diff-modal.open .commit-btn")
	warning := strconv.Quote(graphicalCommitCleanupProblem)
	browser.wait(`document.getElementById('notification-bar').textContent.includes(`+warning+`)`, "returned applied-warning OOB")
	assert.Equal(t, "true", browser.evaluate(`(function(){var bar=document.getElementById('notification-bar');return bar.classList.contains('notification-info')&&bar.textContent.includes(`+strconv.Quote(terminalOutputCommitSuccessful)+`)&&bar.getBoundingClientRect().height>0&&getComputedStyle(bar).display!=='none';})()`), "the real successful advisory is visible before SSE delivery")
	release()
	browser.wait(`window.__commitNotificationEvents===1`, "delayed same-commit EventSource delivery")
	assert.Equal(t, "true", browser.evaluate(`document.getElementById('notification-bar').textContent.includes(`+warning+`)`), "the delayed event must not erase an applied warning")
	for _, reason := range []string{"second configuration change", "third configuration change"} {
		BroadcastConfigChange(broker, "bob", reason)
		browser.wait(`document.querySelector('#notification-bar .notification-reason').textContent.includes(`+strconv.Quote(reason)+`)`, "latest real config-change banner")
		assert.Equal(t, "1", browser.evaluate(`document.querySelectorAll('#notification-bar .notification-banner').length`), "subsequent events replace one banner rather than accumulate")
		assert.Equal(t, "true", browser.evaluate(`(function(){var text=document.getElementById('notification-bar').textContent;return text.split(`+warning+`).length===2&&text.includes(`+strconv.Quote(terminalOutputCommitSuccessful)+`);})()`), "the complete applied advisory remains exactly once beside the latest banner")
	}
	assert.Equal(t, "false", browser.evaluate(`document.getElementById('notification-bar').textContent.includes('second configuration change')`), "the superseded banner is removed")
	assert.Equal(t, "false", browser.evaluate(`!!document.querySelector('#diff-modal.open')`), "the committed review stays closed")
}

// TestBrowserCleanNotificationBarRefreshes is the negative control: an ordinary
// bar still receives configuration changes, replacing the prior banner without
// retaining stale ordinary content or requiring an informational answer.
func TestBrowserCleanNotificationBarRefreshes(t *testing.T) {
	browser, broker, release := newCommitNotificationBrowser(t, false)
	BroadcastConfigChange(broker, "alice", "first clean change")
	release()
	browser.wait(`window.__commitNotificationEvents===1`, "first clean banner")
	assert.Equal(t, "true", browser.evaluate(`document.getElementById('notification-bar').textContent.includes('first clean change')`))
	BroadcastConfigChange(broker, "bob", "second clean change")
	browser.wait(`window.__commitNotificationEvents===2`, "second clean banner")
	assert.Equal(t, "true", browser.evaluate(`document.getElementById('notification-bar').textContent.includes('second clean change')`))
	assert.Equal(t, "false", browser.evaluate(`document.getElementById('notification-bar').textContent.includes('first clean change')`))
	assert.Equal(t, "1", browser.evaluate(`document.querySelectorAll('#notification-bar .notification-banner').length`))
}

// commitNotificationStreamWriter holds event bytes after the real EventSource
// connects, until the test has observed the HTTP OOB. It changes delivery order,
// never the EventBroker producer, framing, browser EventSource or event handler.
type commitNotificationStreamWriter struct {
	http.ResponseWriter
	flush   http.Flusher
	ctx     context.Context
	release <-chan struct{}
}

func (w commitNotificationStreamWriter) Write(data []byte) (int, error) {
	select {
	case <-w.release:
		return w.ResponseWriter.Write(data)
	case <-w.ctx.Done():
		return 0, context.Cause(w.ctx)
	}
}

func (w commitNotificationStreamWriter) Flush() { w.flush.Flush() }

func newCommitNotificationBrowser(t *testing.T, graphicalHandler bool) (*downloadBrowser, *EventBroker, func()) {
	t.Helper()
	_, err := exec.LookPath("agent-browser")
	require.NoError(t, err, "real commit-warning acceptance requires agent-browser")
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	store := &graphicalCommitCleanupStore{Storage: mgr.store}
	mgr.store = store
	mgr.SetCommitHook(func() error {
		if err := storage.PromoteCandidate(store, mgr.configPath); err != nil {
			return err
		}
		store.promotions++
		store.failNextLock = true
		return nil
	})
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	renderer, err := NewRenderer()
	require.NoError(t, err)
	sessions := NewSessionStore(nil)
	session, err := sessions.createSession("alice", authz.AuthResult{Authorizer: fakeAuthorizer{allowEdit: true}})
	require.NoError(t, err)
	broker := NewEventBroker(2)
	gate := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate) }) }
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		flush, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "test event writer cannot flush", http.StatusInternalServerError)
			return
		}
		broker.ServeHTTP(commitNotificationStreamWriter{ResponseWriter: w, flush: flush, ctx: r.Context(), release: gate}, r)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if err := renderer.RenderLayout(w, LayoutData{Title: "Commit warning acceptance", ActiveUI: uiModeTokenFinder, ChangeCount: mgr.ChangeCount(GetUsernameFromRequest(r))}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.Handle("GET /config/diff", webGoldenDiffHandler(renderer, mgr))
	mux.HandleFunc("GET /config/changes", HandleConfigChanges(mgr, renderer, fakeAuthorizer{allowEdit: true}))
	if graphicalHandler {
		mux.HandleFunc("POST /config/commit", HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, broker, fakeAuthorizer{allowEdit: true}, nil))
	} else {
		// Isolate the existing consumer from the unfixed graphical response.
		// The message, commit and OOB renderer are production implementations.
		mux.HandleFunc("POST /config/commit", func(w http.ResponseWriter, r *http.Request) {
			answer, err := mgr.runCommit(GetUsernameFromRequest(r), contract.CommitRequest{Action: contract.CommitNow}, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !answer.applied {
				http.Error(w, "test commit did not apply: "+answer.message+answer.refusal, http.StatusConflict)
				return
			}
			BroadcastConfigChange(broker, "alice", "committed")
			modal, err := renderer.RenderDiffModal()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeHTML(w, string(modal))
			writeCLINotification(w, renderer, answer.message, "info")
		})
	}
	server := httptest.NewServer(authMiddleware(sessions, &recordingAuthenticator{}, noopRenderer, mux))
	t.Cleanup(func() { release(); broker.Close(); server.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	browser := &downloadBrowser{t: t, ctx: ctx, session: fmt.Sprintf("ze-commit-%d-%x", os.Getpid(), downloadBrowserSequence.Add(1))}
	t.Cleanup(func() {
		cancel()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		browser.ctx = closeCtx
		browser.command("close")
	})
	headers, err := json.Marshal(map[string]string{"Cookie": "ze-session=" + session.Token})
	require.NoError(t, err)
	browser.command("--ignore-https-errors", "set", "headers", string(headers))
	browser.command("open", server.URL+"/")
	browser.wait(`!!window.zeSSE&&!!document.getElementById('commit-review-btn')`, "real graphical page assets")
	browser.evaluate(`window.__commitNotificationEvents=0;window.zeSSE.on('config-change',function(){window.__commitNotificationEvents++;});true`)
	require.Eventually(t, func() bool { return broker.ClientCount() == 1 }, 5*time.Second, 20*time.Millisecond, "native EventSource connected to the actual broker")
	return browser, broker, release
}
