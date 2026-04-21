// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper config

package l2tpshaper

import (
	"encoding/json"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// shaperConfig holds parsed shaper settings from YANG JSON.
type shaperConfig struct {
	QdiscType   traffic.QdiscType
	DefaultRate uint64 // download rate in bps
	UploadRate  uint64 // upload rate in bps (0 = same as download)
}

// parseShaperConfig extracts shaper settings from the l2tp YANG JSON.
// JSON shape: {"shaper":{"qdisc-type":"tbf","default-rate":"10mbit","upload-rate":"5mbit"}}.
func parseShaperConfig(data string) (*shaperConfig, bool, error) {
	if data == "" {
		return nil, false, nil
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return nil, false, fmt.Errorf("%s: invalid config JSON: %w", Name, err)
	}

	shaperBlock, ok := tree["shaper"].(map[string]any)
	if !ok {
		return nil, false, nil
	}

	cfg := &shaperConfig{}

	qdiscStr, _ := shaperBlock["qdisc-type"].(string)
	if qdiscStr == "" {
		qdiscStr = "tbf"
	}
	qt, ok := traffic.ParseQdiscType(qdiscStr)
	if !ok {
		return nil, false, fmt.Errorf("%s: unsupported qdisc-type %q", Name, qdiscStr)
	}
	if qt != traffic.QdiscTBF && qt != traffic.QdiscHTB {
		return nil, false, fmt.Errorf("%s: qdisc-type must be tbf or htb, got %q", Name, qdiscStr)
	}
	cfg.QdiscType = qt

	rateStr, _ := shaperBlock["default-rate"].(string)
	if rateStr == "" {
		return nil, false, fmt.Errorf("%s: shaper requires default-rate", Name)
	}
	rate, err := traffic.ParseRateBps(rateStr)
	if err != nil {
		return nil, false, fmt.Errorf("%s: invalid default-rate %q: %w", Name, rateStr, err)
	}
	if err := traffic.ValidateRate(rate); err != nil {
		return nil, false, fmt.Errorf("%s: default-rate: %w", Name, err)
	}
	cfg.DefaultRate = rate

	if uploadStr, ok := shaperBlock["upload-rate"].(string); ok && uploadStr != "" {
		upload, err := traffic.ParseRateBps(uploadStr)
		if err != nil {
			return nil, false, fmt.Errorf("%s: invalid upload-rate %q: %w", Name, uploadStr, err)
		}
		cfg.UploadRate = upload
	}

	return cfg, true, nil
}
