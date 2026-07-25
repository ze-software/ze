// Design: plan/learned/748-cpe-6-self-update.md — standalone update server with enhanced manifest

//go:build ze_core

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

type serveManifest struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Paused  bool   `json:"paused,omitempty"`
}

// runUpdateServe starts a minimal HTTP server that serves:
//   - GET /version.json           — enhanced version manifest for update checks
//   - GET /<goos>/<goarch>        — the running binary for download
//   - GET /<goos>/<goarch>/sha256 — hex SHA-256 digest of the binary
//
// Pause mechanisms (ORed):
//   - File: create `update-paused` in the binary's directory
//   - Signal: SIGUSR1 toggles in-memory pause state
//
// Intended for build/release infrastructure, not for routers in production.
func runUpdateServe(args []string) int {
	listen := ":8080"
	for i := range len(args) - 1 {
		if args[i] == "--listen" {
			listen = args[i+1]
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine executable path: %v\n", err)
		return 1
	}

	binaryDir := filepath.Dir(execPath)
	var tb textbuf.Buffer
	selfArch := tb.Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH).String()
	archPath := tb.Reset().Byte('/').Str(selfArch).String()

	// Compute SHA-256 and size at startup
	binaryHash, err := computeBinaryHash(execPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot compute binary hash: %v\n", err)
		return 1
	}

	info, err := os.Stat(execPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot stat binary: %v\n", err)
		return 1
	}
	binarySize := info.Size()

	// In-memory pause state (toggled by SIGUSR1)
	var (
		pauseMu      sync.RWMutex
		signalPaused bool
	)

	isPaused := func() bool {
		// File-based pause
		_, statErr := os.Stat(filepath.Join(binaryDir, "update-paused"))
		filePaused := statErr == nil

		// Signal-based pause
		pauseMu.RLock()
		sigPaused := signalPaused
		pauseMu.RUnlock()

		return filePaused || sigPaused
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /version.json", func(w http.ResponseWriter, r *http.Request) {
		reqArch := r.Header.Get("X-Ze-Arch")
		if reqArch != "" && reqArch != selfArch {
			http.Error(w, "arch mismatch: serving "+selfArch+", got "+reqArch, http.StatusNotFound)
			return
		}

		m := serveManifest{
			Version: zeversion.Release(),
			SHA256:  binaryHash,
			Size:    binarySize,
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
			"arch:    "+selfArch+"\n"+
			"sha256:  "+binaryHash+"\n\n"+
			"endpoints:\n"+
			"  GET /version.json\n"+
			"  GET "+archPath+"\n"+
			"  GET "+archPath+"/sha256\n")
	})

	mux.HandleFunc("GET "+archPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=ze")
		http.ServeFile(w, r, execPath)
	})

	mux.HandleFunc("GET "+archPath+"/sha256", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, binaryHash+"\n") //nolint:errcheck // best-effort write to HTTP response
	})

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGUSR1 {
				pauseMu.Lock()
				signalPaused = !signalPaused
				state := signalPaused
				pauseMu.Unlock()
				if state {
					fmt.Fprintln(os.Stderr, "update serving paused")
				} else {
					fmt.Fprintln(os.Stderr, "update serving resumed")
				}
				continue
			}
			// SIGINT/SIGTERM: shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = srv.Shutdown(ctx)
			cancel()
			return
		}
	}()

	fmt.Fprintf(os.Stderr, "serving update on %s (version %s, binary at %s, sha256 %s)\n",
		listen, zeversion.Release(), archPath, binaryHash[:12]+"...")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
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
