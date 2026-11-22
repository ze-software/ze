// Design: docs/architecture/api/commands.md -- the ordered pipe list a structured route takes.
package command

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestDecodePipeListReadsStrictly asks whether the JSON pipe list is read as
// declared and refused, never trimmed, when an element carries a key the
// element does not define.
//
// Method: a list with a name and args, JSON null, no bytes, a misspelled args
// key, and a list followed by more JSON.
//
// MUTATION: drop DisallowUnknownFields; the misspelled key decodes to an
// element with no arguments and no error.
func TestDecodePipeListReadsStrictly(t *testing.T) {
	chain, err := DecodePipeList([]byte(`[{"name":"received"},{"name":"match","args":["a b","c|d"]}]`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []PipeListElement{{Name: "received"}, {Name: "match", Args: []string{"a b", "c|d"}}}
	if !reflect.DeepEqual(chain, want) {
		t.Errorf("decoded %v, want %v", chain, want)
	}

	for _, absent := range []string{"", "null", "  "} {
		chain, err := DecodePipeList([]byte(absent))
		if err != nil || chain != nil {
			t.Errorf("absent list %q decoded %v, %v; want nil, nil", absent, chain, err)
		}
	}

	for _, refused := range []string{`[{"name":"match","arguments":["a"]}]`, `[{"name":"count"}] []`, `{"name":"count"}`} {
		if _, err := DecodePipeList([]byte(refused)); err == nil {
			t.Errorf("list %s was accepted", refused)
		}
	}
}

// TestWritePipeListRefusesBeforeWriting asks whether a list holding one name
// the command does not take writes nothing at all, and names the element.
//
// MUTATION: write each stage as it is judged; the buffer holds the first
// stage when the second is refused.
func TestWritePipeListRefusesBeforeWriting(t *testing.T) {
	const path = "show fixture command pipe list"
	RegisterPipeFilters([]string{path}, PipeFilter{Name: "received", Description: "Only the received rows", Argument: ArgNone})
	var tb textbuf.Buffer
	err := WritePipeList(&tb, path, []PipeListElement{{Name: "received"}, {Name: "bogus"}})
	if err == nil {
		t.Fatal("bogus was accepted")
	}
	if tb.String() != "" {
		t.Errorf("a refused list wrote %q", tb.String())
	}
	if !strings.Contains(err.Error(), "element 2") || !strings.Contains(err.Error(), `"bogus"`) {
		t.Errorf("refusal does not name element 2 and its name: %v", err)
	}
}
