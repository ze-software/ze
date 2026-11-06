// Design: docs/architecture/testing/runner-architecture.md -- fail-closed editor result IPC.

package editorcase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
	"github.com/ze-software/ze/internal/test/trace"
)

type resultWire struct {
	Passed   *bool           `json:"passed"`
	Error    *string         `json:"error"`
	TempDir  *string         `json:"temp-dir"`
	Duration *time.Duration  `json:"duration"`
	Steps    json.RawMessage `json:"steps"`
}

func readResult(file *os.File, directory string) (*editortest.TestResult, error) {
	var raw json.RawMessage
	if err := readMessage(file, &raw); err != nil {
		return nil, fmt.Errorf("editor result IPC: %w", err)
	}
	if err := completeObject(raw, reflect.TypeFor[resultWire](), false); err != nil {
		return nil, err
	}
	var wire resultWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, fmt.Errorf("editor result fields: %w", err)
	}
	if wire.Passed == nil {
		return nil, errors.New("editor result lacks passed")
	}
	if wire.Error == nil {
		return nil, errors.New("editor result lacks error")
	}
	if wire.TempDir == nil {
		return nil, errors.New("editor result lacks temp-dir")
	}
	if wire.Duration == nil {
		return nil, errors.New("editor result lacks duration")
	}
	if len(wire.Steps) == 0 {
		return nil, errors.New("editor result lacks steps")
	}
	if *wire.TempDir != directory {
		return nil, errors.New("editor result names another working directory")
	}
	if *wire.Duration < 0 {
		return nil, errors.New("editor result has negative duration")
	}
	if *wire.Passed != (*wire.Error == "") {
		return nil, errors.New("editor result contradicts its error")
	}
	var steps []trace.StepResult
	var stepFrames []json.RawMessage
	if err := json.Unmarshal(wire.Steps, &stepFrames); err != nil {
		return nil, fmt.Errorf("editor result steps: %w", err)
	}
	for _, frame := range stepFrames {
		if err := completeObject(frame, reflect.TypeFor[trace.StepResult](), true); err != nil {
			return nil, fmt.Errorf("editor result step: %w", err)
		}
	}
	decoder = json.NewDecoder(bytes.NewReader(wire.Steps))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&steps); err != nil {
		return nil, fmt.Errorf("editor result steps: %w", err)
	}
	previous := 0
	for _, step := range steps {
		if step.Step <= previous {
			return nil, errors.New("editor result steps are not ordered")
		}
		if step.Kind == "" {
			return nil, errors.New("editor result step lacks its kind")
		}
		if step.Line < 0 {
			return nil, errors.New("editor result step has negative line")
		}
		if !step.Passed {
			if step.Detail == "" {
				return nil, errors.New("editor failed step has no cause")
			}
		}
		if *wire.Passed {
			if !step.Passed {
				return nil, errors.New("editor result passed with a failed step")
			}
		}
		previous = step.Step
	}
	return &editortest.TestResult{Passed: *wire.Passed, Error: *wire.Error, TempDir: *wire.TempDir, Duration: *wire.Duration, Steps: steps}, nil
}

// completeObject derives the canonical keys from the wire type. It refuses
// missing, unknown and duplicate keys instead of accepting JSON's last-key-wins
// or case-insensitive field matching. The outer IPC byte limit bounds its input.
func completeObject(raw []byte, shape reflect.Type, noNull bool) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil {
		return err
	}
	if start != json.Delim('{') {
		return errors.New("editor result must be one object")
	}
	allowed := make(map[string]struct{}, shape.NumField())
	for field := range shape.Fields() {
		allowed[field.Tag.Get("json")] = struct{}{}
	}
	seen := make(map[string]struct{}, shape.NumField())
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("editor result key is not a string")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate editor result field %q", key)
		}
		if _, exists := allowed[key]; !exists {
			return fmt.Errorf("unknown editor result field %q", key)
		}
		seen[key] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		if noNull {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("editor result field %q is null", key)
			}
		}
	}
	if len(seen) != shape.NumField() {
		return errors.New("editor result object lacks required fields")
	}
	_, err = decoder.Token()
	return err
}
