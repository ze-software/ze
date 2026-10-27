// Design: docs/contributing/gh-pages.md -- renderer peers share canonical lab pins.
package siteterminaldemo

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestRendererPeerArgumentsReadsReleaseDeclarations exercises stable-release
// parsing with fixture declarations, including missing and malformed pins.
func TestRendererPeerArgumentsReadsReleaseDeclarations(t *testing.T) {
	checksum := strings.Repeat("ab", 32)
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{"valid", "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_SHA256=" + checksum + "\n", "7.8.9"},
		{"missing version", "ARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
		{"missing checksum", "ARG KEEPALIVED_VERSION=7.8.9\n", ""},
		{"empty version", "ARG KEEPALIVED_VERSION=\nARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
		{"prerelease", "ARG KEEPALIVED_VERSION=7.8.9-rc1\nARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
		{"short checksum", "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_SHA256=abcd\n", ""},
		{"invalid checksum", "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_SHA256=" + strings.Repeat("zz", 32) + "\n", ""},
		{"duplicate version", "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_VERSION=8.9.0\nARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
		{"malformed duplicate", "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_VERSION=8.9.0 extra\nARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
		{"comment is no declaration", "# ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_SHA256=" + checksum + "\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, rendererKeepalivedPin), test.body)
			arguments, err := rendererPeerArguments(root)
			if test.want == "" {
				if err == nil {
					t.Fatalf("invalid declarations accepted: %v", arguments)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if arguments[1] != "KEEPALIVED_VERSION="+test.want {
				t.Errorf("release = %q, want %q", arguments[1], test.want)
			}
			if arguments[2] != "KEEPALIVED_SHA256="+checksum {
				t.Errorf("checksum = %q, want %q", arguments[2], checksum)
			}
		})
	}
}

// TestRendererPeerArgumentsRefusesMissingPin proves an unreadable canonical
// declaration is an error, never a fallback to a distro package or copied pin.
func TestRendererPeerArgumentsRefusesMissingPin(t *testing.T) {
	if arguments, err := rendererPeerArguments(t.TempDir()); err == nil {
		t.Fatalf("missing canonical declaration accepted: %v", arguments)
	}
}

// TestRendererPeerPinChangeInvalidatesArtifact renders a fixture, changes only
// one canonical peer input, and asks the artifact checker to judge the old output.
func TestRendererPeerPinChangeInvalidatesArtifact(t *testing.T) {
	for _, test := range []struct {
		name     string
		version  string
		checksum string
	}{
		{"release", "8.9.0", strings.Repeat("ab", 32)},
		{"checksum", "7.8.9", strings.Repeat("cd", 32)},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDemoFixture(t)
			pin := filepath.Join(fixture.root, rendererKeepalivedPin)
			mustWrite(t, pin, "ARG KEEPALIVED_VERSION=7.8.9\nARG KEEPALIVED_SHA256="+strings.Repeat("ab", 32)+"\n")
			engine := fixture.engine(&bytes.Buffer{})
			if _, err := engine.RenderAll("26.08.27"); err != nil {
				t.Fatalf("render fixture: %v", err)
			}
			if _, err := engine.checkAll(""); err != nil {
				t.Fatalf("unchanged peer pin rejected: %v", err)
			}
			mustWrite(t, pin, "ARG KEEPALIVED_VERSION="+test.version+"\nARG KEEPALIVED_SHA256="+test.checksum+"\n")
			_, err := engine.checkAll("")
			if err == nil {
				t.Fatal("canonical peer pin changed but the old artifact is current")
			}
			if err.Error() != "term: source changed since the last render" {
				t.Fatalf("peer pin drift error = %v", err)
			}
		})
	}
}
