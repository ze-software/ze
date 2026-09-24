// Design: docs/architecture/pki/pki-store.md -- candidate config decoding without live-store mutation
// Related: config.go -- ParseConfig validates certificate material
package pki

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/component/config"
)

// ParseJSON parses a root-wrapped pki ConfigSection without changing the live
// store. An empty object represents removal. Config verifiers resolve against
// this candidate snapshot; only an apply path may install one with Load.
func ParseJSON(data string) (*PKIConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("pki: config section must be a JSON object")
	}
	if len(raw) != 0 {
		if len(raw) != 1 {
			return nil, errors.New("pki: config section must contain only the pki container")
		}
		if _, ok := raw["pki"].(map[string]any); !ok {
			return nil, errors.New("pki: config section must wrap a pki container")
		}
	}
	tree, err := config.TreeFromPluginMap(raw)
	if err != nil {
		return nil, err
	}
	return ParseConfig(tree)
}
