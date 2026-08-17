// Design: ai/rules/plugins.md -- ze_ssh present build validation
//
//go:build ze_ssh

package hub

// VALIDATES: with the ze_ssh build tag (the default ze / ze-appliance feature
// set), the ssh compile-out seam is installed (build + wire + standalone).
// PREVENTS: a regression where ze_ssh is set but ssh is not wired (e.g. the
// register_ssh.go init() is dropped or the tag stops reaching the generator).

import (
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
)

const sshTaggedConfig = `
environment {
	ssh {
		enabled true
		server main {
			ip 127.0.0.1
		}
		host-key "/tmp/ze-ssh-host-key"
		host-certificate "/tmp/ze-ssh-host-cert"
		idle-timeout 90
		max-sessions 8
	}
}
system {
	authentication {
		user operator {
			password "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
			profile [ readonly ]
			public-keys laptop {
				type ssh-ed25519
				key AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere
			}
		}
	}
	authorization {
		profile readonly {
			run { default-action allow; }
			edit { default-action deny; }
		}
	}
}
`

func TestBuildTag_SSH_Present(t *testing.T) {
	if sshBuild == nil || sshWirePostStart == nil || sshBuildStandalone == nil {
		t.Fatal("ze_ssh build: ssh seam not installed (sshBuild/sshWirePostStart/sshBuildStandalone)")
	}
}

func TestBuildTag_SSH_PresentAcceptsSSHConfig(t *testing.T) {
	tree, err := zeconfig.ParseTreeWithYANG(sshTaggedConfig, nil)
	if err != nil {
		t.Fatalf("ze_ssh build rejected current SSH config syntax: %v", err)
	}

	cfg := infra.ExtractSSHConfig(tree)
	if !cfg.HasConfig {
		t.Fatal("ExtractSSHConfig did not report the configured SSH block")
	}
	if cfg.Listen != "127.0.0.1:2222" {
		t.Fatalf("SSH listen = %q, want defaulted 127.0.0.1:2222", cfg.Listen)
	}
	if cfg.HostKeyPath != "/tmp/ze-ssh-host-key" || cfg.HostCertPath != "/tmp/ze-ssh-host-cert" {
		t.Fatalf("SSH host key paths = (%q, %q), want configured paths", cfg.HostKeyPath, cfg.HostCertPath)
	}
	if cfg.IdleTimeout != 90 || cfg.MaxSessions != 8 {
		t.Fatalf("SSH limits = (%d, %d), want (90, 8)", cfg.IdleTimeout, cfg.MaxSessions)
	}

	users := infra.ExtractAuthUsers(tree.GetContainer("system").ToMap())
	if len(users) != 1 {
		t.Fatalf("ExtractAuthUsers returned %d users, want 1", len(users))
	}
	operator := users[0]
	if operator.Name != "operator" || operator.Hash == "" {
		t.Fatalf("tagged shared user = %#v, want operator with password", operator)
	}
	if len(operator.Profiles) != 1 || operator.Profiles[0] != "readonly" {
		t.Fatalf("tagged user profiles = %v, want [readonly]", operator.Profiles)
	}
	if len(operator.PublicKeys) != 1 {
		t.Fatalf("tagged user public keys = %#v, want one", operator.PublicKeys)
	}
	key := operator.PublicKeys[0]
	if key.Name != "laptop" || key.Type != "ssh-ed25519" || key.Key != "AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyDataHere" {
		t.Fatalf("tagged SSH public key = %#v, want preserved name, type, and key bytes", key)
	}
}
