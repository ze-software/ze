package host

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/smart"
)

func TestSmartInfoJSON(t *testing.T) {
	info := &smart.Info{
		Healthy:      true,
		TempCelsius:  38,
		PowerOnHours: 12345,
		ErrorCount:   0,
	}
	data, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"healthy":true`)
	assert.Contains(t, string(data), `"temp-celsius":38`)
}
