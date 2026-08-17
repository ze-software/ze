//go:build ze_ssh && ze_bgp

package hub

import (
	"context"
	"crypto/ed25519"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// sshKeyPair returns a client-side signer and the base64 key data in the form
// the configuration file stores it.
func sshKeyPair(t *testing.T) (gossh.Signer, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(cryptoRand.Reader)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	sshPub, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	return signer, base64.StdEncoding.EncodeToString(sshPub.Marshal())
}

// sshDialWithKey completes a real SSH handshake offering one public key and no
// other authentication method, so the returned error is the server's public-key
// decision and nothing else.
func sshDialWithKey(addr, username string, signer gossh.Signer) error {
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

// TestInfraSetupSSHPublicKeyFollowsRunningConfig drives the daemon's own SSH
// wiring: infraSetup builds and starts the server through the sshBuild seam, and
// a real client then offers a configured key against the bound listener.
//
// VALIDATES: AC-11, AC-12 -- the live user source reaches the public-key handler
// through infraSetup -> sshBuild -> zessh.Config.UsersFunc, so a user a reload
// removed is refused while a user it kept still authenticates.
// PREVENTS: the field being added to the seam and never threaded, which the
// package-level test cannot see: it constructs the server itself. The web half
// of this spec shipped exactly that fault once (a second stale snapshot in front
// of the fixed one), and only a wiring test found it.
func TestInfraSetupSSHPublicKeyFollowsRunningConfig(t *testing.T) {
	require.NoError(t, env.Set("ze.ssh.ephemeral", ""))
	defer func() { _ = env.Set("ze.ssh.ephemeral", "") }()

	keptSigner, keptKey := sshKeyPair(t)
	goneSigner, goneKey := sshKeyPair(t)

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

	// The boot list carries BOTH users for the whole test and only the live
	// source loses one, so any answer read from the boot list accepts goneuser
	// after the reload and this test cannot pass on a stale read.
	var mu sync.Mutex
	running := []aaa.UserCredential{kept, gone}
	liveUsers := func() ([]aaa.UserCredential, error) {
		mu.Lock()
		defer mu.Unlock()
		return running, nil
	}

	params := infra.HookParams{
		Reactor: reactor.New(&reactor.Config{}),
		SSHConfig: infra.SSHExtractedConfig{
			Listen:      "127.0.0.1:0",
			HostKeyPath: t.TempDir() + "/test_host_key",
			HasConfig:   true,
		},
	}

	sshSrv := infraSetup(params, nil, nil, liveUsers)
	require.NotNil(t, sshSrv, "infraSetup should return a running SSH server")
	addr := sshSrv.Address()
	stopper, ok := sshSrv.(interface{ Stop(context.Context) error })
	require.True(t, ok, "the SSH test server must expose its shutdown method")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, stopper.Stop(ctx))
	})

	require.Eventually(t, func() bool {
		return sshDialWithKey(addr, "keepuser", keptSigner) == nil
	}, 5*time.Second, 20*time.Millisecond, "the configured key authenticates once the server serves")
	require.NoError(t, sshDialWithKey(addr, "goneuser", goneSigner),
		"both users authenticate before the reload, so the refusal below is the reload's doing")

	// The reload removes goneuser from the running configuration.
	mu.Lock()
	running = []aaa.UserCredential{kept}
	mu.Unlock()

	assert.Error(t, sshDialWithKey(addr, "goneuser", goneSigner),
		"AC-11: the daemon's own SSH wiring must refuse a user the reload removed")
	assert.NoError(t, sshDialWithKey(addr, "keepuser", keptSigner),
		"AC-12: a user the reload kept must still authenticate by key")
}

// TestSSHStandaloneBuildPublicKeyFollowsRunningConfig covers the OTHER server
// construction site. sshBuildStandaloneImpl serves the no-bgp{} daemon (the
// gokrazy appliance, an environment{}-only config), which never reaches
// infraSetup, so the test above says nothing about it.
//
// VALIDATES: AC-11, AC-12 on the no-bgp{} startup path.
// PREVENTS: one of the two zessh.NewServer call sites keeping the boot snapshot,
// which would leave the appliance -- the most exposed deployment -- unfixed.
func TestSSHStandaloneBuildPublicKeyFollowsRunningConfig(t *testing.T) {
	keptSigner, keptKey := sshKeyPair(t)
	goneSigner, goneKey := sshKeyPair(t)

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

	var mu sync.Mutex
	running := []aaa.UserCredential{kept, gone}

	addrFile := filepath.Join(t.TempDir(), "ssh.addr")
	stop := sshBuildStandaloneImpl(&sshStandaloneInputs{
		Config: infra.SSHExtractedConfig{
			Listen:      "127.0.0.1:0",
			HostKeyPath: t.TempDir() + "/test_host_key",
			HasConfig:   true,
		},
		Users: []aaa.UserCredential{kept, gone},
		UsersFunc: func() ([]aaa.UserCredential, error) {
			mu.Lock()
			defer mu.Unlock()
			return running, nil
		},
		ConfigDir:     t.TempDir(),
		EphemeralFile: addrFile,
		Log:           slogutil.Logger("hub.ssh.test"),
	})
	require.NotNil(t, stop, "the standalone builder should return a shutdown func")
	t.Cleanup(stop)

	// The builder writes the bound address to the ephemeral file after Start,
	// which is how `ze config edit` finds its daemon.
	raw, err := os.ReadFile(addrFile)
	require.NoError(t, err)
	addr := string(raw)

	require.Eventually(t, func() bool {
		return sshDialWithKey(addr, "keepuser", keptSigner) == nil
	}, 5*time.Second, 20*time.Millisecond, "the configured key authenticates once the server serves")
	require.NoError(t, sshDialWithKey(addr, "goneuser", goneSigner),
		"both users authenticate before the reload")

	mu.Lock()
	running = []aaa.UserCredential{kept}
	mu.Unlock()

	assert.Error(t, sshDialWithKey(addr, "goneuser", goneSigner),
		"AC-11: the no-bgp{} path must refuse a user the reload removed")
	assert.NoError(t, sshDialWithKey(addr, "keepuser", keptSigner),
		"AC-12: a user the reload kept must still authenticate by key")
}
