// Design: docs/contributing/spec-workflow.md -- immutable blob framing refuses partial reads.
package objects

import (
	"strconv"
	"strings"
	"testing"
)

// TestBlobFramingRejectsEveryIncompleteOutcome drives the batch decoder with
// malformed headers and bodies; only exact length-framed payloads succeed.
func TestBlobFramingRejectsEveryIncompleteOutcome(t *testing.T) {
	id := strings.Repeat("a", 40)
	for name, raw := range map[string]string{
		"header-missing":     "",
		"object-missing":     id + " missing\n",
		"wrong-object":       strings.Repeat("b", 40) + " blob 0\n\n",
		"wrong-type":         id + " tree 0\n\n",
		"invalid-size":       id + " blob huge\n\n",
		"negative-size":      id + " blob -1\n\n",
		"truncated-payload":  id + " blob 3\nab",
		"missing-terminator": id + " blob 3\nabc",
		"invalid-terminator": id + " blob 3\nabcX",
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := takeBlob([]byte(raw), id); err == nil {
				t.Fatalf("malformed batch succeeded: %q", raw)
			}
		})
	}
	for _, payload := range []string{"", "binary\x00\nbytes"} {
		raw := id + " blob " + strconv.Itoa(len(payload)) + "\n" + payload + "\nnext"
		blob, rest, err := takeBlob([]byte(raw), id)
		if err != nil {
			t.Fatal(err)
		}
		if string(blob) != payload || string(rest) != "next" {
			t.Fatalf("framing changed bytes: blob %q rest %q", blob, rest)
		}
	}
}
