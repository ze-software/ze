package kernelbuilder

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// slowDownloadServer answers a .sha256 path with a checksum line at once,
// unless stallChecksum holds it until the request ends, and any other path
// with 1 KiB every 50 ms of a 100 MB body, so a stop about 300 ms in always
// lands in the middle of the download.
func slowDownloadServer(t *testing.T, stallChecksum bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			if stallChecksum {
				<-r.Context().Done()
				return
			}
			_, _ = w.Write([]byte(strings.Repeat("a", 64) + "  x.iso\n"))
			return
		}
		w.Header().Set("Content-Length", "100000000")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("httptest writer cannot flush")
			return
		}
		for range 600 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
			if _, err := w.Write(make([]byte, 1024)); err != nil {
				return
			}
			flusher.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDownloadStopNamesTheStep proves a build stopped while it downloads the
// Alpine ISO's checksum, the Alpine ISO, or the kernel tarball names that
// download and why it stopped, and a deadline ends with the curl command to
// run by hand. A slow httptest server plays the mirror, over the three ways a
// build stops.
//
// VALIDATES: round-9 ISSUE 1.
// PREVENTS: a Ctrl-C reported as a bare "context canceled", and Ze's limit
// reported as the limit's own noun phrase ("the 2-hour limit Ze gives one
// kernel build"), with no step and no next step, because net/http answers
// context.Cause(ctx).
func TestDownloadStopNamesTheStep(t *testing.T) {
	steps := []struct {
		name  string
		stall bool
		stage string
		run   func(ctx context.Context, t *testing.T, url string) error
		next  func(url string) string
	}{
		{"alpine checksum", true, "the Alpine ISO checksum download was stopped",
			func(ctx context.Context, t *testing.T, url string) error {
				_, err := ensureAlpineISO(ctx, archX8664)
				return err
			},
			func(url string) string { return "curl -fL -o /dev/null '" + url + "/" }},
		{"alpine iso", false, "the Alpine ISO download was stopped",
			func(ctx context.Context, t *testing.T, url string) error {
				_, err := ensureAlpineISO(ctx, archX8664)
				return err
			},
			func(url string) string { return "curl -fL -o /dev/null '" + url + "/" }},
		{"kernel tarball", false, "the kernel tarball download was stopped",
			func(ctx context.Context, t *testing.T, url string) error {
				tarball := filepath.Join(t.TempDir(), "linux.tar.xz")
				err := fetchKernelTarball(ctx, url+"/linux.tar.xz", tarball)
				if _, statErr := os.Stat(tarball); statErr == nil {
					t.Error("a stopped tarball download left a partial tarball the next build takes as whole")
				}
				return err
			},
			func(url string) string { return "curl -fL -o " }},
	}
	for _, step := range steps {
		for _, tc := range stopCases() {
			t.Run(step.name+"/"+tc.name, func(t *testing.T) {
				srv := slowDownloadServer(t, step.stall)
				t.Setenv("XDG_CACHE_HOME", t.TempDir())
				old := alpineReleaseBaseURL
				alpineReleaseBaseURL = srv.URL
				t.Cleanup(func() { alpineReleaseBaseURL = old })
				ctx, cancel := tc.context()
				defer cancel()
				err := step.run(ctx, t, srv.URL)
				checkStopped(t, err, tc, step.stage, step.next(srv.URL))
				if err != nil && strings.HasPrefix(err.Error(), "the 300ms limit") {
					t.Errorf("a stopped download is reported by the limit's name alone:\n%v", err)
				}
			})
		}
	}
}

// TestStopReportNamesAnUnnamedStop proves Build's backstop: a step that
// returned under a stop without naming it (net/http's "context canceled", or
// the limit's own noun phrase) is reported as the build stopping, with that
// error kept as evidence, while a step that named its stop, or an error with
// no stop, passes unchanged.
//
// VALIDATES: round-9 ISSUE 1 (the sweep).
// PREVENTS: a stop reported by a library's error alone at a step no one
// named, such as freeTCPPort's Listen or QEMU's Start.
func TestStopReportNamesAnUnnamedStop(t *testing.T) {
	raw := errors.New("listen tcp 127.0.0.1:0: operation was canceled")
	for _, tc := range stopCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.context()
			defer cancel()
			<-ctx.Done()
			checkStopped(t, stopReport(ctx, raw), tc,
				"the kernel build was stopped part way (its last step said: "+raw.Error()+")",
				"the build output above shows the last step it reached")
			named := errBuildStopped(ctx, "STEP", "NEXT")
			if got := stopReport(ctx, named); got.Error() != named.Error() {
				t.Errorf("a step that named its stop is reported again:\n%v", got)
			}
		})
	}
	if got := stopReport(context.Background(), raw); got.Error() != raw.Error() {
		t.Errorf("an error with no stop is reported as a stop:\n%v", got)
	}
}

// TestGuestBuildAdviceLogsInAsTheBuildDoes proves the ssh command a stopped
// guest build prints carries every option the build itself logs in with
// (sshOptions, less the port), so the advice is the same login.
//
// VALIDATES: round-9 NOTE 1.
// PREVENTS: advice that claims to match sshOptions while it carries two of
// its four options.
func TestGuestBuildAdviceLogsInAsTheBuildDoes(t *testing.T) {
	login := sshOptions(2222)
	options := strings.Join(login[:len(login)-2], " ")
	lines := strings.Split(guestBuildNextStep(), "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, " "+options+" -p <port> ") {
		t.Errorf("the ssh advice does not log in as the build does, want options %q:\n%s", options, last)
	}
}
