package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiffMapsEmpty verifies that identical maps produce an empty diff.
//
// VALIDATES: Same maps produce empty diff with no Added/Removed/Changed.
// PREVENTS: False positives when comparing identical configs.
func TestDiffMapsEmpty(t *testing.T) {
	old := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65001,
	}
	new := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65001,
	}

	diff := DiffMaps(old, new)

	assert.Empty(t, diff.Added, "identical maps should have no added keys")
	assert.Empty(t, diff.Removed, "identical maps should have no removed keys")
	assert.Empty(t, diff.Changed, "identical maps should have no changed keys")
}

// TestDiffMapsAdded verifies detection of new keys.
//
// VALIDATES: Keys present in new but not old appear in Added.
// PREVENTS: Missing detection of new config additions.
func TestDiffMapsAdded(t *testing.T) {
	old := map[string]any{
		"router-id": "1.2.3.4",
	}
	new := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65001,
	}

	diff := DiffMaps(old, new)

	assert.Contains(t, diff.Added, "asn")
	assert.Equal(t, 65001, diff.Added["asn"])
	assert.Empty(t, diff.Removed)
	assert.Empty(t, diff.Changed)
}

// TestDiffMapsRemoved verifies detection of deleted keys.
//
// VALIDATES: Keys present in old but not new appear in Removed.
// PREVENTS: Missing detection of config deletions.
func TestDiffMapsRemoved(t *testing.T) {
	old := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65001,
	}
	new := map[string]any{
		"router-id": "1.2.3.4",
	}

	diff := DiffMaps(old, new)

	assert.Empty(t, diff.Added)
	assert.Contains(t, diff.Removed, "asn")
	assert.Equal(t, 65001, diff.Removed["asn"])
	assert.Empty(t, diff.Changed)
}

// TestDiffMapsChanged verifies detection of modified values.
//
// VALIDATES: Keys with different values appear in Changed with old/new pair.
// PREVENTS: Missing detection of config value changes.
func TestDiffMapsChanged(t *testing.T) {
	old := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65001,
	}
	new := map[string]any{
		"router-id": "1.2.3.4",
		"asn":       65002,
	}

	diff := DiffMaps(old, new)

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Removed)
	assert.Contains(t, diff.Changed, "asn")
	assert.Equal(t, 65001, diff.Changed["asn"].Old)
	assert.Equal(t, 65002, diff.Changed["asn"].New)
}

// TestDiffMapsNested verifies deep comparison of nested maps.
//
// VALIDATES: Nested map changes detected with dotted key paths.
// PREVENTS: Missing nested config changes in peer blocks.
func TestDiffMapsNested(t *testing.T) {
	old := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65001,
			},
		},
	}
	new := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65002,
			},
		},
	}

	diff := DiffMaps(old, new)

	// Nested changes use dotted path
	assert.Contains(t, diff.Changed, "peer/192.168.1.1/asn")
	assert.Equal(t, 65001, diff.Changed["peer/192.168.1.1/asn"].Old)
	assert.Equal(t, 65002, diff.Changed["peer/192.168.1.1/asn"].New)
}

// TestDiffMapsNestedAdd verifies detection of new nested keys.
//
// VALIDATES: New keys in nested maps detected with dotted path.
// PREVENTS: Missing new peer or capability additions.
func TestDiffMapsNestedAdd(t *testing.T) {
	old := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65001,
			},
		},
	}
	new := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65001,
			},
			"192.168.1.2": map[string]any{
				"asn": 65002,
			},
		},
	}

	diff := DiffMaps(old, new)

	// New peer subtree appears as added
	assert.Contains(t, diff.Added, "peer/192.168.1.2")
}

// TestDiffMapsNestedRemove verifies detection of removed nested keys.
//
// VALIDATES: Removed keys in nested maps detected with dotted path.
// PREVENTS: Missing peer or capability deletions.
func TestDiffMapsNestedRemove(t *testing.T) {
	old := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65001,
			},
			"192.168.1.2": map[string]any{
				"asn": 65002,
			},
		},
	}
	new := map[string]any{
		"peer": map[string]any{
			"192.168.1.1": map[string]any{
				"asn": 65001,
			},
		},
	}

	diff := DiffMaps(old, new)

	// Removed peer subtree appears as removed
	assert.Contains(t, diff.Removed, "peer/192.168.1.2")
}

// TestDiffMapsNilOld verifies diff with nil old map (initial config).
//
// VALIDATES: Nil old map treats all new keys as added.
// PREVENTS: Panic or incorrect handling of initial config load.
func TestDiffMapsNilOld(t *testing.T) {
	new := map[string]any{
		"router-id": "1.2.3.4",
	}

	diff := DiffMaps(nil, new)

	assert.Contains(t, diff.Added, "router-id")
	assert.Empty(t, diff.Removed)
	assert.Empty(t, diff.Changed)
}

// TestDiffMapsNilNew verifies diff with nil new map (config cleared).
//
// VALIDATES: Nil new map treats all old keys as removed.
// PREVENTS: Panic or incorrect handling of config clear.
func TestDiffMapsNilNew(t *testing.T) {
	old := map[string]any{
		"router-id": "1.2.3.4",
	}

	diff := DiffMaps(old, nil)

	assert.Empty(t, diff.Added)
	assert.Contains(t, diff.Removed, "router-id")
	assert.Empty(t, diff.Changed)
}

// TestDiffMapsBothNil verifies diff with both nil maps.
//
// VALIDATES: Both nil produces empty diff.
// PREVENTS: Panic on double-nil comparison.
func TestDiffMapsBothNil(t *testing.T) {
	diff := DiffMaps(nil, nil)

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Removed)
	assert.Empty(t, diff.Changed)
}

// TestDiffMapsTypeChange verifies detection when value type changes.
//
// VALIDATES: Changing value from one type to another is detected as change.
// PREVENTS: Type coercion hiding config changes.
func TestDiffMapsTypeChange(t *testing.T) {
	old := map[string]any{
		"value": "65001", // string
	}
	new := map[string]any{
		"value": 65001, // int
	}

	diff := DiffMaps(old, new)

	assert.Contains(t, diff.Changed, "value")
	assert.Equal(t, "65001", diff.Changed["value"].Old)
	assert.Equal(t, 65001, diff.Changed["value"].New)
}

// TestDiffMapsSliceChange verifies detection when slice values change.
//
// VALIDATES: Changing slice values detected as change.
// PREVENTS: Missing detection of multi-value config changes.
func TestDiffMapsSliceChange(t *testing.T) {
	old := map[string]any{
		"families": []string{"ipv4/unicast"},
	}
	new := map[string]any{
		"families": []string{"ipv4/unicast", "ipv6/unicast"},
	}

	diff := DiffMaps(old, new)

	assert.Contains(t, diff.Changed, "families")
}
