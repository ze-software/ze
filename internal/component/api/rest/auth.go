// Design: docs/architecture/api/architecture.md -- REST transport authentication
// Related: server.go -- RESTServer owns the stable accepted-state provider
//
// Request authentication for the REST transport. Each request obtains one
// immutable accepted generation and carries its authorizer through every
// dispatcher and configuration-session authorization decision.

package rest

import (
	"context"
	"net/http"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/api"
	"github.com/ze-software/ze/internal/core/audit"
)

type callerKeyType struct{}

var callerKey = callerKeyType{}

// Authenticated reports whether the currently published generation gates every
// request. Fail-closed staging is authenticated for exposure checks even though
// it admits no credential.
func (s *RESTServer) Authenticated() bool {
	return s.authentication().Required
}

// withAuth obtains one immutable generation for the whole request. Successful
// authentication stores the generation-carried authorizer with the caller, so
// later publication cannot change the policy used by this request.
func (s *RESTServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		}

		authentication := s.authentication()
		caller := api.CallerIdentity{
			Username:   aaa.ReservedSharedAPIUsername,
			RemoteAddr: r.RemoteAddr,
			Surface:    audit.REST,
			ReadOnly:   !authentication.Required,
		}
		if authentication.Required {
			auth := r.Header.Get("Authorization")
			if authentication.Authenticate == nil {
				s.recordAuthFailure(r, attemptedBearerUser(auth))
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			authenticated, ok := authentication.Authenticate(auth)
			if !ok {
				s.recordAuthFailure(r, attemptedBearerUser(auth))
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			caller = authenticated
			caller.RemoteAddr = r.RemoteAddr
			caller.Surface = audit.REST
		}

		ctx := context.WithValue(r.Context(), callerKey, caller)
		next(w, r.WithContext(ctx))
	}
}

// callerIdentity extracts trusted caller metadata from the request.
func (s *RESTServer) callerIdentity(r *http.Request) api.CallerIdentity {
	if caller, ok := r.Context().Value(callerKey).(api.CallerIdentity); ok {
		return caller
	}
	return api.CallerIdentity{RemoteAddr: r.RemoteAddr, Surface: audit.REST, ReadOnly: true}
}

func (s *RESTServer) requireWriteAccess(w http.ResponseWriter, caller api.CallerIdentity, command string) bool {
	if caller.ReadOnly {
		writeError(w, http.StatusForbidden, "read-only API caller cannot modify configuration")
		return false
	}
	if caller.Authorizer == nil || caller.Authorizer.Authorize(caller.Username, caller.RemoteAddr, command, false) {
		return true
	}
	writeError(w, http.StatusForbidden, "API caller is not authorized to modify configuration")
	return false
}
