// VALIDATES: the gate in front of the gokrazy proxy forwards a request, with
// gokrazy's update credential, only when the account's Ze authorization grants
// the command that does what the page does; it never forwards the uploaded
// program runner (divert, uploadtemp) or poweroff; it refuses unknown, unclean
// and wrongly-methoded paths before the socket is dialled.
// PREVENTS: any authenticated web account, whatever its profile, reaching
// gokrazy's update handlers with the device's update credential (installing
// arbitrary software), and the browser's Ze credentials leaking to gokrazy.
// METHOD: each test drives handlerWithAuth against a fake gokrazy on a Unix
// socket that records every request it receives, so "refused" is proven by
// the fake receiving nothing, not by the status code alone, and each refusal
// is an operator error whose last line is its fix.
package gokrazy

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// testUpdateToken is the Basic Auth header the gate tests inject.
const testUpdateToken = "Basic Z29rcmF6eTpzM2NyZXQ="

// allowAll is an authorizer whose profile grants every command: an admin.
func allowAll(*http.Request, string, bool) bool { return true }

// readOnlyAccount grants reads only, as the built-in read-only profile does:
// its run section allows, its edit section denies every command.
func readOnlyAccount(_ *http.Request, _ string, isReadOnly bool) bool { return isReadOnly }

// lastLine answers the last line of a refusal body: the operator error's fix.
func lastLine(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	return lines[len(lines)-1]
}

// upstreamRequest is one request the fake gokrazy received.
type upstreamRequest struct {
	method string
	path   string
	auth   string
}

// upstreamLog records every request the fake gokrazy received. Safe for
// concurrent use.
type upstreamLog struct {
	mu       sync.Mutex
	requests []upstreamRequest
}

func (l *upstreamLog) add(r upstreamRequest) {
	l.mu.Lock()
	l.requests = append(l.requests, r)
	l.mu.Unlock()
}

func (l *upstreamLog) all() []upstreamRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]upstreamRequest(nil), l.requests...)
}

// recordingGokrazy starts a fake gokrazy that answers 200 to every request
// and records each one, and returns its socket path and log.
func recordingGokrazy(t *testing.T) (string, *upstreamLog) {
	t.Helper()
	socketPath := filepath.Join(socketDir(t), "gokrazy.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	log := &upstreamLog{}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(upstreamRequest{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization")})
		if _, werr := io.WriteString(w, "ok"); werr != nil {
			return
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socketPath, log
}

// TestGateRefusesDeviceChangeWithoutPermission proves an account whose profile
// grants reads only cannot reach any gokrazy handler that changes the device:
// each answers 403 naming the Ze command it needs, and gokrazy receives no
// request, so the update credential is never sent.
func TestGateRefusesDeviceChangeWithoutPermission(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	h := handlerWithAuth(sock, testUpdateToken, readOnlyAccount)

	cases := []struct {
		method, path, command string
	}{
		{http.MethodPut, "/gokrazy/update/root", commandFirmwareApply},
		{http.MethodPut, "/gokrazy/update/boot", commandFirmwareApply},
		{http.MethodPut, "/gokrazy/update/mbr", commandFirmwareApply},
		{http.MethodPost, "/gokrazy/update/switch", commandFirmwareApply},
		{http.MethodPost, "/gokrazy/update/testboot", commandFirmwareApply},
		{http.MethodGet, "/gokrazy/update/features", commandFirmwareApply},
		{http.MethodGet, "/gokrazy/%75pdate/root", commandFirmwareApply},
		{http.MethodPost, "/gokrazy/reboot", commandReboot},
		{http.MethodPost, "/gokrazy/stop", commandShutdown},
		{http.MethodPost, "/gokrazy/restart", commandShutdown},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := doRequest(t, h, tc.method, tc.path, nil)
			if resp.code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.code)
			}
			if got := lastLine(resp.body); got != "fix: "+tc.command {
				t.Fatalf("refusal last line = %q, want the fix naming the command %q\nbody: %s", got, tc.command, resp.body)
			}
		})
	}
	if got := upstream.all(); len(got) != 0 {
		t.Fatalf("gokrazy received %d requests from a read-only account, want none: %+v", len(got), got)
	}
}

// TestGateRefusesUploadedCodeForEveryAccount proves divert, uploadtemp and
// poweroff are refused even for an account whose profile grants every
// command: no Ze authorization maps to them, and gokrazy receives nothing.
func TestGateRefusesUploadedCodeForEveryAccount(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	asked := 0
	counting := func(*http.Request, string, bool) bool {
		asked++
		return true
	}
	h := handlerWithAuth(sock, testUpdateToken, counting)

	for _, tc := range []struct{ method, path, fix string }{
		{http.MethodPost, "/gokrazy/divert", fixPushImage},
		{http.MethodGet, "/gokrazy/divert", fixPushImage},
		{http.MethodPut, "/gokrazy/uploadtemp/ze", fixPushImage},
		{http.MethodPost, "/gokrazy/uploadtemp/", fixPushImage},
		{http.MethodPost, "/gokrazy/poweroff", commandReboot},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := doRequest(t, h, tc.method, tc.path, nil)
			if resp.code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.code)
			}
			if !strings.HasPrefix(resp.body, "forbidden: Ze never forwards "+tc.path+"\n") {
				t.Fatalf("refusal does not open by saying the page is never forwarded: %s", resp.body)
			}
			if got := lastLine(resp.body); got != "fix: "+tc.fix {
				t.Fatalf("refusal last line = %q, want %q\nbody: %s", got, "fix: "+tc.fix, resp.body)
			}
		})
	}
	if got := upstream.all(); len(got) != 0 {
		t.Fatalf("gokrazy received %d requests for never-forwarded pages, want none: %+v", len(got), got)
	}
	if asked != 0 {
		t.Fatalf("authorizer asked %d times, want 0: these pages are refused before any profile is read", asked)
	}
}

// TestGateForwardsReadPagesWithToken proves a read-only account still sees
// gokrazy's status pages, and that the forwarded request carries the token.
func TestGateForwardsReadPagesWithToken(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	h := handlerWithAuth(sock, testUpdateToken, readOnlyAccount)

	paths := []string{"/gokrazy/", "/gokrazy/status", "/gokrazy/log", "/gokrazy/assets/gokrazy-logo.svg"}
	for _, p := range paths {
		if resp := doGet(t, h, p); resp.code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", p, resp.code)
		}
	}
	got := upstream.all()
	if len(got) != len(paths) {
		t.Fatalf("gokrazy received %d requests, want %d: %+v", len(got), len(paths), got)
	}
	for _, r := range got {
		if r.auth != testUpdateToken {
			t.Fatalf("forwarded %s carried Authorization %q, want the update token", r.path, r.auth)
		}
	}
}

// TestGateForwardsDeviceChangeWhenGranted proves an account whose profile
// grants the Ze command reaches the gokrazy handler, with the token, and that
// the gate asked for exactly that command in the edit section.
func TestGateForwardsDeviceChangeWhenGranted(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	type question struct {
		command    string
		isReadOnly bool
	}
	var asked []question
	recording := func(_ *http.Request, command string, isReadOnly bool) bool {
		asked = append(asked, question{command, isReadOnly})
		return true
	}
	h := handlerWithAuth(sock, testUpdateToken, recording)

	if resp := doRequest(t, h, http.MethodPut, "/gokrazy/update/root", nil); resp.code != http.StatusOK {
		t.Fatalf("PUT update/root: status = %d, want 200", resp.code)
	}
	if resp := doRequest(t, h, http.MethodPost, "/gokrazy/reboot", nil); resp.code != http.StatusOK {
		t.Fatalf("POST reboot: status = %d, want 200", resp.code)
	}

	want := []question{{commandFirmwareApply, false}, {commandReboot, false}}
	if len(asked) != len(want) || asked[0] != want[0] || asked[1] != want[1] {
		t.Fatalf("authorizer asked %+v, want %+v", asked, want)
	}
	got := upstream.all()
	if len(got) != 2 || got[0].path != "/update/root" || got[1].path != "/reboot" {
		t.Fatalf("gokrazy received %+v, want /update/root then /reboot", got)
	}
	for _, r := range got {
		if r.auth != testUpdateToken {
			t.Fatalf("forwarded %s carried Authorization %q, want the update token", r.path, r.auth)
		}
	}
}

// TestGateRefusesUnknownUncleanAndWrongMethod proves a path gokrazy's list
// does not name, a path that is not clean, and a write to a read-only page are
// refused with their own status before gokrazy is reached.
func TestGateRefusesUnknownUncleanAndWrongMethod(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	h := handlerWithAuth(sock, testUpdateToken, allowAll)

	cases := []struct {
		method, path string
		status       int
		fix          string
	}{
		{http.MethodGet, "/gokrazy/api", http.StatusNotFound, "/gokrazy/"},
		{http.MethodGet, "/gokrazy/update", http.StatusNotFound, "/gokrazy/"},
		{http.MethodGet, "/gokrazy/assets/../update/root", http.StatusBadRequest, "/gokrazy/"},
		{http.MethodGet, "/gokrazy//update/root", http.StatusBadRequest, "/gokrazy/"},
		{http.MethodPost, "/gokrazy/status", http.StatusMethodNotAllowed, "/gokrazy/status"},
		{http.MethodDelete, "/gokrazy/", http.StatusMethodNotAllowed, "/gokrazy/"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := doRequest(t, h, tc.method, tc.path, nil)
			if resp.code != tc.status {
				t.Fatalf("status = %d, want %d\nbody: %s", resp.code, tc.status, resp.body)
			}
			if got := lastLine(resp.body); got != "fix: "+tc.fix {
				t.Fatalf("refusal last line = %q, want %q\nbody: %s", got, "fix: "+tc.fix, resp.body)
			}
			if tc.status == http.StatusMethodNotAllowed && resp.header.Get("Allow") != "GET, HEAD" {
				t.Fatalf("Allow = %q, want \"GET, HEAD\"", resp.header.Get("Allow"))
			}
		})
	}
	if got := upstream.all(); len(got) != 0 {
		t.Fatalf("gokrazy received %d refused requests, want none: %+v", len(got), got)
	}
}

// TestGateDoesNotForwardBrowserCredentials proves the browser's own
// Authorization header, which carries its Ze credentials, never reaches
// gokrazy, even when no update token is configured.
func TestGateDoesNotForwardBrowserCredentials(t *testing.T) {
	sock, upstream := recordingGokrazy(t)
	h := handlerWithAuth(sock, "", allowAll)

	header := http.Header{"Authorization": {"Basic YWxpY2U6emUtcGFzc3dvcmQ="}}
	if resp := doRequest(t, h, http.MethodGet, "/gokrazy/status", header); resp.code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.code)
	}
	got := upstream.all()
	if len(got) != 1 || got[0].auth != "" {
		t.Fatalf("gokrazy received %+v, want one request with no Authorization", got)
	}
}
