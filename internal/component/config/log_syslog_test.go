// Design: docs/guide/logging.md -- syslog facility per subsystem
// Related: internal/core/slogutil/syslog.go -- facilityFor, syslogConfigEntries

package config

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogSyslogBlockReachesTheEnvKeys proves an environment/log/syslog block
// written in a config file parses, and that the load hands slogutil the
// facility leaf and each subsystem name under the env keys facilityFor reads.
//
// Method: ReadConfig over the file text, then EnvironmentEntries over what
// ExtractEnvironment answers, the same two steps the load applies.
func TestLogSyslogBlockReachesTheEnvKeys(t *testing.T) {
	input := "environment {\n" +
		"    log {\n" +
		"        level info\n" +
		"        bgp.server debug\n" +
		"        syslog {\n" +
		"            facility local3\n" +
		"            bgp local7\n" +
		"        }\n" +
		"    }\n" +
		"}\n"

	result, err := ReadConfig(input, "", nil)
	require.NoError(t, err)

	entries := EnvironmentEntries(ExtractEnvironment(result.Tree))
	for _, want := range []string{
		"ze.log=info",
		"ze.log.bgp.server=debug",
		"ze.log.syslog.facility=local3",
		"ze.log.syslog.bgp=local7",
	} {
		assert.True(t, slices.Contains(entries, want), "entries %q lack %q", entries, want)
	}
}

// TestLogSyslogFacilityLeafRefusesAnUnknownWord proves the facility leaf is
// checked when the config parses, so a misspelled facility never boots.
func TestLogSyslogFacilityLeafRefusesAnUnknownWord(t *testing.T) {
	input := "environment {\n    log {\n        syslog {\n            facility local9\n        }\n    }\n}\n"

	_, err := ReadConfig(input, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local9")
}
