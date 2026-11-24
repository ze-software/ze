// Design: docs/architecture/api/architecture.md -- unit tests for BusBlock parsing, Q-1 none exclusivity, and equality
// Related: bus_config.go -- BusBlock AST and ParseBusBlockFromMap

package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestParseBusBlockFromMap verifies direction-grouped bus block extraction,
// Q-1 none exclusivity, and order-independent set equality.
//
// VALIDATES: ParseBusBlockFromMap extracts receive/send allow, to/from,
// per-message rules, and tags; refuses combining none with explicit entries;
// BusBlockEqual ignores list ordering.
// PREVENTS: Ambiguous none+value combinations or order-sensitive Refusal 6 false conflicts.
func TestParseBusBlockFromMap(t *testing.T) {
	rawA := map[string]any{
		"receive": map[string]any{
			"allow": []string{"sys:state", "sys:update"},
			"from":  []string{"/sys/node/b", "/sys/node/a"},
			"message": map[string]any{
				"sys:update": map[string]any{
					"from": "/sys/node/a",
					"tag": map[string]any{
						"direction": map[string]any{
							"value": []string{"sent", "received"},
						},
					},
				},
			},
		},
		"send": map[string]any{
			"allow": "none",
			"to":    []any{"none"},
		},
	}

	rawB := map[string]any{
		"receive": map[string]any{
			"allow": []any{"sys:update", "sys:state"},
			"from":  []string{"/sys/node/a", "/sys/node/b"},
			"message": map[string]any{
				"sys:update": map[string]any{
					"from": []string{"/sys/node/a"},
					"tag": map[string]any{
						"direction": map[string]any{
							"value": []string{"received", "sent"},
						},
					},
				},
			},
		},
		"send": map[string]any{
			"allow": []string{"none"},
			"to":    "none",
		},
	}

	blockA, err := ParseBusBlockFromMap(rawA, "bus > endpoint test")
	require.NoError(t, err)
	blockB, err := ParseBusBlockFromMap(rawB, "bus > participant /bus/endpoint/test")
	require.NoError(t, err)

	assert.False(t, blockA.IsEmpty())
	assert.True(t, blockA.Send.AllowPresent)
	assert.True(t, blockA.Send.AllowNone)
	assert.Empty(t, blockA.Send.Allow)
	assert.True(t, blockA.Send.TargetsPresent)
	assert.True(t, blockA.Send.TargetsNone)
	assert.True(t, BusBlockEqual(blockA, blockB), "order-independent set comparison must treat A and B as equal")

	// Clone produces an independent deep copy.
	cloned := blockA.Clone()
	cloned.Receive.Allow[0] = "changed:msg"
	assert.False(t, BusBlockEqual(blockA, cloned))
}

// TestParseBusBlockRefusesNoneCombinedWithEntries verifies Q-1 exclusivity.
//
// VALIDATES: Combining "none" with any other value in allow, to, from, or
// per-message target lists returns an *opererr.Error naming the offending line.
// PREVENTS: Silent ambiguity between empty and non-empty bus lists.
func TestParseBusBlockRefusesNoneCombinedWithEntries(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{
			name: "receive allow none plus msg",
			raw: map[string]any{
				"receive": map[string]any{"allow": []string{"none", "sys:state"}},
			},
			want: "receive > allow",
		},
		{
			name: "receive from none plus path",
			raw: map[string]any{
				"receive": map[string]any{"from": []string{"/sys/a", "none"}},
			},
			want: "receive > from",
		},
		{
			name: "send to none plus path",
			raw: map[string]any{
				"send": map[string]any{"to": []string{"none", "/sys/a"}},
			},
			want: "send > to",
		},
		{
			name: "per-message to none plus path",
			raw: map[string]any{
				"send": map[string]any{
					"message": map[string]any{
						"sys:update": map[string]any{"to": []string{"none", "/sys/a"}},
					},
				},
			},
			want: "send > message sys:update > to",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseBusBlockFromMap(tc.raw, "bus > endpoint sample")
			require.Error(t, err)
			opErr, ok := errors.AsType[*opererr.Error](err)
			require.True(t, ok, "refusal must be an *opererr.Error")
			assert.Contains(t, opErr.Error(), tc.want)
			assert.Contains(t, opErr.Error(), "none")
			assert.NotEmpty(t, opErr.Fix())
		})
	}
}
