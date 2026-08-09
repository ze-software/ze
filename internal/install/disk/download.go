// Design: docs/architecture/appliance/on-device-installer.md -- download with retry and integrity check

package disk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	retryMax            = 3
	retryDelay          = 5 * time.Second
	metadataTimeout     = 30 * time.Second
	defaultStallTimeout = 60 * time.Second
)

// stallTimeout is the maximum time without receiving any data before the
// streaming download is considered stalled. A var so tests can shorten it.
var stallTimeout = defaultStallTimeout

var (
	metadataClient = &http.Client{Timeout: metadataTimeout}
	streamClient   = &http.Client{} // no timeout; image transfers are multi-GB
)

// downloadToFile downloads url to dest with retry. Returns error after
// retryMax failed attempts.
func downloadToFile(url, dest string) error {
	delay := retryDelay
	for attempt := 1; attempt <= retryMax; attempt++ {
		err := doDownloadToFile(url, dest)
		if err == nil {
			return nil
		}
		slog.Warn("download failed", "url", url, "attempt", attempt, "error", err)
		if attempt < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return fmt.Errorf("download %s failed after %d attempts", url, retryMax)
}

func doDownloadToFile(url, dest string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", url, err)
	}
	resp, err := metadataClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	f, err := os.Create(dest) //nolint:gosec // dest from installer logic
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	closed := false
	defer func() {
		if !closed {
			f.Close() //nolint:errcheck // cleanup on error path
		}
	}()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	closed = true
	return f.Close()
}

// downloadToDisk streams url directly to a block device (or file), computing
// sha256 on the fly. When expectedSHA is non-empty, the hash is compared
// after the transfer completes. A partial transfer (truncated body) is
// caught by the SHA mismatch. Retries on failure.
func downloadToDisk(url, disk, expectedSHA string) error {
	delay := retryDelay
	for attempt := 1; attempt <= retryMax; attempt++ {
		err := doDownloadToDisk(url, disk, expectedSHA)
		if err == nil {
			return nil
		}
		slog.Warn("stream to disk failed", "url", url, "disk", disk, "attempt", attempt, "error", err)
		if attempt < retryMax {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return fmt.Errorf("stream %s to %s failed after %d attempts", url, disk, retryMax)
}

// stallReader wraps an io.Reader with a per-read stall timeout. Each Read
// resets a timer; if no data arrives within the timeout the read returns an
// error. A steady slow transfer (bytes arriving before each deadline) is
// NOT killed; only a complete stall triggers.
type stallReader struct {
	r       io.Reader
	timeout time.Duration
	timer   *time.Timer
}

func newStallReader(r io.Reader, timeout time.Duration) *stallReader {
	return &stallReader{
		r:       r,
		timeout: timeout,
		timer:   time.NewTimer(timeout),
	}
}

func (sr *stallReader) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := sr.r.Read(p)
		ch <- result{n, err}
	}()

	sr.timer.Reset(sr.timeout)
	select {
	case res := <-ch:
		if !sr.timer.Stop() {
			<-sr.timer.C
		}
		return res.n, res.err
	case <-sr.timer.C:
		return 0, fmt.Errorf("download stalled: no data received for %v", sr.timeout)
	}
}

func (sr *stallReader) Close() {
	sr.timer.Stop()
}

func doDownloadToDisk(url, disk, expectedSHA string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", url, err)
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
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
