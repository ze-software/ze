// Design: docs/labs/l2tp-interop.md -- paired L2TP and subscriber RADIUS LAC images.
// Related: sourcepin.go -- buildArgument parses Dockerfile ARG declarations.

package interoplab

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: both LAC image declarations select the same pppd and xl2tpd
// releases and archive hashes; required defaults are present and hashes decode.
// PREVENTS: updating L2TP's peer while subscriber RADIUS silently retains an
// older release, or a version bump that leaves one paired archive hash behind.
func TestLACPeerReleasePinsAgree(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"PPP_VERSION", "PPP_SHA256", "XL2TPD_VERSION", "XL2TPD_SHA256"}
	var first map[string]string
	for _, relative := range []string{
		"test/interop-l2tp/Dockerfile.lac",
		"test/interop-radius-subscriber/Dockerfile.lac",
	} {
		body, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		pins := make(map[string]string, len(keys))
		for line := range strings.SplitSeq(string(body), "\n") {
			name, value, found := buildArgument(line)
			if !found || !slices.Contains(keys, name) {
				continue
			}
			if _, repeated := pins[name]; repeated {
				t.Fatalf("%s declares %s more than once", relative, name)
			}
			pins[name] = value
		}
		for _, key := range keys {
			value, present := pins[key]
			if !present || value == "" {
				t.Fatalf("%s has no nonempty %s default", relative, key)
			}
			if strings.HasSuffix(key, "_SHA256") {
				digest, err := hex.DecodeString(value)
				if err != nil || len(digest) != 32 {
					t.Fatalf("%s %s is not a SHA256 digest: %q", relative, key, value)
				}
			}
			if first != nil && value != first[key] {
				t.Errorf("%s %s=%s differs from the paired L2TP image's %s", relative, key, value, first[key])
			}
		}
		if first == nil {
			first = pins
		}
	}
}
