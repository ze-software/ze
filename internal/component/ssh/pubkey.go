// Design: docs/architecture/system-architecture.md -- SSH public key authentication
// Related: ssh.go -- SSH server wires this into wish.WithPublicKeyAuth

package ssh

import (
	"errors"
	"fmt"
	"log/slog"

	"charm.land/ssh"

	"github.com/ze-software/ze/internal/component/authz"
)

var errMissingTypeOrKeyData = errors.New("missing type or key data")

// matchPublicKey checks whether the presented SSH public key matches any of
// the configured keys for the given username. Returns the matched user's
// profiles on success, or nil on failure.
func matchPublicKey(users []authz.UserConfig, username string, presented ssh.PublicKey) []string {
	for _, u := range users {
		if u.Name != username {
			continue
		}
		for _, pk := range u.PublicKeys {
			configured, err := parseConfiguredKey(pk.Type, pk.Key)
			if err != nil {
				slog.Warn("SSH public key parse error",
					"username", username,
					"key_name", pk.Name,
					"error", err)
				continue
			}
			if ssh.KeysEqual(presented, configured) {
				return u.Profiles
			}
		}
	}
	return nil
}

// parseConfiguredKey reconstructs an ssh.PublicKey from the stored type and
// base64-encoded key data. The type prefix and key data are concatenated into
// authorized_keys format and parsed by ssh.ParseAuthorizedKey.
func parseConfiguredKey(keyType, keyData string) (ssh.PublicKey, error) {
	if keyType == "" || keyData == "" {
		return nil, errMissingTypeOrKeyData
	}
	line := keyType + " " + keyData
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return key, nil
}
