// Design: docs/architecture/provisioning/tftp-server.md -- TFTP server config parsing

package tftpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/tftp"
)

// The defaults of ze-tftp-server-conf.yang. TestTFTPConfigDefaultsMatchYANG
// pins each one to its leaf's YANG default.
const (
	defaultMaxTransfers   = 10
	defaultMaxWindowsize  = 64 // blocks, leaf max-windowsize
	defaultTimeoutSeconds = 5  // leaf timeout
)

type tftpConfig struct {
	Enabled          bool
	ListenInterfaces []string
	RootDirectory    string
	MaxTransfers     int
	// MaxWindowsize is the largest RFC 7440 window granted, in blocks.
	MaxWindowsize int
	// Timeout is the retransmission interval, in seconds, of a transfer whose
	// client requested no RFC 2349 timeout.
	Timeout int
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
	// A delivery with no tftp-server block answered the zero config above,
	// which compares equal to a server that never had one. With the block,
	// each leaf it leaves out takes its YANG default.
	cfg.MaxTransfers = defaultMaxTransfers

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

	var err error
	if cfg.MaxWindowsize, err = rangedLeaf(tftpMap, "max-windowsize", defaultMaxWindowsize,
		tftp.WindowsizeMin, tftp.WindowsizeMax); err != nil {
		return cfg, err
	}
	if cfg.Timeout, err = rangedLeaf(tftpMap, "timeout", defaultTimeoutSeconds,
		tftp.TimeoutMin, tftp.TimeoutMax); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// rangedLeaf reads the integer leaf name of the tftp-server map, answers
// defaultValue when the leaf is absent, and refuses a value that is not a
// number from low to high, naming the fix. The YANG range refuses the same
// values when the config is parsed; this is the second check of the pair, for
// a delivery that did not pass that parser.
func rangedLeaf(tftpMap map[string]any, name string, defaultValue, low, high int) (int, error) {
	raw, present := tftpMap[name]
	// refuse-by-default: guards parseConfig -- an absent leaf takes its YANG default, which TestTFTPConfigDefaultsMatchYANG pins
	if !present {
		return defaultValue, nil
	}
	text, ok := raw.(string)
	if !ok {
		return 0, fmt.Errorf("tftp-server %s: the value is a %T, not a number.\n"+
			"Fix: set service tftp-server %s to a number from %d to %d", name, raw, name, low, high)
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("tftp-server %s: %q is not a number.\n"+
			"Fix: set service tftp-server %s to a number from %d to %d", name, text, name, low, high)
	}
	if n < low {
		return 0, fmt.Errorf("tftp-server %s: %d is below %d.\n"+
			"Fix: set service tftp-server %s to a number from %d to %d", name, n, low, name, low, high)
	}
	if n > high {
		return 0, fmt.Errorf("tftp-server %s: %d is above %d.\n"+
			"Fix: set service tftp-server %s to a number from %d to %d", name, n, high, name, low, high)
	}
	return n, nil
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
