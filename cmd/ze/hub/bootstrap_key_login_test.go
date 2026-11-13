//go:build ze_ssh

// Design: docs/guide/authentication.md -- appliance bootstrap public keys share the accepted local identity.
// Related: main_servers.go -- usersFromStore and liveLocalUsers; service_ssh.go -- standalone daemon builder.
// Upstream: golang.org/x/crypto v0.58.0, ssh/client.go and ssh/keys.go;
// https://github.com/golang/crypto/tree/v0.58.0/ssh.

package hub

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/zefs"
)

const bootstrapAuthorizedKey = "meta/auth/local/authorized-keys"
const bootstrapKeyPassword = "bootstrap-key-test-password"

func bootstrapSigner(t *testing.T) gossh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func bootstrapStore(t *testing.T, local, outbound string, disabled bool) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapKeyPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		key   string
		value []byte
	}{
		{zefs.KeyLocalAdminUsername.Pattern, []byte("admin")},
		{zefs.KeyLocalAdminPassword.Pattern, hash},
	} {
		if err := store.WriteFile(entry.key, entry.value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if local != "" {
		if err := store.WriteFile(bootstrapAuthorizedKey, []byte(local), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if outbound != "" {
		if err := store.WriteFile("meta/ssh/authorized-keys", []byte(outbound), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if disabled {
		if err := store.WriteFile(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func bootstrapSSHServer(t *testing.T, store storage.Storage, configured []authz.UserConfig) (string, gossh.PublicKey) {
	t.Helper()
	resetAAABundleForTest(t)
	resolve := liveLocalUsers(bootPowerUsers(store, nil), func() ([]authz.UserConfig, error) {
		return configured, nil
	}, nil)
	users, err := resolve()
	if err != nil {
		t.Fatal(err)
	}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, authz.NewStore(), resolve, ""))
	// The known test host key is stored through the same store the real builder reads.
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host, err := gossh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	block, err := gossh.MarshalPrivateKey(private, "owned bootstrap-key test host")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	hostPath := "bootstrap-host-key"
	if err := store.WriteFile(hostPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	addressFile := filepath.Join(dir, "ssh.addr")
	stop := sshBuildStandaloneImpl(&sshStandaloneInputs{
		Config:        infra.SSHExtractedConfig{Listen: "127.0.0.1:0", HostKeyPath: hostPath, HasConfig: true},
		Users:         users,
		UsersFunc:     liveAcceptedLocalUsers,
		Authenticator: &authz.LocalAuthenticator{UsersFunc: liveAcceptedLocalUsers},
		Authorizer:    liveLocalAuthorizer{},
		Storage:       store,
		ConfigDir:     dir,
		ConfigOwner:   &configMutationOwner{},
		EphemeralFile: addressFile,
		Log:           slogutil.Logger("hub.ssh.bootstrap-key-test"),
	})
	if stop == nil {
		t.Fatal("daemon SSH builder did not start")
	}
	t.Cleanup(stop)
	address, err := os.ReadFile(addressFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(address), host.PublicKey()
}

func bootstrapSSHDial(address, username string, host gossh.PublicKey, method gossh.AuthMethod) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dialer net.Dialer
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close() //nolint:errcheck // The owned test transport always closes.
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	transport, channels, requests, err := gossh.NewClientConn(connection, address, &gossh.ClientConfig{
		User: username, Auth: []gossh.AuthMethod{method},
		HostKeyCallback: gossh.FixedHostKey(host),
	})
	if err != nil {
		return err
	}
	client := gossh.NewClient(transport, channels, requests)
	defer client.Wait() //nolint:errcheck // Join transport workers after close.
	return client.Close()
}

func TestBootstrapKeysAuthenticateThroughAcceptedIdentity(t *testing.T) {
	first, second, stranger := bootstrapSigner(t), bootstrapSigner(t), bootstrapSigner(t)
	keys := "# appliance keys\r\n" + string(gossh.MarshalAuthorizedKey(first.PublicKey())) +
		string(gossh.MarshalAuthorizedKey(second.PublicKey()))
	store := bootstrapStore(t, keys, "", false)
	address, host := bootstrapSSHServer(t, store, nil)
	for _, signer := range []gossh.Signer{first, second} {
		if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(signer)); err != nil {
			t.Fatalf("assembled bootstrap key did not authenticate through daemon identity: %v", err)
		}
	}
	if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(stranger)); err == nil {
		t.Fatal("an unrelated key authenticated")
	}
	if err := bootstrapSSHDial(address, "somebody-else", host, gossh.PublicKeys(first)); err == nil {
		t.Fatal("bootstrap key authenticated under a different username")
	}
	if err := bootstrapSSHDial(address, "admin", host, gossh.Password(bootstrapKeyPassword)); err != nil {
		t.Fatalf("installing keys removed password recovery: %v", err)
	}
	users, err := liveAcceptedLocalUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("accepted bootstrap users = %d, want one", len(users))
	}
	if !slices.Contains(users[0].Profiles, aaa.ReservedRecoveryProfile) {
		t.Fatal("public-key bootstrap identity lost the recovery profile")
	}
}

func TestBootstrapKeysRespectAdminDisabled(t *testing.T) {
	signer := bootstrapSigner(t)
	store := bootstrapStore(t, string(gossh.MarshalAuthorizedKey(signer.PublicKey())), "", true)
	address, host := bootstrapSSHServer(t, store, nil)
	if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(signer)); err == nil {
		t.Fatal("disabled bootstrap admin authenticated with its key")
	}
	if err := bootstrapSSHDial(address, "admin", host, gossh.Password(bootstrapKeyPassword)); err == nil {
		t.Fatal("disabled bootstrap admin authenticated with its password")
	}
}

func TestBootstrapKeysConfigUserOverridesRecovery(t *testing.T) {
	bootstrap, configured := bootstrapSigner(t), bootstrapSigner(t)
	store := bootstrapStore(t, string(gossh.MarshalAuthorizedKey(bootstrap.PublicKey())), "", false)
	user := authz.UserConfig{
		Name: "admin", PublicKeys: []aaa.SSHPublicKey{{
			Name: "configured", Type: configured.PublicKey().Type(),
			Key: base64.StdEncoding.EncodeToString(configured.PublicKey().Marshal()),
		}},
	}
	address, host := bootstrapSSHServer(t, store, []authz.UserConfig{user})
	if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(bootstrap)); err == nil {
		t.Fatal("same-name configured user retained a stale bootstrap key")
	}
	if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(configured)); err != nil {
		t.Fatalf("same-name configured user's own key was refused: %v", err)
	}
	users, err := liveAcceptedLocalUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("accepted configured users = %d, want one", len(users))
	}
	if slices.Contains(users[0].Profiles, aaa.ReservedRecoveryProfile) {
		t.Fatal("same-name configured user inherited bootstrap recovery authority")
	}
}

func TestBootstrapKeysIgnoreOutboundNamespace(t *testing.T) {
	signer := bootstrapSigner(t)
	store := bootstrapStore(t, "", string(gossh.MarshalAuthorizedKey(signer.PublicKey())), false)
	address, host := bootstrapSSHServer(t, store, nil)
	if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(signer)); err == nil {
		t.Fatal("outbound SSH namespace became a local administrative credential")
	}
	if err := bootstrapSSHDial(address, "admin", host, gossh.Password(bootstrapKeyPassword)); err != nil {
		t.Fatalf("outbound key changed password recovery: %v", err)
	}
}

func TestBootstrapKeysRejectMalformedAndRestrictedKeys(t *testing.T) {
	signer := bootstrapSigner(t)
	key := string(gossh.MarshalAuthorizedKey(signer.PublicKey()))
	for _, content := range []string{
		"not-an-ssh-key\n",
		key + "ssh-ed25519 invalid-data\n",
		"not-an-ssh-key\n" + key,
		"restrict " + key,
		"from=\"192.0.2.0/24\" " + key,
		"command=\"show version\" " + key,
	} {
		store := bootstrapStore(t, content, "", false)
		users, err := usersFromStore(store)
		if err == nil {
			t.Fatal("malformed or restricted bootstrap key file was accepted")
		}
		if users != nil {
			t.Fatal("refused bootstrap key file exposed a partial administrative identity")
		}
	}
}

func TestBootstrapKeysAbsentOrCommentsKeepPassword(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{name: "absent"},
		{name: "comments", content: "\n# no keys configured\r\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := bootstrapStore(t, tc.content, "", false)
			address, host := bootstrapSSHServer(t, store, nil)
			if err := bootstrapSSHDial(address, "admin", host, gossh.Password(bootstrapKeyPassword)); err != nil {
				t.Fatalf("an optional key file removed password recovery: %v", err)
			}
			users, err := usersFromStore(store)
			if err != nil {
				t.Fatal(err)
			}
			if len(users) != 1 {
				t.Fatalf("bootstrap users = %d, want one", len(users))
			}
			if len(users[0].PublicKeys) != 0 {
				t.Fatal("missing/comment-only key file created a credential")
			}
		})
	}
}
