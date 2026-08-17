// Design: docs/architecture/system-architecture.md -- SSH public key authentication
// Related: ssh.go -- SSH server wires this into wish.WithPublicKeyAuth

package ssh

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

	"charm.land/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

var errMissingTypeOrKeyData = errors.New("missing type or key data")

// authenticatePublicKeyResult returns the authentication result resolved from
// the same user snapshot that accepted the key. A later login for the same
// username cannot replace this connection's authorization view.
func (s *Server) authenticatePublicKeyResult(username string, presented ssh.PublicKey, peer net.Addr) aaa.AuthResult {
	remote := ""
	if peer != nil {
		remote = peer.String()
	}
	users, err := s.users()
	if err != nil {
		s.logger.Warn("SSH auth failure: cannot read the running config users",
			"username", username, "remote", remote, "error", err)
		s.recordAuthFailure(username, remote)
		return aaa.AuthResult{}
	}
	profiles, matched := matchPublicKey(users, username, presented)
	if !matched {
		s.logger.Warn("SSH auth failure", "username", username, "remote", remote, "source", "public-key")
		s.recordAuthFailure(username, remote)
		return aaa.AuthResult{}
	}
	var generation uint64
	for _, user := range users {
		if user.Name == username {
			generation = user.LocalGeneration
			break
		}
	}
	s.logger.Info("SSH auth success",
		"username", username, "remote", remote,
		"source", "public-key",
		"profiles", truncateProfiles(profiles))
	return aaa.AuthResult{
		Authenticated:   true,
		Source:          aaa.SourceLocal,
		Profiles:        profiles,
		LocalGeneration: generation,
	}
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
