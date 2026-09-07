package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAndAdd_EnvVarOutsidePeerBlockAccepted verifies that option=env
// placed above the stdin=peer block is parsed into Record.EnvVars.
//
// VALIDATES: AC-1 — option=env outside peer block populates rec.EnvVars.
// PREVENTS: Regression where the previously-valid placement stops working.
func TestParseAndAdd_EnvVarOutsidePeerBlockAccepted(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "outside.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
option=env:var=ze.log.bgp.server:value=debug
stdin=peer:terminator=EOF_PEER
option=timeout:value=5s
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err)

	rec := et.GetByNick("1")
	require.NotNil(t, rec)
	assert.Equal(t, []string{"ze.log.bgp.server=debug"}, rec.EnvVars)
}

// TestParseAndAdd_EnvVarInsidePeerBlockRejected verifies that option=env
// placed inside the stdin=peer block causes parseAndAdd to return a
// non-nil error whose message names the directive and says "outside".
//
// VALIDATES: AC-2 — parser rejects option=env inside peer block with
// an actionable error referencing the directive text.
// PREVENTS: the silent drop of option=env inside a stdin=peer block, which
// masked broken tests for months.
func TestParseAndAdd_EnvVarInsidePeerBlockRejected(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "inside.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
stdin=peer:terminator=EOF_PEER
option=timeout:value=5s
option=env:var=ze.log.bgp.server:value=debug
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err, "expected parse error for option=env inside peer block")

	msg := err.Error()
	// Error must quote the directive text so the author sees exactly what to move.
	assert.Contains(t, msg, "option=env:var=ze.log.bgp.server:value=debug",
		"error should name the directive")
	// Error must explain the remedy: move OUTSIDE the peer block.
	assert.Contains(t, msg, "outside",
		"error should tell the author to place the directive outside the peer block")
	// Error must identify the position inside the block so the author can jump to it.
	assert.Contains(t, msg, "stdin=peer block line",
		"error should name the stdin=peer block and line offset")
}

// TestParseAndAdd_OptionTimeoutInsidePeerBlockPasses verifies that
// option=timeout inside the peer block is still accepted. Two corrections to
// what this comment used to claim: ze-peer does NOT consume it (parseOptionConfig
// answers "handled by test runner"), and it does not "pass through unchanged"
// either -- the runner parses it to prove it is not a typo and then discards the
// value, because the timeout's scope is the whole test and not one peer
// (validateOnePeerBlock, peer_contract.go).
//
// The fixture's option=open and option=update values must be REAL ones. It
// carried option=update:value=inspect-update-message, a value no branch of
// ze-peer has ever had, and asserted that a peer block accepts it -- the very
// silent drop the peer-block guard exists to close, pinned by a test.
//
// VALIDATES: AC-5 — non-env option directives inside peer blocks
// are not rejected by the hardening check.
// PREVENTS: Over-broad rejection that would also kill valid peer
// block directives (timeout, open, update, tcp_connections).
func TestParseAndAdd_OptionTimeoutInsidePeerBlockPasses(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "timeout.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := `option=file:path=test.conf
stdin=peer:terminator=EOF_PEER
option=timeout:value=10s
option=open:value=inspect-open-message
option=update:value=send-default-route
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
EOF_PEER
`
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err, "non-env option directives inside peer block must be accepted")
}

// TestParseOptionUnknownStillErrors verifies that an unknown option type in a
// .ci file is rejected by parseOption's fail-closed default branch. The error
// must name the offending type so the author sees what to fix; a zero value or
// silent skip would be a false green (ai/rules/evidence.md).
//
// The fixture below is `option=netns`, which the orphaned test/pppoe/*.ci once
// carried and no parseOption case ever implemented. Those files were repaired
// and run again on 2026-08-07; they now declare `option=netns-link:...:peer=`,
// so `option=netns` belongs to nothing and stays a pure unknown-type fixture.
//
// VALIDATES: AC-4 (spec-fixit-pppoe-orphaned-tests) — parseOption's fail-closed
// default is not weakened; an unknown option type stays a hard error.
// PREVENTS: relaxing the default to a warning/skip so stale .ci "parse" quietly.
//
// Driven from the parse entry point (parseAndAdd on a real .ci line), not the
// parseOption helper alone, per ai/rules/evidence.md.
func TestParseOptionUnknownStillErrors(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	confFile := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	ciFile := filepath.Join(tmpDir, "unknown-option.ci")
	// option=netns matches no parseOption case and must hit the fail-closed
	// default. It is not `netns-link`, which is a real option with its own tests.
	ciContent := "option=file:path=test.conf\n" +
		"option=netns:veth=veth-bng,veth-sub\n" +
		"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n"
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err, "an unknown option type must be a hard parse error, not silently accepted")

	msg := err.Error()
	assert.Contains(t, msg, "unknown option type", "error must state the option type is unknown")
	assert.Contains(t, msg, "netns", "error must name the offending option type so the author can fix it")
}

// TestDiscoverSkipsUnparseableFile verifies that a single unparseable .ci file
// is recorded as a failure but does NOT abort discovery of the rest of the
// directory. A bad file used to error out of Discover and hide every other test
// in the suite (see plan/handover-review-followups.md §1).
//
// VALIDATES: Discover returns nil with a bad file present; the good file is
// discovered normally; the bad file is a permanent failure (ParseFailed +
// StateFail + Error) so it still fails the suite loudly.
// PREVENTS: one malformed .ci silently hiding an entire suite again.
func TestDiscoverSkipsUnparseableFile(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	confFile := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	// good.ci parses cleanly.
	goodCI := `option=file:path=test.conf
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "good.ci"), []byte(goodCI), 0o600))

	// bad.ci uses an unknown reject type, which parseReject rejects.
	badCI := "option=file:path=test.conf\nreject=bogus:pattern=x\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.ci"), []byte(badCI), 0o600))

	et := NewEncodingTests(tmpDir)
	require.NoError(t, et.Discover(tmpDir), "discovery must not abort on one unparseable file")

	require.Equal(t, 2, et.Count(), "both files must be present after discovery")

	var good, bad *Record
	for _, rec := range et.Registered() {
		switch rec.Name {
		case "good":
			good = rec
		case "bad":
			bad = rec
		}
	}

	require.NotNil(t, good, "good.ci must be discovered")
	assert.False(t, good.ParseFailed, "good.ci must not be marked as a parse failure")
	assert.Len(t, good.Messages, 1, "good.ci expectations must be parsed")

	require.NotNil(t, bad, "bad.ci must still be recorded so it fails the suite")
	assert.True(t, bad.ParseFailed, "bad.ci must be marked as a parse failure")
	assert.Equal(t, StateFail, bad.State, "bad.ci must be a hard failure, not a skip")
	require.Error(t, bad.Error, "bad.ci must carry the parse error")
	assert.Equal(t, failParseError, bad.FailureType)
}

// TestUnknownDirectiveNamesLineAndAccepted drives the refusal from a file on
// disk, which is the only way to judge the line number: the parser is handed a
// slice the blocks and comments were already removed from.
//
// VALIDATES: AC-6 of spec-fixit-ci-runner-cannot-test-stdin. A directive the
// runner cannot honor fails the file at parse time, and the message names the
// directive, the line number, and what the runner accepts.
// PREVENTS: the two halves this refusal was missing. It listed nothing, so an
// author had to read the parser to learn the vocabulary; and it counted lines
// in the compacted directive slice, so it named line 2 for a line that is
// number 12 in the file the author is looking at.
func TestUnknownDirectiveNamesLineAndAccepted(t *testing.T) {
	ResetNickCounter()

	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "typo.ci")
	// The blocks and comments are what makes the line number worth asserting:
	// the bad directive is the SECOND surviving directive and the TWELFTH line.
	ciContent := "# A comment.\n" +
		"\n" +
		"stdin=config:terminator=EOF_CONF\n" +
		"bgp {\n" +
		"}\n" +
		"EOF_CONF\n" +
		"\n" +
		"# Another comment.\n" +
		"option=timeout:value=20s\n" +
		"\n" +
		"# The typo below is the subject of this test.\n" +
		"exepct=stdout:contains=ze\n"
	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))

	et := NewEncodingTests(tmpDir)
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err, "a directive no arm reads must fail the file")
	assert.Contains(t, err.Error(), `unknown action "exepct"`, "the refusal names what the author wrote")
	assert.Contains(t, err.Error(), "(accepts action, await, cmd, command, expect, http, option, reject, stream)",
		"the refusal lists what the runner accepts")
	assert.Contains(t, err.Error(), "line 12:", "the refusal names the line of the FILE, not the index among directives")
}

// TestUnknownDirectiveTypeNamesAcceptedSet covers the five type switches under
// the action word, each of which used to answer with the type alone.
//
// VALIDATES: AC-6 for every directive parser, not only the top-level word.
func TestUnknownDirectiveTypeNamesAcceptedSet(t *testing.T) {
	et := NewEncodingTests(t.TempDir())
	r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}

	for _, row := range []struct {
		name     string
		parse    func() error
		wants    string
		accepted string
	}{
		{
			name:     "option",
			parse:    func() error { return et.parseOption(r, "x.ci", "flie", map[string]string{}) },
			wants:    `unknown option type "flie"`,
			accepted: "asn, bind, env, exclusive, file,",
		},
		{
			name:     "expect",
			parse:    func() error { return et.parseExpect(r, "stdoutt", map[string]string{}) },
			wants:    `unknown expect type "stdoutt"`,
			accepted: "bgp, command-error, event, exit, file, json, output, stderr, stdout, stream, syslog",
		},
		{
			name:     "reject",
			parse:    func() error { return et.parseReject(r, "stdoutt", map[string]string{}) },
			wants:    `unknown reject type "stdoutt"`,
			accepted: "(accepts bgp, stderr, stdout, syslog)",
		},
		{
			name:     "action",
			parse:    func() error { return et.parseAction(r, "sighub", map[string]string{}) },
			wants:    `unknown action type "sighub"`,
			accepted: "(accepts notification, rewrite, send, sighup, sigterm)",
		},
		{
			name:     "cmd",
			parse:    func() error { return et.parseCmd(r, "forground", map[string]string{}, "cmd=forground:seq=1:exec=x") },
			wants:    `unknown cmd type "forground"`,
			accepted: "(accepts api, background, foreground, stop)",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			err := row.parse()
			require.Error(t, err)
			assert.Contains(t, err.Error(), row.wants)
			assert.Contains(t, err.Error(), row.accepted)
		})
	}
}

// TestDirectiveVocabularyIsLive is the one-declaration check. Each list in
// record_parse_vocabulary.go gates its parser AND is printed by its refusal, so
// a word that is listed and has no arm behind it would be advertised to authors
// and then refused as unlisted. Nothing else would say so.
//
// VALIDATES: ai/rules/principles.md, one declaration. The reverse direction (an
// arm whose word is not listed) needs no test: the gate refuses that word
// before the switch, so every .ci writing it goes red.
func TestDirectiveVocabularyIsLive(t *testing.T) {
	et := NewEncodingTests(t.TempDir())

	assertReaches := func(t *testing.T, kind, name string, err error) {
		t.Helper()
		if err == nil {
			return // The arm accepted the minimal directive outright.
		}
		assert.NotContains(t, err.Error(), "unknown "+kind, "%s %q is listed but the gate refused it", kind, name)
		assert.NotContains(t, err.Error(), "is listed as accepted and no parser reads it",
			"%s %q is listed and no arm reads it", kind, name)
	}

	for _, name := range recordOptionTypes {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		assertReaches(t, "option type", name, et.parseOption(r, "x.ci", name, map[string]string{}))
	}
	for _, name := range recordExpectTypesParsed {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		assertReaches(t, "expect type", name, et.parseExpect(r, name, map[string]string{}))
	}
	for _, name := range recordRejectTypes {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		assertReaches(t, "reject type", name, et.parseReject(r, name, map[string]string{}))
	}
	for _, name := range recordActionTypes {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		assertReaches(t, "action type", name, et.parseAction(r, name, map[string]string{}))
	}
	for _, name := range recordCmdTypes {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		assertReaches(t, "cmd type", name, et.parseCmd(r, name, map[string]string{}, "cmd="+name+":seq=1:exec=x:name=n"))
	}
	// The action loop asks a narrower question: the word reached an arm. Its
	// TYPE is deliberately nonsense, so the arm's own refusal is the expected
	// answer and only the top-level gate's refusal is a finding.
	for _, name := range recordActions {
		r := &Record{Conf: map[string]any{}, Extra: map[string]string{}}
		err := et.parseLine(r, "x.ci", name+"=zzz-not-a-type:seq=1")
		if err == nil {
			continue
		}
		assert.NotContains(t, err.Error(), `unknown action "`+name+`"`,
			"action %q is listed but the gate refused it", name)
		assert.NotContains(t, err.Error(), "is listed as accepted and no parser reads it",
			"action %q is listed and no arm reads it", name)
	}
}
