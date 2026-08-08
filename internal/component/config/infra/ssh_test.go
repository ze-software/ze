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

// The map form ExtractAuthUsers reads is what the running daemon holds: every
// applied reload writes config.Tree.ToMap() into the shared ConfigProvider, and
// the web fallback authenticator reads that provider per login. These tests pin
// the one property that makes the arrangement safe -- the map reader and the
// tree reader must describe the same users, or authentication and startup would
// disagree about who exists.

// VALIDATES: ExtractAuthUsers and ExtractSSHConfig report the same users for the
// same configuration, so the per-login reader cannot drift from the startup one.
// PREVENTS: a user who exists to one reader and not the other.
func TestExtractAuthUsersAgreesWithExtractSSHConfig(t *testing.T) {
	input := sshTestBoilerplate + `
system {
    authentication {
        user alice {
            password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01234"
            profile admin
            public-keys laptop {
                type ssh-ed25519
                key AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere
            }
        }
        user bob {
            password "$2a$10$zyxwvutsrqponmlkjihgfZYXWVUTSRQPONMLKJIHGFEDCBA98765"
        }
    }
}
`
	tree, err := config.ParseTreeWithYANG(input, nil)
	require.NoError(t, err)

	fromTree := infra.ExtractSSHConfig(tree).Users
	fromMap := infra.ExtractAuthUsers(tree.GetContainer("system").ToMap())

	require.Len(t, fromMap, 2)
	assert.Equal(t, fromTree, fromMap,
		"the per-login map reader must report exactly the users the startup tree reader reports")
	assert.Equal(t, "alice", fromMap[0].Name, "users come back sorted; the map form carries no order of its own")
	assert.Equal(t, []string{"admin"}, fromMap[0].Profiles)
	require.Len(t, fromMap[0].PublicKeys, 1)
	assert.Equal(t, "laptop", fromMap[0].PublicKeys[0].Name)
	assert.Empty(t, fromMap[1].Profiles, "bob declares no profile")
}

// VALIDATES: a leaf-list survives every shape the map form can carry it in.
// Tree.ToMap collapses a one-member leaf-list to a bare string and emits
// []string beyond that, and a JSON round trip turns either into []any.
// PREVENTS: a single-profile user losing their profile, which would silently
// change what they are authorized to do.
func TestExtractAuthUsersLeafListShapes(t *testing.T) {
	shapes := map[string]struct {
		raw  any
		want []string
	}{
		"one member as a bare string":   {raw: "admin", want: []string{"admin"}},
		"several members as []string":   {raw: []string{"admin", "ro"}, want: []string{"admin", "ro"}},
		"several members as []any":      {raw: []any{"admin", "ro"}, want: []string{"admin", "ro"}},
		"an empty string is no profile": {raw: "", want: nil},
		"an unexpected type is ignored": {raw: 42, want: nil},
	}
	for name, tc := range shapes {
		t.Run(name, func(t *testing.T) {
			users := infra.ExtractAuthUsers(map[string]any{
				"authentication": map[string]any{
					"user": map[string]any{
						"alice": map[string]any{"password": "hash", "profile": tc.raw},
					},
				},
			})
			require.Len(t, users, 1)
			assert.Equal(t, tc.want, users[0].Profiles)
		})
	}
}

// VALIDATES: a subtree that does not describe users yields no users, at every
// depth the shape can go missing.
// PREVENTS: an unreadable or absent config reading as a user list the caller
// would then authenticate against.
func TestExtractAuthUsersMissingSections(t *testing.T) {
	cases := map[string]map[string]any{
		"a nil subtree":               nil,
		"an empty subtree":            {},
		"no authentication container": {"login": map[string]any{}},
		"authentication is not a map": {"authentication": "yes"},
		"no user list":                {"authentication": map[string]any{}},
		"the user list is not a map":  {"authentication": map[string]any{"user": "alice"}},
		"a user entry is not a map":   {"authentication": map[string]any{"user": map[string]any{"alice": "hash"}}},
		"public-keys is not a keyed list": {"authentication": map[string]any{
			"user": map[string]any{"alice": map[string]any{"password": "h", "public-keys": "laptop"}},
		}},
	}
	for name, subtree := range cases {
		t.Run(name, func(t *testing.T) {
			users := infra.ExtractAuthUsers(subtree)
			if name == "a user entry is not a map" || name == "public-keys is not a keyed list" {
				// The user list itself is well-formed here; only the entry is
				// not. A shapeless entry is dropped, never invented.
				for _, u := range users {
					assert.Empty(t, u.PublicKeys)
				}
				return
			}
			assert.Empty(t, users, "a subtree that describes no users must authenticate nobody")
		})
	}
}
