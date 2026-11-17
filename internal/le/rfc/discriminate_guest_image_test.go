// Design: docs/contributing/rfc-conformance-gates.md -- explicit existing guest pair selection
// Related: actions.go -- the actual registered record grammar
package rfc

import (
	"path/filepath"
	"testing"
)

// The public record grammar must retain an explicitly selected image with its
// kernel. This proves argument admission only: it does not build, boot, inspect
// an image or record a protocol proof. The old table must fail this named test
// because it refuses the image operand before any observation can start.
func TestRecordGrammarAcceptsExplicitGuestImage(t *testing.T) {
	root := t.TempDir()
	kernel := filepath.Join(root, "accepted kernel", "vmlinuz")
	image := filepath.Join(root, "accepted image", "docker-host.img")
	parsed, err := actions.Parse("record", []string{
		"id", "RFC2516-5.5-4", "polarity", "positive",
		"unit", "internal/component/l2tp/pppoe/rfc2516_padt_lifetime_linux_test.go::TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport",
		"route", "revert", "producer", "internal/component/l2tp/pppoe/server.go::handlePADT",
		"kernel", kernel, "image", image,
	})
	if err != nil {
		t.Fatalf("record grammar refused the explicit guest image/kernel pair: %v", err)
	}
	if !parsed.Has("image") || parsed.One("image") != image {
		t.Fatalf("record grammar lost the requested image: got %q, want %q", parsed.One("image"), image)
	}
	if parsed.One("kernel") != kernel {
		t.Fatalf("record grammar changed the requested kernel: got %q, want %q", parsed.One("kernel"), kernel)
	}
}
