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
	metadataClient = &http.Client{Timeout: metadataTimeout, CheckRedirect: sameOrigin}
	streamClient   = &http.Client{CheckRedirect: sameOrigin} // no timeout; image transfers are multi-GB
)

// sameOrigin refuses a redirect that leaves the scheme and host of the first
// request. The operator named that server and the SHA-256 is optional, so a
// redirect to another host is the one way a fetch would install bytes from a
// server nobody named, and a redirect from https to http drops the TLS that
// authenticates it.
func sameOrigin(req *http.Request, via []*http.Request) error {
	if len(via) >= redirectMax {
		return fmt.Errorf("stopped after %d redirects", redirectMax)
	}
	first := via[0].URL
	if req.URL.Scheme != first.Scheme {
		return fmt.Errorf("redirect to %s refused: it leaves scheme %s", redact.URL(req.URL.String()), first.Scheme)
	}
	if req.URL.Host != first.Host {
		return fmt.Errorf("redirect to %s refused: it leaves host %s", redact.URL(req.URL.String()), first.Host)
	}
	return nil
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
	if expectedSHA != "" {
		if err := ValidSHA256(expectedSHA); err != nil {
			return err
		}
	}
	if err := toFileRetry(url, dest); err != nil {
		return err
	}
	if expectedSHA == "" {
		return nil
	}
	return CheckSHA256(dest, expectedSHA)
}

func toFileRetry(url, dest string) error {
	delay := retryDelay
	for attempt := 1; attempt <= retryMax; attempt++ {
		err := doToFile(url, dest)
		if err == nil {
			return nil
		}
		slog.Warn("download failed", "url", redact.URL(url), "attempt", attempt, "error", err)
		if attempt < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return fmt.Errorf("download %s failed after %d attempts", redact.URL(url), retryMax)
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
	for attempt := 1; attempt <= retryMax; attempt++ {
		err := doToDisk(url, disk, expectedSHA)
		if err == nil {
			return nil
		}
		slog.Warn("stream to disk failed", "url", redact.URL(url), "disk", disk, "attempt", attempt, "error", err)
		if attempt < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return fmt.Errorf("stream %s to %s failed after %d attempts", redact.URL(url), disk, retryMax)
}

// stallReader closes a response body when no Read returns within the
// timeout, so a stalled transfer fails instead of hanging. Each Read re-arms
// the timer, so a steady slow transfer (bytes arriving before each deadline)
// is NOT killed; only a complete stall triggers. The timer's function runs
// on the runtime's timer goroutine, and a stalled Read returns because
// closing the body unblocks it. Close MUST be called once the copy ends.
// Not safe for concurrent Reads.
type stallReader struct {
	body    io.ReadCloser
	timeout time.Duration
	timer   *time.Timer
	stalled atomic.Bool
}

func newStallReader(body io.ReadCloser, timeout time.Duration) *stallReader {
	sr := &stallReader{body: body, timeout: timeout}
	sr.timer = time.AfterFunc(timeout, sr.stall)
	return sr
}

func (sr *stallReader) stall() {
	sr.stalled.Store(true)
	sr.body.Close() //nolint:errcheck // unblocks the stalled Read; the Read reports the stall
}

func (sr *stallReader) Read(p []byte) (int, error) {
	sr.timer.Reset(sr.timeout)
	n, err := sr.body.Read(p)
	if err != nil && sr.stalled.Load() {
		return n, fmt.Errorf("download stalled: no data received for %v", sr.timeout)
	}
	return n, err
}

func (sr *stallReader) Close() {
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

	sr := newStallReader(resp.Body, stallTimeout)
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
