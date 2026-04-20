// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server

// OAuth 2.1 resource-server authentication strategy + RFC 9728 protected
// resource metadata handler.
//
// The strategy glues the pieces from Phase D and E together: extract the
// Bearer token, run verifyJWT against the JWKS cache with issuer / audience /
// required-scope expectations, return an Identity on success. Rejections
// map onto RFC 6750 Bearer challenge fields so the 401 response tells the
// client which specific check failed.
//
// The RFC 9728 metadata handler is a tiny static-JSON endpoint that lists
// the authorization servers the client should use. It is served on
// /.well-known/oauth-protected-resource with no authentication (required by
// the spec so clients can discover the AS without an existing token).

package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// oauthAuthenticator implements the authenticator interface for AuthMode=OAuth.
type oauthAuthenticator struct {
	expectedIssuer   string
	expectedAudience string
	requiredScopes   []string
	keys             jwksLookup
	metadataURL      string
	clock            func() time.Time
	leeway           time.Duration
}

// Authenticate parses and validates a Bearer JWT. Errors map to RFC 6750
// invalid_request / invalid_token / insufficient_scope + the specific
// error_description the client needs to fix the offending field.
func (a oauthAuthenticator) Authenticate(r *http.Request) (Identity, *authError) {
	token := extractBearerToken(r)
	if token == "" {
		return Identity{}, a.challengeError("invalid_request", "missing or malformed Authorization header")
	}
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   a.expectedIssuer,
		ExpectedAudience: a.expectedAudience,
		RequiredScopes:   a.requiredScopes,
		Keys:             a.keys,
		Clock:            a.clock,
		Leeway:           a.leeway,
	})
	if err != nil {
		return Identity{}, a.mapVerifyError(err)
	}
	return Identity{Name: res.Subject, Scopes: res.Scopes}, nil
}

// mapVerifyError converts a verifier sentinel into the challenge fields the
// client receives on the 401. The error_description strings come from a
// fixed vocabulary -- never from err.Error() -- so internal infra errors
// (JWKS URLs, fetch timeouts, key-type details) cannot leak into HTTP
// responses.
func (a oauthAuthenticator) mapVerifyError(err error) *authError {
	switch {
	case errors.Is(err, errJWTAlgNone):
		return a.challengeError("invalid_token", "alg=none is not accepted")
	case errors.Is(err, errJWTAlgUnsupported):
		return a.challengeError("invalid_token", "unsupported alg")
	case errors.Is(err, errJWTExpired):
		return a.challengeError("invalid_token", "token expired")
	case errors.Is(err, errJWTNotYetValid):
		return a.challengeError("invalid_token", "token not yet valid")
	case errors.Is(err, errJWTMissingExp):
		return a.challengeError("invalid_token", "missing exp claim")
	case errors.Is(err, errJWTMissingSub):
		return a.challengeError("invalid_token", "missing sub claim")
	case errors.Is(err, errJWTUnsafeSub):
		return a.challengeError("invalid_token", "sub claim contains control characters")
	case errors.Is(err, errJWTIssuerMismatch):
		return a.challengeError("invalid_token", "invalid issuer")
	case errors.Is(err, errJWTAudienceMismatch):
		return a.challengeError("invalid_token", "invalid audience")
	case errors.Is(err, errJWTBadSignature):
		return a.challengeError("invalid_token", "signature does not verify")
	case errors.Is(err, errJWTUnknownKid):
		return a.challengeError("invalid_token", "unknown key")
	case errors.Is(err, errJWTInsufficientScope):
		scope := ""
		if len(a.requiredScopes) > 0 {
			scope = joinScopes(a.requiredScopes)
		}
		return &authError{
			Status:           http.StatusUnauthorized,
			Scheme:           "Bearer",
			Realm:            mcpRealm,
			ErrorCode:        "insufficient_scope",
			ErrorDescription: "required scope missing",
			Scope:            scope,
			ResourceMetadata: a.metadataURL,
		}
	case errors.Is(err, errJWTMalformed):
		return a.challengeError("invalid_request", "malformed token")
	default:
		// Unrecognized verifier error -- surface a generic rejection so
		// internal infrastructure details (JWKS URLs, fetch failure text)
		// never appear in the HTTP response. The underlying error is
		// available to operators via structured logs if needed.
		return a.challengeError("invalid_token", "token rejected")
	}
}

func (a oauthAuthenticator) challengeError(code, desc string) *authError {
	return &authError{
		Status:           http.StatusUnauthorized,
		Scheme:           "Bearer",
		Realm:            mcpRealm,
		ErrorCode:        code,
		ErrorDescription: desc,
		ResourceMetadata: a.metadataURL,
	}
}

// joinScopes joins scopes with single spaces (RFC 6749 scope serialization).
func joinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}

// buildOAuthAuthenticator constructs an oauthAuthenticator from a config +
// key source. Returns an error if the config is not usable (e.g., no
// audience). The AS metadata fetch + JWKS cache wiring is Phase H's job;
// this function only glues the ready pieces.
func buildOAuthAuthenticator(cfg OAuthConfig, keys jwksLookup, metadataURL string) (oauthAuthenticator, error) {
	if cfg.Audience == "" {
		return oauthAuthenticator{}, errors.New("oauth: audience required")
	}
	if cfg.AuthorizationServer == "" {
		return oauthAuthenticator{}, errors.New("oauth: authorization-server required")
	}
	if keys == nil {
		return oauthAuthenticator{}, errors.New("oauth: keys required")
	}
	return oauthAuthenticator{
		expectedIssuer:   cfg.AuthorizationServer,
		expectedAudience: cfg.Audience,
		requiredScopes:   cfg.RequiredScopes,
		keys:             keys,
		metadataURL:      metadataURL,
	}, nil
}

// resourceMetadataDocument is the RFC 9728 JSON body. Built once per
// request from the OAuthConfig; the `resource` field is operator-configured
// (cfg.OAuth.MetadataResource or cfg.OAuth.Audience as fallback) rather
// than derived from the request Host header so a reverse proxy cannot change
// the advertised identity of the resource.
type resourceMetadataDocument struct {
	Resource               string   `json:"-"`
	AuthorizationServers   []string `json:"-"`
	ScopesSupported        []string `json:"-"`
	BearerMethodsSupported []string `json:"-"`
}

// MarshalJSON emits snake_case keys per RFC 9728 while bypassing the
// project's kebab-case JSON tag hook -- MCP OAuth metadata is an external
// spec whose field names are fixed.
func (d resourceMetadataDocument) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"resource":                 d.Resource,
		"authorization_servers":    d.AuthorizationServers,
		"bearer_methods_supported": d.BearerMethodsSupported,
	}
	if len(d.ScopesSupported) > 0 {
		out["scopes_supported"] = d.ScopesSupported
	}
	return json.Marshal(out)
}

// writeResourceMetadata renders the RFC 9728 document for the given
// OAuthConfig. No authentication is required on this endpoint.
func writeResourceMetadata(w http.ResponseWriter, cfg OAuthConfig) {
	resource := cfg.MetadataResource
	if resource == "" {
		resource = cfg.Audience
	}
	doc := resourceMetadataDocument{
		Resource:               resource,
		AuthorizationServers:   []string{cfg.AuthorizationServer},
		ScopesSupported:        cfg.RequiredScopes,
		BearerMethodsSupported: []string{"header"},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=300")
	if _, werr := w.Write(body); werr != nil {
		// Client closed the connection; nothing we can do.
		return
	}
}
