// VALIDATES: the stream name on an expect= selects that stream of its
//            command's span alone; a reject= never reads less than it did
//            before the scoping: reject=stderr:pattern= reads the whole run's
//            stderr, every other reject both streams of its command's span.
// PREVENTS: the 2026-09-07 journal row's remaining half: a command's span
//           covered both streams, so expect=stdout:contains= passed on text the
//           command wrote to stderr. And the converse slip: a reject narrowed
//           to one stream or one command checks less than it did.

package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamRecord builds a record whose one command wrote "out-text" to stdout
// and "err-text" to stderr.
func streamRecord(cmd *RunCommand) *Record {
	const stdout, stderr = "out-text\n", "err-text\n"
	rec := &Record{RunCommands: []RunCommand{*cmd}}
	span := &rec.RunCommands[0]
	span.Seq = 1
	span.stdoutFrom, span.stdoutTo = 0, len(stdout)
	span.stderrFrom, span.stderrTo = 0, len(stderr)
	rec.ClientStdout, rec.ClientStderr = stdout, stderr
	rec.ClientOutput = stdout + stderr
	return rec
}

// checkRun runs the two checks a run makes over its stream assertions, in the
// order the run makes them, and reports whether both held. The error is on
// rec.Error.
func checkRun(rec *Record) bool {
	if !checkOutputAssertions(rec) {
		return false
	}
	if err := (&Runner{}).validateLogging(rec, rec.ClientStderr, nil); err != nil {
		rec.Error = err
		return false
	}
	return true
}

// A reject= other than reject=stderr:pattern= reads both streams, as it did
// before expect= learned to select one; reject=stderr:pattern= reads the run's
// stderr, which holds no stdout byte.
func TestStreamAssertionReadsTheStreamItNames(t *testing.T) {
	tests := []struct {
		name string
		cmd  RunCommand
		pass bool
	}{
		{"stdout contains stdout text", RunCommand{ExpectStdout: []string{"out-text"}}, true},
		{"stdout contains stderr text", RunCommand{ExpectStdout: []string{"err-text"}}, false},
		{"stdout pattern stderr text", RunCommand{ExpectStdoutRe: []string{"err-t.xt"}}, false},
		{"stdout reject stderr text", RunCommand{RejectStdout: []string{"err-text"}}, false},
		{"stdout reject pattern stderr text", RunCommand{RejectStdoutRe: []string{"err-t.xt"}}, false},
		{"stderr contains stderr text", RunCommand{ExpectStderrHas: []string{"err-text"}}, true},
		{"stderr contains stdout text", RunCommand{ExpectStderrHas: []string{"out-text"}}, false},
		{"stderr reject stdout text", RunCommand{RejectStderrHas: []string{"out-text"}}, false},
		{"stderr reject stderr text", RunCommand{RejectStderrHas: []string{"err-text"}}, false},
		{"stderr pattern stderr text", RunCommand{ExpectStderrRe: []string{"err-t.xt"}}, true},
		{"stderr pattern stdout text", RunCommand{ExpectStderrRe: []string{"out-t.xt"}}, false},
		{"stderr reject pattern stdout text", RunCommand{RejectStderrRe: []string{"out-t.xt"}}, true},
		{"stderr reject pattern stderr text", RunCommand{RejectStderrRe: []string{"err-t.xt"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := streamRecord(&tt.cmd)
			got := checkRun(rec)
			assert.Equal(t, tt.pass, got, "error: %v", rec.Error)
		})
	}
}

// TestStreamAssertionMissNamesTheOtherStream proves a needle the command wrote
// to the other stream is diagnosed as such, not as absent.
func TestStreamAssertionMissNamesTheOtherStream(t *testing.T) {
	rec := streamRecord(&RunCommand{ExpectStdout: []string{"err-text"}})
	require.False(t, checkOutputAssertions(rec))
	assert.Contains(t, rec.Error.Error(), "stderr")
}

// TestStderrPatternIsScopedToItsCommand proves expect=stderr:pattern= reads the
// stderr span of the cmd= line above it and nothing else.
//
// VALIDATES: a pattern under seq=1 that only seq=2's stderr matches fails, and
//
//	the failure names seq=2 as the command that wrote it.
//
// PREVENTS: test/ui/test-help-usage-modifiers.ci's shape: the pattern lived on
//
//	the record and read the whole run's stderr, so seq=1's regex
//	passed on the line seq=2 printed.
func TestStderrPatternIsScopedToItsCommand(t *testing.T) {
	rec, err := parseCILines(t,
		"cmd=foreground:seq=1:exec=ze version",
		"expect=stderr:pattern=be.a",
		"cmd=foreground:seq=2:exec=ze version",
		"reject=stderr:pattern=alp.a",
	)
	require.NoError(t, err)
	require.Len(t, rec.RunCommands, 2)
	assert.Equal(t, []string{"be.a"}, rec.RunCommands[0].ExpectStderrRe)
	assert.Equal(t, []string{"alp.a"}, rec.RunCommands[1].RejectStderrRe)
	assert.Empty(t, rec.RunCommands[1].ExpectStderrRe)

	const stderr = "alpha\nbeta\n"
	first, second := &rec.RunCommands[0], &rec.RunCommands[1]
	first.stderrFrom, first.stderrTo = 0, len("alpha\n")
	second.stderrFrom, second.stderrTo = len("alpha\n"), len(stderr)
	rec.ClientStderr, rec.ClientOutput = stderr, stderr

	require.False(t, checkOutputAssertions(rec), "seq=1 wrote no beta, so its pattern must miss")
	assert.Contains(t, rec.Error.Error(), "cmd seq=1")
	assert.Contains(t, rec.Error.Error(), "cmd seq=2 (ze version) wrote it")

	rec.Error = nil
	first.ExpectStderrRe = []string{"alp.a"}
	require.False(t, checkRun(rec),
		"seq=2's reject=stderr:pattern= is a whole-run log check, so the alpha seq=1 wrote is its to see")
	assert.Contains(t, rec.Error.Error(), "written under cmd seq=2")
	assert.Contains(t, rec.Error.Error(), "found inside the stderr span of cmd seq=1 (ze version)")

	rec.Error = nil
	second.RejectStderrRe = []string{"gamm.a"}
	assert.True(t, checkRun(rec), "no command printed gamma: %v", rec.Error)
}

// daemonAndFixture builds a two-command record: seq=1 is the daemon and wrote
// "daemon-out" to stdout and "daemon-err" to stderr, seq=2 is a fixture and
// wrote "fixture-out" and "fixture-err". The assertions under test sit on the
// fixture.
func daemonAndFixture(fixture *RunCommand) *Record {
	const daemonOut, daemonErr = "daemon-out\n", "daemon-err\n"
	const fixtureOut, fixtureErr = "fixture-out\n", "fixture-err\n"
	rec := &Record{RunCommands: []RunCommand{{Seq: 1, Exec: "ze start"}, *fixture}}
	daemon, fix := &rec.RunCommands[0], &rec.RunCommands[1]
	fix.Seq, fix.Exec = 2, "le test fixture x"
	daemon.stdoutFrom, daemon.stdoutTo = 0, len(daemonOut)
	daemon.stderrFrom, daemon.stderrTo = 0, len(daemonErr)
	fix.stdoutFrom, fix.stdoutTo = len(daemonOut), len(daemonOut)+len(fixtureOut)
	fix.stderrFrom, fix.stderrTo = len(daemonErr), len(daemonErr)+len(fixtureErr)
	rec.ClientStdout = daemonOut + fixtureOut
	rec.ClientStderr = daemonErr + fixtureErr
	rec.ClientOutput = rec.ClientStdout + rec.ClientStderr
	return rec
}

// TestRejectUnderASiblingIgnoresTheDaemon proves a contains= reject, and a
// reject=stdout:pattern=, read the span of the command they sit under, as an
// expect= does, and not the whole run.
//
// VALIDATES: such a reject under the fixture passes when only the daemon, a
//
//	sibling command, printed the forbidden text.
//
// PREVENTS: the whole-run reject, under which a reject written about one
//
//	command's output tripped on text a different command printed:
//	test/ui/cli-argument-length-refused.ci refuses a message under the
//	command that must not print it while a sibling command prints it.
func TestRejectUnderASiblingIgnoresTheDaemon(t *testing.T) {
	tests := []struct {
		name string
		cmd  RunCommand
	}{
		{"reject stderr contains", RunCommand{RejectStderrHas: []string{"daemon-err"}}},
		{"reject stdout pattern", RunCommand{RejectStdoutRe: []string{"daemon-o.t"}}},
		{"reject stdout contains", RunCommand{RejectStdout: []string{"daemon-out"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := daemonAndFixture(&tt.cmd)
			assert.True(t, checkRun(rec), "only the sibling daemon printed it: %v", rec.Error)
		})
	}
}

// TestStderrPatternRejectReadsTheWholeRun proves reject=stderr:pattern= is a
// log check over the whole run's stderr wherever it sits, exactly what the
// file-wide check read before the scoping.
//
// VALIDATES: the reject under the fixture fails when only the daemon, which
//
//	started before the fixture, printed the forbidden text.
//
// PREVENTS: test/ipsec's shape: the reject under the initiator was blind to
//
//	the responder started before it, and under one daemon blind to a
//	second daemon (test/plugin/iface-tunnel-restart.ci).
func TestStderrPatternRejectReadsTheWholeRun(t *testing.T) {
	rec := daemonAndFixture(&RunCommand{RejectStderrRe: []string{"daemon-e.r"}})
	require.False(t, checkRun(rec), "the daemon's stderr is the run's stderr")
	assert.Contains(t, rec.Error.Error(), "written under cmd seq=2")
	assert.Contains(t, rec.Error.Error(), "found inside the stderr span of cmd seq=1 (ze start)")

	rec = daemonAndFixture(&RunCommand{RejectStderrRe: []string{"daemon-o.t"}})
	assert.True(t, checkRun(rec), "the run's stderr holds no stdout byte: %v", rec.Error)
}

// daemonCoveringFixture builds the record a real run produces: seq=1 is a
// daemon whose span opens at its start and closes at the end of the run, so it
// covers the bytes seq=2, a fixture, wrote after it. The assertions under test
// sit on the daemon.
func daemonCoveringFixture(daemonAsserts *RunCommand) *Record {
	const daemonOut, daemonErr = "daemon-out\n", "daemon-err\n"
	const fixtureOut, fixtureErr = "fixture-out\n", "fixture-err\n"
	rec := &Record{RunCommands: []RunCommand{*daemonAsserts, {Seq: 2, Exec: "le test fixture x"}}}
	daemon, fix := &rec.RunCommands[0], &rec.RunCommands[1]
	daemon.Seq, daemon.Exec = 1, "ze start"
	rec.ClientStdout = daemonOut + fixtureOut
	rec.ClientStderr = daemonErr + fixtureErr
	rec.ClientOutput = rec.ClientStdout + rec.ClientStderr
	daemon.stdoutFrom, daemon.stdoutTo = 0, len(rec.ClientStdout)
	daemon.stderrFrom, daemon.stderrTo = 0, len(rec.ClientStderr)
	fix.stdoutFrom, fix.stdoutTo = len(daemonOut), len(rec.ClientStdout)
	fix.stderrFrom, fix.stderrTo = len(daemonErr), len(rec.ClientStderr)
	return rec
}

// TestRejectUnderTheDaemonSeesLaterOutput proves a reject under the daemon's
// cmd= line reads every byte written after the daemon started, a later
// command's included. That is what lets a reject about the daemon's log sit
// under the daemon and read no less than the file-wide check it replaced.
//
// VALIDATES: a reject under the daemon fails on text the later fixture wrote,
//
//	and the failure names the fixture as the writer.
//
// PREVENTS: a daemon span closed early, which would let a panic the daemon
//
//	logs while a later command runs pass every reject.
func TestRejectUnderTheDaemonSeesLaterOutput(t *testing.T) {
	tests := []struct {
		name string
		cmd  RunCommand
	}{
		{"reject stderr pattern", RunCommand{RejectStderrRe: []string{"fixture-e.r"}}},
		{"reject stderr contains", RunCommand{RejectStderrHas: []string{"fixture-err"}}},
		{"reject stdout pattern", RunCommand{RejectStdoutRe: []string{"fixture-o.t"}}},
		{"reject stdout contains", RunCommand{RejectStdout: []string{"fixture-out"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := daemonCoveringFixture(&tt.cmd)
			require.False(t, checkRun(rec), "the daemon's span covers the fixture's bytes")
			assert.Contains(t, rec.Error.Error(), "cmd seq=1")
			assert.Contains(t, rec.Error.Error(), "span of cmd seq=2 (le test fixture x)")
		})
	}
}

// TestRejectReadsBothStreamsOfItsSpan proves a reject other than
// reject=stderr:pattern= reads both streams of its span, as it did before
// expect= learned to select one: a stderr reject trips on a stdout line.
func TestRejectReadsBothStreamsOfItsSpan(t *testing.T) {
	for _, cmd := range []RunCommand{
		{RejectStderrHas: []string{"fixture-out"}},
		{RejectStdoutRe: []string{"fixture-e.r"}},
		{RejectStdout: []string{"fixture-err"}},
	} {
		rec := daemonCoveringFixture(&cmd)
		require.False(t, checkRun(rec), "%+v", cmd)
		assert.Contains(t, rec.Error.Error(), "found inside the output span of cmd seq=2 (le test fixture x)")
	}
	rec := daemonCoveringFixture(&RunCommand{RejectStderrRe: []string{"fixture-o.t"}})
	assert.True(t, checkRun(rec), "the run's stderr holds no stdout byte: %v", rec.Error)
}

// TestExpectUnderAFixtureIgnoresTheDaemon proves an expect= stays scoped to the
// command it sits under: text only the daemon printed does not satisfy it.
func TestExpectUnderAFixtureIgnoresTheDaemon(t *testing.T) {
	tests := []struct {
		name string
		cmd  RunCommand
	}{
		{"expect stderr pattern", RunCommand{ExpectStderrRe: []string{"daemon-e.r"}}},
		{"expect stderr contains", RunCommand{ExpectStderrHas: []string{"daemon-err"}}},
		{"expect stdout pattern", RunCommand{ExpectStdoutRe: []string{"daemon-o.t"}}},
		{"expect stdout contains", RunCommand{ExpectStdout: []string{"daemon-out"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := daemonAndFixture(&tt.cmd)
			require.False(t, checkOutputAssertions(rec), "only the daemon printed it")
			assert.Contains(t, rec.Error.Error(), "cmd seq=1 (ze start) wrote it")
		})
	}
}

// TestStderrPatternNeedsACommand proves a stderr pattern with no cmd= line
// above it is refused at parse time, as every scoped assertion is.
func TestStderrPatternNeedsACommand(t *testing.T) {
	for _, line := range []string{"expect=stderr:pattern=x", "reject=stderr:pattern=x"} {
		_, err := parseCILines(t, line, "cmd=foreground:seq=1:exec=ze version")
		require.Error(t, err, line)
		assert.Contains(t, err.Error(), "no cmd= line above it", line)
	}
}

// TestRejectNamesTheNarrowestWriter proves the writer a reject failure names is
// the latest command whose span lies inside the asserting one and holds the
// text. A daemon's span stays open to the end of the run and so covers a later
// command's bytes; naming the daemon itself there would send the author to the
// wrong process.
func TestRejectNamesTheNarrowestWriter(t *testing.T) {
	const stderr = "daemon-err\nlater-err\n"
	rec := &Record{RunCommands: []RunCommand{
		{Seq: 1, Exec: "ze start", RejectStderrHas: []string{"later-err"}},
		{Seq: 2, Exec: "le test fixture x"},
		{Seq: 3, Exec: "ze cli"},
	}}
	daemon, fix, later := &rec.RunCommands[0], &rec.RunCommands[1], &rec.RunCommands[2]
	daemon.stderrFrom, daemon.stderrTo = 0, len(stderr)
	fix.stderrFrom, fix.stderrTo = len("daemon-err\n"), len("daemon-err\n")
	later.stderrFrom, later.stderrTo = len("daemon-err\n"), len(stderr)
	rec.ClientStderr, rec.ClientOutput = stderr, stderr

	require.False(t, checkOutputAssertions(rec))
	assert.Contains(t, rec.Error.Error(), "found inside the output span of cmd seq=3 (ze cli)")
}

// TestRejectNeverBlamesAWiderSpan proves a reject under a command whose own
// span holds the text does not name the daemon, whose span is wider and
// overlaps it: the asserting command is the narrowest span that holds it.
func TestRejectNeverBlamesAWiderSpan(t *testing.T) {
	rec := daemonCoveringFixture(&RunCommand{})
	rec.RunCommands[1].RejectStderrHas = []string{"fixture-err"}

	require.False(t, checkOutputAssertions(rec))
	assert.Contains(t, rec.Error.Error(), "cmd seq=2 (le test fixture x)")
	assert.NotContains(t, rec.Error.Error(), "found inside")
}
