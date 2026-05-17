// Design: plan/spec-cpe-5-firmware-update.md — standalone update server (version + binary)

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

// runUpdateServe starts a minimal HTTP server that serves:
//   - GET /version.json        — version manifest for update checks
//   - GET /<goos>/<goarch>     — the running binary for download
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

	mux := http.NewServeMux()

	archPath := "/" + runtime.GOOS + "/" + runtime.GOARCH

	selfArch := runtime.GOOS + "/" + runtime.GOARCH

	mux.HandleFunc("GET /version.json", func(w http.ResponseWriter, r *http.Request) {
		reqArch := r.Header.Get("X-Ze-Arch")
		if reqArch != "" && reqArch != selfArch {
			http.Error(w, "arch mismatch: serving "+selfArch+", got "+reqArch, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		if _, err := io.WriteString(w, `{"version":"`+zeversion.Release()+`"}`+"\n"); err != nil {
			return
		}
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := io.WriteString(w, "ze update server\n\n"+
			"version: "+zeversion.Release()+"\n"+
			"arch:    "+selfArch+"\n\n"+
			"endpoints:\n"+
			"  GET /version.json\n"+
			"  GET "+archPath+"\n"); err != nil {
			return
		}
	})

	mux.HandleFunc("GET "+archPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=ze")
		http.ServeFile(w, r, execPath)
	})

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	fmt.Fprintf(os.Stderr, "serving update on %s (version %s, binary at %s)\n", listen, zeversion.Release(), archPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
