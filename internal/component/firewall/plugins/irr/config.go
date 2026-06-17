// Design: plan/spec-firewall-irr.md -- firewall IRR config parsing

package irr

import (
	"encoding/json"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func readUint(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case string:
		x, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return x, true
	default:
		return 0, false
	}
}

const (
	defaultServer       = "whois.radb.net"
	defaultPeeringDBURL = "https://www.peeringdb.com"
)

type irrConfig struct {
	Server          string
	PeeringDBURL    string
	RefreshInterval uint32
	refs            []irrRef
}

type irrRef struct {
	Name      string
	IsASSet   bool
	IsSrc     bool
	TableName string
}

func (c *irrConfig) allRefs() []irrRef {
	if c == nil {
		return nil
	}
	return c.refs
}

func parseIRRConfig(sections []sdk.ConfigSection) *irrConfig {
	cfg := &irrConfig{
		Server:       defaultServer,
		PeeringDBURL: defaultPeeringDBURL,
	}
	for _, s := range sections {
		if s.Root != "firewall" {
			continue
		}
		var root map[string]any
		if json.Unmarshal([]byte(s.Data), &root) != nil {
			continue
		}
		fw, ok := root["firewall"].(map[string]any)
		if !ok {
			continue
		}
		if irrBlock, ok := fw["irr"].(map[string]any); ok {
			if srv, ok := irrBlock["server"].(string); ok && srv != "" {
				cfg.Server = srv
			}
			if pdbURL, ok := irrBlock["peeringdb-url"].(string); ok && pdbURL != "" {
				cfg.PeeringDBURL = pdbURL
			}
			if v := irrBlock["refresh-interval"]; v != nil {
				if n, ok := readUint(v); ok && n <= 86400 {
					cfg.RefreshInterval = uint32(n) //nolint:gosec // range checked
				}
			}
		}
		cfg.refs = extractRefsFromConfig(root)
	}
	return cfg
}

const tableNamePrefix = "ze_"

type refKey struct {
	name  string
	table string
}

func extractRefsFromConfig(root map[string]any) []irrRef {
	fw, ok := root["firewall"].(map[string]any)
	if !ok {
		return nil
	}
	tableMap, ok := fw["table"].(map[string]any)
	if !ok {
		return nil
	}
	seen := make(map[refKey]bool)
	var refs []irrRef
	for tblName, tblVal := range tableMap {
		tblMap, ok := tblVal.(map[string]any)
		if !ok {
			continue
		}
		fullName := tableNamePrefix + tblName
		chainMap, ok := tblMap["chain"].(map[string]any)
		if !ok {
			continue
		}
		for _, chainVal := range chainMap {
			chainData, ok := chainVal.(map[string]any)
			if !ok {
				continue
			}
			termMap, ok := chainData["term"].(map[string]any)
			if !ok {
				continue
			}
			for _, termVal := range termMap {
				termData, ok := termVal.(map[string]any)
				if !ok {
					continue
				}
				from, ok := termData["from"].(map[string]any)
				if !ok {
					continue
				}
				extractFromBlock(from, seen, &refs, fullName)
			}
		}
	}
	return refs
}

func extractFromBlock(from map[string]any, seen map[refKey]bool, refs *[]irrRef, tableName string) {
	addRef := func(name string, isASSet, isSrc bool) {
		k := refKey{name: name, table: tableName}
		if seen[k] {
			return
		}
		seen[k] = true
		*refs = append(*refs, irrRef{Name: name, IsASSet: isASSet, IsSrc: isSrc, TableName: tableName})
	}
	if v, ok := from["source-asn"].(string); ok {
		addRef(asnName(v), false, true)
	}
	if v, ok := from["source-as-set"].(string); ok {
		addRef(v, true, true)
	}
	if v, ok := from["destination-asn"].(string); ok {
		addRef(asnName(v), false, false)
	}
	if v, ok := from["destination-as-set"].(string); ok {
		addRef(v, true, false)
	}
}

func asnName(v string) string {
	if len(v) >= 2 && (v[0] == 'A' || v[0] == 'a') && (v[1] == 'S' || v[1] == 's') {
		return v
	}
	var tb textbuf.Buffer
	return tb.Str("AS").Str(v).String()
}
