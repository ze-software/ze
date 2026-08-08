// Design: docs/architecture/config/syntax.md -- daemon-startup SSH config extraction
// Related: authz.go -- authorization extraction from the same tree
// Related: hook.go -- the HookParams contract these feed

package infra

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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

	cfg.Users = ExtractAuthUsers(tree.GetContainer("system").ToMap())

	return cfg
}

// ExtractAuthUsers returns the local user credentials the `system` subtree
// describes. The subtree is in resolved map form, which is what
// config.Tree.ToMap() produces and what config.Provider.Get("system") returns.
//
// It is the ONE producer of that shape. ExtractSSHConfig derives its Users from
// it, and the web fallback authenticator calls it per login against the live
// ConfigProvider, so both answers come from the same reader and cannot drift
// apart (ai/rules/evidence.md, "derive, never hardcode").
//
// A nil or shapeless subtree yields no users. Every caller treats that as "this
// configuration authenticates nobody", which denies, so the empty result is the
// closed answer rather than a permissive one.
//
// Users and their public keys come back sorted by name. The map form has no
// order of its own, and a caller that merges or logs the result needs one.
func ExtractAuthUsers(system map[string]any) []authz.UserConfig {
	auth, ok := system["authentication"].(map[string]any)
	if !ok {
		return nil
	}
	entries, ok := auth["user"].(map[string]any)
	if !ok {
		return nil
	}
	users := make([]authz.UserConfig, 0, len(entries))
	for name, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uc := authz.UserConfig{Name: name}
		if pw, ok := entry["password"].(string); ok {
			uc.Hash = pw
		}
		uc.Profiles = leafListValues(entry["profile"])
		uc.PublicKeys = extractPublicKeys(entry["public-keys"])
		users = append(users, uc)
	}
	slices.SortFunc(users, func(a, b authz.UserConfig) int { return strings.Compare(a.Name, b.Name) })
	return users
}

// extractPublicKeys reads the `public-keys` keyed list of one user entry.
func extractPublicKeys(raw any) []authz.SSHPublicKey {
	entries, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]authz.SSHPublicKey, 0, len(entries))
	for name, rawKey := range entries {
		entry, ok := rawKey.(map[string]any)
		if !ok {
			continue
		}
		pk := authz.SSHPublicKey{Name: name}
		if t, ok := entry["type"].(string); ok {
			pk.Type = t
		}
		if k, ok := entry["key"].(string); ok {
			pk.Key = k
		}
		keys = append(keys, pk)
	}
	if len(keys) == 0 {
		return nil
	}
	slices.SortFunc(keys, func(a, b authz.SSHPublicKey) int { return strings.Compare(a.Name, b.Name) })
	return keys
}

// leafListValues reads a YANG leaf-list from resolved map form. Tree.ToMap
// collapses a one-member leaf-list to a plain string and emits []string for
// more, while the same value arrives as []any after a JSON round trip, so all
// three shapes are accepted.
func leafListValues(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
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
