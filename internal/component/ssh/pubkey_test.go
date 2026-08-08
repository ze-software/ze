package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/core/audit"
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

	profiles, matched := matchPublicKey(users, "alice", key)
	assert.True(t, matched)
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

	profiles, matched := matchPublicKey(users, "alice", otherKey)
	assert.False(t, matched)
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

	profiles, matched := matchPublicKey(users, "bob", key1)
	assert.True(t, matched)
	assert.Equal(t, []string{"operator"}, profiles)
}

func TestPublicKeyLookupUnknownUser(t *testing.T) {
	key, _ := generateEd25519Key(t)

	users := []authz.UserConfig{
		{Name: "alice", Profiles: []string{"admin"}},
	}

	profiles, matched := matchPublicKey(users, "unknown", key)
	assert.False(t, matched)
	assert.Nil(t, profiles)
}

func TestPublicKeyLookupUserNoKeys(t *testing.T) {
	key, _ := generateEd25519Key(t)

	users := []authz.UserConfig{
		{Name: "carol", Hash: "$2a$10$dummy", Profiles: []string{"read-only"}},
	}

	profiles, matched := matchPublicKey(users, "carol", key)
	assert.False(t, matched)
	assert.Nil(t, profiles)
}

// VALIDATES: a key match is reported as a match even when the user carries no
// profile. The `profile` leaf-list is optional (ze-ssh-conf.yang), so this
// configuration is legal.
// PREVENTS: reading the match off the emptiness of the profiles, which refused
// every profile-less user's key while the same account logged in by password --
// a zero value that read as a valid answer (ai/rules/evidence.md).
func TestPublicKeyMatchUserWithoutProfiles(t *testing.T) {
	key, b64 := generateEd25519Key(t)

	users := []authz.UserConfig{
		{
			Name: "dave",
			PublicKeys: []authz.SSHPublicKey{
				{Name: "laptop", Type: "ssh-ed25519", Key: b64},
			},
		},
	}

	profiles, matched := matchPublicKey(users, "dave", key)
	assert.True(t, matched, "the key matches, so authentication must succeed")
	assert.Nil(t, profiles, "the user carries no profile; what they may run is the authorizer's decision")
}

// generateEd25519Signer returns a client-side signer and the base64 key data in
// the form the configuration file stores, so one key can be both offered by a
// real SSH client and declared in a user entry.
func generateEd25519Signer(t *testing.T) (gossh.Signer, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	return signer, base64.StdEncoding.EncodeToString(sshPub.Marshal())
}

// dialWithKey completes a real SSH handshake offering exactly one public key
// and no other authentication method, so the returned error is the server's
// public-key decision and nothing else.
func dialWithKey(addr, username string, signer gossh.Signer) error {
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            username,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // the test server generates a fresh host key per run
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return err
	}
	return client.Close()
}

// VALIDATES: AC-11, AC-12 -- SSH public-key authentication answers from the
// RUNNING configuration: a user a reload removed is refused and the refusal is
// audited, while a user the same reload kept still authenticates by key.
// PREVENTS: the boot user list outliving the reload that deleted a user, which
// gave that user a shell with config-edit rights until the daemon restarted.
//
// The fixture is built so a stale read cannot pass: Config.Users keeps BOTH
// users for the whole test, and only the live source loses one. A server that
// consults the snapshot accepts goneuser after the reload.
func TestPublicKeyAuthFollowsRunningConfig(t *testing.T) {
	keptSigner, keptKey := generateEd25519Signer(t)
	goneSigner, goneKey := generateEd25519Signer(t)
	bareSigner, bareKey := generateEd25519Signer(t)

	kept := authz.UserConfig{
		Name:       "keepuser",
		Profiles:   []string{"admin"},
		PublicKeys: []authz.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: keptKey}},
	}
	gone := authz.UserConfig{
		Name:       "goneuser",
		Profiles:   []string{"admin"},
		PublicKeys: []authz.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: goneKey}},
	}
	// A legal user with no profile leaf: the leaf-list is optional in YANG.
	bare := authz.UserConfig{
		Name:       "bareuser",
		PublicKeys: []authz.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: bareKey}},
	}

	var mu sync.Mutex
	running := []authz.UserConfig{kept, gone, bare}

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	srv, err := NewServer(Config{
		Listen:      "127.0.0.1:0",
		HostKeyPath: t.TempDir() + "/test_host_key",
		Users:       []authz.UserConfig{kept, gone, bare},
		UsersFunc: func() ([]authz.UserConfig, error) {
			mu.Lock()
			defer mu.Unlock()
			return running, nil
		},
		AuditRecorder: recorder,
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background(), nil, nil))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(stopCtx))
	})

	addr := srv.Address()
	require.Eventually(t, func() bool {
		return dialWithKey(addr, "keepuser", keptSigner) == nil
	}, 5*time.Second, 20*time.Millisecond, "the configured key authenticates once the server serves")
	require.NoError(t, dialWithKey(addr, "goneuser", goneSigner),
		"both users authenticate before the reload, so the refusal below is the reload's doing")

	// The reload removes goneuser from the running configuration.
	mu.Lock()
	running = []authz.UserConfig{kept, bare}
	mu.Unlock()

	assert.Error(t, dialWithKey(addr, "goneuser", goneSigner),
		"AC-11: a user the reload removed must be refused when presenting their configured key")
	assert.NoError(t, dialWithKey(addr, "keepuser", keptSigner),
		"AC-12: a user the reload kept must still authenticate by key")
	assert.NoError(t, dialWithKey(addr, "bareuser", bareSigner),
		"AC-12: a kept user with no profile leaf authenticates too; profiles are the authorizer's business")

	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.NotEmpty(t, entries, "AC-11: the refusal is recorded like any other SSH auth failure")
	assert.Equal(t, "goneuser", entries[0].Actor)
	assert.Equal(t, audit.SSH, entries[0].Surface)
	assert.Equal(t, audit.OutcomeDenied, entries[0].Outcome)
}

// VALIDATES: an unreadable running configuration refuses the key rather than
// authenticating from the boot snapshot (ai/rules/evidence.md, fail closed).
// PREVENTS: a wiring fault reading as "the configuration declares no users",
// which would silently fall back to whoever existed at startup.
func TestPublicKeyAuthDeniesWhenRunningConfigUnreadable(t *testing.T) {
	signer, key := generateEd25519Signer(t)
	user := authz.UserConfig{
		Name:       "alice",
		Profiles:   []string{"admin"},
		PublicKeys: []authz.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: key}},
	}

	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)

	srv, err := NewServer(Config{
		Listen:        "127.0.0.1:0",
		HostKeyPath:   t.TempDir() + "/test_host_key",
		Users:         []authz.UserConfig{user},
		UsersFunc:     func() ([]authz.UserConfig, error) { return nil, errMissingTypeOrKeyData },
		AuditRecorder: recorder,
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background(), nil, nil))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Stop(stopCtx))
	})

	addr := srv.Address()
	require.Eventually(t, func() bool {
		return dialWithKey(addr, "alice", signer) != nil
	}, 5*time.Second, 20*time.Millisecond,
		"a configuration that cannot be read must refuse the key held in the boot snapshot")

	entries := recorder.Query(audit.Filter{Action: audit.ActionAuthFail})
	require.NotEmpty(t, entries, "the refusal is audited, so an unreadable config is visible")
	assert.Equal(t, "alice", entries[0].Actor)
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
