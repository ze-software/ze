package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// peerBlockCI writes a .ci file whose ze-peer stdin block carries the given
// lines, and parses it. The surrounding shape is the smallest one that reaches
// the peer-block guard: a check-mode peer with one expectation, and a daemon.
func peerBlockCI(t *testing.T, blockLines ...string) (*Record, error) {
	t.Helper()
	body := strings.Join(blockLines, "\n")
	content := "stdin=peer:terminator=EOF_PEER\n" +
		"option=tcp_connections:value=1\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		body + "\n" +
		"EOF_PEER\n" +
		"stdin=ze-bgp:terminator=EOF_CONF\n" +
		"bgp {\n}\n" +
		"EOF_CONF\n" +
		"cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer\n" +
		"cmd=foreground:seq=2:exec=ze -:stdin=ze-bgp:timeout=10s\n"
	path := filepath.Join(t.TempDir(), "guard.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	et := NewEncodingTests(t.TempDir())
	return et.parseAndAdd(path)
}

// TestPeerBlockRefusesUnclaimedDirective is the guard: a directive no consumer
// acts on fails the file at parse time instead of being dropped.
//
// VALIDATES: AC-1 — the runner refuses the file with an error naming the
// directive and the line.
func TestPeerBlockRefusesUnclaimedDirective(t *testing.T) {
	for name, line := range map[string]string{
		"unknown option":  "option=mode:value=sink",
		"unknown reject":  "reject=widget:pattern=x",
		"unknown expect":  "expect=widget:value=1",
		"unknown action":  "wibble=1:value=2",
		"malformed known": "option=timeout",
		// A value the option does not have is the same silent drop one level
		// down. inspect-update-message was pinned as ACCEPTED by a runner test
		// until 2026-08-15; ze-peer has never had a branch for it.
		"unknown update value": "option=update:value=inspect-update-message",
		"unknown open value":   "option=open:value=send-unknown-capabilty",
		"asn not a number":     "option=asn:value=abc",
		"conn count not a num": "option=tcp_connections:value=many",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := peerBlockCI(t, line)
			require.Error(t, err, "a directive nothing consumes must fail the file")
			assert.Contains(t, err.Error(), line, "the error names the offending line")
			assert.Contains(t, err.Error(), "stdin=peer block line", "the error names the block and the line number")
		})
	}
}

// TestRejectBGPOutsidePeerBlockNamesTheRemedy proves the other half of the
// original defect is closed too: reject=bgp has no runner-side meaning, and
// saying so is what stops the next author writing it where nothing reads it.
//
// VALIDATES: AC-4 — reject=bgp exists, and its one legal position is named.
func TestRejectBGPOutsidePeerBlockNamesTheRemedy(t *testing.T) {
	content := "stdin=peer:terminator=EOF_PEER\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"EOF_PEER\n" +
		"reject=bgp:conn=1:pattern=180A0100\n" +
		"cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer\n"
	path := filepath.Join(t.TempDir(), "outside.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	et := NewEncodingTests(t.TempDir())
	_, err := et.parseAndAdd(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stdin block", "the error names where the directive belongs")
	assert.NotContains(t, err.Error(), "unknown reject type",
		"a real directive in the wrong place must not read as a typo")
}

// TestPeerBlockAcceptsRunnerDirective proves a directive ze-peer hands to the
// runner is now parsed where it stands rather than dropped.
//
// VALIDATES: AC-3 — no line of a peer block is silently discarded.
func TestPeerBlockAcceptsRunnerDirective(t *testing.T) {
	rec, err := peerBlockCI(t,
		"option=timeout:value=42s",
		"reject=stderr:pattern=level=DEBUG",
		"expect=json:conn=1:seq=1:json={\"a\":1}",
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"level=DEBUG"}, rec.RejectStderr, "reject=stderr reaches the record")
	assert.Equal(t, "42s", rec.Extra["timeout"],
		"this fixture declares no file-level timeout, so the block's is adopted")
}

// TestPeerBlockTimeoutDoesNotOverrideTheFileLevelOne pins the one runner
// directive a peer block may not apply.
//
// VALIDATES: AC-3 — the guard proves a line is readable without giving a block
// authority over the record.
func TestPeerBlockTimeoutDoesNotOverrideTheFileLevelOne(t *testing.T) {
	content := "option=timeout:value=90s\n" +
		"stdin=peer:terminator=EOF_PEER\n" +
		"option=tcp_connections:value=1\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"option=timeout:value=5s\n" +
		"EOF_PEER\n" +
		"cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer\n"
	path := filepath.Join(t.TempDir(), "timeout.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	et := NewEncodingTests(t.TempDir())
	rec, err := et.parseAndAdd(path)
	require.NoError(t, err)
	assert.Equal(t, "90s", rec.Extra["timeout"], "the file-level timeout wins over a peer block's")

	// The other half of the rule: with no file-level value the block's is
	// adopted, which is the state of most of the 450 tracked peer-block timeouts.
	noFileLevel := "stdin=peer:terminator=EOF_PEER\n" +
		"option=tcp_connections:value=1\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"option=timeout:value=5s\n" +
		"EOF_PEER\n" +
		"cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer\n"
	path2 := filepath.Join(t.TempDir(), "adopt.ci")
	require.NoError(t, os.WriteFile(path2, []byte(noFileLevel), 0o600))
	rec2, err := NewEncodingTests(t.TempDir()).parseAndAdd(path2)
	require.NoError(t, err)
	assert.Equal(t, "5s", rec2.Extra["timeout"], "with no file-level timeout the block's is adopted")
}

// TestPeerBlockRefusesMalformedReject proves a wire rejection that can never
// match fails the FILE, not the peer.
//
// VALIDATES: AC-1, AC-4 — the guard fires at parse time, before any process
// starts, and reads the line with ze-peer's own parser rather than a second one.
func TestPeerBlockRefusesMalformedReject(t *testing.T) {
	for name, line := range map[string]string{
		"odd hex":      "reject=bgp:conn=1:pattern=180A010",
		"not hex":      "reject=bgp:conn=1:pattern=zzzz",
		"no conn":      "reject=bgp:pattern=180A0100",
		"conn zero":    "reject=bgp:conn=0:pattern=180A0100",
		"no pattern":   "reject=bgp:conn=1",
		"key disorder": "reject=bgp:pattern=180A0100:conn=1",
		// No key=value tail at all. consumes() answers true for it, so without
		// its own branch it would be forwarded to ze-peer and fail in
		// parseExpectRule -- a bind timeout by a longer route.
		"bare": "reject=bgp",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := peerBlockCI(t, line)
			require.Error(t, err, "a rejection that can never match must fail the file")
			assert.NotContains(t, err.Error(), "did not start listening",
				"the diagnosis must be the typo, never a bind timeout")
		})
	}
}

// TestPeerBlockRefusesRejectOnANonCheckPeer proves the directive is refused
// where it could not be attributed to a connection.
//
// VALIDATES: AC-4 — sink and echo peers read every connection concurrently
// against one checker, so conn= means nothing there.
func TestPeerBlockRefusesRejectOnANonCheckPeer(t *testing.T) {
	content := "stdin=peer:terminator=EOF_PEER\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"reject=bgp:conn=1:pattern=180A0100\n" +
		"EOF_PEER\n" +
		"cmd=background:seq=1:exec=ze-peer --mode sink --port $PORT:stdin=peer\n"
	path := filepath.Join(t.TempDir(), "sink.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	et := NewEncodingTests(t.TempDir())
	_, err := et.parseAndAdd(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check-mode")
}

// TestPeerBlockNamesAreSorted pins the order the guard walks its blocks in.
//
// VALIDATES: AC-1 — a file with two bad blocks names the same one on every run.
// The set is built in a map, so without the sort the error text is random.
func TestPeerBlockNamesAreSorted(t *testing.T) {
	r := &Record{RunCommands: []RunCommand{
		{Exec: "ze-peer --port $PORT", Stdin: "zulu"},
		{Exec: "ze-peer --port $PORT", Stdin: "alpha"},
		{Exec: "helper --stdin ze-peer", Stdin: "ignored"},
	}}
	assert.Equal(t, []string{"alpha", "peer", "zulu"}, peerBlockNames(r))
}

// TestPeerBlockAcceptsPeerDirective proves the directives ze-peer owns are left
// to ze-peer, including the new wire rejection.
//
// VALIDATES: AC-4 — reject=bgp is a real directive, accepted where ze-peer reads it.
func TestPeerBlockAcceptsPeerDirective(t *testing.T) {
	_, err := peerBlockCI(t,
		"option=linger:value=true",
		"action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304",
		"reject=bgp:conn=1:pattern=180A0100",
		"# a comment is not a directive",
		"cmd=api:conn=1:seq=1:text=peer * update text nlri ipv4/unicast eor",
	)
	require.NoError(t, err)
}

// TestPeerBlockRejectNeedsDelivery refuses a wire rejection that no delivery on
// the same connection can make fire.
//
// VALIDATES: AC-4 — a rejection that cannot discriminate is refused, rather
// than passing because the peer closed before the forbidden bytes arrived.
func TestPeerBlockRejectNeedsDelivery(t *testing.T) {
	_, err := peerBlockCI(t, "reject=bgp:conn=2:pattern=180A0100")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conn=2", "the error names the connection with no delivery")
	assert.Contains(t, err.Error(), "expect=bgp:conn=2", "the error names the remedy")
}

// TestCIPeerBlockCorpusParses parses every committed .ci file that drives a
// ze-peer.
//
// Discover already records an unparseable file as a suite failure, but only for
// the suites a given target runs. This reaches the whole corpus at unit-test
// speed, so a directive the peer-block guard refuses is named with its file here
// rather than found by whoever next runs that suite.
//
// The walk is limited to files carrying a peer block because EncodingTests is
// not the parser for every suite: test/decode and test/exabgp-compat have their
// own, and their files do not parse under this one.
//
// It is a corpus NET rather than a discriminator: delete the guard and this test
// stays green, because every file would then parse trivially. What it catches is
// the opposite direction -- a committed file the guard refuses. Its AC-3
// discriminator is TestPeerBlockGuardCoversEveryPeerBlock, which goes red the
// moment peerBlockNames narrows back to the block literally named "peer".
//
// VALIDATES: AC-3 — no committed .ci is refused by the guard.
func TestCIPeerBlockCorpusParses(t *testing.T) {
	root := filepath.Join("..", "..", "..", "test")
	if _, err := os.Stat(root); err != nil {
		// Deliberately fatal, not a skip: a gate that vanishes when its input
		// moves reads green forever (ai/rules/evidence.md).
		t.Fatalf("test fixture tree not reachable from %s: %v", root, err)
	}
	var failures []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ is invisible to every gate, this one included
		// (test/draft/README.md).
		if d.IsDir() && isDraftPath(root, path) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".ci") {
			return nil
		}
		content, readErr := os.ReadFile(path) //nolint:gosec // fixture path from the repo's own tree
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(content), "stdin=peer:") && !strings.Contains(string(content), "exec=ze-peer") {
			return nil
		}
		et := NewEncodingTests(filepath.Join("..", "..", ".."))
		if _, perr := et.parseAndAdd(path); perr != nil {
			failures = append(failures, path+": "+perr.Error())
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, failures, "every committed .ci must parse:\n%s", strings.Join(failures, "\n"))
}

// TestPeerBlockGuardCoversEveryPeerBlock proves the guard is scoped by which
// blocks reach ze-peer, not by the block being named "peer".
//
// VALIDATES: AC-3 — a peer block under any name is covered.
func TestPeerBlockGuardCoversEveryPeerBlock(t *testing.T) {
	content := "stdin=upstream:terminator=EOF_UP\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"option=mode:value=sink\n" +
		"EOF_UP\n" +
		"cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=upstream\n"
	path := filepath.Join(t.TempDir(), "named.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	et := NewEncodingTests(t.TempDir())
	_, err := et.parseAndAdd(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stdin=upstream block line")
}

// TestPeerBlockRefusesRejectNoPeerReads proves the block-level twin of the
// dropped-directive defect is closed for this spec's own directive.
//
// A .ci with cmd= lines and a peer block no ze-peer command names starts no
// peer, so nothing in that block is read. Seven committed files are in that
// state and none carries a reject=; this refuses the case where one would.
//
// VALIDATES: AC-3 — a rejection nothing can read fails the file.
func TestPeerBlockRefusesRejectNoPeerReads(t *testing.T) {
	content := "stdin=peer:terminator=EOF_PEER\n" +
		"expect=bgp:conn=1:seq=1:contains=18C00002\n" +
		"reject=bgp:conn=1:pattern=180A0100\n" +
		"EOF_PEER\n" +
		"stdin=ze-bgp:terminator=EOF_CONF\n" +
		"bgp {\n}\n" +
		"EOF_CONF\n" +
		"cmd=foreground:seq=2:exec=ze -:stdin=ze-bgp:timeout=10s\n"
	path := filepath.Join(t.TempDir(), "orphan.ci")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	_, err := NewEncodingTests(t.TempDir()).parseAndAdd(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no cmd= line in this file launches a ze-peer")

	// The same block WITH a peer reading it is accepted, so the refusal is
	// keyed on the missing peer and not on the shape of the block.
	withPeer := content + "cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer\n"
	path2 := filepath.Join(t.TempDir(), "withpeer.ci")
	require.NoError(t, os.WriteFile(path2, []byte(withPeer), 0o600))
	_, err = NewEncodingTests(t.TempDir()).parseAndAdd(path2)
	require.NoError(t, err)
}
