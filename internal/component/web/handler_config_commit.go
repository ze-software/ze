// Design: docs/architecture/web-interface.md -- Commit, discard, and pending-change handlers
// Overview: handler_config.go -- Config tree view handlers
// Related: sse.go -- SSE broker for live config change notifications

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// HandleConfigChanges returns a GET handler for /config/changes that returns
// the commit bar HTML reflecting current pending change count.
func HandleConfigChanges(mgr *EditorManager, renderer *Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		count := mgr.ChangeCount(username)
		html := renderer.renderComponent("oob_save_ok", oobSaveOK(saveOKData{ChangeCount: count}))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := w.Write([]byte(html)); writeErr != nil {
			return
		}
	}
}

// handleConfigCommit returns a handler for /config/commit/.
// GET: shows the commit page with a diff of pending changes.
// POST: applies the user's pending changes via mgr.Commit.
//
// On successful commit redirects to /config/edit/ (config root); the
// production path with SSE broadcast and audit is
// HandleConfigCommitWithAuthorizerAndAudit below.
// On conflict, re-renders the commit page with conflict errors.
// HTMX requests receive HX-Redirect instead of an HTTP redirect.
func handleConfigCommit(mgr *EditorManager, renderer *Renderer) http.HandlerFunc {
	return handleConfigCommitWithAuthorizer(mgr, renderer, nil, nil)
}

// handleConfigCommitWithAuthorizer returns a handler for /config/commit/ that
// enforces profile-based RBAC on POST before committing the user's draft.
func handleConfigCommitWithAuthorizer(mgr *EditorManager, renderer *Renderer, broker *EventBroker, authorizer aaa.Authorizer) http.HandlerFunc {
	return HandleConfigCommitWithAuthorizerAndAudit(mgr, renderer, broker, authorizer, nil)
}

// HandleConfigCommitWithAuthorizerAndAudit returns a handler for /config/commit/
// that enforces profile-based RBAC and records successful commits.
func HandleConfigCommitWithAuthorizerAndAudit(mgr *EditorManager, renderer *Renderer, broker *EventBroker, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method == http.MethodGet {
			handleCommitGet(w, mgr, renderer, username)
			return
		}

		if r.Method == http.MethodPost {
			if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigCommit) {
				return
			}
			handleCommitPost(w, r, mgr, renderer, username, broker, recorder)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCommitGet renders the commit page showing a diff of pending changes.
func handleCommitGet(w http.ResponseWriter, mgr *EditorManager, renderer *Renderer, username string) {
	diff, err := mgr.Diff(username)
	if err != nil {
		http.Error(w, fmt.Sprintf("diff: %v", err), http.StatusInternalServerError)
		return
	}

	layoutData := LayoutData{
		Title:    "Commit Changes",
		ActiveUI: uiModeTokenFinder,
	}

	if diff != "" {
		layoutData.NotificationHTML = template.HTML("<pre>" + template.HTMLEscapeString(diff) + "</pre>") //nolint:gosec // escaped
	}

	if err := renderer.RenderLayout(w, layoutData); err != nil {
		http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
	}
}

// commitModalData is the payload for the diff_modal_open fragment. It carries
// the pending diff, a merge-conflict report, or a commit error so each is shown
// inside the open review modal rather than dropped.
type commitModalData struct {
	Diff        string
	ChangeCount int
}

// handleCommitPost applies pending changes and redirects or re-renders on conflict.
// On successful commit (no conflicts), broadcasts a config-change SSE event.
func handleCommitPost(w http.ResponseWriter, r *http.Request, mgr *EditorManager, renderer *Renderer, username string, broker *EventBroker, recorder audit.Recorder) {
	detail, _ := mgr.Diff(username)
	result, err := mgr.Commit(username)
	if err != nil {
		// htmx drops non-2xx bodies, so a bare http.Error leaves the modal open
		// with no feedback (F3). For HX requests, re-render the open modal with
		// the failure text; non-HX clients still receive a 500 with the message.
		var tb textbuf.Buffer
		if r.Header.Get("HX-Request") == htmxRequestTrue {
			modal, renderErr := renderer.RenderDiffModalOpen(
				tb.Str("Commit failed:\n").Err(err).String(), mgr.ChangeCount(username))
			if renderErr != nil {
				http.Error(w, "render error", http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			if _, writeErr := w.Write([]byte(modal)); writeErr != nil {
				return
			}
			return
		}
		http.Error(w, tb.Str("commit: ").Err(err).String(), http.StatusInternalServerError)
		return
	}

	if len(result.Conflicts) > 0 {
		var msg textbuf.Buffer
		msg.Str("Commit conflicts:\n")

		for _, c := range result.Conflicts {
			msg.Str("  ").Str(c.Path).Str(": want ").Quoted(c.MyValue).Str(", other (").Str(c.OtherUser).Str(") has ").Quoted(c.OtherValue).Byte('\n')
		}

		if r.Header.Get("HX-Request") == htmxRequestTrue {
			modal, renderErr := renderer.RenderDiffModalOpen(msg.String(), mgr.ChangeCount(username))
			if renderErr != nil {
				http.Error(w, "render error", http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			if _, writeErr := w.Write([]byte(modal)); writeErr != nil {
				return
			}
			return
		}

		layoutData := LayoutData{
			Title:            "Commit Conflicts",
			NotificationHTML: template.HTML("<pre>" + template.HTMLEscapeString(msg.String()) + "</pre>"), //nolint:gosec // escaped
			ActiveUI:         uiModeTokenFinder,
		}

		if err := renderer.RenderLayout(w, layoutData); err != nil {
			http.Error(w, fmt.Sprintf("render: %v", err), http.StatusInternalServerError)
		}

		return
	}

	recordWebAudit(recorder, r, username, audit.ActionConfigCommit, detail)
	BroadcastConfigChange(broker, username, "committed")

	// Return closed diff modal + empty commit bar. No redirect -- the page
	// underneath the overlay stays unchanged.
	if r.Header.Get("HX-Request") == htmxRequestTrue {
		modal, renderErr := renderer.RenderDiffModal()
		if renderErr != nil {
			http.Error(w, "render error", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		bar := renderer.renderComponent("oob_save_ok", oobSaveOK(saveOKData{ChangeCount: 0}))
		if _, writeErr := w.Write([]byte(modal)); writeErr != nil {
			return
		}
		if _, writeErr := w.Write([]byte(bar)); writeErr != nil {
			return
		}
		return
	}

	htmxRedirect(w, r, "/")
}

// handleConfigDiscard returns a POST handler for /config/discard/.
// It discards the user's pending changes and redirects to /config/edit/.
func handleConfigDiscard(mgr *EditorManager) http.HandlerFunc {
	return handleConfigDiscardWithAuthorizer(mgr, nil)
}

// handleConfigDiscardWithAuthorizer returns a POST handler for /config/discard/
// that enforces profile-based RBAC before discarding the user's draft.
func handleConfigDiscardWithAuthorizer(mgr *EditorManager, authorizer aaa.Authorizer) http.HandlerFunc {
	return HandleConfigDiscardWithAuthorizerAndAudit(mgr, authorizer, nil)
}

// HandleConfigDiscardWithAuthorizerAndAudit returns a POST handler for
// /config/discard/ that enforces profile-based RBAC and records successful discards.
func HandleConfigDiscardWithAuthorizerAndAudit(mgr *EditorManager, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigDiscard) {
			return
		}

		detail, _ := mgr.Diff(username)
		if err := mgr.Discard(username); err != nil {
			http.Error(w, fmt.Sprintf("discard: %v", err), http.StatusInternalServerError)
			return
		}
		recordWebAudit(recorder, r, username, audit.ActionConfigDiscard, detail)

		// Navigate back one level from where the user was.
		target := parentFromCurrentURL(r)
		htmxRedirect(w, r, target)
	}
}

func recordWebAudit(recorder audit.Recorder, r *http.Request, username, action, detail string) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(audit.Entry{
		Actor:      username,
		RemoteAddr: r.RemoteAddr,
		Surface:    audit.Web,
		Action:     action,
		Detail:     detail,
		Outcome:    audit.OutcomeSuccess,
	}); err != nil {
		serverLogger.Warn("audit record failed", "action", action, "user", username, "error", err)
	}
}

// parentFromCurrentURL extracts the parent path from the HTMX HX-Current-URL
// header (or Referer). Used by handlers like discard that have no path in their
// own URL but need to navigate back one level from where the user was.
// Falls back to /config/edit/ if no usable URL is available.
//
// Both headers are supplied by the client, so the result is a redirect target
// only when it stays on this origin. A scheme-relative value ("//host/a/b")
// survives every step below unchanged, and a full URL with no path ("https://host")
// survives the scheme strip; both are rejected by isSameOriginPath rather than
// returned. Anything rejected falls back to configEditPath.
func parentFromCurrentURL(r *http.Request) string {
	current := r.Header.Get("HX-Current-URL")
	if current == "" {
		current = r.Referer()
	}
	if current == "" {
		return configEditPath
	}

	// Strip scheme+host if present (HX-Current-URL is a full URL).
	if idx := strings.Index(current, "://"); idx >= 0 {
		if slash := strings.Index(current[idx+3:], "/"); slash >= 0 {
			current = current[idx+3+slash:]
		}
	}

	// Strip trailing slash, then remove the last segment.
	current = strings.TrimSuffix(current, "/")
	if last := strings.LastIndex(current, "/"); last > 0 {
		if parent := current[:last+1]; isSameOriginPath(parent) {
			return parent
		}
	}

	return configEditPath
}
