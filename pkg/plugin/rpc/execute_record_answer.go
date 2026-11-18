// Design: docs/architecture/api/process-protocol.md -- record outcomes on value calls.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// ExecuteCommandRecordAnswer is the original walk's terminator beside the
// document it collapsed to. It is an untrusted boundary DTO, not authority.
// Nil means no original terminator was supplied by a built-value producer.
// Explicit zero counts can accompany an empty collection or an absent built
// document; DocumentAbsent distinguishes them. Counts never come from JSON keys.
type ExecuteCommandRecordAnswer struct {
	Count    uint64   `json:"count"`
	Faults   uint64   `json:"faults"`
	Message  string   `json:"message,omitempty"`
	Warning  bool     `json:"warning,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	// DocumentAbsent is set only from the producer's actual absence of data.
	// A no-record failure keeps its existing raw Data reason, which can itself
	// be JSON, so payload bytes cannot establish whether a document existed.
	DocumentAbsent bool `json:"document-absent,omitempty"`
}

// UnmarshalJSON retains the ordinary output fields and optional metadata.
// The nonpointer field decoder sees EVERY supplied metadata occurrence,
// including null, which encoding/json skips for a pointer destination. The
// enclosing value keeps its existing unknown-field and status/data semantics.
// Failure leaves the destination unchanged; the outer RPC bounds the wire.
func (output *ExecuteCommandOutput) UnmarshalJSON(wire []byte) error {
	if output == nil {
		return errors.New("execute-command output destination is absent")
	}
	type value ExecuteCommandOutput
	// A null enclosing value has the same no-op behavior as json.Unmarshal on
	// the original struct. It does not supply the optional metadata field.
	if bytes.Equal(bytes.TrimSpace(wire), []byte("null")) {
		return nil
	}
	candidate := value(*output)
	// RawMessage.UnmarshalJSON may reuse its destination buffer, so detach it
	// before decoding to keep a failed decode from changing the caller's data.
	candidate.Data = slices.Clone(output.Data)
	input := struct {
		*value
		RecordAnswer executeCommandRecordField `json:"record-answer"`
	}{value: &candidate, RecordAnswer: executeCommandRecordField{metadata: candidate.RecordAnswer}}
	if err := json.Unmarshal(wire, &input); err != nil {
		return err
	}
	candidate.RecordAnswer = input.RecordAnswer.metadata
	*output = ExecuteCommandOutput(candidate)
	return nil
}

// This value field intercepts null on the optional pointer in the public DTO.
// Each occurrence decodes into a fresh candidate before publishing it.
type executeCommandRecordField struct{ metadata *ExecuteCommandRecordAnswer }

func (field *executeCommandRecordField) UnmarshalJSON(wire []byte) error {
	var candidate ExecuteCommandRecordAnswer
	if err := json.Unmarshal(wire, &candidate); err != nil {
		return err
	}
	field.metadata = &candidate
	return nil
}

// UnmarshalJSON accepts only the six declared metadata fields, once each.
// Count and Faults are mandatory numbers, including explicit zero. Failed
// decoding leaves the destination unchanged. The enclosing RPC bounds wire.
func (metadata *ExecuteCommandRecordAnswer) UnmarshalJSON(wire []byte) error {
	if metadata == nil {
		return errors.New("execute-command record metadata destination is absent")
	}
	if len(wire) > MaxMessageSize {
		return errors.New("execute-command record metadata exceeds one message")
	}
	decoder := json.NewDecoder(bytes.NewReader(wire))
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if opening != json.Delim('{') {
		return errors.New("execute-command record metadata is not an object")
	}
	var candidate ExecuteCommandRecordAnswer
	seen := make(map[string]bool, 6)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return errors.New("execute-command metadata field name is invalid")
		}
		if seen[name] {
			return errors.New("execute-command record metadata repeats a field")
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return err
		}
		if bytes.Equal(raw, []byte("null")) {
			return errors.New("execute-command record metadata field is null")
		}
		switch name {
		case "count":
			err = json.Unmarshal(raw, &candidate.Count)
		case "faults":
			err = json.Unmarshal(raw, &candidate.Faults)
		case "message":
			err = json.Unmarshal(raw, &candidate.Message)
		case "warning":
			err = json.Unmarshal(raw, &candidate.Warning)
		case "warnings":
			err = json.Unmarshal(raw, &candidate.Warnings)
		case "document-absent":
			err = json.Unmarshal(raw, &candidate.DocumentAbsent)
		default:
			return errors.New("execute-command record metadata has an unknown field") // Open JSON boundary refuses.
		}
		if err != nil {
			return err
		}
		seen[name] = true
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if !seen["count"] {
		return errors.New("execute-command record metadata has no produced count")
	}
	if !seen["faults"] {
		return errors.New("execute-command record metadata has no refused count")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("execute-command record metadata has trailing input")
	}
	*metadata = candidate
	return nil
}

// Tail returns an independent read-only snapshot of the original terminator.
// The caller MUST validate boundary metadata before relying on its outcome.
func (metadata *ExecuteCommandRecordAnswer) Tail() AnswerTail {
	return AnswerTail{Kind: AnswerKindTerminator, Count: metadata.Count, Faults: metadata.Faults,
		Message: metadata.Message, Warning: metadata.Warning, Warnings: slices.Clone(metadata.Warnings)}
}

func (metadata *ExecuteCommandRecordAnswer) Validate(status string) error {
	if metadata == nil {
		return errors.New("execute-command record metadata is absent")
	}
	if metadata.Count > ^uint64(0)-metadata.Faults {
		return errors.New("execute-command record counts overflow")
	}
	if metadata.DocumentAbsent {
		if metadata.Count != 0 {
			return errors.New("execute-command absent document has produced rows")
		}
		if metadata.Faults != 0 {
			return errors.New("execute-command absent document has refused rows")
		}
	}
	if !metadata.Warning {
		if len(metadata.Warnings) != 0 {
			return errors.New("execute-command warning lines have no warning terminator")
		}
	}
	tail := metadata.Tail()
	if Verdict(&tail) != status {
		return errors.New("execute-command status contradicts its original record terminator")
	}
	return nil
}

// RecordAnswerMetadata MUST be read after the single range over Records. It
// copies the actual terminator, including warnings, and refuses an unfinished
// answer. It never infers successful evaluation from an empty collection.
func (answer *Answer) RecordAnswerMetadata() (*ExecuteCommandRecordAnswer, error) {
	if answer == nil {
		return nil, errors.New("command answer is absent")
	}
	if answer.terminator == nil {
		return nil, errors.New("command answer has no completed terminator")
	}
	tail := answer.terminator
	return &ExecuteCommandRecordAnswer{Count: tail.Count, Faults: tail.Faults, Message: tail.Message,
		Warning: tail.Warning, Warnings: slices.Clone(tail.Warnings)}, nil
}

// Failure MUST be read after Records. The command's own public failure text
// remains unchanged; fault-only outcomes get a bounded explanation alongside
// their complete structured records. A transport error remains distinct.
func (answer *Answer) Failure() error {
	if answer == nil {
		return errors.New("command answer is absent")
	}
	if err := answer.Err(); err != nil {
		return err
	}
	return answerFailure(answer.Verdict(), answer.Message())
}

// Failure reports the original command's outcome without walking its document.
// The caller MUST first Validate this boundary metadata against its status.
// Nil metadata reports an absent answer, as Answer.Failure does.
func (metadata *ExecuteCommandRecordAnswer) Failure() error {
	if metadata == nil {
		return errors.New("command answer is absent")
	}
	tail := metadata.Tail()
	return answerFailure(Verdict(&tail), tail.Message)
}

// Both buffered and streamed readers retain one public command failure policy.
func answerFailure(verdict, message string) error {
	if message != "" {
		return errors.New(message)
	}
	switch verdict {
	case VerdictDone, VerdictWarning:
		return nil
	case VerdictPartial, VerdictError, VerdictAborted, VerdictTruncated:
		return fmt.Errorf("command answer is %s; its structured data retains the completed and rejected records", verdict)
	default:
		return errors.New("command answer has an unknown outcome") // Boundary outcome refuses.
	}
}
