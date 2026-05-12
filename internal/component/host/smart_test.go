package host

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canned smartctl JSON for a healthy NVMe drive.
const smartctlHealthyJSON = `{
  "json_format_version": [1, 0],
  "smartctl": {
    "version": [7, 4],
    "exit_status": 0
  },
  "smart_status": {
    "passed": true
  },
  "temperature": {
    "current": 38
  },
  "power_on_time": {
    "hours": 12345
  },
  "ata_smart_error_log": {
    "summary": {
      "count": 0
    }
  }
}`

// Canned smartctl JSON for a device that does not support SMART.
const smartctlUnsupportedJSON = `{
  "json_format_version": [1, 0],
  "smartctl": {
    "version": [7, 4],
    "exit_status": 2,
    "messages": [
      {
        "string": "SMART support is: Unavailable",
        "severity": "error"
      }
    ]
  }
}`

// Canned smartctl JSON for a healthy SATA drive with non-zero errors.
const smartctlSATAJSON = `{
  "json_format_version": [1, 0],
  "smartctl": {
    "version": [7, 4],
    "exit_status": 0
  },
  "smart_status": {
    "passed": true
  },
  "temperature": {
    "current": 42
  },
  "power_on_time": {
    "hours": 55000
  },
  "ata_smart_error_log": {
    "summary": {
      "count": 7
    }
  }
}`

func TestParseSMARTJSON_Healthy(t *testing.T) {
	info, err := parseSMARTJSON([]byte(smartctlHealthyJSON))
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.True(t, info.Healthy)
	assert.Equal(t, 38, info.TempCelsius)
	assert.Equal(t, uint64(12345), info.PowerOnHours)
	assert.Equal(t, uint64(0), info.ErrorCount)
	assert.False(t, info.Unavailable)
}

func TestParseSMARTJSON_Unsupported(t *testing.T) {
	info, err := parseSMARTJSON([]byte(smartctlUnsupportedJSON))
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.True(t, info.Unavailable)
	assert.Contains(t, info.UnavailableNote, "Unavailable")
}

func TestParseSMARTJSON_SATAWithErrors(t *testing.T) {
	info, err := parseSMARTJSON([]byte(smartctlSATAJSON))
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.True(t, info.Healthy)
	assert.Equal(t, 42, info.TempCelsius)
	assert.Equal(t, uint64(55000), info.PowerOnHours)
	assert.Equal(t, uint64(7), info.ErrorCount)
}

func TestParseSMARTJSON_InvalidJSON(t *testing.T) {
	_, err := parseSMARTJSON([]byte(`{not json`))
	assert.Error(t, err)
}

func TestSmartInfoJSON(t *testing.T) {
	info := &SmartInfo{
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
