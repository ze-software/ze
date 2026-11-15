// Design: docs/architecture/config/system-update.md -- unit tests for update checker

package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

func TestUpdateCheckConfigParse(t *testing.T) {
	t.Run("url and default interval", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		uc := sys.GetOrCreateContainer("update-check")
		uc.Set("url", "https://example.com/version.json")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "https://example.com/version.json" {
			t.Errorf("UpdateCheckURL = %q, want %q", sc.UpdateCheckURL, "https://example.com/version.json")
		}
		if sc.UpdateCheckInterval != 86400 {
			t.Errorf("UpdateCheckInterval = %d, want %d", sc.UpdateCheckInterval, 86400)
		}
	})

	t.Run("url and custom interval", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		uc := sys.GetOrCreateContainer("update-check")
		uc.Set("url", "https://example.com/v.json")
		uc.Set("interval", "3600")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "https://example.com/v.json" {
			t.Errorf("UpdateCheckURL = %q, want %q", sc.UpdateCheckURL, "https://example.com/v.json")
		}
		if sc.UpdateCheckInterval != 3600 {
			t.Errorf("UpdateCheckInterval = %d, want %d", sc.UpdateCheckInterval, 3600)
		}
	})

	t.Run("no update-check block", func(t *testing.T) {
		tree := config.NewTree()
		sys := tree.GetOrCreateContainer("system")
		sys.Set("host", "router1")

		sc := ExtractSystemConfig(tree)
		if sc.UpdateCheckURL != "" {
			t.Errorf("UpdateCheckURL = %q, want empty", sc.UpdateCheckURL)
		}
		if sc.UpdateCheckInterval != 0 {
			t.Errorf("UpdateCheckInterval = %d, want 0", sc.UpdateCheckInterval)
		}
	})
}

func writeVersionJSON(w http.ResponseWriter, ver string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(versionManifest{Version: ver})
}

func TestUpdateCheckFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "26.06.01")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	ver, err := uc.fetchVersion(context.Background())
	if err != nil {
		t.Fatalf("fetchVersion() error = %v", err)
	}
	if ver != "26.06.01" {
		t.Errorf("fetchVersion() = %q, want %q", ver, "26.06.01")
	}
}

func TestUpdateCheckFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprint(w, "server error"); err != nil {
			panic(err)
		}
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	_, err := uc.fetchVersion(context.Background())
	if err == nil {
		t.Fatal("fetchVersion() expected error for 500 response with invalid JSON")
	}
}

func TestUpdateCheckFetchInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := fmt.Fprint(w, "not json at all"); err != nil {
			panic(err)
		}
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	_, err := uc.fetchVersion(context.Background())
	if err == nil {
		t.Fatal("fetchVersion() expected error for invalid JSON")
	}
}

func TestUpdateCheckEvent(t *testing.T) {
	report.ResetForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "99.99.99")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	uc.running = "26.05.17"
	uc.check(context.Background())

	st := uc.Status()
	if !st.UpdateAvailable {
		t.Error("expected UpdateAvailable = true for version 99.99.99")
	}
	if st.RemoteVersion != "99.99.99" {
		t.Errorf("RemoteVersion = %q, want %q", st.RemoteVersion, "99.99.99")
	}

	warnings := report.Warnings()
	found := false
	for _, w := range warnings {
		if w.Source == "system" && w.Code == "update-available" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected report bus warning with source=system, code=update-available")
	}
}

func TestUpdateCheckNoEvent(t *testing.T) {
	report.ResetForTest()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeVersionJSON(w, "00.00.01")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 86400)
	uc.running = "26.05.17"
	uc.check(context.Background())

	st := uc.Status()
	if st.UpdateAvailable {
		t.Error("expected UpdateAvailable = false for old version")
	}

	warnings := report.Warnings()
	for _, w := range warnings {
		if w.Source == "system" && w.Code == "update-available" {
			t.Error("unexpected update-available warning for old version")
		}
	}
}

func TestUpdateCheckStartStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeVersionJSON(w, "99.99.99")
	}))
	defer srv.Close()

	uc := newUpdateChecker(srv.URL, 1)
	ctx, cancel := context.WithCancel(context.Background())
	uc.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	st := uc.Status()
	if st.LastCheck.IsZero() {
		t.Error("expected at least one check after Start")
	}

	cancel()
	uc.Stop()
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		remote  string
		running string
		want    bool
	}{
		{"26.06.01", "26.05.17", true},
		{"26.05.17", "26.05.17", false},
		{"26.04.01", "26.05.17", false},
		{"dev", "26.05.17", false},
		{"99.99.99", "dev", false},
		{"27.01.01", "26.12.31", true},
	}
	for _, tt := range tests {
		t.Run(tt.remote+"_vs_"+tt.running, func(t *testing.T) {
			if got := isNewer(tt.remote, tt.running); got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.remote, tt.running, got, tt.want)
			}
		})
	}
}

// newCountingServer starts a server that answers with handler and counts every
// request it receives. A redirect target an update request must never reach
// is one: it answers as a valid update server would, so a followed redirect
// shows as a count rather than as a failure that could pass for the refusal.
// httptest's TLS servers share one certificate, so a client that trusts one
// TLS server would complete the handshake with any other.
func newCountingServer(t *testing.T, useTLS bool, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	counted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		handler(w, r)
	})
	var srv *httptest.Server
	if useTLS {
		srv = httptest.NewTLSServer(counted)
	} else {
		srv = httptest.NewServer(counted)
	}
	t.Cleanup(srv.Close)
	return srv, hits
}

// requireUpdateRedirectRefusal fails t unless err is the operator error an
// update request answers for a refused redirect: its problem names target,
// its why names what the redirect leaves, and its fix is fix.
func requireUpdateRedirectRefusal(t *testing.T, err error, target, left, fix string) {
	t.Helper()
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("refused redirect is not an operator error: %v", err)
	}
	if !strings.Contains(refusal.Problem(), "redirects to "+target) {
		t.Errorf("problem does not name the redirect target %s: %q", target, refusal.Problem())
	}
	if !strings.Contains(refusal.Why(), "leaves "+left) {
		t.Errorf("why does not name what the redirect leaves (%s): %q", left, refusal.Why())
	}
	if refusal.Fix() != fix {
		t.Errorf("fix = %q, want %q", refusal.Fix(), fix)
	}
}

// VALIDATES: the version check follows a redirect within the scheme and host
// of the configured url, and refuses one to another host or from https to
// http with an operator error naming the target and the fix.
// PREVENTS: a manifest request, and the userinfo of the configured url, sent
// to a server nobody configured or over plain http because a server answered
// with a redirect.
// METHOD: the configured server is TLS and redirects /version.json; the
// target counts its requests and answers a valid manifest, so a followed
// redirect is a count above zero. The checker's client keeps its redirect
// policy and only takes the transport that trusts the test certificate.
func TestUpdateCheckRedirectLeavingOriginRefused(t *testing.T) {
	tests := []struct {
		name      string
		targetTLS bool
		left      func(origin *httptest.Server) string
		fix       func(origin, target *httptest.Server) string
	}{
		{
			name:      "other host",
			targetTLS: true,
			left:      func(origin *httptest.Server) string { return "host " + origin.Listener.Addr().String() },
			fix: func(_, target *httptest.Server) string {
				return "set system update-check url " + target.URL + "/version.json"
			},
		},
		{
			name: "https to http",
			left: func(*httptest.Server) string { return "scheme https" },
			fix: func(origin, _ *httptest.Server) string {
				return "serve the manifest at " + origin.URL + "/version.json over https, with no redirect"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, targetHits := newCountingServer(t, tt.targetTLS, func(w http.ResponseWriter, _ *http.Request) {
				writeVersionJSON(w, "26.06.01")
			})
			origin, _ := newCountingServer(t, true, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/version.json", http.StatusFound)
			})

			uc := newUpdateChecker(origin.URL+"/version.json", 86400)
			uc.client.Transport = origin.Client().Transport
			_, err := uc.fetchVersion(context.Background())
			if err == nil {
				t.Fatal("fetchVersion followed a redirect that leaves the configured origin")
			}
			if targetHits.Load() != 0 {
				t.Fatalf("redirect target received %d requests, want 0", targetHits.Load())
			}
			requireUpdateRedirectRefusal(t, err, target.URL+"/version.json", tt.left(origin), tt.fix(origin, target))

			uc.check(context.Background())
			if !strings.Contains(uc.Status().LastError, "fix: "+tt.fix(origin, target)) {
				t.Errorf("status does not report the refusal and its fix: %q", uc.Status().LastError)
			}
		})
	}
}

// VALIDATES: a redirect within the scheme and host of the configured url is
// still followed by the version check.
// PREVENTS: a redirect policy that refuses every redirect, which would break
// an update server that moves its manifest on the same host.
func TestUpdateCheckRedirectWithinOriginFollowed(t *testing.T) {
	origin, _ := newCountingServer(t, true, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version.json" {
			http.Redirect(w, r, "/current/version.json", http.StatusFound)
			return
		}
		writeVersionJSON(w, "26.06.01")
	})
	uc := newUpdateChecker(origin.URL+"/version.json", 86400)
	uc.client.Transport = origin.Client().Transport
	ver, err := uc.fetchVersion(context.Background())
	if err != nil {
		t.Fatalf("same-origin redirect refused: %v", err)
	}
	if ver != "26.06.01" {
		t.Errorf("fetchVersion() = %q, want %q", ver, "26.06.01")
	}
}
