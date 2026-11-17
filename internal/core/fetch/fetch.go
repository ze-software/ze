// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check
// Related: scheme.go -- the scheme table a remote source resolves through

// Package fetch downloads an operator-supplied URL to a file or to a block
// device, with retry, a stall timeout on the stream, a same-origin redirect
// policy and an optional SHA-256 check. The on-device installer and
// `ze init --from` share it.
package fetch

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/redact"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	retryMax            = 3
	defaultRetryDelay   = 5 * time.Second
	metadataTimeout     = 30 * time.Second
	defaultStallTimeout = 60 * time.Second

	// redirectMax is the number of same-origin redirects a fetch follows
	// before it stops. It is the net/http default, written out so a change of
	// that default cannot change a fetch in silence.
	redirectMax = 10

	// fileSizeMax bounds what ToFile writes, so a hostile or misconfigured
	// server cannot fill the disk. ToFile fetches SHA-256 sum files and ZeFS
	// blobs, and one GiB is far above either.
	fileSizeMax = 1 << 30

	// sha256HexLength is the length of a SHA-256 digest written in hex.
	sha256HexLength = 2 * sha256.Size
)

// retryDelay is the wait before the first retry, doubled on each further
// attempt. A var so tests can shorten it, as stallTimeout below is.
var retryDelay = defaultRetryDelay

// stallTimeout is the maximum time without receiving any data before the
// streaming download is considered stalled. A var so tests can shorten it.
var stallTimeout = defaultStallTimeout

var (
	metadataClient = &http.Client{Timeout: metadataTimeout, CheckRedirect: SameOrigin}
	streamClient   = &http.Client{CheckRedirect: SameOrigin} // no timeout; image transfers are multi-GB
)

// SameOrigin is an http.Client CheckRedirect policy: it follows a redirect
// within the scheme and host of the first request and refuses one that leaves
// either, with a *RedirectRefused. The operator named that server and the
// SHA-256 is optional, so a redirect to another host is the one way a fetch
// would install bytes from a server nobody named, and a redirect from https to
// http drops the TLS that authenticates it. It is exported so that a client
// outside this package that fetches what an operator named, the self-update
// client (internal/component/config/system), holds the same policy rather
// than a copy of it.
func SameOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= redirectMax {
		return fmt.Errorf("stopped after %d redirects", redirectMax)
	}
	first := via[0].URL
	if req.URL.Scheme != first.Scheme {
		return newRedirectRefused(req, "scheme "+first.Scheme)
	}
	if req.URL.Host != first.Host {
		return newRedirectRefused(req, "host "+first.Host)
	}
	return nil
}

// RedirectRefused is the error SameOrigin answers for a redirect that leaves
// the scheme or the host of the first request. net/http wraps it in a
// *url.Error, so a caller finds it with errors.As to name the redirect target
// and its own fix. It holds the target only in its redacted form, so no
// rendering of it can publish a credential the Location carried.
//
// Only SameOrigin builds one. It is immutable after construction and safe for
// concurrent use.
type RedirectRefused struct {
	target       string // the redirect's URL, through redact.URL
	targetScheme string // the redirect's scheme, lower case as net/url parses it
	left         string // what the redirect leaves: "scheme https" or "host a.example"
}

// newRedirectRefused builds the refusal of the redirect request redirect.
func newRedirectRefused(redirect *http.Request, left string) *RedirectRefused {
	return &RedirectRefused{
		target:       redact.URL(redirect.URL.String()),
		targetScheme: redirect.URL.Scheme,
		left:         left,
	}
}

// Target answers the URL the redirect named, its userinfo redacted.
func (e *RedirectRefused) Target() string { return e.target }

// TargetScheme answers the scheme of the URL the redirect named.
func (e *RedirectRefused) TargetScheme() string { return e.targetScheme }

// Left answers what the redirect leaves: "scheme <scheme>" or "host <host>"
// of the first request.
func (e *RedirectRefused) Left() string { return e.left }

func (e *RedirectRefused) Error() string {
	return "redirect to " + e.target + " refused: it leaves " + e.left
}

// ToFile downloads url to dest with retry, then checks the SHA-256 of the
// bytes it wrote when expectedSHA is not empty. It returns an error after
// retryMax failed attempts, for a body larger than fileSizeMax, and for a
// digest mismatch. A mismatch is not retried: a truncated body fails the copy
// and is retried there, so a complete body with another digest is another
// file. The caller owns dest and MUST remove it when ToFile fails.
//
// The URL is operator-supplied and may carry userinfo for a private mirror,
// so every site that names it here writes redact.URL and every wrapped
// transport error goes through redact.URLError: an installer console is read
// over a serial line and photographed.
func ToFile(url, dest, expectedSHA string) error {
	return fileFetch(doToFile, url, dest, expectedSHA)
}

// fileFetch is what every scheme's file fetch shares: it refuses a malformed
// digest before any byte is fetched, runs attempt up to retryMax times with a
// doubling delay, then checks the SHA-256 of the bytes written when
// expectedSHA is not empty. attempt writes url to dest, bounded at
// fileSizeMax and with mode 0600, and the caller owns dest.
func fileFetch(attempt func(url, dest string) error, url, dest, expectedSHA string) error {
	if expectedSHA != "" {
		if err := ValidSHA256(expectedSHA); err != nil {
			return err
		}
	}
	if err := toFileRetry(attempt, url, dest); err != nil {
		return err
	}
	if expectedSHA == "" {
		return nil
	}
	return CheckSHA256(dest, expectedSHA)
}

func toFileRetry(attempt func(url, dest string) error, url, dest string) error {
	delay := retryDelay
	var err error
	for try := 1; try <= retryMax; try++ {
		err = attempt(url, dest)
		if err == nil {
			return nil
		}
		slog.Warn("download failed", "url", redact.URL(url), "attempt", try, "error", err)
		if try < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	// The last attempt's error names the reason (a status, a refused
	// redirect, a size bound); a CLI caller never sees the log lines above.
	return fmt.Errorf("download %s failed after %d attempts: %w", redact.URL(url), retryMax, err)
}

func doToFile(url, dest string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", redact.URL(url), redact.URLError(err))
	}
	resp, err := metadataClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", redact.URL(url), redact.URLError(err))
	}
	defer resp.Body.Close() //nolint:errcheck // read-only

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", redact.URL(url), resp.StatusCode)
	}

	// 0600: a fetched blob carries credentials and the SSH host key.
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // dest chosen by the caller
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	closed := false
	defer func() {
		if !closed {
			f.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	written, err := io.Copy(f, io.LimitReader(resp.Body, fileSizeMax+1))
	if err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if written > fileSizeMax {
		return fmt.Errorf("GET %s: body larger than %d bytes", redact.URL(url), fileSizeMax)
	}
	closed = true
	return f.Close()
}

// ValidSHA256 refuses a digest that is not exactly 64 hexadecimal digits, so
// a mistyped digest is named before any byte is fetched.
func ValidSHA256(digest string) error {
	if len(digest) != sha256HexLength {
		return fmt.Errorf("sha256 %q: want %d hexadecimal digits, got %d characters", digest, sha256HexLength, len(digest))
	}
	for _, c := range digest {
		if strings.ContainsRune("0123456789abcdefABCDEF", c) {
			continue
		}
		return fmt.Errorf("sha256 %q: %q is not a hexadecimal digit", digest, c)
	}
	return nil
}

// CheckSHA256 compares the SHA-256 of the file at path with expectedSHA, in
// either letter case. The error names the path and both digests.
func CheckSHA256(path, expectedSHA string) error {
	if err := ValidSHA256(expectedSHA); err != nil {
		return err
	}
	f, err := os.Open(path) //nolint:gosec // path chosen by the caller
	if err != nil {
		return err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	if err := errors.Join(copyErr, f.Close()); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	actual := textbuf.StringHex(h.Sum(nil))
	if !strings.EqualFold(actual, expectedSHA) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, actual, expectedSHA)
	}
	return nil
}

// ToDisk streams url directly to a block device (or file), computing
// sha256 on the fly. When expectedSHA is non-empty, the hash is compared
// after the transfer completes. A partial transfer (truncated body) is
// caught by the SHA mismatch. Retries on failure.
func ToDisk(url, disk, expectedSHA string) error {
	delay := retryDelay
	var err error
	for attempt := 1; attempt <= retryMax; attempt++ {
		err = doToDisk(url, disk, expectedSHA)
		if err == nil {
			return nil
		}
		slog.Warn("stream to disk failed", "url", redact.URL(url), "disk", disk, "attempt", attempt, "error", err)
		if attempt < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return fmt.Errorf("stream %s to %s failed after %d attempts: %w", redact.URL(url), disk, retryMax, err)
}

// ErrStalled is the error a StallReader's Read wraps once its timer closed
// the body, so a caller can tell a stall from another read failure.
var ErrStalled = errors.New("download stalled")

// StallReader closes a response body when no Read returns within the
// timeout, so a stalled transfer fails instead of hanging. Each Read re-arms
// the timer, so a steady slow transfer (bytes arriving before each deadline)
// is NOT killed; only a complete stall triggers. The timer's function runs
// on the runtime's timer goroutine, and a stalled Read returns because
// closing the body unblocks it. Close MUST be called once the copy ends.
// Not safe for concurrent Reads. Exported for the container registry
// client, which bounds each registry answer the same way.
type StallReader struct {
	body    io.ReadCloser
	timeout time.Duration
	timer   *time.Timer
	stalled atomic.Bool
}

// NewStallReader arms the timer and wraps body. The caller MUST call Close
// once it stops reading.
func NewStallReader(body io.ReadCloser, timeout time.Duration) *StallReader {
	sr := &StallReader{body: body, timeout: timeout}
	sr.timer = time.AfterFunc(timeout, sr.stall)
	return sr
}

func (sr *StallReader) stall() {
	sr.stalled.Store(true)
	sr.body.Close() //nolint:errcheck // unblocks the stalled Read; the Read reports the stall
}

// Read reads from the body and re-arms the timer. Once the timer closed the
// body, the error wraps ErrStalled.
func (sr *StallReader) Read(p []byte) (int, error) {
	sr.timer.Reset(sr.timeout)
	n, err := sr.body.Read(p)
	if err != nil && sr.stalled.Load() {
		return n, fmt.Errorf("%w: no data received for %v", ErrStalled, sr.timeout)
	}
	return n, err
}

// Close stops the timer. It MUST be called once the copy ends; it does not
// close the body, which stays the caller's.
func (sr *StallReader) Close() {
	sr.timer.Stop()
}

func doToDisk(url, disk, expectedSHA string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", redact.URL(url), redact.URLError(err))
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", redact.URL(url), redact.URLError(err))
	}
	defer resp.Body.Close() //nolint:errcheck // read-only

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", redact.URL(url), resp.StatusCode)
	}

	f, err := os.OpenFile(disk, os.O_WRONLY, 0) //nolint:gosec // disk from validated cmdline
	if err != nil {
		return fmt.Errorf("open %s: %w", disk, err)
	}
	closed := false
	defer func() {
		if !closed {
			f.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	sr := NewStallReader(resp.Body, stallTimeout)
	defer sr.Close()

	var w io.Writer = f
	var h hash.Hash
	if expectedSHA != "" {
		h = sha256.New()
		w = io.MultiWriter(f, h)
	}

	if _, err := io.Copy(w, sr); err != nil {
		return fmt.Errorf("write to %s: %w", disk, err)
	}

	closed = true
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", disk, err)
	}

	if h != nil {
		actual := textbuf.StringHex(h.Sum(nil))
		if actual != expectedSHA {
			return fmt.Errorf("sha256 mismatch: got %s, want %s", actual, expectedSHA)
		}
	}

	return nil
}
