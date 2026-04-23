package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
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

	cfg := ExtractSSHConfig(tree)
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

	cfg := ExtractSSHConfig(tree)
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

	cfg := ExtractSSHConfig(tree)
	require.Len(t, cfg.Users, 1)

	carol := cfg.Users[0]
	assert.Equal(t, "carol", carol.Name)
	assert.NotEmpty(t, carol.Hash)
	assert.Empty(t, carol.PublicKeys)
}
