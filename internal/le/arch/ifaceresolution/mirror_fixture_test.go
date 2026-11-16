// Design: docs/architecture/iface/logical-name-resolution.md -- legitimate raw-device lookups have exact allowlist boundaries.
package archifaceresolution

import "testing"

// The mirror fixture reads its own kernel topology, so its exact file is
// permitted. A similarly named file must still use logical-name resolution.
func TestMirrorFixtureAllowlistHasAnExactFileBoundary(t *testing.T) {
	const fixture = "internal/test/fixture/iface_mirror_fixture_linux.go"
	if _, allowed := allowedBy(fixture); !allowed {
		t.Fatal("the mirror fixture's raw kernel observations are not allowed")
	}
	if prefix, allowed := allowedBy(fixture + "2.go"); allowed {
		t.Fatalf("the neighboring .go2.go file is allowed by %q, want an exact-file boundary", prefix)
	}
}
