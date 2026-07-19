package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAndAdd_StopDirective drives the full .ci parse pipeline (parseLine ->
// parseCmd -> parseCmdExec/parseCmdStop), not just the leaf parsers, so it proves
// the naming + stop grammar is reachable exactly as a real test/runner/*.ci file
// exercises it: a named cmd=background and a cmd=stop referencing it land in
// Record.RunCommands with the right modes, name, and signal.
//
// VALIDATES: the .ci grammar wiring for spec-fixit-runner-kill-background (the
// parse half of the stop-background.ci wiring row).
// PREVENTS: a regression where cmd=stop parses in isolation but is dropped or
// mis-dispatched by parseCmd/parseLine and never reaches the executor.
func TestParseAndAdd_StopDirective(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "stop.ci")
	ciContent := "option=timeout:value=20s\n" +
		"cmd=background:seq=1:exec=sh holder.sh:name=holder\n" +
		"cmd=foreground:seq=2:exec=sh wait-start.sh\n" +
		"cmd=stop:seq=3:name=holder:signal=kill\n" +
		"cmd=foreground:seq=4:exec=sh assert-gone.sh\n"
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	rec, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)
	require.Len(t, rec.RunCommands, 4)

	// The order in RunCommands follows the file; the executor sorts by seq.
	bg := rec.RunCommands[0]
	assert.Equal(t, "background", bg.Mode)
	assert.Equal(t, "holder", bg.Name)
	assert.Equal(t, "sh holder.sh", bg.Exec)

	stop := rec.RunCommands[2]
	assert.Equal(t, modeStop, stop.Mode)
	assert.Equal(t, 3, stop.Seq)
	assert.Equal(t, "holder", stop.Name)
	assert.Equal(t, signalKill, stop.Signal)
	assert.Empty(t, stop.Exec, "a stop directive runs no binary")
}

// TestParseAndAdd_StopUnknownSignalRejected confirms the fail-closed guard fires
// through the full pipeline too: a bad signal= aborts parseAndAdd rather than
// producing a silently-defaulted stop step.
func TestParseAndAdd_StopUnknownSignalRejected(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "stop-bad.ci")
	ciContent := "option=timeout:value=20s\n" +
		"cmd=stop:seq=1:name=holder:signal=hup\n"
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal=")
}

// TestParseCmdExecName verifies parseCmdExec extracts the optional name= handle a
// cmd=background line assigns, in any marker order, without truncating the exec value.
//
// VALIDATES: the naming half of the grammar (spec Phase 2) -- a background process
// can be named at start so a later cmd=stop can reference it.
// PREVENTS: name= silently swallowing the exec value (or vice versa) when the two
// markers sit adjacent, which would either lose the command or lose its handle.
func TestParseCmdExecName(t *testing.T) {
	tests := []struct {
		name string
		line string
		want RunCommand
	}{
		{
			name: "name_after_exec",
			line: "cmd=background:seq=1:exec=sleep 60:name=sleeper",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "sleep 60", Name: "sleeper"},
		},
		{
			name: "name_with_stdin",
			line: "cmd=background:seq=2:exec=ze -:stdin=cfg:name=responder",
			want: RunCommand{Mode: "background", Seq: 2, Exec: "ze -", Stdin: "cfg", Name: "responder"},
		},
		{
			name: "exec_with_colon_and_name",
			line: "cmd=background:seq=1:exec=ze-chaos --web :8000:name=chaos",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "ze-chaos --web :8000", Name: "chaos"},
		},
		{
			name: "no_name_is_empty",
			line: "cmd=background:seq=1:exec=sleep 60",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "sleep 60", Name: ""},
		},
		{
			// Order-independence: name= placed BEFORE exec= must not swallow the
			// exec value (the name= end-marker set includes exec=).
			name: "name_before_exec",
			line: "cmd=background:seq=1:name=sleeper:exec=sleep 60",
			want: RunCommand{Mode: "background", Seq: 1, Exec: "sleep 60", Name: "sleeper"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCmdExec("background", tt.line)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseStopBackgroundDirective verifies parseCmdStop parses cmd=stop lines and
// FAILS CLOSED on malformed ones (AC-2 at the parse boundary): a missing/empty
// name=, a bad seq=, or an unknown signal= is a hard parse error, never a silent
// default that would let a stop step no-op.
//
// VALIDATES: spec Phase 1 (wiring) + the fail-closed guard -- the directive parses,
// defaults signal to kill, accepts explicit kill/term, and rejects everything else.
// PREVENTS: a typo'd stop directive being accepted and silently doing nothing,
// leaving the test green while the process it meant to kill keeps running.
func TestParseStopBackgroundDirective(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    RunCommand
		wantErr string
	}{
		{
			name: "default_signal_is_kill",
			line: "cmd=stop:seq=3:name=responder",
			want: RunCommand{Mode: "stop", Seq: 3, Name: "responder", Signal: "kill"},
		},
		{
			name: "explicit_kill",
			line: "cmd=stop:seq=3:name=responder:signal=kill",
			want: RunCommand{Mode: "stop", Seq: 3, Name: "responder", Signal: "kill"},
		},
		{
			name: "explicit_term",
			line: "cmd=stop:seq=4:name=daemon:signal=term",
			want: RunCommand{Mode: "stop", Seq: 4, Name: "daemon", Signal: "term"},
		},
		{
			name: "signal_before_name",
			line: "cmd=stop:seq=2:signal=term:name=sleeper",
			want: RunCommand{Mode: "stop", Seq: 2, Name: "sleeper", Signal: "term"},
		},
		{
			name:    "missing_name",
			line:    "cmd=stop:seq=3",
			wantErr: "missing name=",
		},
		{
			name:    "empty_name",
			line:    "cmd=stop:seq=3:name=",
			wantErr: "missing name=",
		},
		{
			name:    "missing_seq",
			line:    "cmd=stop:name=responder",
			wantErr: "missing seq=",
		},
		{
			name:    "invalid_seq_zero",
			line:    "cmd=stop:seq=0:name=responder",
			wantErr: "invalid seq=",
		},
		{
			name:    "invalid_signal",
			line:    "cmd=stop:seq=3:name=responder:signal=hup",
			wantErr: "invalid signal=",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCmdStop(tt.line)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
