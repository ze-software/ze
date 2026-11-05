//go:build integration

// Design: docs/labs/l2tp-interop.md -- paired LAC image version and plugin evidence.
// Related: docker.go -- RunOneShot owns each probe container and its cleanup.

package interoplab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: each already-built LAC image runs the pppd release its own
// Dockerfile pins and contains that release's PPPoL2TP plugin ABI path.
// PREVENTS: matching Dockerfile pins being mistaken for a rebuilt runtime,
// or a successful pppd version probe hiding an absent PPPoL2TP plugin.
func TestLACPeerRuntimeVersions(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	docker := NewDocker()
	// These are the two suite-owned physical image tags, not the logical
	// peer name "lac": l2tp.imageBuilds and radiussubscriber.suiteFor.
	for _, peer := range []struct {
		name       string
		dockerfile string
		image      string
	}{
		{"l2tp", "test/interop-l2tp/Dockerfile.lac", "ze-l2tp-lac"},
		{"radius-subscriber", "test/interop-radius-subscriber/Dockerfile.lac", "ze-radius-subscriber-lac"},
	} {
		t.Run(peer.name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(root, peer.dockerfile))
			if err != nil {
				t.Fatalf("read %s: %v", peer.dockerfile, err)
			}
			var version string
			for line := range strings.SplitSeq(string(body), "\n") {
				name, value, found := buildArgument(line)
				if found && name == "PPP_VERSION" {
					if version != "" {
						t.Fatalf("%s declares PPP_VERSION more than once", peer.dockerfile)
					}
					version = value
				}
			}
			if version == "" {
				t.Fatalf("%s has no nonempty PPP_VERSION default", peer.dockerfile)
			}
			result, err := docker.RunOneShot(ctx, OneShotContainer{
				Image: peer.image, Network: "none", Timeout: 30 * time.Second,
				Arguments: []string{"--pull=never"}, Command: []string{"pppd", "--version"},
			})
			if err != nil {
				t.Fatalf("run pppd in rebuilt %s: %v; stdout=%q stderr=%q", peer.image, err, result.Stdout, result.Stderr)
			}
			answer := strings.TrimSpace(result.Stdout + result.Stderr)
			if want := "pppd version " + version; answer != want {
				t.Fatalf("%s reports %q, want %q from %s", peer.image, answer, want, peer.dockerfile)
			}
			plugin := "/usr/lib/pppd/" + version + "/pppol2tp.so"
			result, err = docker.RunOneShot(ctx, OneShotContainer{
				Image: peer.image, Network: "none", Timeout: 30 * time.Second,
				Arguments: []string{"--pull=never"},
				Command:   []string{"sh", "-c", `test -f "$1" && printf '%s\n' "$1"`, "lac-plugin", plugin},
			})
			if err != nil {
				t.Fatalf("find PPPoL2TP plugin in rebuilt %s: %v; stdout=%q stderr=%q", peer.image, err, result.Stdout, result.Stderr)
			}
			if answer := strings.TrimSpace(result.Stdout); answer != plugin {
				t.Fatalf("%s plugin probe reports %q, want %q", peer.image, answer, plugin)
			}
			t.Logf("rebuilt image %s: pppd version %s; PPPoL2TP plugin %s", peer.image, version, plugin)
		})
	}
}
