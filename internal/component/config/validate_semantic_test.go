package config

import "testing"

// gnmiTree builds an enabled environment.gnmi block with one server endpoint on
// the default gNMI port.
func gnmiTree(host, token string) *Tree {
	tree := NewTree()
	env := tree.GetOrCreateContainer("environment")
	gnmi := env.GetOrCreateContainer("gnmi")
	gnmi.Set("enabled", "true")
	if token != "" {
		gnmi.Set("token", token)
	}
	srv := NewTree()
	srv.Set("ip", host)
	srv.Set("port", "9339")
	gnmi.AddListEntry("server", "default", srv)
	return tree
}

// gnmiFlagged reports whether ValidateSemantics emitted config-gnmi-invalid for
// the tree. The entry point is deliberate: doctor reaches gNMI exposure only
// through ValidateSemantics, never through GNMIListenConfig.Validate directly.
func gnmiFlagged(tree *Tree) bool {
	diags := ValidateSemantics(tree)
	for i := range diags {
		if diags[i].Code == "config-gnmi-invalid" {
			return true
		}
	}
	return false
}

func TestValidateSemanticsFlagsGNMI(t *testing.T) {
	// VALIDATES: AC-6 -- the gNMI exposure is reported through the semantic
	// entry point ValidateSemantics, not only by the GNMIListenConfig.Validate
	// helper. `ze doctor` reaches gNMI exposure only through this function
	// (internal/component/doctor/checks_config.go checkSemanticValidation), so a
	// Validate that nothing calls leaves doctor blind to the exposure.
	// PREVENTS: an operator config binding gNMI to 0.0.0.0 with no token passing
	// `ze doctor` clean while the daemon refuses to boot on it.
	t.Run("non-loopback without token is flagged", func(t *testing.T) {
		if !gnmiFlagged(gnmiTree("0.0.0.0", "")) {
			t.Fatal("expected config-gnmi-invalid for a tokenless 0.0.0.0 gNMI listener")
		}
	})

	t.Run("loopback without token is clean", func(t *testing.T) {
		if gnmiFlagged(gnmiTree("127.0.0.1", "")) {
			t.Fatal("loopback gNMI without a token must not be flagged")
		}
	})

	t.Run("non-loopback with token is clean", func(t *testing.T) {
		if gnmiFlagged(gnmiTree("0.0.0.0", "s3cret")) {
			t.Fatal("an authenticated gNMI listener must not be flagged")
		}
	})

	t.Run("gnmi block absent is clean", func(t *testing.T) {
		if gnmiFlagged(NewTree()) {
			t.Fatal("a tree with no gnmi block must not be flagged")
		}
	})

	t.Run("gnmi disabled is clean", func(t *testing.T) {
		// A block with enabled false never starts a listener, so it exposes
		// nothing and must not refuse an otherwise-good config.
		tree := gnmiTree("0.0.0.0", "")
		tree.GetContainer("environment").GetContainer("gnmi").Set("enabled", "false")
		if gnmiFlagged(tree) {
			t.Fatal("a disabled gNMI block must not be flagged")
		}
	})

	t.Run("default endpoint with no server entry is flagged", func(t *testing.T) {
		// ExtractGNMIConfig synthesizes 0.0.0.0:9339 when the block names no
		// server, which is the exposure operators hit without writing an
		// address at all.
		tree := NewTree()
		env := tree.GetOrCreateContainer("environment")
		gnmi := env.GetOrCreateContainer("gnmi")
		gnmi.Set("enabled", "true")
		if !gnmiFlagged(tree) {
			t.Fatal("expected config-gnmi-invalid for the synthesized 0.0.0.0:9339 default")
		}
	})
}
