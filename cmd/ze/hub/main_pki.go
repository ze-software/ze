package hub

import (
	"strconv"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zepki "github.com/ze-software/ze/internal/component/pki"
)

func preparePKIConfig(tree map[string]any) (*zepki.PKIConfig, error) {
	cfg, err := zepki.ParseConfig(configTreeFromMap(tree))
	if err != nil {
		return nil, err
	}
	if err := zepki.Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func configTreeFromMap(m map[string]any) *zeconfig.Tree {
	if m == nil {
		return nil
	}
	t := zeconfig.NewTree()
	for k, v := range m {
		switch val := v.(type) {
		case string:
			t.Set(k, val)
		case float64:
			t.Set(k, strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			if val {
				t.Set(k, "true")
			} else {
				t.Set(k, "false")
			}
		case map[string]any:
			t.SetContainer(k, configTreeFromMap(val))
			if mapValuesAreMaps(val) {
				for entryKey, entryVal := range val {
					entryMap, ok := entryVal.(map[string]any)
					if !ok {
						continue
					}
					t.AddListEntry(k, entryKey, configTreeFromMap(entryMap))
				}
			}
		case []any:
			for _, item := range val {
				itemStr, ok := item.(string)
				if !ok {
					continue
				}
				t.AppendValue(k, itemStr)
			}
		}
	}
	return t
}

func mapValuesAreMaps(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for _, v := range m {
		if _, ok := v.(map[string]any); !ok {
			return false
		}
	}
	return true
}
