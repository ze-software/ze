package geodns

import "testing"

// VALIDATES: the listen-capability check warns only for an enabled geodns on a
// privileged port (<1024) that cannot be bound; disabled geodns, non-privileged
// ports, and bindable privileged ports produce no diagnostic.
// PREVENTS: a false positive on the default port 5300, and a missed
// CAP_NET_BIND_SERVICE warning when an operator moves geodns onto port 53.
func TestGeodnsListenDiagnostic(t *testing.T) {
	t.Parallel()
	bindOK := func(string, int) bool { return true }
	bindFail := func(string, int) bool { return false }
	priv := []probeTarget{{host: "127.0.0.1", port: 53}, {host: "::1", port: 53}}
	unpriv := []probeTarget{{host: "127.0.0.1", port: 5300}}

	if d := geodnsListenDiagnostic(false, priv, bindFail); d != nil {
		t.Errorf("disabled geodns should produce no diagnostic, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, unpriv, bindFail); d != nil {
		t.Errorf("non-privileged port should produce no diagnostic, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, priv, bindOK); d != nil {
		t.Errorf("bindable privileged port should produce no diagnostic, got %v", d)
	}
	d := geodnsListenDiagnostic(true, priv, bindFail)
	if len(d) != 1 || d[0].Code != "doctor-geodns-port-unavailable" || d[0].Severity != "warning" {
		t.Errorf("expected a port-unavailable warning, got %v", d)
	}
	// Boundary: 1023 is the last privileged port (checked), 1024 is the first
	// unprivileged port (skipped).
	if d := geodnsListenDiagnostic(true, []probeTarget{{host: "127.0.0.1", port: 1023}}, bindFail); len(d) != 1 {
		t.Errorf("port 1023 (privileged boundary) should warn, got %v", d)
	}
	if d := geodnsListenDiagnostic(true, []probeTarget{{host: "127.0.0.1", port: 1024}}, bindFail); d != nil {
		t.Errorf("port 1024 (first unprivileged) should not warn, got %v", d)
	}
}
