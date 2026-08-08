// Design: docs/architecture/system-architecture.md -- SSH public key authentication
// Related: ssh.go -- SSH server wires this into wish.WithPublicKeyAuth

package ssh

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

	"charm.land/ssh"

	"github.com/ze-software/ze/internal/component/authz"
)

var errMissingTypeOrKeyData = errors.New("missing type or key data")

// authenticatePublicKey runs the SSH public-key decision for one connection.
// It reads the credentials that are valid RIGHT NOW through Server.users(), so
// a user the operator deleted and reloaded is refused at the next connection
// instead of keeping their shell until the daemon restarts. Extracted from the
// wish callback for the same reason authenticatePassword was: the decision is
// then testable without standing up a live SSH server, and both auth methods
// record a refusal the same way.
//
// A user list that cannot be read is a refusal, not an empty list, and it is
// logged with its cause: the audit record alone says "denied" and cannot tell
// an unknown key from an unreadable configuration.
func (s *Server) authenticatePublicKey(username string, presented ssh.PublicKey, peer net.Addr) bool {
	remote := ""
	if peer != nil {
		remote = peer.String()
	}
	users, err := s.users()
	if err != nil {
		s.logger.Warn("SSH auth failure: cannot read the running config users",
			"username", username, "remote", remote, "error", err)
		s.recordAuthFailure(username, remote)
		return false
	}
	profiles, matched := matchPublicKey(users, username, presented)
	if !matched {
		s.logger.Warn("SSH auth failure", "username", username, "remote", remote, "source", "public-key")
		s.recordAuthFailure(username, remote)
		return false
	}
	s.logger.Info("SSH auth success",
		"username", username, "remote", remote,
		"source", "public-key",
		"profiles", truncateProfiles(profiles))
	return true
}

// matchPublicKey reports whether the presented SSH public key matches any of
// the configured keys for the given username, and returns that user's profiles.
//
// The match is the SECOND return value, never the emptiness of the first. The
// profile leaf-list is optional in YANG (ze-ssh-conf.yang, leaf-list profile),
// so a user configured with a key and no profile has nil profiles and a real
// key match. Reading the match off the profiles refused that user on every
// connection while the same account logged in by PASSWORD, because
// authz.LocalAuthenticator authenticates on the hash and carries the profiles
// along. What a user may then RUN is the authorizer's decision
// (authz.Store.Authorize), not this function's.
func matchPublicKey(users []authz.UserConfig, username string, presented ssh.PublicKey) ([]string, bool) {
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
				return u.Profiles, true
			}
		}
	}
	return nil, false
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
