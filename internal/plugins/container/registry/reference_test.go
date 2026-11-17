// Design: docs/architecture/container/image-store.md -- the reference forms and their completion
// Related: reference.go -- ParseReference under test

package registry

import (
	"strings"
	"testing"
)

// testDigest is a well-formed digest for the reference tests.
const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestParseReferenceForms proves each row of the reference table gives its
// registry, repository and pin, and that the short and the whole spelling of
// one image are one value.
func TestParseReferenceForms(t *testing.T) {
	for _, tc := range []struct {
		text, registry, repository, tag, digest, whole string
	}{
		{"registry.example.com/team/app:1.2", "registry.example.com", "team/app", "1.2", "", "registry.example.com/team/app:1.2"},
		{"registry.example.com/team/app@" + testDigest, "registry.example.com", "team/app", "", testDigest, "registry.example.com/team/app@" + testDigest},
		{"redis:7", "docker.io", "library/redis", "7", "", "docker.io/library/redis:7"},
		{"bitnami/redis:7", "docker.io", "bitnami/redis", "7", "", "docker.io/bitnami/redis:7"},
		{"redis", "docker.io", "library/redis", "latest", "", "docker.io/library/redis:latest"},
		{"registry.example.com/app", "registry.example.com", "app", "latest", "", "registry.example.com/app:latest"},
		{"redis@" + testDigest, "docker.io", "library/redis", "", testDigest, "docker.io/library/redis@" + testDigest},
		{"redis:7@" + testDigest, "docker.io", "library/redis", "7", testDigest, "docker.io/library/redis:7@" + testDigest},
		{"index.docker.io/library/redis", "docker.io", "library/redis", "latest", "", "docker.io/library/redis:latest"},
	} {
		ref, err := ParseReference(tc.text)
		if err != nil {
			t.Errorf("%s: %v", tc.text, err)
			continue
		}
		digest, pinned := ref.Digest()
		if ref.Registry() != tc.registry || ref.Repository() != tc.repository || ref.Tag() != tc.tag || digest.String() != tc.digest || pinned != (tc.digest != "") {
			t.Errorf("%s = %q %q %q %q, want %q %q %q %q", tc.text, ref.Registry(), ref.Repository(), ref.Tag(), digest, tc.registry, tc.repository, tc.tag, tc.digest)
		}
		if ref.String() != tc.whole {
			t.Errorf("%s completes to %s, want %s", tc.text, ref, tc.whole)
		}
		whole, err := ParseReference(tc.whole)
		if err != nil || whole != ref {
			t.Errorf("%s and its whole form %s are not one reference: %v", tc.text, tc.whole, err)
		}
		wantPin := tc.tag
		if tc.digest != "" {
			wantPin = tc.digest
		}
		if ref.pin() != wantPin {
			t.Errorf("%s asks for %s, want %s: a digest is never resolved again", tc.text, ref.pin(), wantPin)
		}
	}
	if endpointHost("docker.io") != "registry-1.docker.io" || endpointHost("registry.example.com") != "registry.example.com" {
		t.Error("the hub is not reached at its registry host, or another registry is not reached at its own")
	}
}

// TestParseReferenceHostRule proves a first component with a dot, with a
// colon, and the word localhost are registry hosts, and any other first
// component is part of a repository of the hub.
func TestParseReferenceHostRule(t *testing.T) {
	for _, tc := range []struct{ text, registry, repository string }{
		{"a.b/app", "a.b", "app"},
		{"host:5000/app", "host:5000", "app"},
		{"localhost/app", "localhost", "app"},
		{"localhost:5000/team/app", "localhost:5000", "team/app"},
		{"[::1]:5000/app", "[::1]:5000", "app"},
		{"team/app", "docker.io", "team/app"},
		{"localhostx/app", "docker.io", "localhostx/app"},
	} {
		ref, err := ParseReference(tc.text)
		if err != nil {
			t.Errorf("%s: %v", tc.text, err)
			continue
		}
		if ref.Registry() != tc.registry || ref.Repository() != tc.repository {
			t.Errorf("%s = %s %s, want %s %s", tc.text, ref.Registry(), ref.Repository(), tc.registry, tc.repository)
		}
	}
}

// TestParseReferenceRefusesMalformed proves an upper-case repository, an
// empty component, a bad digest and the other malformed parts are refused,
// each naming the part, and that nothing is corrected.
func TestParseReferenceRefusesMalformed(t *testing.T) {
	for _, tc := range []struct{ text, named string }{
		{"registry.example.com/Team/app", "upper-case"},
		{"registry.example.com//app", "empty"},
		{"registry.example.com/app/", "empty"},
		{"app@sha256:abc", "digest"},
		{"app@md5:" + strings.Repeat("a", 32), "digest algorithm"},
		{"app@sha256:" + strings.Repeat("A", 64), "lower case"},
		{"app:", "tag"},
		{"app:.dot", "tag"},
		{"Registry.Example.com/app", "registry"},
		{"registry.example.com:99999/app", "port"},
		{"app...x", "separator"},
		{"-app", "separator"},
		{"", "empty"},
		{strings.Repeat("a", referenceLengthMax+1), "bytes"},
	} {
		_, err := ParseReference(tc.text)
		if err == nil {
			t.Errorf("%q was accepted, want it refused naming %s", tc.text, tc.named)
			continue
		}
		if !strings.Contains(err.Error(), tc.named) {
			t.Errorf("%q: %v, want %s named", tc.text, err, tc.named)
		}
	}
	if _, err := ParseReference("registry.example.com/a__b.c-d/e--f_g:v1.0-rc_1"); err != nil {
		t.Errorf("every separator of the grammar in one reference was refused: %v", err)
	}
}
