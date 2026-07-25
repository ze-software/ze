// VALIDATES: AC-2 / C-2 -- resolveNexthopIndex, when no iface backend is loaded,
// returns an error that names BOTH the next-hop interface and the missing
// `interface { backend ... }` stanza, instead of the bare "iface: no backend
// loaded" the shared resolver produces.
// PREVENTS: an operator seeing an unactionable error and not knowing an
// interface backend stanza is required for interface-only static next-hops.

//go:build linux

package static

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

func TestResolveNexthopIndexNoBackendErrorIsDiagnosable(t *testing.T) {
	// Ensure the no-backend precondition regardless of test ordering.
	if iface.ActiveBackendName() != "" {
		_ = iface.CloseBackend()
	}

	const dev = "nostaticbackend0" // unique: iface.Resolve caches per logical name
	_, err := resolveNexthopIndex(dev)
	if err == nil {
		t.Fatal("resolveNexthopIndex: want an error with no iface backend loaded")
	}
	msg := err.Error()
	if !strings.Contains(msg, dev) {
		t.Errorf("error must name the interface %q: %q", dev, msg)
	}
	if !strings.Contains(msg, "interface { backend") {
		t.Errorf("error must name the missing `interface { backend ... }` stanza: %q", msg)
	}
}
