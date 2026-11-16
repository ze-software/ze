// Design: docs/architecture/api/process-protocol.md -- owner-registered engine operations.
// Related: register_engine_op.go -- the declarations whose handlers decode with it.

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DecodeEngineOperationInput decodes params into input strictly: one JSON
// object holding only the fields input declares, within the frame bound. The
// direct path has no frame reader, so the bound is checked here for both
// transports. subject names the request in the operator error, as in "Owned
// interface device request". No part of a refused request is applied.
func DecodeEngineOperationInput(params json.RawMessage, input any, subject string) error {
	const why = "No part of this request was applied"
	const fix = "Resend one complete JSON object holding only the input fields the request declares, as ze schema methods lists them"
	if len(params) > rpc.MaxMessageSize {
		return opererr.New(subject+" exceeds the message size limit", "Direct and external requests have the same frame bound", fix)
	}
	decoder := json.NewDecoder(bytes.NewReader(params))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return opererr.Wrap(err, subject+" is malformed", why, fix)
	}
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return opererr.New(subject+" contains trailing JSON", "Exactly one input object is accepted per request", fix)
}
