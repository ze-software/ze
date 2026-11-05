// Design: docs/contributing/rfc-conformance-gates.md -- committed quote sources.
package rfc

import (
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestRFC7084TextIsCommitted reads both the summary and its source from HEAD,
// then asks the real quote consumer about every row. Working-tree text cannot
// make an absent Git blob satisfy this committed-source contract.
func TestRFC7084TextIsCommitted(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	const summary = "rfc/short/rfc7084.md"
	const source = "rfc/full/rfc7084.txt"
	blobs, known := gitCatBlobs(root, headRevision, []string{summary, source})
	if !known {
		t.Fatal("cannot read RFC7084 committed source")
	}
	text, held := blobs[source]
	if !held {
		t.Fatal("RFC7084 source is not a committed Git blob")
	}
	if text == "" {
		t.Fatal("RFC7084 source is not a committed Git blob")
	}
	rows, err := ParseSummaryText(blobs[summary], "rfc7084", summary)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("committed RFC7084 summary has no quote population")
	}
	quotes := newQuoteSource(text)
	for i := range rows {
		if message, refused := rowQuoteRefusal(&rows[i], quotes); refused {
			t.Error(message)
		}
	}

	t.Run("local-only source is not committed evidence", func(t *testing.T) {
		fixture := t.TempDir()
		writeFixtureFiles(t, fixture, map[string]string{"README": "fixture\n"})
		gitFixture(t, fixture, []string{"init", "-q"})
		commitFixture(t, fixture, "fixture baseline")
		writeFixtureFiles(t, fixture, map[string]string{source: text})
		if _, found := SourceText(fixture, "rfc7084"); !found {
			t.Fatal("negative control did not supply a readable local-only source")
		}
		committed, known := gitCatBlobs(fixture, headRevision, []string{source})
		if !known {
			t.Fatal("negative fixture Git cannot answer")
		}
		if _, held := committed[source]; held {
			t.Fatal("local-only source was accepted as committed evidence")
		}
		if _, refused := rowQuoteRefusal(&rows[0], nil); !refused {
			t.Fatal("quote consumer accepted an absent committed source")
		}
	})
}
