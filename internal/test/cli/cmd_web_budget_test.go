// Design: docs/architecture/testing/runner-architecture.md -- shared startup/browser budget.
package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var webBudgetFixture = flag.String("web-budget-fixture", "", "owned deadline fixture role")
var webBudgetCoordinator = flag.String("web-budget-coordinator", "", "owned deadline fixture coordinator")

// TestWebBudgetFixture serves a subprocess boundary for the callback tests.
// The production callback owns and joins the server and each browser command.
func TestWebBudgetFixture(t *testing.T) {
	if *webBudgetFixture == "" {
		return
	}
	if *webBudgetFixture == "server" {
		args := flag.Args()
		for i, arg := range args {
			if arg == "--listen" {
				server := &http.Server{
					Addr:              args[i+1],
					ReadHeaderTimeout: time.Second,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						if err := webBudgetRequest(req.Context(), "/ready"); err != nil {
							if req.Context().Err() != nil {
								return
							}
							t.Error(err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(http.StatusOK)
					}),
				}
				if err := server.ListenAndServe(); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatal("server fixture received no listen address")
	}
	args := strings.Join(flag.Args(), " ")
	path := "/other"
	switch {
	case strings.Contains(args, "set viewport"):
		path = "/viewport"
	case strings.Contains(args, "set headers"):
		path = "/locale"
	case strings.Contains(args, "open "):
		path = "/open"
	case strings.HasSuffix(args, "close"):
		path = "/close"
	}
	if err := webBudgetRequest(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}

func webBudgetRequest(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *webBudgetCoordinator+path, http.NoBody)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return errors.Join(err, resp.Body.Close())
	}
	return resp.Body.Close()
}

// TestWebCallbackSharesStartupBudget drives the real callback with controlled
// HTTP and executable boundaries. Startup and viewport finish, then locale
// exhausts the remaining authored budget. Resetting the browser budget permits
// open, which this test refuses. The slow-start case never reaches a browser.
func TestWebCallbackSharesStartupBudget(t *testing.T) {
	for _, slowStartup := range []bool{false, true} {
		t.Run(strconv.FormatBool(slowStartup), func(t *testing.T) {
			var mu sync.Mutex
			var observed []string
			startup := time.Now()
			coordinator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				mu.Lock()
				observed = append(observed, req.URL.Path)
				readyAt := startup.Add(time.Second)
				mu.Unlock()
				delay := time.Duration(0)
				switch req.URL.Path {
				case "/ready":
					delay = time.Until(readyAt)
					if slowStartup {
						delay = 4 * time.Second
					}
				case "/viewport":
					delay = 300 * time.Millisecond
				case "/locale":
					delay = time.Second
				}
				// The timer injects latency; request cancellation is the state
				// under test. No sleep is used to infer readiness.
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-req.Context().Done():
					return
				case <-timer.C:
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer coordinator.Close()
			dir := t.TempDir()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range []string{"server", "agent-browser"} {
				// Race-instrumented fixture commands must not add the runtime's
				// exit sleep to the deliberately injected browser latency.
				script := "#!/bin/sh\nexport GORACE=\"$GORACE atexit_sleep_ms=0\"\nexec " + strconv.Quote(binary) +
					" -test.run=^TestWebBudgetFixture$ -web-budget-fixture=" + role +
					" -web-budget-coordinator=" + strconv.Quote(coordinator.URL) + " -- \"$@\"\n"
				if err := os.WriteFile(filepath.Join(dir, role), []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			path := filepath.Join(dir, "budget.wb")
			body := "option=timeout:value=2s\noption=server:kind=lg-no-engine\n" +
				"option=viewport:width=800:height=600\noption=locale:lang=en\naction=open:path=/\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			mu.Lock()
			startup = time.Now()
			mu.Unlock()
			test := discoverWebCase(path, 1)
			if test.admission.ParseFailed {
				t.Fatal(test.admission.Error)
			}
			ok, err := zeTestRunWebTest(ctx, test, zeTestWebBinaries{zeTest: filepath.Join(dir, "server")}, 1)
			if ok {
				t.Fatal("callback passed despite cumulative startup/browser budget exhaustion")
			}
			if err == nil {
				t.Fatal("callback lost the timeout cause")
			}
			if !strings.Contains(err.Error(), "option=timeout budget 2s exhausted") {
				t.Fatalf("wrong failure: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			calls := strings.Join(observed, " ")
			t.Logf("observed=%s failure=%v", calls, err)
			if strings.Contains(calls, "/open") {
				t.Fatal("browser received a fresh budget after startup")
			}
			if !strings.Contains(calls, "/ready") {
				t.Fatal("fixture never reached HTTP readiness")
			}
			if slowStartup {
				if strings.Contains(calls, "/viewport") {
					t.Fatal("browser started after startup exhausted the budget")
				}
				return
			}
			for _, want := range []string{"/viewport", "/locale", "/close"} {
				if !strings.Contains(calls, want) {
					t.Errorf("fixture did not reach %s", want)
				}
			}
		})
	}
}
