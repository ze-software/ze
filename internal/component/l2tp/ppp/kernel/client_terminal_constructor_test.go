// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- complete terminal operations
// Related: client_control_test.go -- fresh fully valid constructor inputs

package kernel

import "testing"

// TestClientPairRefusesMissingDisconnect removes only the newly required
// terminal operation from fresh valid inputs. Refusal must consume both new
// owners rather than admitting a pair that cannot execute its shutdown contract.
func TestClientPairRefusesMissingDisconnect(t *testing.T) {
	cfg, ops, closers, released, _ := clientConfigurationInputs(t)
	ops.disconnect = nil
	client, err := newClientSession(cfg, ops, closers)
	if client != nil {
		t.Fatal("missing terminal operation admitted client")
	}
	if err == nil {
		t.Fatal("missing terminal operation had no cause")
	}
	if len(*released) != 2 {
		t.Fatalf("refusal release count = %v", *released)
	}
	if (*released)[0] != 31 {
		t.Fatalf("channel refusal order = %v", *released)
	}
	if (*released)[1] != 32 {
		t.Fatalf("unit refusal order = %v", *released)
	}
}
