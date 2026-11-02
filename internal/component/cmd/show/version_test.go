package show

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

// TestShowVersionOneProducer proves AC-14 for show version on the daemon
// route: handleShowVersion answers zeversion.Answer, the producer the
// no-daemon route (printVersion in cmd/ze) prints.
//
// Method: stamp a release no other source holds, then compare the RPC's
// version value with the producer's, short and `--extended`. A handler that
// built its own line from another source (pluginserver.GetVersion, which this
// test does not set) answers "dev" and fails the release check.
func TestShowVersionOneProducer(t *testing.T) {
	release, built := zeversion.Release(), zeversion.BuildDate()
	t.Cleanup(func() { zeversion.Stamp(release, built) })
	zeversion.Stamp("one-producer-release", "2026-11-02T00:00:00Z")

	for _, tc := range []struct {
		words    []string
		extended bool
	}{
		{nil, false},
		{[]string{"--extended"}, true},
	} {
		resp, err := handleShowVersion(nil, commandtest.Args(tc.words...))
		if err != nil {
			t.Fatalf("%q: %v", tc.words, err)
		}
		if resp.Status != plugin.StatusDone {
			t.Fatalf("%q: status %q, want done", tc.words, resp.Status)
		}
		data, ok := resp.Data.(plugin.Map)
		if !ok {
			t.Fatalf("%q: data %T, want plugin.Map", tc.words, resp.Data)
		}
		want := zeversion.Answer(tc.extended)
		if data["version"] != want {
			t.Errorf("%q: daemon %q, producer %q", tc.words, data["version"], want)
		}
		if !strings.Contains(want, "one-producer-release") {
			t.Errorf("%q: producer %q lacks the stamped release", tc.words, want)
		}
	}
}
