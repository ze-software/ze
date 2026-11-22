package bgp

import (
	"testing"
)

// zePipedRowsDocument writes the document Ze prints for a row command sent with
// one row-shaping pipe operation and `| json`. The rows come FIRST so a test
// that rewrites a row field with strings.Replace(..., 1) reaches the row and
// not the chain, whose operation can share a word with a row value
// (`received`, `sent`).
func zePipedRowsDocument(envelope, operation, rows string) string {
	return `{"` + envelope + `":` + rows + `,"` + zePipeChainKey + `":[{"op":"` + operation + `"}]}`
}

// TestDecodeZePipedRows proves the reader takes the one document a piped row
// query prints and refuses every neighboring shape: the argument form's bare
// array, the RPC envelope with no chain, a chain that lacks the operation the
// caller relies on, and rows that are absent or null. Each refusal is a case a
// checker would otherwise read as "no rows".
func TestDecodeZePipedRows(t *testing.T) {
	type row struct {
		Prefix string `json:"prefix"`
	}
	const rows = `[{"prefix":"10.0.0.0/24"},{"prefix":"10.0.1.0/24"}]`

	// The shape of the first live pipe-form answer, chain key first as
	// encoding/json orders it, with an argument on the step.
	live := `{"best-path":` + rows + `,"pipe":[{"arg":"10","op":"first"}]}`
	var got []row
	if err := decodeZePipedRows(live, zeBestPathRowsKey, zePipeFirst, &got); err != nil {
		t.Fatalf("live document refused: %v", err)
	}
	if len(got) != 2 || got[0].Prefix != "10.0.0.0/24" || got[1].Prefix != "10.0.1.0/24" {
		t.Fatalf("rows = %+v", got)
	}

	// A measured answer with no row is an empty slice, never nil and never an error.
	got = nil
	if err := decodeZePipedRows(zePipedRowsDocument(zeRIBRowsKey, zePipeReceived, `[]`), zeRIBRowsKey, zePipeReceived, &got); err != nil {
		t.Fatalf("empty rows refused: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty rows decoded to %#v, want an empty slice", got)
	}

	// The operation is found anywhere in the chain: `| received | community X`.
	chained := `{"routes":` + rows + `,"pipe":[{"op":"received"},{"arg":"65000:1","op":"community"}]}`
	if err := decodeZePipedRows(chained, zeRIBRowsKey, zePipeReceived, &got); err != nil {
		t.Fatalf("two-step chain refused: %v", err)
	}

	for name, output := range map[string]string{
		"empty output":         "",
		"null":                 "null",
		"argument-form array":  rows,
		"RPC envelope":         `{"routes":` + rows + `}`,
		"empty object":         `{}`,
		"chain without rows":   `{"pipe":[{"op":"received"}]}`,
		"null rows":            `{"routes":null,"pipe":[{"op":"received"}]}`,
		"rows are an object":   `{"routes":{},"pipe":[{"op":"received"}]}`,
		"another envelope":     zePipedRowsDocument(zeBestPathRowsKey, zePipeReceived, rows),
		"another operation":    zePipedRowsDocument(zeRIBRowsKey, zePipeSent, rows),
		"empty chain":          `{"routes":` + rows + `,"pipe":[]}`,
		"chain is not a list":  `{"routes":` + rows + `,"pipe":"received"}`,
		"malformed":            `{"routes":` + rows,
		"wrong row type":       zePipedRowsDocument(zeRIBRowsKey, zePipeReceived, `[42]`),
		"operation as a value": `{"routes":` + rows + `,"pipe":[{"op":"match","arg":"received"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var refused []row
			if err := decodeZePipedRows(output, zeRIBRowsKey, zePipeReceived, &refused); err == nil {
				t.Fatalf("accepted %s as the received rows document: %+v", output, refused)
			}
		})
	}
}
