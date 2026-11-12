// Design: docs/contributing/spec-workflow.md -- immutable blob framing refuses partial reads.
package objects

import (
	"bufio"
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

// TestStreamFramingRejectsEveryIncompleteOutcome drives the streaming decoder
// with the malformed batches takeBlob refuses, plus output after the last
// frame: each refuses, and exact frames hand their bytes to visit unchanged.
func TestStreamFramingRejectsEveryIncompleteOutcome(t *testing.T) {
	id := strings.Repeat("a", 40)
	accept := func(string, []byte) error { return nil }
	for name, raw := range map[string]string{
		"header-missing":     "",
		"header-unended":     id + " blob 0",
		"object-missing":     id + " missing\n",
		"wrong-object":       strings.Repeat("b", 40) + " blob 0\n\n",
		"wrong-type":         id + " tree 0\n\n",
		"invalid-size":       id + " blob huge\n\n",
		"negative-size":      id + " blob -1\n\n",
		"truncated-payload":  id + " blob 3\nab",
		"missing-terminator": id + " blob 3\nabc",
		"invalid-terminator": id + " blob 3\nabcX",
		"trailing-output":    id + " blob 3\nabc\nmore",
	} {
		t.Run(name, func(t *testing.T) {
			if err := streamBlobs(bufio.NewReader(strings.NewReader(raw)), []string{id}, accept); err == nil {
				t.Fatalf("malformed stream succeeded: %q", raw)
			}
		})
	}
	payloads := []string{"", "binary\x00\nbytes"}
	var raw strings.Builder
	for _, payload := range payloads {
		raw.WriteString(id + " blob " + strconv.Itoa(len(payload)) + "\n" + payload + "\n")
	}
	var visited []string
	err := streamBlobs(bufio.NewReader(strings.NewReader(raw.String())), []string{id, id}, func(_ string, blob []byte) error {
		visited = append(visited, string(blob))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(visited) != len(payloads) || visited[0] != payloads[0] || visited[1] != payloads[1] {
		t.Fatalf("framing changed bytes: visited %q, want %q", visited, payloads)
	}
}

// TestListingRejectsMalformedRecords drives the tree listing parser with
// records git never writes: each refuses the whole listing, and two exact
// records parse to their fields.
func TestListingRejectsMalformedRecords(t *testing.T) {
	id := strings.Repeat("c", 40)
	for name, raw := range map[string]string{
		"unterminated":     "100644 blob " + id + "\ta.go",
		"empty-record":     "\x00",
		"no-path":          "100644 blob " + id + " a.go\x00",
		"empty-path":       "100644 blob " + id + "\t\x00",
		"no-kind":          "100644 " + id + "\ta.go\x00",
		"short-mode":       "10064 blob " + id + "\ta.go\x00",
		"non-octal-mode":   "10064x blob " + id + "\ta.go\x00",
		"tree-kind":        "040000 tree " + id + "\tdir\x00",
		"unknown-kind":     "100644 tag " + id + "\ta.go\x00",
		"abbreviated-id":   "100644 blob " + id[:12] + "\ta.go\x00",
		"commit-not-link":  "100644 commit " + id + "\ta.go\x00",
		"link-not-commit":  "160000 blob " + id + "\tsub\x00",
		"extra-id-field":   "100644 blob " + id + " " + id + "\ta.go\x00",
		"second-malformed": "100644 blob " + id + "\ta.go\x00broken\x00",
	} {
		t.Run(name, func(t *testing.T) {
			if entries, err := parseListing([]byte(raw), id); err == nil {
				t.Fatalf("malformed listing %q answered %+v", raw, entries)
			}
		})
	}
	entries, err := parseListing([]byte("100755 blob "+id+"\tbin/a b\x00160000 commit "+id+"\tsub\x00"), id)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{Mode: "100755", Kind: "blob", ID: id, Path: "bin/a b"}, {Mode: "160000", Kind: "commit", ID: id, Path: "sub"}}
	if len(entries) != len(want) || entries[0] != want[0] || entries[1] != want[1] {
		t.Fatalf("listing parsed to %+v, want %+v", entries, want)
	}
}

// TestFirstParentsRejectsMalformedLines drives the history parser with lines
// git log never writes for the fixed format, and with a history that does not
// end at the requested commit: each refuses, and exact lines parse.
func TestFirstParentsRejectsMalformedLines(t *testing.T) {
	commit := strings.Repeat("d", 40)
	tree := strings.Repeat("e", 40)
	for name, raw := range map[string]string{
		"empty":            "",
		"unterminated":     commit + " " + tree + " 1 2",
		"three-fields":     commit + " " + tree + " 1\n",
		"five-fields":      commit + " " + tree + " 1 2 3\n",
		"double-space":     commit + "  " + tree + " 1 2\n",
		"abbreviated-id":   commit[:12] + " " + tree + " 1 2\n",
		"abbreviated-tree": commit + " " + tree[:12] + " 1 2\n",
		"author-time":      commit + " " + tree + " soon 2\n",
		"committer-time":   commit + " " + tree + " 1 later\n",
		"ends-elsewhere":   tree + " " + tree + " 1 2\n",
	} {
		t.Run(name, func(t *testing.T) {
			if commits, err := parseFirstParents([]byte(raw), commit); err == nil {
				t.Fatalf("malformed history %q answered %+v", raw, commits)
			}
		})
	}
	commits, err := parseFirstParents([]byte(tree+" "+tree+" 10 20\n"+commit+" "+tree+" 30 40\n"), commit)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || commits[0].ID != tree || commits[1].ID != commit {
		t.Fatalf("history parsed to %+v", commits)
	}
	if commits[1].AuthorTime.Unix() != 30 || commits[1].CommitTime.Unix() != 40 {
		t.Fatalf("history times parsed to %+v", commits[1])
	}
}
