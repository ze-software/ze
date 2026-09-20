// Design: docs/architecture/api/birdwatcher-compat.md -- the status endpoint's contract
// Related: handler_api.go -- handleAPIStatus, engineAnswer and the command constants
// Related: server_test.go -- mockDispatch, which answers only commands the daemon serves

package lg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// The YANG modules that declare the commands handleAPIStatus dispatches.
	// A test binary registers only the modules its own imports pull in, so the
	// two schema packages are named here to put the command tree under the
	// walk below. Both are schema-only leaves (an embed plus a
	// configyang.RegisterModule call), so this is not the looking glass
	// reaching into a plugin's implementation.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// statusRequest drives GET /api/looking-glass/status through the server's own
// handler chain, which is the entry point an operator's client reaches:
// securityHeaders, the error fragment middleware and bearerAuth all sit above
// the mux, so a test that called the method directly would prove less.
func statusRequest(t *testing.T, dispatch CommandDispatcher) *httptest.ResponseRecorder {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    dispatch,
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/looking-glass/status", http.NoBody)
	srv.server.Handler.ServeHTTP(rec, req)

	return rec
}

// TestAPIStatusAnswersTheRouterIdentity proves the endpoint publishes the
// identity it exists to publish when every command it asks for is served.
//
// VALIDATES: the four commands handleAPIStatus dispatches reach mockDispatch's
// table, and each fact lands in its birdwatcher field.
// PREVENTS: the endpoint answering an empty router id and an empty version,
// which is what it did while it asked for `bgp status`
// (plan/journal/zero-value-as-valid-answer.md, 2026-09-17).
func TestAPIStatusAnswersTheRouterIdentity(t *testing.T) {
	rec := statusRequest(t, mockDispatch())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var answer struct {
		Status struct {
			RouterID     string `json:"router_id"`
			Version      string `json:"version"`
			LastReboot   string `json:"last_reboot"`
			LastReconfig string `json:"last_reconfig"`
		} `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode body: %v; body %s", err, rec.Body.String())
	}

	want := map[string]string{
		"router_id":     "1.2.3.4",
		"version":       "ze test (built 2026-01-01)",
		"last_reboot":   "2026-01-01T00:00:00Z",
		"last_reconfig": "2026-03-01T12:00:00Z",
	}
	got := map[string]string{
		"router_id":     answer.Status.RouterID,
		"version":       answer.Status.Version,
		"last_reboot":   answer.Status.LastReboot,
		"last_reconfig": answer.Status.LastReconfig,
	}
	for field, expected := range want {
		if got[field] != expected {
			t.Errorf("status.%s = %q, want %q", field, got[field], expected)
		}
	}
}

// TestAPIStatusRefusesAFailedDispatch proves the endpoint fails closed.
//
// Each case refuses ONE of the four commands and serves the other three, which
// is the shape of the live failure: the daemon answered every other looking
// glass command and refused this one. A handler that reads the keys it wants
// out of the refusal renders an empty identity under HTTP 200, and that is what
// this test makes impossible.
//
// VALIDATES: a command the daemon does not serve produces a refusal that names
// the command, and no status object at all.
// PREVENTS: a dispatch name going stale again without a red test, and a failed
// dispatch answering 200 (ai/rules/principles.md).
func TestAPIStatusRefusesAFailedDispatch(t *testing.T) {
	for _, refused := range []string{cmdBGPOverview, cmdShowVersion, cmdShowUptime, cmdShowReloadStatus} {
		t.Run(refused, func(t *testing.T) {
			rec := statusRequest(t, refusing(refused))

			if rec.Code == http.StatusOK {
				t.Fatalf("%q refused and the endpoint still answered 200: %s", refused, rec.Body.String())
			}
			if rec.Code != http.StatusBadGateway {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadGateway)
			}

			body := rec.Body.String()
			if !strings.Contains(body, refused) {
				t.Errorf("body does not name the command that failed: %s", body)
			}
			if strings.Contains(body, "router_id") {
				t.Errorf("a refusal carries a status object: %s", body)
			}
		})
	}
}

// refusing wraps mockDispatch and refuses one command, the way the daemon
// refuses a command no YANG module declares.
func refusing(command string) CommandDispatcher {
	served := mockDispatch()

	return func(ctx context.Context, caller plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		if cmd == command {
			return nil, errUnknownCommand
		}

		return served(ctx, caller, cmd)
	}
}

// TestStatusCommandsAreDeclared reads the command tree the daemon builds and
// fails when a command handleAPIStatus dispatches is not in it.
//
// The looking glass sends command TEXT, so nothing in the type system ties
// these four strings to the grammar the daemon serves: `bgp status` compiled
// for as long as it existed and failed on every request. This is the tie. It
// walks the same YANG command tree that plugin/server walks when it loads the
// builtins (server.go, yang.WireMethodToPaths).
//
// VALIDATES: each command constant is a path the command tree declares.
// PREVENTS: a renamed or removed command leaving the looking glass asking for
// a command that no longer exists.
func TestStatusCommandsAreDeclared(t *testing.T) {
	loader, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatalf("yang loader: %v", err)
	}

	declared := make(map[string]bool)
	for _, paths := range configyang.WireMethodToPaths(loader) {
		for _, path := range paths {
			declared[path] = true
		}
	}
	if len(declared) == 0 {
		t.Fatal("the command tree is empty, so this test proves nothing")
	}

	for _, command := range []string{cmdBGPOverview, cmdShowVersion, cmdShowUptime, cmdShowReloadStatus} {
		if !declared[command] {
			t.Errorf("handleAPIStatus dispatches %q, which the command tree does not declare", command)
		}
	}
}
