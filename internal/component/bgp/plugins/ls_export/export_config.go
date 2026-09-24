// Design: docs/architecture/wire/nlri-bgpls.md -- routing-universe identity
// RFC: rfc/short/rfc9552.md -- operator-configurable 64-bit Instance-ID

package ls_export

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type exportConfig struct {
	enabled bool
	domains map[exportDomainKey]uint64
}

func parseExportConfig(sections []rpc.ConfigSection) (exportConfig, error) {
	var cfg exportConfig
	for _, section := range sections {
		if section.Root != exporterName {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(section.Data), &wrapper); err != nil {
			return cfg, err
		}
		tree := configvalue.Section(exporterName, wrapper)
		if tree == nil {
			return cfg, nil
		}
		cfg.enabled = true
		cfg.domains = make(map[exportDomainKey]uint64)
		for _, entry := range configvalue.ListEntries(tree["domain"]) {
			source, _ := entry.Fields["source"].(string)
			if source == "" {
				return cfg, errors.New("BGP-LS domain source is required")
			}
			protocol, ok := configvalue.Int(entry.Fields["protocol-id"])
			if !ok || protocol < 1 || protocol > 7 {
				return cfg, errors.New("BGP-LS domain protocol-id must be 1..7")
			}
			instance, err := exportUint64(entry.Fields["native-instance"])
			if err != nil {
				return cfg, fmt.Errorf("native-instance: %w", err)
			}
			area, ok := configvalue.Int(entry.Fields["native-area"])
			if !ok || area < 0 || area > 0xffffffff {
				return cfg, errors.New("BGP-LS native-area must be 0..4294967295")
			}
			identifier, err := exportUint64(entry.Fields["instance-id"])
			if err != nil {
				return cfg, fmt.Errorf("instance-id: %w", err)
			}
			key := exportDomainKey{source: source, domain: linkstateevents.Domain{Protocol: linkstateevents.Protocol(protocol), Instance: instance, Area: uint32(area)}}
			if _, exists := cfg.domains[key]; exists {
				return cfg, errors.New("duplicate BGP-LS native domain mapping")
			}
			cfg.domains[key] = identifier
		}
		return cfg, nil
	}
	return cfg, nil
}

func exportUint64(value any) (uint64, error) {
	// Tree.ToMap delivers decimal leaf strings. Parse directly rather than
	// passing uint64 through float64 and losing the high Instance-ID bits.
	text, ok := value.(string)
	if !ok {
		return 0, errors.New("expected decimal uint64 config leaf")
	}
	return strconv.ParseUint(text, 10, 64)
}
