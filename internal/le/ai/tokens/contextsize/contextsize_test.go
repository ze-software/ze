// Related: contextsize.go -- the context size of a call and the transcript tail reader
//
// VALIDATES: Last answers the newest main-session usage record of a
// transcript's tail, summing input, cache-write and cache-read tokens, and
// reads no further back than TailBytes.
// PREVENTS: a figure taken from a subagent's side chain, a harness record that
// measures nothing, the partial record at the cut, or a zero answered for a
// transcript whose context could not be read.
package contextsize

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func assistantLine(input, cacheWrite, cacheRead int64, extra string) string {
	return `{"type":"assistant"` + extra + `,"message":{"id":"m","usage":{"input_tokens":` + itoa(input) +
		`,"cache_creation_input_tokens":` + itoa(cacheWrite) + `,"cache_read_input_tokens":` + itoa(cacheRead) +
		`,"output_tokens":999999}}}` + "\n"
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func writeTranscript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTokensSumsTheThreeInputFields(t *testing.T) {
	if got := Tokens(3, 40, 500); got != 543 {
		t.Fatalf("Tokens(3, 40, 500) = %d, want 543", got)
	}
}

func TestLastAnswersTheNewestMainSessionCall(t *testing.T) {
	body := assistantLine(1, 2, 3, "") +
		`{"type":"user","message":{"role":"user","content":"x"}}` + "\n" +
		assistantLine(10, 200, 700000, "") +
		assistantLine(5, 5, 5, `,"isSidechain":true`) +
		assistantLine(0, 0, 0, "") +
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"usage"}]}}` + "\n" +
		`{"type":"assistant","message":{"usage":{"input_tok`
	got, err := Last(writeTranscript(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if got != 700210 {
		t.Fatalf("Last = %d, want 700210 (the newest main-session record, side chain, zero and partial records skipped)", got)
	}
}

func TestLastAnswersNoUsageRatherThanZero(t *testing.T) {
	body := `{"type":"user","message":{"content":"hello"}}` + "\n" + assistantLine(0, 0, 0, "")
	got, err := Last(writeTranscript(t, body))
	if !errors.Is(err, ErrNoUsage) {
		t.Fatalf("Last = %d, %v; want ErrNoUsage", got, err)
	}
	if _, err := Last(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil || errors.Is(err, ErrNoUsage) {
		t.Fatalf("Last of an absent file = %v, want the open error", err)
	}
}

// The head of the file holds a record past any threshold and the last
// TailBytes hold none, so an answer other than ErrNoUsage means Last read past
// its bound. The record placed just inside the bound proves the bound is not
// shorter than declared.
func TestLastReadsOnlyTheTail(t *testing.T) {
	head := assistantLine(900000, 0, 0, "")
	filler := `{"type":"user","message":{"content":"` + strings.Repeat("f", 4000) + `"}}` + "\n"
	var body strings.Builder
	body.WriteString(head)
	for body.Len() < 3*TailBytes {
		body.WriteString(filler)
	}
	if _, err := Last(writeTranscript(t, body.String())); !errors.Is(err, ErrNoUsage) {
		t.Fatalf("Last over a tail with no usage = %v, want ErrNoUsage: the head was read", err)
	}
	inside := assistantLine(650000, 0, 0, "")
	var near strings.Builder
	near.WriteString(head)
	for near.Len() < 3*TailBytes {
		near.WriteString(filler)
	}
	near.WriteString(inside)
	for written := 0; written+len(filler) < TailBytes-2*len(inside); written += len(filler) {
		near.WriteString(filler)
	}
	got, err := Last(writeTranscript(t, near.String()))
	if err != nil || got != 650000 {
		t.Fatalf("Last = %d, %v; want 650000 from the record inside the tail", got, err)
	}
}
