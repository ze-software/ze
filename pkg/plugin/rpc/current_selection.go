// Design: docs/architecture/api/process-protocol.md -- current selection validation.
// Related: selected.go -- final selected-route receipts are a separate contract.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"time"
)

// MethodCurrentSelection asks the exact selecting connection for current desired
// state. The callback confers no forwarding permission and returns no candidates.
const MethodCurrentSelection = "ze-plugin-callback:current-selection"

// CurrentSelectionPresence has no valid zero: absence must be explicit.
type CurrentSelectionPresence uint8

const (
	CurrentSelectionUnspecified CurrentSelectionPresence = iota
	CurrentSelectionPresent
	CurrentSelectionAbsent
)

// CurrentSelectionKey is raw callback input. KeyNLRI has native advertisement
// framing without the source ADD-PATH identifier; Revision names proposed work.
type CurrentSelectionKey struct {
	Family   string `json:"family"`
	KeyNLRI  []byte `json:"key-nlri"`
	Revision uint64 `json:"revision"`
}

// CurrentSelectionInput identifies one destination incarnation on this selecting
// connection. Callers borrow their input until the call returns.
type CurrentSelectionInput struct {
	Destination        string                `json:"destination"`
	DestinationSession uint64                `json:"destination-session"`
	DeadlineMS         int64                 `json:"deadline-ms"`
	Keys               []CurrentSelectionKey `json:"keys"`
}

// CurrentSelectionResult echoes a query. An absent key has CurrentRevision zero;
// its scope's Cut provides the current frontier without per-key tombstones.
type CurrentSelectionResult struct {
	Family          string                   `json:"family"`
	KeyNLRI         []byte                   `json:"key-nlri"`
	Revision        uint64                   `json:"revision"`
	Presence        CurrentSelectionPresence `json:"presence"`
	CurrentRevision uint64                   `json:"current-revision"`
}

// CurrentSelectionOutput is raw callback output, never forwarding authority.
// Explicit zero Cut denotes a fresh empty scope. Nil output is invalid.
type CurrentSelectionOutput struct {
	Destination        string                   `json:"destination"`
	DestinationSession uint64                   `json:"destination-session"`
	Cut                uint64                   `json:"cut"`
	Results            []CurrentSelectionResult `json:"results"`
}

func currentSelectionFields(data []byte, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		field, ok := fields[name]
		if !ok {
			return fmt.Errorf("current-selection requires %s; send the complete callback shape", name)
		}
		if bytes.Equal(field, []byte("null")) {
			return fmt.Errorf("current-selection %s is null; send an explicit value", name)
		}
	}
	return nil
}

// UnmarshalJSON distinguishes a required explicit zero from an omitted field.
func (out *CurrentSelectionOutput) UnmarshalJSON(data []byte) error {
	if err := currentSelectionFields(data, "destination", "destination-session", "cut", "results"); err != nil {
		return err
	}
	type raw CurrentSelectionOutput
	var decoded raw
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*out = CurrentSelectionOutput(decoded)
	return nil
}

// UnmarshalJSON requires every identity and explicit presence/revision field.
func (result *CurrentSelectionResult) UnmarshalJSON(data []byte) error {
	if err := currentSelectionFields(data, "family", "key-nlri", "revision", "presence", "current-revision"); err != nil {
		return err
	}
	type raw CurrentSelectionResult
	var decoded raw
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*result = CurrentSelectionResult(decoded)
	return nil
}

// Validate checks raw input at either transport's admission boundary.
func (input CurrentSelectionInput) Validate(now time.Time) error {
	if _, err := netip.ParseAddr(input.Destination); err != nil {
		return fmt.Errorf("current-selection destination %q is not an address; use the destination peer address: %w", input.Destination, err)
	}
	if input.DestinationSession == 0 {
		return errors.New("current-selection destination session is zero; use the exact destination UP token")
	}
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("current-selection deadline expired; retry with a live deadline")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("current-selection deadline exceeds thirty seconds; split the lookup into bounded calls")
	}
	if len(input.Keys) == 0 {
		return errors.New("current-selection has no keys; send a nonempty native-key batch")
	}
	for _, key := range input.Keys {
		if key.Family == "" {
			return errors.New("current-selection family is empty; supply the native key's registered family")
		}
		if len(key.KeyNLRI) == 0 {
			return errors.New("current-selection native key is empty; supply advertisement framing")
		}
		if key.Revision == 0 {
			return errors.New("current-selection requested revision is zero; use its published selection revision")
		}
	}
	size, err := CurrentSelectionExchangeSize(input)
	if err != nil {
		return err
	}
	if size > MaxMessageSize {
		return errors.New("current-selection request or reply exceeds the frame limit; split the native-key batch")
	}
	return nil
}

// Validate refuses incomplete snapshots and rebound scope on both transports.
// Requested revisions newer than Cut remain observable; selected admission must
// refuse them as unpublished, rather than inventing a current frontier.
func (out *CurrentSelectionOutput) Validate(input CurrentSelectionInput) error {
	if out == nil {
		return errors.New("current-selection reply is missing; return the complete scope snapshot")
	}
	if out.Destination != input.Destination {
		return errors.New("current-selection reply destination differs; echo the requested destination")
	}
	if out.DestinationSession != input.DestinationSession {
		return errors.New("current-selection reply session differs; echo the requested destination session")
	}
	if len(out.Results) != len(input.Keys) {
		return errors.New("current-selection reply is incomplete; return one ordered result per requested key")
	}
	for i, key := range input.Keys {
		result := out.Results[i]
		if result.Family != key.Family {
			return fmt.Errorf("current-selection result %d family differs; echo the requested family", i)
		}
		if !bytes.Equal(result.KeyNLRI, key.KeyNLRI) {
			return fmt.Errorf("current-selection result %d key differs; echo the requested native key", i)
		}
		if result.Revision != key.Revision {
			return fmt.Errorf("current-selection result %d revision differs; echo the requested revision", i)
		}
		switch result.Presence {
		case CurrentSelectionPresent:
			if result.CurrentRevision == 0 {
				return errors.New("current-selection present key has revision zero; return its published revision")
			}
			if result.CurrentRevision > out.Cut {
				return errors.New("current-selection key is newer than its cut; read one consistent scope snapshot")
			}
		case CurrentSelectionAbsent:
			if result.CurrentRevision != 0 {
				return errors.New("current-selection absent key has a revision; return zero and the current scope cut")
			}
		case CurrentSelectionUnspecified:
			return errors.New("current-selection presence is unspecified; return explicit present or absent")
		default:
			return errors.New("current-selection presence is unknown; return explicit present or absent")
		}
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if idPrefixMax+len("ok ")+len(encoded) > MaxMessageSize {
		return errors.New("current-selection reply exceeds the frame limit; split the native-key batch")
	}
	return nil
}

// CurrentSelectionExchangeSize bounds request and the widest complete reply.
// Batch producers use this to split lookups before either transport admits them.
func CurrentSelectionExchangeSize(input CurrentSelectionInput) (int, error) {
	input.DeadlineMS = math.MaxInt64
	encoded, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	size := idPrefixMax + len(MethodCurrentSelection) + 1 + len(encoded)
	out := CurrentSelectionOutput{Destination: input.Destination, DestinationSession: input.DestinationSession,
		Cut: math.MaxUint64, Results: make([]CurrentSelectionResult, len(input.Keys))}
	for i, key := range input.Keys {
		out.Results[i] = CurrentSelectionResult{Family: key.Family, KeyNLRI: key.KeyNLRI, Revision: key.Revision,
			Presence: CurrentSelectionPresent, CurrentRevision: math.MaxUint64}
	}
	encoded, err = json.Marshal(out)
	if err != nil {
		return 0, err
	}
	return max(size, idPrefixMax+len("ok ")+len(encoded)), nil
}

// CurrentSelectionCallContext bounds the callback and stamps its wire deadline.
// The returned cancel MUST be called. This read-only lookup needs no remote
// mutation cancellation protocol; the worker honors the absolute deadline.
func CurrentSelectionCallContext(ctx context.Context, input *CurrentSelectionInput) (context.Context, context.CancelFunc) {
	work, cancel := context.WithTimeout(ctx, CancellableRequestTimeout)
	if input.DeadlineMS > 0 {
		deadline, _ := work.Deadline()
		if time.UnixMilli(input.DeadlineMS).Before(deadline) {
			cancel()
			work, cancel = context.WithDeadline(ctx, time.UnixMilli(input.DeadlineMS))
		}
	}
	deadline, _ := work.Deadline()
	input.DeadlineMS = deadline.UnixMilli()
	return work, cancel
}

// CurrentSelection queries this exact connection, bypassing ordinary callback
// execution at the recipient through its registered independent ingress lane.
func (m *MuxConn) CurrentSelection(ctx context.Context, input CurrentSelectionInput) (*CurrentSelectionOutput, error) {
	ctx, cancel := CurrentSelectionCallContext(ctx, &input)
	defer cancel()
	if err := input.Validate(time.Now()); err != nil {
		return nil, err
	}
	data, err := m.CallRPC(ctx, MethodCurrentSelection, input)
	if err != nil {
		return nil, err
	}
	var output CurrentSelectionOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("current-selection reply: %w", err)
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return &output, nil
}
