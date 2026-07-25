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
