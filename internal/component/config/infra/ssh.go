// Design: docs/architecture/config/syntax.md -- daemon-startup SSH config extraction
// Related: authz.go -- authorization extraction from the same tree
// Related: hook.go -- the HookParams contract these feed

package infra

import (
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/paths"
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

	if sys := tree.GetContainer("system"); sys != nil {
		if auth := sys.GetContainer("authentication"); auth != nil {
			for name, entry := range auth.GetList("user") {
				var uc authz.UserConfig
				uc.Name = name
				if pw, ok := entry.Get("password"); ok {
					uc.Hash = pw
				}
				uc.Profiles = entry.GetSlice("profile")
				for keyName, keyEntry := range entry.GetList("public-keys") {
					pk := authz.SSHPublicKey{Name: keyName}
					if t, ok := keyEntry.Get("type"); ok {
						pk.Type = t
					}
					if k, ok := keyEntry.Get("key"); ok {
						pk.Key = k
					}
					uc.PublicKeys = append(uc.PublicKeys, pk)
				}
				cfg.Users = append(cfg.Users, uc)
			}
		}
	}

	return cfg
}

// ResolveSSHStorage returns blob storage for SSH host key persistence.
// When the main storage is already blob-backed, it is used directly.
// Otherwise, opens the zefs database independently so SSH host keys
// always go into the blob store rather than the filesystem.
// Tries configDir first, then DefaultConfigDir (binary-relative), because
// configDir may not contain database.zefs (e.g., stdin mode, temp dirs).
// Falls back to the passed store if zefs is not available anywhere.
func ResolveSSHStorage(mainStore storage.Storage, configDir string) storage.Storage {
	if storage.IsBlobStorage(mainStore) {
		return mainStore
	}
	// Try configDir first, then binary-relative default.
	// configDir is almost never empty (LoadConfig sets it to cwd for stdin),
	// but may not contain database.zefs when the config file is elsewhere.
	candidates := [2]string{configDir, paths.DefaultConfigDir()}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		dbPath := filepath.Join(dir, "database.zefs")
		blobStore, err := storage.NewBlob(dbPath, dir)
		if err == nil {
			return blobStore
		}
	}
	return mainStore
}
