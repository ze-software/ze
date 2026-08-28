// Design: docs/architecture/testing/test-health.md -- heredoc-aware carrier coverage
//
// VALIDATES: a .ci/.et carrier's coverage count is judged on its own
// expect=/reject=/cmd= directives, never on the program a heredoc body
// happens to carry (a Python fixture, a pushed config file, ...). A directive
// genuinely lost from the carrier is still reported, whether or not an
// unrelated heredoc sits beside it, terminated or not.
// PREVENTS: the migration from embedded Python heredocs to compiled Go
// fixtures (internal/test/fixture) reporting a false "removing expectations"
// finding for every carrier whose heredoc body used to call runtime_fail(),
// fail(), assert, wait_until(), or dispatch_until() -- measured at 493 of 500
// flagged .ci files losing no real directive at all.
package testweakened

import "testing"

func TestDetectIgnoresEmbeddedHeredocBodyReplacedByFixtureRun(t *testing.T) {
	t.Parallel()

	oldText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_FIX\n" +
		"#!/usr/bin/env python3\n" +
		"def test():\n" +
		"    assert value\n" +
		"    runtime_fail(\"boom\")\n" +
		"    wait_until(lambda: True)\n" +
		"EOF_FIX\n"
	newText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"run \"ze-test fixture plugin/foo\"\n"

	verdict := detect(oldText, newText, "test/plugin/foo.ci")
	if len(verdict.blocking) != 0 || len(verdict.advisory) != 0 {
		t.Fatalf("detect() = blocking %q, advisory %q; want no finding once the removed "+
			"body is a heredoc, not a lost directive", verdict.blocking, verdict.advisory)
	}
}

func TestDetectStillCatchesARealExpectLoss(t *testing.T) {
	t.Parallel()

	oldText := "tmpfs=fixture.run:mode=755:terminator=EOF_FIX\n" +
		"assert value\n" +
		"EOF_FIX\n" +
		"cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"expect=stderr:contains=ready\n"
	newText := "tmpfs=fixture.run:mode=755:terminator=EOF_FIX\n" +
		"assert value\n" +
		"EOF_FIX\n" +
		"cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n"

	verdict := detect(oldText, newText, "test/plugin/foo.ci")
	if !containsDetail(verdict.advisory, "removing expectations (3 -> 2 ") {
		t.Fatalf("detect().advisory = %q, want a real expect= loss still reported", verdict.advisory)
	}
}

func TestDetectStillCatchesARealRejectLoss(t *testing.T) {
	t.Parallel()

	oldText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"reject=stderr:pattern=panic\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_FIX\n" +
		"assert value\n" +
		"EOF_FIX\n"
	newText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_FIX\n" +
		"assert value\n" +
		"EOF_FIX\n"

	verdict := detect(oldText, newText, "test/plugin/foo.ci")
	if !containsDetail(verdict.advisory, "removing negative expectations (1 -> 0 reject=") {
		t.Fatalf("detect().advisory = %q, want the reject= loss reported", verdict.advisory)
	}
}

func TestDetectUnterminatedHeredocDoesNotCrashOrSwallowPrecedingDirectives(t *testing.T) {
	t.Parallel()

	oldText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"expect=exit:code=0\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_NEVER\n" +
		"assert value\n" +
		"runtime_fail(\"boom\")\n"
	newText := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_NEVER\n" +
		"assert value\n" +
		"runtime_fail(\"boom\")\n"

	verdict := detect(oldText, newText, "test/plugin/foo.ci")
	if !containsDetail(verdict.advisory, "removing expectations (2 -> 1 ") {
		t.Fatalf("detect().advisory = %q, want the preceding expect= loss still reported "+
			"despite the unterminated heredoc that follows it", verdict.advisory)
	}
}

// TestStripHeredocBodiesDropsAFixtureAndKeepsAPeerBlock states the distinction
// the strip turns on. A `tmpfs=` block is a file written to disk, so its bytes
// are a fixture. A `stdin=peer` block is carrier input and holds the
// `expect=bgp:...:hex=...` wire expectations themselves, which are the
// strongest assertions this repository has: 548 of 1955 carriers keep an
// expect= or reject= inside a block, and dropping one must stay visible.
func TestStripHeredocBodiesDropsAFixtureAndKeepsAPeerBlock(t *testing.T) {
	t.Parallel()

	peer := "stdin=peer:terminator=EOF_PEER\n" +
		"option=tcp_connections:value=1\n" +
		"expect=bgp:conn=1:seq=1:hex=FF\n" +
		"EOF_PEER\n" +
		"cmd=foreground:seq=1:exec=ze -:stdin=peer\n"
	if got := stripHeredocBodies(peer); got != peer {
		t.Fatalf("a peer block was stripped: %q", got)
	}

	fixture := "tmpfs=probe.run:mode=755:terminator=EOF_RUN\n" +
		"#!/usr/bin/env python3\n" +
		"runtime_fail(\"boom\")\n" +
		"EOF_RUN\n" +
		"expect=bgp:conn=1:seq=1:hex=FF\n"
	want := "tmpfs=probe.run:mode=755:terminator=EOF_RUN\n" +
		"EOF_RUN\n" +
		"expect=bgp:conn=1:seq=1:hex=FF\n"
	if got := stripHeredocBodies(fixture); got != want {
		t.Fatalf("stripHeredocBodies() = %q, want %q", got, want)
	}
}

func TestStripHeredocBodiesUnterminatedConsumesRestOfFileWithoutPanicking(t *testing.T) {
	t.Parallel()

	text := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_NEVER\n" +
		"assert value\n" +
		"runtime_fail(\"boom\")\n"
	want := "cmd=foreground:seq=1:exec=ze -:stdin=cfg\n" +
		"tmpfs=fixture.run:mode=755:terminator=EOF_NEVER"
	if got := stripHeredocBodies(text); got != want {
		t.Fatalf("stripHeredocBodies() = %q, want %q", got, want)
	}
}
