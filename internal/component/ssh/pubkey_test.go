package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
)

func generateEd25519Key(t *testing.T) (gossh.PublicKey, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	b64 := base64.StdEncoding.EncodeToString(sshPub.Marshal())
	return sshPub, b64
}

func TestPublicKeyMatch(t *testing.T) {
	key, b64 := generateEd25519Key(t)

	users := []authz.UserConfig{
		{
			Name:     "alice",
			Profiles: []string{"admin"},
			PublicKeys: []authz.SSHPublicKey{
				{Name: "laptop", Type: "ssh-ed25519", Key: b64},
			},
		},
	}

	profiles := matchPublicKey(users, "alice", key)
	assert.Equal(t, []string{"admin"}, profiles)
}

func TestPublicKeyNoMatch(t *testing.T) {
	_, b64 := generateEd25519Key(t)
	otherKey, _ := generateEd25519Key(t)

	users := []authz.UserConfig{
		{
			Name:     "alice",
			Profiles: []string{"admin"},
			PublicKeys: []authz.SSHPublicKey{
				{Name: "laptop", Type: "ssh-ed25519", Key: b64},
			},
		},
	}

	profiles := matchPublicKey(users, "alice", otherKey)
	assert.Nil(t, profiles)
}

func TestPublicKeyLookupMultipleKeys(t *testing.T) {
	key1, b64_1 := generateEd25519Key(t)
	_, b64_2 := generateEd25519Key(t)

	users := []authz.UserConfig{
		{
			Name:     "bob",
			Profiles: []string{"operator"},
			PublicKeys: []authz.SSHPublicKey{
				{Name: "desktop", Type: "ssh-ed25519", Key: b64_1},
				{Name: "phone", Type: "ssh-ed25519", Key: b64_2},
			},
		},
	}

	profiles := matchPublicKey(users, "bob", key1)
	assert.Equal(t, []string{"operator"}, profiles)
}

func TestPublicKeyLookupUnknownUser(t *testing.T) {
	key, _ := generateEd25519Key(t)

	users := []authz.UserConfig{
		{Name: "alice", Profiles: []string{"admin"}},
	}

	profiles := matchPublicKey(users, "unknown", key)
	assert.Nil(t, profiles)
}

func TestPublicKeyLookupUserNoKeys(t *testing.T) {
	key, _ := generateEd25519Key(t)

	users := []authz.UserConfig{
		{Name: "carol", Hash: "$2a$10$dummy", Profiles: []string{"read-only"}},
	}

	profiles := matchPublicKey(users, "carol", key)
	assert.Nil(t, profiles)
}

func TestParseConfiguredKeyInvalidBase64(t *testing.T) {
	_, err := parseConfiguredKey("ssh-ed25519", "not-valid-base64!!!")
	assert.Error(t, err)
}

func TestParseConfiguredKeyEmptyFields(t *testing.T) {
	_, err := parseConfiguredKey("", "AAAA")
	assert.Error(t, err)

	_, err = parseConfiguredKey("ssh-ed25519", "")
	assert.Error(t, err)
}
