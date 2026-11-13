// Design: docs/guide/authentication.md -- complete local bootstrap key files.
// Related: authorizedkeys.go -- no partial acceptance or discarded restrictions.

package authorizedkeys

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"golang.org/x/crypto/ssh"
)

// FuzzParse checks atomic refusal, input ownership and returned key validity.
// Positive and negative daemon/assembly tests independently pin credential use.
func FuzzParse(f *testing.F) {
	key, err := ssh.NewPublicKey(ed25519.PublicKey(bytes.Repeat([]byte{1}, ed25519.PublicKeySize)))
	if err != nil {
		f.Fatal(err)
	}
	line := ssh.MarshalAuthorizedKey(key)
	for _, seed := range [][]byte{
		{},
		[]byte("\r\n# no public keys\n"),
		line,
		append([]byte("# first\r\n"), line...),
		append(bytes.Clone(line), line...),
		append([]byte("invalid\n"), line...),
		append(bytes.Clone(line), []byte("invalid\n")...),
		append([]byte("restrict "), line...),
		append([]byte("from=\"192.0.2.0/24\" "), line...),
		append([]byte("command=\"show version\" "), line...),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		before := bytes.Clone(content)
		keys, err := Parse(content)
		if !bytes.Equal(content, before) {
			t.Fatal("key parsing changed its caller's input")
		}
		if err != nil {
			if keys != nil {
				t.Fatal("refused key file exposed partial keys")
			}
			return
		}
		for _, key := range keys {
			decoded, err := ssh.ParsePublicKey(key.Marshal())
			if err != nil {
				t.Fatalf("accepted key is not a valid SSH public-key blob: %v", err)
			}
			if decoded.Type() != key.Type() {
				t.Fatal("accepted key's algorithm disagrees with its encoded blob")
			}
		}
	})
}
