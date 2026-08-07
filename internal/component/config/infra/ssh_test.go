package infra_test

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/config/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

const sshTestBoilerplate = `
bgp {
    peer loopback {
        connection {
            remote { ip 127.0.0.1; }
            local { ip 127.0.0.1; }
        }
        session {
            asn { local 65533; remote 65533; }
        }
    }
}

environment {
    ssh {
        enabled true
        server main {
            ip 127.0.0.1
            port 2222
        }
    }
}
`

func TestExtractSSHConfigPublicKeys(t *testing.T) {
	input := sshTestBoilerplate + `
system {
    authentication {
        user alice {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
            public-keys laptop {
                type ssh-ed25519
                key AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	cfg := infra.ExtractSSHConfig(tree)
	require.Len(t, cfg.Users, 1)

	alice := cfg.Users[0]
	assert.Equal(t, "alice", alice.Name)
	assert.NotEmpty(t, alice.Hash)
	require.Len(t, alice.PublicKeys, 1)

	pk := alice.PublicKeys[0]
	assert.Equal(t, "laptop", pk.Name)
	assert.Equal(t, "ssh-ed25519", pk.Type)
	assert.Equal(t, "AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere", pk.Key)
}

func TestExtractSSHConfigPublicKeysMultiple(t *testing.T) {
	input := sshTestBoilerplate + `
system {
    authentication {
        user bob {
            public-keys workstation {
                type ssh-rsa
                key AAAAB3NzaC1yc2EAAAADAQABAAABgQExample
            }
            public-keys phone {
                type ssh-ed25519
                key AAAAC3NzaC1lZDI1NTE5AAAAISecondKey
            }
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	cfg := infra.ExtractSSHConfig(tree)
	require.Len(t, cfg.Users, 1)

	bob := cfg.Users[0]
	assert.Equal(t, "bob", bob.Name)
	assert.Empty(t, bob.Hash)
	require.Len(t, bob.PublicKeys, 2)

	keysByName := map[string]struct{ Type, Key string }{}
	for _, pk := range bob.PublicKeys {
		keysByName[pk.Name] = struct{ Type, Key string }{pk.Type, pk.Key}
	}

	ws, ok := keysByName["workstation"]
	require.True(t, ok, "workstation key should exist")
	assert.Equal(t, "ssh-rsa", ws.Type)
	assert.Equal(t, "AAAAB3NzaC1yc2EAAAADAQABAAABgQExample", ws.Key)

	ph, ok := keysByName["phone"]
	require.True(t, ok, "phone key should exist")
	assert.Equal(t, "ssh-ed25519", ph.Type)
	assert.Equal(t, "AAAAC3NzaC1lZDI1NTE5AAAAISecondKey", ph.Key)
}

func TestExtractSSHConfigPublicKeysEmpty(t *testing.T) {
	input := sshTestBoilerplate + `
system {
    authentication {
        user carol {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	cfg := infra.ExtractSSHConfig(tree)
	require.Len(t, cfg.Users, 1)

	carol := cfg.Users[0]
	assert.Equal(t, "carol", carol.Name)
	assert.NotEmpty(t, carol.Hash)
	assert.Empty(t, carol.PublicKeys)
}

// TestResolveSSHStorage verifies that SSH storage always resolves to blob when zefs exists.
//
// VALIDATES: SSH host key goes into blob store even when main store is filesystem.
// PREVENTS: Host key written as plain file when config loaded from filesystem.
func TestResolveSSHStorage(t *testing.T) {
	dir := t.TempDir()
	blobPath := filepath.Join(dir, "database.zefs")

	t.Run("blob store passed through", func(t *testing.T) {
		blob, err := storage.NewBlob(blobPath, dir)
		require.NoError(t, err)
		defer blob.Close() //nolint:errcheck // test

		got := infra.ResolveSSHStorage(blob, dir)
		assert.True(t, storage.IsBlobStorage(got), "blob storage should pass through")
		got.Close() //nolint:errcheck // test
	})

	t.Run("filesystem upgraded to blob when zefs exists", func(t *testing.T) {
		// Create zefs database first
		blob, err := storage.NewBlob(blobPath, dir)
		require.NoError(t, err)
		blob.Close() //nolint:errcheck // just creating

		fs := storage.NewFilesystem()
		got := infra.ResolveSSHStorage(fs, dir)
		assert.True(t, storage.IsBlobStorage(got), "filesystem should be upgraded to blob when zefs exists")
		got.Close() //nolint:errcheck // test
	})

	t.Run("filesystem kept when no config dir", func(t *testing.T) {
		fs := storage.NewFilesystem()
		got := infra.ResolveSSHStorage(fs, "")
		assert.False(t, storage.IsBlobStorage(got), "should stay filesystem when no config dir")
	})
}

// TestExtractSSHConfigEmptyLeafKeepsDefault verifies a present-but-EMPTY ip or
// port leaf falls back to the default, as extractServerList
// (internal/component/config/loader_extract.go) does for every other service.
//
// Without the emptiness test the port branch produced "127.0.0.1:", which the
// kernel binds on an ephemeral port while ze doctor probes 2222: the daemon and
// its readiness check disagreed about which endpoint exists.
//
// The tree is built directly rather than parsed, and that is deliberate: the file
// parser rejects `port ""` with "invalid uint16" before ExtractSSHConfig is
// reached, so no config FILE can drive this today. ExtractSSHConfig takes a
// *config.Tree, not a file, and the guard is what makes its contract hold for
// every tree rather than for the one shape today's parser happens to allow.
// VALIDATES: an empty leaf is "unset", not "bind whatever the kernel picks".
// PREVENTS: ssh listening on a port no operator asked for and no check probes.
func TestExtractSSHConfigEmptyLeafKeepsDefault(t *testing.T) {
	tree := config.NewTree()
	env := config.NewTree()
	ssh := config.NewTree()
	ssh.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "")
	srv.Set("port", "")
	ssh.AddListEntry("server", "main", srv)
	env.SetContainer("ssh", ssh)
	tree.SetContainer("environment", env)

	cfg := infra.ExtractSSHConfig(tree)
	require.True(t, cfg.HasConfig, "the ssh container is present, so extraction must report it")
	require.Len(t, cfg.ListenAddrs, 1)
	assert.Equal(t, "0.0.0.0:2222", cfg.ListenAddrs[0],
		"an empty leaf must keep the default; a bare host:port with no port binds an ephemeral one")
	assert.Equal(t, "0.0.0.0:2222", cfg.Listen, "Listen is the first address and must agree with it")
}
