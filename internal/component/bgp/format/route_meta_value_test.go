// Design: docs/architecture/meta/README.md -- Key Registry, the route-meta value types
// Related: text_update.go -- appendRouteMetaValue, the route-meta object writer

package format

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppendRouteMetaValue verifies each registered route-meta value type is
// written as the JSON json.Marshal would produce for it, a string with escaping
// included, and an unregistered type still goes through json.Marshal.
//
// VALIDATES: the direct writers for string, uint8 and bool, and the fallback.
// PREVENTS: a received event's route-meta carrying malformed JSON for a role
// name or the stale level once the per-key json.Marshal is gone.
func TestAppendRouteMetaValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"string", "customer"},
		{"string-escaped", "a\"b\\c\n"},
		{"uint8-stale", uint8(2)},
		{"bool", true},
		{"fallback-int", 7},
		{"fallback-slice", []string{"x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := json.Marshal(c.value)
			require.NoError(t, err)
			got, written := appendRouteMetaValue(nil, c.value)
			require.True(t, written)
			assert.Equal(t, string(want), string(got))
		})
	}
}

// TestAppendRouteMetaValueRefusesUnencodable verifies a value json.Marshal
// cannot encode is reported unwritten and leaves the buffer as it was.
//
// VALIDATES: the caller can drop the key without leaving a dangling "key":.
// PREVENTS: a route-meta object that does not parse.
func TestAppendRouteMetaValueRefusesUnencodable(t *testing.T) {
	buf := []byte("prefix")
	got, written := appendRouteMetaValue(buf, make(chan int))
	assert.False(t, written)
	assert.Equal(t, "prefix", string(got))
}
