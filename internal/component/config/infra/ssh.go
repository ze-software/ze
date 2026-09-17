// Design: docs/architecture/config/syntax.md -- daemon-startup SSH config extraction
// Related: authz.go -- authorization extraction from the same tree
// Related: hook.go -- the HookParams contract these feed

package infra

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
)

// ExtractSSHConfig extracts SSH server configuration from the parsed config tree.
// Returns plain data (no ssh package types). The caller converts to ssh.Config.
func ExtractSSHConfig(tree *config.Tree) SSHExtractedConfig {
	env := tree.GetContainer("environment")
	if env == nil {
		return SSHExtractedConfig{}
	}

	sshContainer := env.GetContainer("ssh")
	if sshContainer == nil {
		return SSHExtractedConfig{}
	}

	var cfg SSHExtractedConfig
	cfg.HasConfig = true

	if servers := sshContainer.GetListOrdered("server"); len(servers) > 0 {
		for _, s := range servers {
			ip := "0.0.0.0"
			port := "2222"
			// A present-but-EMPTY leaf keeps the default, matching
			// extractServerList (internal/component/config/loader_extract.go).
			// Without the emptiness test an empty port produced "<ip>:", which
			// the kernel binds on an ephemeral port while ze doctor probes 2222:
			// the daemon and its readiness check disagreed about the endpoint.
			if v, ok := s.Value.Get("ip"); ok && v != "" {
				ip = v
			}
			if v, ok := s.Value.Get("port"); ok && v != "" {
				port = v
			}
			cfg.ListenAddrs = append(cfg.ListenAddrs, ip+":"+port)
		}
		cfg.Listen = cfg.ListenAddrs[0]
	} else if addrs := sshContainer.GetSlice("listen"); len(addrs) > 0 {
		cfg.Listen = addrs[0]
		cfg.ListenAddrs = addrs
	}
	if v, ok := sshContainer.Get("host-key"); ok {
		cfg.HostKeyPath = v
	}
	if v, ok := sshContainer.Get("host-certificate"); ok {
		cfg.HostCertPath = v
	}
	if v, ok := sshContainer.Get("idle-timeout"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			cfg.IdleTimeout = uint32(n)
		}
	}
	if v, ok := sshContainer.Get("max-sessions"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSessions = n
		}
	}

	return cfg
}
