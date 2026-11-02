package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/peer"
)

// TestOrchestratedPeerBlockNeedsConsumer prevents an unread assertion from
// making an orchestrated fixture appear to cover a peer exchange.
func TestOrchestratedPeerBlockNeedsConsumer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orphan.ci")
	body := "cmd=foreground:seq=1:exec=true\n" +
		"stdin=peer:terminator=END\n" +
		"expect=bgp:conn=1:seq=1:hex=ffffffffffffffffffffffffffffffff001304\nEND\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewEncodingTests(dir).parseAndAdd(path)
	if err == nil || !strings.Contains(err.Error(), "stdin=peer") {
		t.Fatalf("unconsumed peer block returned %v, want refusal naming stdin=peer", err)
	}
}

// TestLegacyPeerBlockRetainsExpectations protects the no-cmd execution path:
// its peer consumes the runner's collected expectations rather than the block.
func TestLegacyPeerBlockRetainsExpectations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.ci")
	body := "stdin=peer:terminator=END\n" +
		"expect=bgp:conn=1:seq=1:hex=ffffffffffffffffffffffffffffffff001304\nEND\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, err := NewEncodingTests(dir).parseAndAdd(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Expects) != 1 {
		t.Fatalf("legacy peer retained %d expectations, want 1", len(rec.Expects))
	}
}

// TestQuotedPeerConsumerParses keeps ownership and mode classification aligned
// with the tokenizer that will execute the command.
func TestQuotedPeerConsumerParses(t *testing.T) {
	for _, command := range []string{
		`"le" test peer --port $PORT`,
		`le "test" peer --port $PORT`,
		`le test 'peer' --port $PORT`,
		`"le" "test" "peer" --port $PORT`,
		`le test peer "--mode" "check" --port $PORT`,
		`le test peer --mode="check" --port $PORT`,
	} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "quoted.ci")
			body := "cmd=background:seq=1:exec=" + command + ":stdin=receiver\n" +
				"stdin=receiver:terminator=END\n" +
				"expect=bgp:conn=1:seq=1:hex=ffffffffffffffffffffffffffffffff001304\n" +
				"reject=bgp:conn=1:pattern=deadbeef\nEND\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			rec, err := NewEncodingTests(dir).parseAndAdd(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(rec.Expects) != 1 {
				t.Fatalf("quoted peer retained %d expectations, want 1", len(rec.Expects))
			}
		})
	}
}

func TestQuotedPeerHeadConsumesConventionalBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quoted-peer.ci")
	body := "cmd=background:seq=1:exec=\"le\" test peer --port $PORT:stdin=peer\n" +
		"stdin=peer:terminator=END\n" +
		"expect=bgp:conn=1:seq=1:hex=ffffffffffffffffffffffffffffffff001304\nEND\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEncodingTests(dir).parseAndAdd(path); err != nil {
		t.Fatal(err)
	}
}

// Quoting must not turn another command into a peer or give a non-check peer
// ownership of a wire rejection it cannot enforce.
func TestQuotedPeerConsumerRefusals(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    string
	}{
		{`helper "le" "test" "peer"`, "no cmd= line"},
		{`le test "peer-extra"`, "no cmd= line"},
		{`le test peer --mode="sink"`, "enforced only by a check-mode"},
		{`le test peer "--mode" 'echo'`, "enforced only by a check-mode"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "refused.ci")
			body := "cmd=background:seq=1:exec=" + tc.command + ":stdin=peer\n" +
				"stdin=peer:terminator=END\n" +
				"expect=bgp:conn=1:seq=1:hex=ffffffffffffffffffffffffffffffff001304\n" +
				"reject=bgp:conn=1:pattern=deadbeef\nEND\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := NewEncodingTests(dir).parseAndAdd(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("returned %v, want refusal containing %q", err, tc.want)
			}
		})
	}
}

// An unfinished quote is not an executable peer command. Mode extraction must
// not grant it a non-check mode while execution will reject its tokenization.
func TestQuotedPeerMalformedCommand(t *testing.T) {
	command := `le test peer --mode=sink "unfinished`
	if _, err := SplitCommand(command); err == nil {
		t.Fatal("unfinished quote unexpectedly tokenized")
	}
	if isPeerExec(command) {
		t.Fatal("malformed command claimed the peer role")
	}
	if mode := zePeerExecMode(command); mode != peer.ModeCheck {
		t.Fatalf("malformed command selected mode %v, want check fallback", mode)
	}
}
