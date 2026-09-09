// Design: docs/architecture/api/process-protocol.md — Stage 1 declare-registration
// Related: types.go — MethodDeclareRegistration, DeclareRegistrationInput
// Related: framing.go — FrameWriter, the newline framing this line goes out in

package rpc

import (
	"encoding/json"
	"fmt"
	"io"
)

// declarationRequestID is the request id a declaration line carries. A live
// start numbers Stage 1 as #1, and a query answer is that same message on a
// second carrier, so it carries the same id.
const declarationRequestID = 1

// WriteDeclaration writes reg to out as one newline-framed Stage 1 request
// line, and returns nil once the line is written.
//
// This is the whole definition of that line: the id, the method, the framing
// and the JSON encoding are stated here and nowhere else. Two binaries write
// it, the ze binary for a plugin it carries and the SDK for a plugin binary it
// does not, and one reader parses both. A second definition would let the two
// disagree with nothing to arbitrate them.
//
// A declaration that names no command and no pipe still writes the line.
// "Declared nothing" and "sent nothing" are different answers, and a reader
// that cannot tell them apart reports a working plugin as silent.
func WriteDeclaration(out io.Writer, reg *DeclareRegistrationInput) error {
	params, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("marshal declaration: %w", err)
	}

	line := FormatRequest(declarationRequestID, MethodDeclareRegistration, params)
	if writeErr := NewFrameWriter(out).Write(line); writeErr != nil {
		return fmt.Errorf("write declaration: %w", writeErr)
	}
	return nil
}
