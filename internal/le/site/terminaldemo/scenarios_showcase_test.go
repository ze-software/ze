package siteterminaldemo

import "testing"

// TestShowcasePeersRequireEachNamedSession checks that readiness associates
// state with the requested peer. An unrelated Established session must not
// substitute for a failed or absent fixture.
func TestShowcasePeersRequireEachNamedSession(t *testing.T) {
	for _, test := range []struct {
		name     string
		document string
		ready    bool
	}{
		{"ready", `{"peers":{"127.0.0.2":{"name":"transit","state":"established"},"172.29.0.2":{"name":"frr","state":"established"}}}`, true},
		{"unrelated-established", `{"peers":{"127.0.0.2":{"name":"transit","state":"established"},"172.29.0.2":{"name":"frr","state":"active"},"127.0.0.4":{"name":"customer","state":"established"}}}`, false},
		{"missing-frr", `{"peers":{"127.0.0.2":{"name":"transit","state":"established"},"127.0.0.4":{"name":"customer","state":"established"}}}`, false},
		{"missing-envelope", `{}`, false},
		{"malformed-answer", `not JSON`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireShowcasePeers(test.document, []string{"transit", "frr"})
			if (err == nil) != test.ready {
				t.Fatalf("readiness error = %v, want ready %t", err, test.ready)
			}
		})
	}
}
