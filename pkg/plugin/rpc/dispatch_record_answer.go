// Design: docs/architecture/api/process-protocol.md -- command outcomes.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
)

// UnmarshalJSON applies the existing original-record decoder to every supplied
// metadata occurrence, including null and repeated outer fields. Failure leaves
// the destination unchanged. Ordinary built values keep their existing shape.
func (output *DispatchCommandOutput) UnmarshalJSON(wire []byte) error {
	if output == nil {
		return errors.New("dispatch-command output destination is absent")
	}
	type value DispatchCommandOutput
	if bytes.Equal(bytes.TrimSpace(wire), []byte("null")) {
		return nil
	}
	candidate := value(*output)
	candidate.Data = slices.Clone(output.Data)
	candidate.Warnings = slices.Clone(output.Warnings)
	input := struct {
		*value
		RecordAnswer executeCommandRecordField `json:"record-answer"`
	}{value: &candidate, RecordAnswer: executeCommandRecordField{metadata: candidate.RecordAnswer}}
	if err := json.Unmarshal(wire, &input); err != nil {
		return err
	}
	candidate.RecordAnswer = input.RecordAnswer.metadata
	*output = DispatchCommandOutput(candidate)
	return nil
}
