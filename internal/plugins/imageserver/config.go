// Design: plan/learned/811-install-3-image-server.md -- image server config parsing

package imageserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/rescueauth"
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
	RescueAuth       string
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
	if v, ok := imgMap["rescue-auth"].(string); ok {
		cfg.RescueAuth = v
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
	if cfg.RescueAuth != "" {
		if err := rescueauth.Validate(cfg.RescueAuth); err != nil {
			return fmt.Errorf("image-server: rescue-auth: %w", err)
		}
	}
	return nil
}
