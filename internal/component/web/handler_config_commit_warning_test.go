// Design: docs/architecture/web-interface.md -- Applied graphical commit warnings
// Related: editor_commit_window.go -- runCommit preserves the editor's advisories

package web

import (
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/audit"
)

const graphicalCommitCleanupProblem = "configuration storage refused the <cleanup> lock; ask an administrator to check configuration storage access"

// graphicalCommitCleanupStore refuses exactly the first lock after candidate
// promotion. Staging, publication, reads and later locks use the real store.
// It is used synchronously by the request and has no concurrent callers.
type graphicalCommitCleanupStore struct {
	storage.Storage
	failNextLock bool
	failures     int
	promotions   int
}

func (s *graphicalCommitCleanupStore) AcquireLock(path string) (storage.WriteGuard, error) {
	if s.failNextLock {
		s.failNextLock = false
		s.failures++
		return nil, errors.New(graphicalCommitCleanupProblem)
	}
	return s.Storage.AcquireLock(path)
}

// TestGraphicalCommitAppliedWarningVisible proves the real post-publication
// cleanup failure remains visible as an advisory on the successful HTTP answer.
// METHOD: promote a real candidate, fail only its cleanup lock, and check both
// graphical transports together with the committed readback, audit and SSE.
// MUTATION: drop answer.message from the applied handler answer; both transports
// lose the advisory while the publication, success audit and SSE still occur.
func TestGraphicalCommitAppliedWarningVisible(t *testing.T) {
	checkGraphicalCommitAppliedResponse(t, true)
}

// TestGraphicalCommitCleanSuccessPreserved is the positive control: a clean
// commit still publishes once, clears its pending changes, closes the HX modal
// or redirects a normal browser, and emits one success audit and SSE event.
func TestGraphicalCommitCleanSuccessPreserved(t *testing.T) {
	checkGraphicalCommitAppliedResponse(t, false)
}

func checkGraphicalCommitAppliedResponse(t *testing.T, failCleanup bool) {
	t.Helper()
	for _, transport := range []struct {
		name string
		hx   bool
	}{
		{name: "HTMX", hx: true},
		{name: "normal browser"},
	} {
		t.Run(transport.name, func(t *testing.T) {
			mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
			store := &graphicalCommitCleanupStore{Storage: mgr.store}
			mgr.store = store
			mgr.SetCommitHook(func() error {
				if err := storage.PromoteCandidate(store, mgr.configPath); err != nil {
					return err
				}
				store.promotions++
				store.failNextLock = failCleanup
				return nil
			})
			renderer, err := NewRenderer()
			require.NoError(t, err)
			recorder, err := audit.NewMemory(100)
			require.NoError(t, err)
			broker := NewEventBroker(2)
			t.Cleanup(broker.Close)
			observer := broker.Subscribe("bob")
			require.NotNil(t, observer)
			require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))

			request := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
			if transport.hx {
				request.Header.Set("HX-Request", htmxRequestTrue)
			}
			response := httptest.NewRecorder()
			handler := HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, broker, adminWebAuthorizer(), recorder)
			handler.ServeHTTP(response, request)

			require.Equal(t, 1, store.promotions, "feedback must not retry the applied commit")
			requireCommitted(t, mgr, "10.0.0.9", "10.0.0.250")
			assert.Contains(t, mgr.ContentAtPath("alice", []string{"bgp"}), "10.0.0.9", "the current editor reads the committed tree")
			entries := recorder.Query(audit.Filter{Action: audit.ActionConfigCommit})
			require.Len(t, entries, 1)
			assert.Equal(t, "alice", entries[0].Actor)
			assert.Equal(t, audit.OutcomeSuccess, entries[0].Outcome)
			assert.Equal(t, audit.Web, entries[0].Surface)
			assert.Contains(t, entries[0].Detail, "router-id")
			require.Len(t, observer.ch, 1, "one published commit emits one change event")
			event := <-observer.ch
			assert.Equal(t, "config-change", event.eventType)
			assert.Contains(t, event.data, "Config changed by alice: committed")
			assert.NotContains(t, event.data, "cleanup", "private cleanup detail is not broadcast to other users")

			body := response.Body.String()
			assert.Empty(t, response.Header().Get("HX-Redirect"))
			assert.NotContains(t, body, "Commit failed", "a committed candidate must not be presented as failed")
			assert.NotContains(t, body, `class="diff-modal open"`, "the applied commit must not leave a retry modal open")
			if transport.hx {
				assert.Equal(t, http.StatusOK, response.Code)
				assert.Contains(t, body, `id="diff-modal" class="diff-modal"`)
				assert.Contains(t, body, `id="commit-bar"`)
			}
			if failCleanup {
				assert.Equal(t, 1, store.failures, "the warning comes from actual editor cleanup")
				assert.Equal(t, http.StatusOK, response.Code, "warning success is rendered, never redirected away")
				assert.Empty(t, response.Header().Get("Location"))
				assert.Contains(t, body, terminalOutputCommitSuccessful)
				assert.Equal(t, 1, strings.Count(body, html.EscapeString(graphicalCommitCleanupProblem)), "the complete actionable advisory appears once")
				assert.NotContains(t, body, "<cleanup>", "the advisory is escaped by templ")
				if transport.hx {
					assert.Contains(t, body, `id="notification-bar" hx-swap-oob="true"`, "the existing notification surface receives the advisory")
				}
				return
			}
			assert.Zero(t, store.failures)
			assert.Zero(t, mgr.ChangeCount("alice"))
			assert.NotContains(t, body, "warning:")
			if !transport.hx {
				assert.Equal(t, http.StatusSeeOther, response.Code)
				assert.Equal(t, "/", response.Header().Get("Location"))
			}
		})
	}
}
