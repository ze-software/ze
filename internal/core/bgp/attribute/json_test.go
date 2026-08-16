package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterJSONFormatter(t *testing.T) {
	code := AttributeCode(200)
	assert.Nil(t, GetJSONFormatter(code))

	RegisterJSONFormatter(code, "test-attr", func(buf []byte, attr Attribute) []byte {
		return append(buf, `"test-value"`...)
	})

	f := GetJSONFormatter(code)
	require.NotNil(t, f)
	assert.Equal(t, "test-attr", f.Key)

	buf := f.AppendValue(nil, nil)
	assert.Equal(t, `"test-value"`, string(buf))
}

// TestIPv6ExtCommunityJSONFormatter moved to filter_community/json_test.go
// (formatter registration moved from attribute/register.go to the plugin that owns community codes)
