package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseOneCI writes content to a .ci file and reads it with the generic parser
// that the orchestrated suites run.
func parseOneCI(t *testing.T, content string) (*Record, error) {
	t.Helper()
	ResetNickCounter()
	dir := t.TempDir()
	path := filepath.Join(dir, "exit.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return NewEncodingTests(dir).parseAndAdd(path)
}

// TestParseRefusesExitTheArmDrops proves an exit= is refused on every command
// whose runOrchestrated arm never compares it, and accepted on the two arms
// that do.
//
// VALIDATES: a background command, a foreground ze daemon and a foreground
// le test peer each refuse exit= with a message naming the reason and the fix;
// a foreground helper and a foreground quick-exit ze keep their exit code.
// PREVENTS: an exit= assertion parsed and then silently dropped, which reads
// as proof while the command's status is never looked at.
func TestParseRefusesExitTheArmDrops(t *testing.T) {
	refused := []struct {
		name, line, want string
	}{
		{"background", "cmd=background:seq=1:exec=sleep 60:exit=0", "never checked on a background command"},
		{"foreground_ze_daemon", "cmd=foreground:seq=1:exec=ze -:stdin=config:exit=0", "never checked on a foreground ze daemon"},
		{"foreground_peer", "cmd=foreground:seq=1:exec=le test peer --mode check:exit=0", "never checked on a foreground ze daemon or le test peer"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOneCI(t, "stdin=config:terminator=EOF\nEOF\n"+tt.line+"\n")
			require.Error(t, err, "exit= on an arm that drops it must be refused")
			assert.Contains(t, err.Error(), tt.want)
			assert.Contains(t, err.Error(), "Fix:")
		})
	}

	accepted := []struct {
		name, line string
		arm        stepArm
		code       int
	}{
		{"foreground_helper", "cmd=foreground:seq=1:exec=sh run.sh:exit=3", stepArmHelper, 3},
		{"foreground_quick_ze", "cmd=foreground:seq=1:exec=ze config validate x.conf:exit=1", stepArmQuickZe, 1},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := parseOneCI(t, tt.line+"\n")
			require.NoError(t, err)
			require.Len(t, rec.RunCommands, 1)
			rc := rec.RunCommands[0]
			require.NotNil(t, rc.ExitCode)
			assert.Equal(t, tt.code, *rc.ExitCode)
			parts, err := SplitCommand(rc.Exec)
			require.NoError(t, err)
			assert.Equal(t, tt.arm, classifyStepArm(rc.Mode, parts))
		})
	}
}

// TestStepArmAssertsExitOnlyOnAwaitedArms pins the one list of arms whose
// exit= runOrchestrated compares: the two foreground arms that await their
// process. A new arm, or a change to an existing one, fails here first.
func TestStepArmAssertsExitOnlyOnAwaitedArms(t *testing.T) {
	want := map[stepArm]bool{
		stepArmUnspecified: false,
		stepArmBackground:  false,
		stepArmHelper:      true,
		stepArmQuickZe:     true,
		stepArmDaemon:      false,
	}
	for arm, asserts := range want {
		assert.Equal(t, asserts, arm.assertsExit(), "arm %d", arm)
	}
}

// TestParseSuiteHonoursCommandExit proves the parse suite keeps a command's
// own exit= as its exit assertion, and refuses one that an expect=exit:code=
// on the same command contradicts.
//
// PREVENTS: the parse suite reading exit= through the shared cmd= parser and
// then dropping it, so a command's wrong exit status passed unseen.
func TestParseSuiteHonoursCommandExit(t *testing.T) {
	dir := t.TempDir()
	pt := NewParsingTests(dir)

	kept := filepath.Join(dir, "kept.ci")
	require.NoError(t, os.WriteFile(kept, []byte("cmd=foreground:seq=1:exec=ze config validate x.conf:exit=2\n"), 0o600))
	test, err := pt.parseCIFile(kept)
	require.NoError(t, err)
	require.Len(t, test.Commands, 1)
	assert.True(t, test.Commands[0].HasExitCode, "exit= must become the command's exit assertion")
	assert.Equal(t, 2, test.Commands[0].ExpectExitCode)

	agree := filepath.Join(dir, "agree.ci")
	require.NoError(t, os.WriteFile(agree, []byte("cmd=foreground:seq=1:exec=ze config validate x.conf:exit=2\nexpect=exit:code=2\n"), 0o600))
	_, err = pt.parseCIFile(agree)
	require.NoError(t, err, "two assertions of the same status agree")

	clash := filepath.Join(dir, "clash.ci")
	require.NoError(t, os.WriteFile(clash, []byte("cmd=foreground:seq=1:exec=ze config validate x.conf:exit=2\nexpect=exit:code=1\n"), 0o600))
	_, err = pt.parseCIFile(clash)
	require.Error(t, err, "contradicting exit assertions must be refused")
	assert.Contains(t, err.Error(), "exit=2 but expect=exit:code=1")
}

// TestSelectForegroundProcessCountsAForegroundPeer proves the end-of-test arms
// synchronize on a cmd=foreground le test peer that follows a background ze,
// the shape of test/plugin/filter-wire-match-conditions-external.ci, while a
// background peer is still never selected and a file with only a foreground
// ze daemon keeps selecting it.
// PREVENTS: the exit-code arm waiting on the background ze, which never exits
// on its own, so the test ran to its timeout although its peer had passed.
func TestSelectForegroundProcessCountsAForegroundPeer(t *testing.T) {
	source, daemon, dest := &ownedProcess{}, &ownedProcess{}, &ownedProcess{}
	peers := map[*ownedProcess]bool{source: true, dest: true}

	got := selectForegroundProcess([]*ownedProcess{source, daemon, dest}, peers, dest)
	assert.Same(t, dest, got, "a cmd=foreground peer after a background ze is the process the file waits on")

	got = selectForegroundProcess([]*ownedProcess{source, dest, daemon}, peers, daemon)
	assert.Same(t, daemon, got, "a cmd=foreground ze after the peers stays the selected process")

	got = selectForegroundProcess([]*ownedProcess{source, dest}, peers, nil)
	assert.Nil(t, got, "background peers alone select no process")
}
