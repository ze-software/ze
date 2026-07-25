// Design: docs/architecture/web-interface.md -- role-based access control
// Related: auth.go -- session profiles + request-context plumbing

package web

import (
	"net/http"

	"github.com/ze-software/ze/internal/component/aaa"
)

// webCommandConfigEdit is the representative command the route gate authorizes
// to decide whether a session may reach configuration-editing pages. Any edit
// (isReadOnly=false) command works: the built-in read-only profile denies the
// whole edit section, so a read-only user is rejected regardless of the exact
// command, while an admin (or an unassigned single-admin deployment) is allowed.
const webCommandConfigEdit = webCommandConfigCommit

// CanEdit reports whether the request's authenticated user may perform
// configuration edits. It consults the same aaa.Authorizer the config-mutation
// handlers use, so page/nav gating and mutation enforcement never diverge. A
// nil authorizer allows all, and an authorizer with no assignments fails open
// (R-1), preserving single-admin deployments. Used by both the route gate and
// nav rendering (to hide gated entries).
func CanEdit(r *http.Request, authorizer aaa.Authorizer) bool {
	if authorizer == nil {
		return true
	}
	return authorizer.Authorize(GetUsernameFromRequest(r), r.RemoteAddr, webCommandConfigEdit, false)
}

// RequireEditAuthz wraps next so only users authorized to edit configuration
// reach it. Read-only users receive 403 with a plain message, so edit-only
// pages (config editor, admin console) are hidden from them (AC-1). Fail-open
// semantics (nil authorizer or no assignments) are inherited from CanEdit (R-1).
func RequireEditAuthz(authorizer aaa.Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if CanEdit(r, authorizer) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden: this account has read-only access", http.StatusForbidden)
	})
}
