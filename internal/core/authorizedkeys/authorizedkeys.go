// Design: docs/guide/authentication.md -- local bootstrap SSH public keys.
// Related: cmd/ze/hub/main_servers.go -- accepted local credential assembly.
// Upstream: golang.org/x/crypto v0.58.0, ssh/keys.go, ParseAuthorizedKey;
// https://github.com/golang/crypto/blob/v0.58.0/ssh/keys.go.

// Package authorizedkeys validates the public keys supplied for bootstrap login.
package authorizedkeys

import (
	"bytes"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/core/opererr"
)

// Parse accepts complete plain OpenSSH public-key lines without authorized_keys
// options. Empty input and comments mean no public keys. A malformed line,
// certificate or unsupported restriction refuses the file without partial keys.
func Parse(content []byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for line := range bytes.SplitSeq(content, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			continue
		}
		// The upstream parser skips bad lines when given a whole file. Giving
		// it one line prevents a later valid key from hiding an earlier error.
		key, _, options, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			return nil, opererr.New("bootstrap SSH public key is invalid", "The local recovery keys were refused; supply a complete OpenSSH public key", "ssh-keygen -y -f <recovery-private-key>")
		}
		if len(options) > 0 {
			return nil, opererr.New("bootstrap SSH public key has unsupported authorized_keys options", "Ze cannot enforce these restrictions for a recovery account; supply a dedicated unrestricted recovery public key", "ssh-keygen -y -f <recovery-private-key>")
		}
		if _, certificate := key.(*ssh.Certificate); certificate {
			return nil, opererr.New("bootstrap SSH certificates are unsupported", "Recovery login accepts plain public keys; certificate validity, principals and critical options cannot be discarded", "ssh-keygen -y -f <recovery-private-key>")
		}
		keys = append(keys, key)
	}
	return keys, nil
}
