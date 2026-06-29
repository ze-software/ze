// Design: plan/learned/811-install-3-image-server.md -- image server config parsing

package imageserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

const defaultListenPort = 80

type imageConfig struct {
	Enabled          bool
	ListenInterfaces []string
	ListenPort       int
	ImageDirectory   string
	BootDirectory    string
	SSHUsername      string
	SSHPasswordHash  string
	ShellAuthSHA256  string
}

func parseConfig(data string) (imageConfig, error) {
	var cfg imageConfig

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("image-server config: unmarshal: %w", err)
	}

	svcMap, ok := root["service"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	imgMap, ok := svcMap["image-server"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := imgMap["enabled"].(string); ok {
		cfg.Enabled = v == "true"
	}

	switch v := imgMap["listen-interface"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				cfg.ListenInterfaces = append(cfg.ListenInterfaces, s)
			}
		}
	case string:
		if v != "" {
			cfg.ListenInterfaces = append(cfg.ListenInterfaces, v)
		}
	}

	cfg.ListenPort = defaultListenPort
	if v, ok := imgMap["listen-port"].(string); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("image-server listen-port: %w", err)
		}
		if n < 1 || n > 65535 {
			return cfg, fmt.Errorf("image-server listen-port: %d out of range 1..65535", n)
		}
		cfg.ListenPort = n
	}

	if v, ok := imgMap["image-directory"].(string); ok {
		cfg.ImageDirectory = v
	}
	if v, ok := imgMap["boot-directory"].(string); ok {
		cfg.BootDirectory = v
	}
	if v, ok := imgMap["ssh-username"].(string); ok {
		cfg.SSHUsername = v
	}
	if v, ok := imgMap["ssh-password-hash"].(string); ok {
		cfg.SSHPasswordHash = v
	}
	if v, ok := imgMap["shell-auth-sha256"].(string); ok {
		cfg.ShellAuthSHA256 = v
	}

	return cfg, nil
}

func verifyConfig(cfg imageConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.ImageDirectory == "" && cfg.BootDirectory == "" {
		return errors.New("image-server: at least one of image-directory or boot-directory is required when enabled")
	}
	// SSH credentials drive the /install/database.zefs endpoint; requiring both
	// or neither prevents a half-configured state that silently 404s the
	// endpoint with no indication the credential pair is incomplete.
	if (cfg.SSHUsername == "") != (cfg.SSHPasswordHash == "") {
		return errors.New("image-server: ssh-username and ssh-password-hash must both be set or both empty")
	}
	if cfg.SSHPasswordHash != "" {
		if _, err := bcrypt.Cost([]byte(cfg.SSHPasswordHash)); err != nil {
			return fmt.Errorf("image-server: ssh-password-hash is not a valid bcrypt hash: %w", err)
		}
	}
	if cfg.ShellAuthSHA256 != "" && !isHexSHA256(cfg.ShellAuthSHA256) {
		return errors.New("image-server: shell-auth-sha256 must be 64 lowercase hex characters")
	}
	return nil
}

// isHexSHA256 reports whether s is exactly 64 lowercase hex digits (a sha256
// digest in hex). The installer compares a typed password's sha256sum against
// this value, so a malformed hash would silently lock out the rescue shell.
func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := range len(s) {
		switch c := s[i]; {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
			// valid lowercase hex digit
		default:
			return false
		}
	}
	return true
}
