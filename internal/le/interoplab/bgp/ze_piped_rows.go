// Design: docs/architecture/testing/interop.md -- the document a piped Ze row query prints.
// Related: check_extras.go -- zeCommand, whose --format json lowers to | json.
// Related: check_llgr.go -- llgrReceivedRoutesPresent reads received rows through it.
package bgp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// The envelope keys of the two RIB row commands. bgp-rib names them
// (showRowsEnvelopeKey and bestPathEnvelopeKey, internal/component/bgp/plugins/rib);
// a checker holds its own spelling because it reads the daemon as an operator's
// script does, from the printed document alone.
const (
	zeRIBRowsKey      = "routes"
	zeBestPathRowsKey = "best-path"
	// zePipeChainKey is where `| json` records the operations that shaped the
	// rows, in chain order (docs/architecture/api/json-format.md).
	zePipeChainKey = "pipe"
)

// The pipe operations the checkers type and then require in the recorded
// chain: the two RIB scopes bgp-rib declares, and the row operations of
// `show bgp rib best`.
const (
	zePipeReceived = "received"
	zePipeSent     = "sent"
	zePipeFirst    = "first"
	zePipePrefix   = "prefix"
)

// zePipeStep is one recorded operation of the chain. Its argument is not read:
// each caller sends one line and knows the argument it typed.
type zePipeStep struct {
	Op string `json:"op"`
}

// decodeZePipedRows reads the rows of the document Ze prints for a row command
// sent with a row-shaping pipe operation and `| json`:
//
//	{"pipe":[{"op":"received"}],"routes":[ ... ]}
//
// The rows sit under the command's envelope key beside the chain that shaped
// them. A line with no such operation prints a bare array instead, because
// `| json` unwraps a document of one key; that array is NOT this document, and
// it is refused here: it was the answer of the argument form (`show bgp rib
// received`), which carried no chain, and a checker that accepts it would pass
// against a line whose scope operation never reached the daemon.
//
// operation is the pipe operation the caller typed and relies on, such as
// `received`. Its absence from the recorded chain is an error rather than an
// empty row set: rows that no scope narrowed are another question's answer.
// rows MUST be a pointer to a slice. A null or absent envelope is an error,
// so the caller's empty slice always means a measured answer with no row.
func decodeZePipedRows(output, envelope, operation string, rows any) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return fmt.Errorf("ze piped row answer is not the %q document: %w", envelope, err)
	}
	if document == nil {
		return errors.New("ze piped row answer is null")
	}
	rawChain, chained := document[zePipeChainKey]
	if !chained {
		return fmt.Errorf("ze answer carries no %q chain, so no pipe operation shaped it", zePipeChainKey)
	}
	var chain []zePipeStep
	if err := json.Unmarshal(rawChain, &chain); err != nil {
		return fmt.Errorf("ze answer %q chain: %w", zePipeChainKey, err)
	}
	if !slices.ContainsFunc(chain, func(step zePipeStep) bool { return step.Op == operation }) {
		return fmt.Errorf("ze answer was not shaped by | %s: chain %s", operation, rawChain)
	}
	rawRows, held := document[envelope]
	if !held {
		return fmt.Errorf("ze answer carries no %q rows", envelope)
	}
	if bytes.Equal(bytes.TrimSpace(rawRows), []byte("null")) {
		return fmt.Errorf("ze answer carries null %q rows", envelope)
	}
	if err := json.Unmarshal(rawRows, rows); err != nil {
		return fmt.Errorf("ze answer %q rows: %w", envelope, err)
	}
	return nil
}
