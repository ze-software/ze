// Design: docs/architecture/provisioning/tftp-server.md -- TFTP server config parsing

package tftpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

const defaultMaxTransfers = 10

type tftpConfig struct {
	Enabled          bool
	ListenInterfaces []string
	RootDirectory    string
	MaxTransfers     int
}

func parseConfig(data string) (tftpConfig, error) {
	var cfg tftpConfig

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("tftp-server config: unmarshal: %w", err)
	}

	svcMap, ok := root["service"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	tftpMap, ok := svcMap["tftp-server"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := tftpMap["enabled"].(string); ok {
		cfg.Enabled = v == "true"
	}

	switch v := tftpMap["listen-interface"].(type) {
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

	if v, ok := tftpMap["root-directory"].(string); ok {
		cfg.RootDirectory = v
	}

	cfg.MaxTransfers = defaultMaxTransfers
	if v, ok := tftpMap["max-transfers"].(string); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("tftp-server max-transfers: %w", err)
		}
		if n < 1 || n > 1000 {
			return cfg, fmt.Errorf("tftp-server max-transfers: %d out of range 1..1000", n)
		}
		cfg.MaxTransfers = n
	}

	return cfg, nil
}

func verifyConfig(cfg tftpConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.RootDirectory == "" {
		return errors.New("tftp-server: root-directory is required when enabled")
	}
	return nil
}
