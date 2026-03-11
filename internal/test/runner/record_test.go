package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCILoggingOptions verifies parsing of logging-related .ci options.
//
// VALIDATES: option=env:, expect=stderr:, reject=stderr:, expect=syslog: are parsed correctly.
// PREVENTS: Logging tests silently failing due to parsing errors.
func TestParseCILoggingOptions(t *testing.T) {
	tests := []struct {
		name          string
		ciContent     string
		confContent   string
		wantEnvVars   []string
		wantExpStderr []string
		wantRejStderr []string
		wantExpSyslog []string
	}{
		{
			name: "single_env_var",
			ciContent: `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   []string{"ze.log.bgp.server=debug"},
			wantExpStderr: nil,
			wantRejStderr: nil,
			wantExpSyslog: nil,
		},
		{
			name: "multiple_env_vars",
			ciContent: `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
option=env:var=ze.log.bgp.filter:value=info
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   []string{"ze.log.bgp.server=debug", "ze.log.bgp.filter=info"},
			wantExpStderr: nil,
			wantRejStderr: nil,
			wantExpSyslog: nil,
		},
		{
			name: "expect_stderr_pattern",
			ciContent: `option=file:path=test.conf
expect=stderr:pattern=subsystem=server
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   nil,
			wantExpStderr: []string{"subsystem=server"},
			wantRejStderr: nil,
			wantExpSyslog: nil,
		},
		{
			name: "reject_stderr_pattern",
			ciContent: `option=file:path=test.conf
reject=stderr:pattern=level=DEBUG
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   nil,
			wantExpStderr: nil,
			wantRejStderr: []string{"level=DEBUG"},
			wantExpSyslog: nil,
		},
		{
			name: "expect_syslog_pattern",
			ciContent: `option=file:path=test.conf
expect=syslog:pattern=subsystem=server
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   nil,
			wantExpStderr: nil,
			wantRejStderr: nil,
			wantExpSyslog: []string{"subsystem=server"},
		},
		{
			name: "combined_logging_options",
			ciContent: `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
option=env:var=ze.log.bgp.backend:value=syslog
expect=stderr:pattern=subsystem=server
reject=stderr:pattern=level=ERROR
expect=syslog:pattern=msg=test
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   []string{"ze.log.bgp.server=debug", "ze.log.bgp.backend=syslog"},
			wantExpStderr: []string{"subsystem=server"},
			wantRejStderr: []string{"level=ERROR"},
			wantExpSyslog: []string{"msg=test"},
		},
		{
			name: "empty_patterns",
			ciContent: `option=file:path=test.conf
expect=stderr:pattern=
reject=stderr:pattern=
expect=syslog:pattern=
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   nil,
			wantExpStderr: []string{""},
			wantRejStderr: []string{""},
			wantExpSyslog: []string{""},
		},
		{
			name: "regex_patterns",
			ciContent: `option=file:path=test.conf
expect=stderr:pattern=level=(INFO|DEBUG)
expect=stderr:pattern=subsystem=\w+
reject=stderr:pattern=error.*fatal
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`,
			confContent:   minimalConfig,
			wantEnvVars:   nil,
			wantExpStderr: []string{"level=(INFO|DEBUG)", "subsystem=\\w+"},
			wantRejStderr: []string{"error.*fatal"},
			wantExpSyslog: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset nick counter for consistent test results
			ResetNickCounter()

			// Create temp directory with test files
			tmpDir := t.TempDir()
			ciFile := filepath.Join(tmpDir, "test.ci")
			confFile := filepath.Join(tmpDir, "test.conf")

			require.NoError(t, os.WriteFile(ciFile, []byte(tt.ciContent), 0o600))
			require.NoError(t, os.WriteFile(confFile, []byte(tt.confContent), 0o600))

			// Parse the .ci file
			et := NewEncodingTests(tmpDir)
			err := et.parseAndAdd(ciFile)
			require.NoError(t, err)

			// Get the parsed record
			rec := et.GetByNick("0")
			require.NotNil(t, rec, "record should exist")

			// Verify logging options
			assert.Equal(t, tt.wantEnvVars, rec.EnvVars, "EnvVars mismatch")
			assert.Equal(t, tt.wantExpStderr, rec.ExpectStderr, "ExpectStderr mismatch")
			assert.Equal(t, tt.wantRejStderr, rec.RejectStderr, "RejectStderr mismatch")
			assert.Equal(t, tt.wantExpSyslog, rec.ExpectSyslog, "ExpectSyslog mismatch")
		})
	}
}

// TestParseCILoggingOptionsNotAffectOthers verifies logging options don't affect other parsing.
//
// VALIDATES: Adding logging options doesn't break existing .ci parsing.
// PREVENTS: Regression in message/option parsing when logging options present.
func TestParseCILoggingOptionsNotAffectOthers(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "test.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
option=asn:value=65000
option=env:var=ze.log.bgp.server:value=debug
expect=stderr:pattern=subsystem=server
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02`

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("0")
	require.NotNil(t, rec)

	// Verify existing options still work
	assert.Equal(t, "65000", rec.Extra["asn"])
	assert.Len(t, rec.Messages, 2, "should have 2 messages")
	assert.Len(t, rec.Expects, 2, "should have 2 expects")

	// Verify logging options also work
	assert.Equal(t, []string{"ze.log.bgp.server=debug"}, rec.EnvVars)
	assert.Equal(t, []string{"subsystem=server"}, rec.ExpectStderr)
}

// minimalConfig is a minimal valid ZeBGP config for testing.
const minimalConfig = `peer 127.0.0.1 {
    router-id 1.2.3.4
    local-address 127.0.0.1
    local-as 1
    peer-as 1
}
`
