// Design: docs/architecture/web-interface.md -- role-based access control
// Related: auth.go -- session profiles + request-context plumbing

package web

import (
	"net/http"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/plugin"
)

// webCommandConfigEdit is the representative command the route gate authorizes
// to decide whether a session may reach configuration-editing pages. Any edit
// (isReadOnly=false) command works: the built-in read-only profile denies the
// whole edit section, so a read-only user is rejected regardless of the exact
// command, while an admin (or an unassigned single-admin deployment) is allowed.
const webCommandConfigEdit = webCommandConfigCommit

// authorizerForRequest returns the session-bound authorizer when authentication
// supplied one. The live authorizer remains the fallback for trusted identities.
func authorizerForRequest(r *http.Request, fallback aaa.Authorizer) aaa.Authorizer {
	if sessionAuthorizer := plugin.CallerAuthorizer(r.Context()); sessionAuthorizer != nil {
		return sessionAuthorizer
	}
	return fallback
}

// canEdit reports whether the request's authenticated user may edit config.
func canEdit(r *http.Request, authorizer aaa.Authorizer) bool {
	authorizer = authorizerForRequest(r, authorizer)
	if authorizer == nil {
		return true
	}
	return authorizer.Authorize(GetUsernameFromRequest(r), r.RemoteAddr, webCommandConfigEdit, false)
}

// saveOK builds the out-of-band commit bar one edit answers with. The
// read-only flag comes from canEdit, the gate the page's own commit bar reads.
// One function decides it for every producer, so no call site can forget it.
func saveOK(r *http.Request, authorizer aaa.Authorizer, count int) saveOKData {
	return saveOKData{ChangeCount: count, ReadOnly: !canEdit(r, authorizer)}
}

// RequireEditAuthz wraps next so only users authorized to edit configuration
// reach it. Read-only users receive 403 with a plain message, so edit-only
// pages (config editor, admin console) are hidden from them (AC-1). Fail-open
// semantics (nil authorizer or no assignments) are inherited from CanEdit (R-1).
func RequireEditAuthz(authorizer aaa.Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if canEdit(r, authorizer) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden: this account has read-only access", http.StatusForbidden)
	})
}
