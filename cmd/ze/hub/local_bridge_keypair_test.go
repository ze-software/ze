// Design: docs/architecture/api/commands.md -- local-data answers on local and daemon routes.
// Related: local_bridge_match_test.go -- local and daemon command contracts.
// Related: internal/plugins/diag/diag.go -- independently random key answers.
package hub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// matchGeneratedKeypairs checks the whole key-generation answer on each
// route. The cryptographic relation is derived independently of the producer's
// wgtypes library. Failure messages never include private key material.
func matchGeneratedKeypairs(t *testing.T, local, remote string) {
	t.Helper()
	localPrivate, localPublic := matchGeneratedKeypair(t, "local", local)
	remotePrivate, remotePublic := matchGeneratedKeypair(t, "daemon", remote)
	if bytes.Equal(localPrivate, remotePrivate) {
		t.Error("independent key-generation calls returned the same private key")
	}
	if bytes.Equal(localPublic, remotePublic) {
		t.Error("independent key-generation calls returned the same public key")
	}
}

func matchGeneratedKeypair(t *testing.T, route, answer string) (private, public []byte) {
	t.Helper()
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer), &document); err != nil {
		t.Fatalf("%s keypair answer must be a JSON object", route)
	}
	if len(document) != 1 {
		t.Fatalf("%s keypair answer must contain exactly the keypair envelope", route)
	}
	var pair map[string]string
	if err := json.Unmarshal(document["wireguard-keypair"], &pair); err != nil {
		t.Fatalf("%s keypair envelope must be an object of key strings", route)
	}
	if len(pair) != 2 {
		t.Fatalf("%s keypair envelope must contain exactly two keys", route)
	}
	keys := make([][]byte, 0, 2)
	for _, field := range []string{"private-key", "public-key"} {
		encoded, present := pair[field]
		if !present {
			t.Fatalf("%s keypair envelope must contain %s", route, field)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s %s must be base64 encoded", route, field)
		}
		if len(decoded) != curve25519.ScalarSize {
			t.Fatalf("%s %s must contain 32 bytes", route, field)
		}
		if base64.StdEncoding.EncodeToString(decoded) != encoded {
			t.Fatalf("%s %s must use canonical base64 encoding", route, field)
		}
		keys = append(keys, decoded)
	}
	private, public = keys[0], keys[1]
	if private[0]&7 != 0 {
		t.Fatalf("%s private key must have its low three bits cleared", route)
	}
	if private[31]&128 != 0 {
		t.Fatalf("%s private key must have its top bit cleared", route)
	}
	if private[31]&64 == 0 {
		t.Fatalf("%s private key must have bit 254 set", route)
	}
	derived, err := curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("%s private key cannot derive a public key", route)
	}
	if !bytes.Equal(derived, public) {
		t.Fatalf("%s public key does not derive from its private key", route)
	}
	return private, public
}
