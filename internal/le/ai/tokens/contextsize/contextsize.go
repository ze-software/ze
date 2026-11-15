// Design: ai/rules/context-economy.md -- where a session's tokens go
// Related: ../tokeneconomy.go -- the whole-transcript scan that sums the same three fields

// Package contextsize holds the one definition of a model call's context size,
// and reads the most recent such figure from the tail of a transcript.
//
// Two readers need the figure. The token report (internal/le/ai/tokens) sums
// it over every call of every transcript, and the agent-spawn hook
// (internal/le/hookruntime) reads the last call of the main session's
// transcript to tell the main thread it is past its handoff threshold. The
// hook's output is under the hook source seal, so the code that decides it is
// too: this package is listed in hookSourcePackages
// (internal/le/ai/hooks/fixtures.go), and the token report calls Tokens rather
// than summing the fields itself.
package contextsize

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// TailBytes bounds how much of a transcript Last reads: the last 1 MiB. A main
// session's transcript grows to hundreds of megabytes, and a hook runs on every
// spawn under a 5 s timeout, so the whole file is never read. One assistant
// record is a few kilobytes; a record larger than the bound, or a tail made only
// of tool results, leaves the tail with no usage record, and Last answers
// ErrNoUsage rather than a figure it did not read.
const TailBytes = 1 << 20

// ErrNoUsage reports that the tail of the transcript holds no assistant record
// with a usable usage block. It is not a zero: the caller cannot tell how large
// the context is.
var ErrNoUsage = errors.New("the transcript tail holds no usage record")

// Tokens answers the tokens fed to the model on one call, which is what the
// call is charged for: the fresh input, the input written to the cache, and the
// input read from it. Output tokens are what the model wrote, not what it read,
// so they are not part of the context.
func Tokens(input, cacheWrite, cacheRead int64) int64 {
	return input + cacheWrite + cacheRead
}

// Last reads the last TailBytes of the transcript at path and answers the
// context size of its most recent model call, read from the newest assistant
// record that carries a usage block. A record of a subagent's side chain is
// skipped, because its usage is the agent's context and not the session's. A
// usage block that sums to zero is skipped too: the harness writes one for a
// message it made itself, and it measures nothing. The first line of the tail
// is skipped when the tail starts inside the file, because it is the end of a
// record whose start was not read. A transcript that cannot be opened or read
// answers its error; one whose tail holds no usable record answers ErrNoUsage.
// Last never answers zero with a nil error.
func Last(path string) (int64, error) {
	tail, err := readTail(path)
	if err != nil {
		return 0, err
	}
	for rest := tail; len(rest) > 0; {
		cut := bytes.LastIndexByte(rest, '\n')
		line := rest[cut+1:]
		rest = rest[:max(cut, 0)]
		if tokens, found := usageTokens(line); found {
			return tokens, nil
		}
	}
	return 0, ErrNoUsage
}

// readTail answers the last TailBytes of the file, with the partial first line
// removed when the read starts inside the file.
func readTail(path string) ([]byte, error) {
	file, err := os.Open(path) //nolint:gosec // the transcript path the harness put in the hook payload
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only file; a close error loses nothing

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := max(info.Size()-TailBytes, 0)
	tail := make([]byte, info.Size()-start)
	// ReadAt answers io.EOF with a full read when the tail ends at the end of
	// the file, and with a short one when the file shrank since Stat; the
	// bytes read are kept either way.
	read, err := file.ReadAt(tail, start)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	tail = tail[:read]
	if start == 0 {
		return tail, nil
	}
	cut := bytes.IndexByte(tail, '\n')
	if cut < 0 {
		return nil, ErrNoUsage
	}
	return tail[cut+1:], nil
}

// usageRecord is the part of one transcript line Last reads.
type usageRecord struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	Message     *struct {
		Usage *struct {
			Input      int64 `json:"input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// usageKey is checked before a line is decoded, so a tool result of several
// hundred kilobytes in the tail costs a byte search rather than a JSON parse.
var usageKey = []byte(`"usage"`)

// usageTokens answers the context size one transcript line records, and false
// when the line is not a main-session assistant record with a usage block that
// measures something. A line that does not decode, the partial record the
// harness is still writing included, answers false.
func usageTokens(line []byte) (int64, bool) {
	if !bytes.Contains(line, usageKey) {
		return 0, false
	}
	var record usageRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return 0, false
	}
	if record.Type != "assistant" {
		return 0, false
	}
	if record.IsSidechain {
		return 0, false
	}
	if record.Message == nil {
		return 0, false
	}
	usage := record.Message.Usage
	if usage == nil {
		return 0, false
	}
	tokens := Tokens(usage.Input, usage.CacheWrite, usage.CacheRead)
	if tokens <= 0 {
		return 0, false
	}
	return tokens, true
}
