// Design: docs/architecture/appliance/self-update.md -- standalone update server with enhanced manifest
// Related: serve_daemon.go -- the same server under the daemon, with start, stop and pause

package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

// updateServeListenDefault is the address both forms listen on when the
// operator names none.
const updateServeListenDefault = ":8080"

type serveManifest struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Paused  bool   `json:"paused,omitempty"`
}

// updateServeContent is what an update server answers, computed once when it
// starts: the running binary, its digest and size, and the paths it serves.
type updateServeContent struct {
	execPath   string
	binaryDir  string
	selfArch   string
	archPath   string
	binaryHash string
	binarySize int64
}

// newUpdateServeContent reads the running binary. Its errors read as the
// command always printed them, after "error: ".
func newUpdateServeContent() (*updateServeContent, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine executable path: %w", err)
	}
	var tb textbuf.Buffer
	selfArch := tb.Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH).String()
	content := &updateServeContent{
		execPath:  execPath,
		binaryDir: filepath.Dir(execPath),
		selfArch:  selfArch,
		archPath:  tb.Reset().Byte('/').Str(selfArch).String(),
	}

	// Compute SHA-256 and size at startup
	content.binaryHash, err = computeBinaryHash(execPath)
	if err != nil {
		return nil, fmt.Errorf("cannot compute binary hash: %w", err)
	}
	info, err := os.Stat(execPath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat binary: %w", err)
	}
	content.binarySize = info.Size()
	return content, nil
}

// filePaused reports the file pause: `update-paused` beside the binary.
func (c *updateServeContent) filePaused() bool {
	_, statErr := os.Stat(filepath.Join(c.binaryDir, "update-paused"))
	return statErr == nil
}

// handler answers the update endpoints. heldPaused is the pause the server's
// owner holds (SIGUSR1 locally, `update serve pause` under the daemon); the
// file pause is ORed with it.
func (c *updateServeContent) handler(heldPaused func() bool) http.Handler {
	isPaused := func() bool {
		return c.filePaused() || heldPaused()
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /version.json", func(w http.ResponseWriter, r *http.Request) {
		reqArch := r.Header.Get("X-Ze-Arch")
		if reqArch != "" && reqArch != c.selfArch {
			http.Error(w, "arch mismatch: serving "+c.selfArch+", got "+reqArch, http.StatusNotFound)
			return
		}

		m := serveManifest{
			Version: zeversion.Release(),
			SHA256:  c.binaryHash,
			Size:    c.binarySize,
			Paused:  isPaused(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")

		data, jsonErr := json.Marshal(m)
		if jsonErr != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		data = append(data, '\n')
		w.Write(data) //nolint:errcheck // best-effort write to HTTP response
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "ze update server\n\n"+ //nolint:errcheck // best-effort write to HTTP response
			"version: "+zeversion.Release()+"\n"+
			"arch:    "+c.selfArch+"\n"+
			"sha256:  "+c.binaryHash+"\n\n"+
			"endpoints:\n"+
			"  GET /version.json\n"+
			"  GET "+c.archPath+"\n"+
			"  GET "+c.archPath+"/sha256\n")
	})

	mux.HandleFunc("GET "+c.archPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=ze")
		http.ServeFile(w, r, c.execPath)
	})

	mux.HandleFunc("GET "+c.archPath+"/sha256", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, c.binaryHash+"\n") //nolint:errcheck // best-effort write to HTTP response
	})
	return mux
}

// runUpdateServe is `ze update serve`: a minimal HTTP server in this process,
// in the foreground until SIGINT or SIGTERM. It serves:
//   - GET /version.json           — enhanced version manifest for update checks
//   - GET /<goos>/<goarch>        — the running binary for download
//   - GET /<goos>/<goarch>/sha256 — hex SHA-256 digest of the binary
//
// Pause mechanisms (ORed):
//   - File: create `update-paused` in the binary's directory
//   - Signal: SIGUSR1 toggles in-memory pause state
//
// The daemon runs the same server with a lifecycle of its own instead
// (serve_daemon.go), because a signal sent to the daemon reaches the daemon.
// Intended for build/release infrastructure, not for routers in production.
func runUpdateServe(validated command.ValidatedArgs) int {
	args := validated.Tokens()
	listen := updateServeListenDefault
	for i := range len(args) - 1 {
		if args[i] == "--listen" {
			listen = args[i+1]
		}
	}

	content, err := newUpdateServeContent()
	if err != nil {
		writeUpdateServeError(err)
		return 1
	}

	// In-memory pause state (toggled by SIGUSR1)
	var signalPaused atomic.Bool

	srv := &http.Server{
		Addr:              listen,
		Handler:           content.handler(signalPaused.Load),
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go watchUpdateServeSignals(sigCh, srv, &signalPaused)

	var tb textbuf.Buffer
	tb.Str("serving update on ").Str(listen).Str(" (version ").Str(zeversion.Release()).
		Str(", binary at ").Str(content.archPath).Str(", sha256 ").Str(content.binaryHash[:12]).Str("...)\n")
	os.Stderr.WriteString(tb.String()) //nolint:errcheck // best-effort operator notice
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		writeUpdateServeError(err)
		return 1
	}
	return 0
}

// writeUpdateServeError prints err as the foreground command always did.
func writeUpdateServeError(err error) {
	var tb textbuf.Buffer
	os.Stderr.WriteString(tb.Str("error: ").Str(err.Error()).Byte('\n').String()) //nolint:errcheck // best-effort operator notice
}

// watchUpdateServeSignals is the signal worker of the foreground server: one
// per process, it reads sigCh until SIGINT or SIGTERM shuts the server down,
// which also ends runUpdateServe and the process with it.
func watchUpdateServeSignals(sigCh <-chan os.Signal, srv *http.Server, signalPaused *atomic.Bool) {
	for sig := range sigCh {
		if sig == syscall.SIGUSR1 {
			state := !signalPaused.Load()
			signalPaused.Store(state)
			if state {
				os.Stderr.WriteString("update serving paused\n") //nolint:errcheck // best-effort operator notice
			} else {
				os.Stderr.WriteString("update serving resumed\n") //nolint:errcheck // best-effort operator notice
			}
			continue
		}
		// SIGINT/SIGTERM: shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(ctx)
		cancel()
		return
	}
}

func computeBinaryHash(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is from os.Executable, not user input
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // best-effort close on read-only hash path

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
