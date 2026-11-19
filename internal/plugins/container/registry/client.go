// Design: docs/architecture/container/image-store.md -- the registry requests: transport, redirects, retries, the mirror
// Related: reference.go -- the references a client fetches
// Related: auth.go -- the challenge and the token
// Related: manifest.go -- the manifest, the index and the platform
// Related: blob.go -- the blobs, checked as they arrive
// Related: errors.go -- the failure classes

package registry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/core/redact"
)

// Transport is how a registry and its mirror are reached. There is no
// transport without certificate checks.
type Transport uint8

const (
	// TransportUnspecified is no choice. New refuses it.
	TransportUnspecified Transport = iota
	// TransportHTTPS is verified TLS, the default an operator gets.
	TransportHTTPS
	// TransportHTTP is plain HTTP, written by the operator for one registry.
	TransportHTTP
)

// scheme answers the URL scheme of t, and the empty string for no choice.
func (t Transport) scheme() string {
	switch t {
	case TransportHTTPS:
		return schemeHTTPS
	case TransportHTTP:
		return schemeHTTP
	case TransportUnspecified:
		return ""
	}
	return ""
}

const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// Architecture is a platform Ze runs on. The operating system is always
// Linux, so the architecture is the whole platform the client selects.
type Architecture uint8

const (
	// ArchitectureUnspecified is no choice. New refuses it.
	ArchitectureUnspecified Architecture = iota
	ArchitectureAMD64
	ArchitectureARM64
)

// String answers the name an index and an image configuration write.
func (a Architecture) String() string {
	switch a {
	case ArchitectureAMD64:
		return "amd64"
	case ArchitectureARM64:
		return "arm64"
	case ArchitectureUnspecified:
		return "unspecified"
	}
	return "unknown"
}

// RunningArchitecture answers the architecture of this binary, and an error
// on one Ze does not run containers on.
func RunningArchitecture() (Architecture, error) {
	switch runtime.GOARCH {
	case "amd64":
		return ArchitectureAMD64, nil
	case "arm64":
		return ArchitectureARM64, nil
	}
	return ArchitectureUnspecified, errors.New("containers run on amd64 and arm64, and this binary is " + runtime.GOARCH)
}

// Credentials are a registry's user name and password. They print as a
// placeholder under every verb, so no format of a Config or a Client shows
// them; only the authorization header sent to the realm or the registry
// carries them.
type Credentials struct {
	username string
	password string
}

// NewCredentials builds the credentials the configuration holds.
func NewCredentials(username, password string) Credentials {
	return Credentials{username: username, password: password}
}

// IsZero answers whether there are no credentials: an anonymous client.
func (c Credentials) IsZero() bool { return c.username == "" && c.password == "" }

// String answers a placeholder, never the credentials.
func (c Credentials) String() string { return redact.Placeholder }

// GoString answers a placeholder for %#v too.
func (c Credentials) GoString() string { return redact.Placeholder }

// basic answers the basic authorization header value.
func (c Credentials) basic() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password))
}

// Config is what one client needs: the registry, how it is reached, the
// credentials and the trust anchor.
type Config struct {
	// Registry is the registry host as a reference completes it.
	Registry string
	// Transport applies to the registry and to its mirror.
	Transport Transport
	// Credentials go to the registry's realm, or to the registry under a
	// basic challenge. Never to the mirror. The zero value is anonymous.
	Credentials Credentials
	// Mirror is host[:port] of the one mirror, asked first, or empty.
	Mirror string
	// MirrorPath is the path the mirror serves the registry protocol under,
	// placed before the /v2/ path of every request to it. It starts with a
	// slash; empty and a single slash are both the mirror's root. New
	// refuses one set with no Mirror.
	MirrorPath string
	// TrustAnchor replaces the system roots when it is not nil: a registry
	// that names an anchor is trusted through that anchor only.
	TrustAnchor *x509.CertPool
	// Architecture is the platform an index is resolved to.
	Architecture Architecture
}

// Bounds and timeouts of the requests (the design's "Registry requests").
const (
	attemptMax          = 3
	retryDelayDefault   = time.Second
	stallTimeoutDefault = 60 * time.Second
	redirectMax         = 10

	dialTimeout           = 30 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 30 * time.Second
	idleConnTimeout       = 90 * time.Second
	idleConnsMax          = 4

	// errorBodySizeMax bounds what is read of an answer that is not 200, to
	// find its error code.
	errorBodySizeMax = 4 << 10
)

// origin is one server a request may go to, the registry or its mirror,
// with the authorization obtained for it. The authorization is sent to that
// origin's exact host and port only.
type origin struct {
	host   string
	scheme string
	// prefix is the path before the /v2/ path of every request: empty for
	// the registry and for a mirror served at its root.
	prefix        string
	credentials   Credentials
	authorization string
}

// key answers the host:port the origin's authorization belongs to.
func (o *origin) key() string {
	return canonicalHost(&url.URL{Scheme: o.scheme, Host: o.host})
}

// Client is a pull-only client of one registry. Not safe for concurrent use:
// the store's one fetch worker owns it. Close MUST be called when it is done.
type Client struct {
	registryName string
	architecture Architecture
	registry     *origin
	mirror       *origin
	http         *http.Client
	transport    *http.Transport
	// retryDelay and stallTimeout are fields so the package's tests can
	// shorten them.
	retryDelay    time.Duration
	stallTimeout  time.Duration
	mirrorFailure error
}

// New builds a client for config, refusing a config that names no valid
// registry, transport or architecture.
func New(config Config) (*Client, error) {
	if err := checkHost(config.Registry); err != nil {
		return nil, fmt.Errorf("registry client: registry: %w", err)
	}
	scheme := config.Transport.scheme()
	if scheme == "" {
		return nil, errors.New("registry client: no transport chosen")
	}
	if config.Architecture == ArchitectureUnspecified {
		return nil, errors.New("registry client: no architecture chosen")
	}
	transport := &http.Transport{
		// No proxy: the configuration names no proxy, and an environment
		// variable is not a Ze setting.
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: dialTimeout}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.TrustAnchor},
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleConnTimeout,
		MaxIdleConns:          idleConnsMax,
		ForceAttemptHTTP2:     true,
	}
	c := &Client{
		registryName: config.Registry,
		architecture: config.Architecture,
		registry:     &origin{host: endpointHost(config.Registry), scheme: scheme, credentials: config.Credentials},
		transport:    transport,
		// Redirects are followed by follow, which owns the rule for the
		// authorization header, so the standard client follows none.
		http: &http.Client{
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		retryDelay:   retryDelayDefault,
		stallTimeout: stallTimeoutDefault,
	}
	if config.Mirror != "" {
		if err := checkHost(config.Mirror); err != nil {
			return nil, fmt.Errorf("registry client: mirror: %w", err)
		}
		prefix, err := mirrorPrefix(config.MirrorPath)
		if err != nil {
			return nil, err
		}
		c.mirror = &origin{host: config.Mirror, scheme: scheme, prefix: prefix}
		return c, nil
	}
	if config.MirrorPath != "" {
		return nil, errors.New("registry client: a mirror path is set and no mirror")
	}
	return c, nil
}

// mirrorPrefix answers what a mirror's path puts before the /v2/ path of a
// request: the path without its trailing slash, so the root is no prefix. A
// path that does not start with a slash, or that holds a query, a fragment
// or a space, is refused: it would change which resource is asked. A path
// that starts with two slashes, or holds a backslash, is refused too: a URL
// reader takes "//host" or "/\host" as naming another host, so neither is
// ever a mirror's path.
func mirrorPrefix(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("registry client: mirror path %q does not start with a slash", shown(path))
	}
	if strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("registry client: mirror path %q starts with two slashes", shown(path))
	}
	if strings.Contains(path, `\`) {
		return "", fmt.Errorf("registry client: mirror path %q holds a backslash", shown(path))
	}
	if strings.ContainsAny(path, "?# ") {
		return "", fmt.Errorf("registry client: mirror path %q holds a query, a fragment or a space", shown(path))
	}
	return strings.TrimSuffix(path, "/"), nil
}

// Close releases the idle connections.
func (c *Client) Close() {
	c.transport.CloseIdleConnections()
}

// LastMirrorFailure answers why the mirror last failed a request that the
// registry then answered, or nil. The fetch worker reports it: the fallback
// to the registry is the design, and the mirror's failure is still news to
// the operator.
func (c *Client) LastMirrorFailure() error {
	return c.mirrorFailure
}

// fromMirrorThenRegistry runs request against the mirror once, and against
// the registry when the mirror fails, with the registry's attempts retried.
// The mirror is not retried: the registry is its fallback.
func (c *Client) fromMirrorThenRegistry(ctx context.Context, request func(o *origin) error) error {
	if c.mirror != nil {
		err := request(c.mirror)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		c.mirrorFailure = err
	}
	return c.withRetry(ctx, func() error { return request(c.registry) })
}

// withRetry runs attempt up to attemptMax times while it fails with an
// Error marked for retry, doubling the delay between attempts.
func (c *Client) withRetry(ctx context.Context, attempt func() error) error {
	delay := c.retryDelay
	for n := 1; ; n++ {
		err := attempt()
		if !retryable(err) {
			return err
		}
		if n == attemptMax {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return stopped(ctx)
		case <-timer.C:
		}
		delay *= 2
	}
}

// retryable answers whether err is an Error another attempt may not meet.
// No error, and an error of another type, are not retried.
func retryable(err error) bool {
	var failure *Error
	if !errors.As(err, &failure) {
		return false
	}
	return failure.retry
}

// stopped answers the error of a request whose context ended: the caller
// went away or the command's bound passed. It wraps the context's cause.
func stopped(ctx context.Context) error {
	return fmt.Errorf("registry request stopped: %w", context.Cause(ctx))
}

// answer is a 200 answer whose body reads under the stall reader. close
// MUST be called once the body is read.
type answer struct {
	response *http.Response
	body     *fetch.StallReader
}

func (a *answer) close() {
	a.body.Close()
	a.response.Body.Close() //nolint:errcheck // read-only body
}

// get asks o for path. It answers one authentication challenge, follows the
// redirects by the client's rule, and classifies an answer that is not 200.
func (c *Client) get(ctx context.Context, o *origin, path, accept, scope string) (*answer, error) {
	target := o.scheme + "://" + o.host + o.prefix + path
	response, err := c.follow(ctx, target, accept, o.key(), o.authorization)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusUnauthorized {
		challenge := response.Header.Get("WWW-Authenticate")
		discard(response)
		if err := c.answerChallenge(ctx, o, challenge, scope); err != nil {
			return nil, err
		}
		response, err = c.follow(ctx, target, accept, o.key(), o.authorization) //nolint:bodyclose // the answer returned below owns the body, and its caller closes it
		if err != nil {
			return nil, err
		}
	}
	if response.StatusCode != http.StatusOK {
		failure := statusFailure(o.host+path, response)
		discard(response)
		return nil, failure
	}
	return &answer{response: response, body: fetch.NewStallReader(response.Body, c.stallTimeout)}, nil
}

// follow sends one GET and follows its redirects, at most redirectMax of
// them. The authorization goes only to the host and port named by authKey,
// the one it was obtained for, so a blob redirect to another host carries
// none. A redirect from HTTPS to HTTP is refused.
func (c *Client) follow(ctx context.Context, target, accept, authKey, authorization string) (*http.Response, error) {
	for redirects := 0; ; redirects++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
		if err != nil {
			return nil, &Error{failure: FailureUnreachable, problem: "the request to " + redact.URL(target) + " cannot be built", why: "the address is not a URL", cause: redact.URLError(err)}
		}
		if accept != "" {
			request.Header.Set("Accept", accept)
		}
		if authorization != "" && canonicalHost(request.URL) == authKey {
			request.Header.Set("Authorization", authorization)
		}
		response, err := c.http.Do(request)
		if err != nil {
			return nil, transportFailure(ctx, request.URL, err)
		}
		if !isRedirect(response.StatusCode) {
			return response, nil
		}
		location, err := response.Location()
		discard(response)
		if err != nil {
			return nil, failed(FailureUnreachable, request.URL.Host+" answered a redirect", "the redirect names no location Ze can follow")
		}
		if redirects == redirectMax {
			return nil, failed(FailureUnreachable, request.URL.Host+" answered too many redirects", "Ze follows at most "+strconv.Itoa(redirectMax)+" redirects for one request")
		}
		if request.URL.Scheme == schemeHTTPS && location.Scheme != schemeHTTPS {
			return nil, failed(FailureUnreachable, request.URL.Host+" redirected to "+location.Scheme+"://"+location.Host, "a redirect from HTTPS to another scheme would drop the TLS the registry is reached over")
		}
		if location.Scheme != schemeHTTP && location.Scheme != schemeHTTPS {
			return nil, failed(FailureUnreachable, request.URL.Host+" redirected to a "+location.Scheme+" address", "a registry is reached over HTTPS or HTTP only")
		}
		target = location.String()
	}
}

// isRedirect answers whether status is a redirect with a location.
func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// canonicalHost answers host:port of u, the port written out from the
// scheme when u names none, so two spellings of one server compare equal.
func canonicalHost(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == schemeHTTP {
			port = "80"
		}
	}
	return net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

// transportFailure classifies an error the HTTP client answered.
func transportFailure(ctx context.Context, target *url.URL, err error) error {
	if ctx.Err() != nil {
		return stopped(ctx)
	}
	cause := redact.URLError(err)
	problem := target.Host + " could not be reached"
	untrusted := &Error{failure: FailureUnreachable, problem: problem, why: "the registry's certificate does not chain to a trusted authority", cause: cause}
	if _, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		return untrusted
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return untrusted
	}
	// net/http answers ErrSchemeMismatch when a TLS handshake read the start
	// of an HTTP answer, and a RecordHeaderError for other plain bytes.
	plain := &Error{failure: FailureUnreachable, problem: problem, why: "the server answered without TLS, and the registry's transport is HTTPS", cause: cause}
	if errors.Is(err, http.ErrSchemeMismatch) {
		return plain
	}
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return plain
	}
	return &Error{failure: FailureUnreachable, problem: problem, why: "the connection failed", cause: cause, retry: true}
}

// statusFailure classifies an answer that is neither 200 nor a redirect. The
// registry's own error text is reduced to its error code.
func statusFailure(what string, response *http.Response) *Error {
	status := "status " + strconv.Itoa(response.StatusCode)
	if code := errorCode(response.Body); code != "" {
		status += " " + code
	}
	switch {
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		return failed(FailureAuthRefused, what+" refused the access", "the registry answered "+status)
	case response.StatusCode == http.StatusNotFound:
		return failed(FailureNotFound, what+" does not exist", "the registry answered "+status)
	case response.StatusCode >= http.StatusInternalServerError:
		return &Error{failure: FailureUnreachable, problem: what + " failed", why: "the registry answered " + status, retry: true}
	}
	return failed(FailureUnreachable, what+" failed", "the registry answered "+status)
}

// errorCode answers the first error code of a distribution error body, or
// the empty string.
func errorCode(body io.Reader) string {
	limited, err := io.ReadAll(io.LimitReader(body, errorBodySizeMax))
	if err != nil {
		return ""
	}
	var answer struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(limited, &answer); err != nil {
		return "" // The body is not the registry's error form, so it names no code.
	}
	if len(answer.Errors) == 0 {
		return ""
	}
	return registryErrorCode(answer.Errors[0].Code)
}

// discard reads a little of a body that is not used, so the connection can
// be reused, and closes it.
func discard(response *http.Response) {
	io.Copy(io.Discard, io.LimitReader(response.Body, errorBodySizeMax)) //nolint:errcheck // best effort before close
	response.Body.Close()                                                //nolint:errcheck // read-only body
}

// readBounded reads all of r, refusing more than sizeMax bytes: the second
// answer is true when r held more.
func readBounded(r io.Reader, sizeMax int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(r, sizeMax+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > sizeMax {
		return nil, true, nil
	}
	return data, false, nil
}
