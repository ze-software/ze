// Design: docs/plugin-development/protocol.md -- routing-view reply presence.
// Related: route_view.go -- public values assigned only after complete decoding.

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The record bound is enforced before allocating the next typed path. Bounding
// only the encoded page would let many tiny objects expand into a huge slice.
type routeViewReplyEntries[T any] []T

func (entries *routeViewReplyEntries[T]) UnmarshalJSON(data []byte) error {
	decoded, err := routeViewDecodeEntries[T](data, RouteViewRecordsMax)
	if err != nil {
		return err
	}
	*entries = decoded
	return nil
}

func routeViewDecodeEntries[T any](data []byte, limit int) ([]T, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if opening == nil {
		// Optional entries may be null; phase and completion carry the outcome.
		return nil, nil
	}
	if opening != json.Delim('[') {
		return nil, errors.New("routing-view entries must be an array")
	}
	decoded := make([]T, 0)
	for decoder.More() {
		if len(decoded) == limit {
			return nil, errors.New("routing-view result exceeds maximum record count")
		}
		var entry T
		if err := decoder.Decode(&entry); err != nil {
			return nil, err
		}
		decoded = append(decoded, entry)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return decoded, nil
}

// UnmarshalJSON applies route-install's gateway-or-device admission to each
// equal-cost member. A malformed target leaves the receiver unchanged.
func (target *RouteViewNextHop) UnmarshalJSON(data []byte) error {
	type targetValue RouteViewNextHop
	var decoded targetValue
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !decoded.NextHop.IsValid() && decoded.Interface == "" {
		return errors.New("routing-view target names neither a next-hop nor an interface")
	}
	*target = RouteViewNextHop(decoded)
	return nil
}

// UnmarshalJSON checks the message budget before decoding a selected path.
// The alias avoids recursion, and assignment preserves the path on failure.
func (path *RouteViewPath) UnmarshalJSON(data []byte) error {
	type pathValue RouteViewPath
	var reply struct {
		pathValue
		Protocol *string `json:"protocol"`
		Instance *uint32 `json:"instance"`
		Distance *uint8  `json:"distance"`
		Metric   *uint32 `json:"metric"`
	}
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := routeViewRequiredToken("protocol", reply.Protocol); err != nil {
		return err
	}
	if reply.Instance == nil {
		return errors.New("routing-view path requires non-null instance; resend the complete selected path")
	}
	if reply.Distance == nil {
		return errors.New("routing-view path requires non-null distance; resend the complete selected path")
	}
	if reply.Metric == nil {
		return errors.New("routing-view path requires non-null metric; resend the complete selected path")
	}
	reply.pathValue.Protocol = *reply.Protocol
	reply.pathValue.Instance = *reply.Instance
	reply.pathValue.Distance = *reply.Distance
	reply.pathValue.Metric = *reply.Metric
	*path = RouteViewPath(reply.pathValue)
	return nil
}

// UnmarshalJSON preserves explicit zero distances and strict request fields.
// A refused row leaves the receiver unchanged.
func (distance *RouteViewDistance) UnmarshalJSON(data []byte) error {
	var input struct {
		Protocol *string `json:"protocol"`
		Distance *uint8  `json:"distance"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewDistances, &input); err != nil {
		return err
	}
	if err := routeViewRequiredToken("protocol", input.Protocol); err != nil {
		return err
	}
	if input.Distance == nil {
		return errors.New("routing-view distance requires non-null distance; resend the complete distance row")
	}
	*distance = RouteViewDistance{Protocol: *input.Protocol, Distance: *input.Distance}
	return nil
}

// Custom input decoders must retain the owner's unknown-field and frame bounds.
func routeViewDecodeInput(data []byte, method string, input any) error {
	if len(data) > RouteViewRequestBudget(method) {
		return errors.New("routing-view request exceeds maximum message size; resend a bounded request")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("routing-view request contains trailing JSON; resend one complete input object")
	}
	return nil
}

// Pointer fields retain presence without copying a page into an intermediate
// raw-message map. Each reply decodes its entries into their final value type.
type routeViewSessionReply struct {
	Session *string `json:"session"`
}

func (reply routeViewSessionReply) validate() error {
	return routeViewRequiredToken("session", reply.Session)
}

type routeViewSnapshotReply struct {
	routeViewSessionReply
	Epoch    *string `json:"epoch"`
	Snapshot *string `json:"snapshot"`
	Cursor   *uint64 `json:"cursor"`
}

func (reply routeViewSnapshotReply) validate() error {
	if err := reply.routeViewSessionReply.validate(); err != nil {
		return err
	}
	if err := routeViewRequiredToken("epoch", reply.Epoch); err != nil {
		return err
	}
	if reply.Cursor == nil {
		return errors.New("routing-view result requires non-null cursor")
	}
	if reply.Snapshot != nil {
		if err := routeViewRequiredToken("snapshot", reply.Snapshot); err != nil {
			return err
		}
	}
	return nil
}

type routeViewMutationReply struct {
	routeViewSessionReply
	Operation *uint64 `json:"operation"`
	Cursor    *uint64 `json:"cursor"`
}

func (reply routeViewMutationReply) validate() error {
	if err := reply.routeViewSessionReply.validate(); err != nil {
		return err
	}
	if reply.Operation == nil {
		return errors.New("routing-view result requires non-null operation")
	}
	if *reply.Operation == 0 {
		return errors.New("routing-view result operation must be positive")
	}
	if reply.Cursor == nil {
		return errors.New("routing-view result requires non-null cursor")
	}
	return nil
}

func routeViewRequiredToken(name string, value *string) error {
	if value == nil {
		return fmt.Errorf("routing-view result requires non-null %s", name)
	}
	if *value == "" {
		return fmt.Errorf("routing-view result has empty %s", name)
	}
	return nil
}

func routeViewDecodeReply(data []byte, reply any) error {
	if len(data) > RouteViewResultBudget() {
		return errors.New("routing-view result exceeds maximum message size")
	}
	return json.Unmarshal(data, reply)
}

// UnmarshalJSON preserves a real zero cursor and refuses an absent frontier.
func (output *RouteViewOpenOutput) UnmarshalJSON(data []byte) error {
	var reply routeViewSnapshotReply
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	if err := routeViewRequiredToken("snapshot", reply.Snapshot); err != nil {
		return err
	}
	*output = RouteViewOpenOutput{Session: *reply.Session, Epoch: *reply.Epoch, Snapshot: *reply.Snapshot, Cursor: *reply.Cursor}
	return nil
}

// UnmarshalJSON decodes selected values once and assigns no partial page.
func (output *RouteViewReadOutput) UnmarshalJSON(data []byte) error {
	var reply struct {
		routeViewSnapshotReply
		Phase               *RouteViewPhase                       `json:"phase"`
		Complete            *bool                                 `json:"complete"`
		Entries             routeViewReplyEntries[RouteViewEntry] `json:"entries"`
		RejectedPublication json.RawMessage                       `json:"rejected-publication"`
	}
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	if reply.Phase == nil {
		return errors.New("routing-view result requires non-null phase")
	}
	if reply.Complete == nil {
		return errors.New("routing-view result requires non-null complete")
	}
	var snapshot string
	if reply.Snapshot != nil {
		snapshot = *reply.Snapshot
	}
	if *reply.Phase == RouteViewPhaseSnapshot {
		if err := routeViewRequiredToken("snapshot", reply.Snapshot); err != nil {
			return err
		}
	}
	var rejected uint64
	if len(reply.RejectedPublication) > 0 {
		if err := json.Unmarshal(reply.RejectedPublication, &rejected); err != nil {
			return err
		}
		if rejected == 0 {
			return errors.New("routing-view publication reset proof requires a positive operation; read a valid reset on the refused publication's session")
		}
		if *reply.Phase != RouteViewPhaseReset {
			return errors.New("routing-view publication reset proof requires reset phase; read a valid reset on the refused publication's session")
		}
		if *reply.Complete {
			return errors.New("routing-view publication reset proof cannot claim completion; read a valid reset on the refused publication's session")
		}
		if reply.Snapshot != nil {
			return errors.New("routing-view publication reset proof cannot name a snapshot; read a valid reset on the refused publication's session")
		}
		if len(reply.Entries) > 0 {
			return errors.New("routing-view publication reset proof cannot carry entries; read a valid reset on the refused publication's session")
		}
	}
	*output = RouteViewReadOutput{Session: *reply.Session, Epoch: *reply.Epoch, Snapshot: snapshot, Cursor: *reply.Cursor, Phase: *reply.Phase, Complete: *reply.Complete, Entries: reply.Entries, RejectedPublication: rejected}
	return nil
}

// UnmarshalJSON retains candidate JSON without an intermediate raw page copy.
func (output *RouteViewCandidatesOutput) UnmarshalJSON(data []byte) error {
	var reply struct {
		routeViewSnapshotReply
		Complete *bool                                  `json:"complete"`
		Entries  routeViewReplyEntries[json.RawMessage] `json:"entries"`
	}
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	if err := routeViewRequiredToken("snapshot", reply.Snapshot); err != nil {
		return err
	}
	if reply.Complete == nil {
		return errors.New("routing-view result requires non-null complete")
	}
	*output = RouteViewCandidatesOutput{Session: *reply.Session, Epoch: *reply.Epoch, Snapshot: *reply.Snapshot, Cursor: *reply.Cursor, Complete: *reply.Complete, Entries: reply.Entries}
	return nil
}

// UnmarshalJSON requires an explicit applied operation and its frontier.
func (output *RouteViewDistancesOutput) UnmarshalJSON(data []byte) error {
	var reply routeViewMutationReply
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	*output = RouteViewDistancesOutput{Session: *reply.Session, Operation: *reply.Operation, Cursor: *reply.Cursor}
	return nil
}

// UnmarshalJSON requires the identity of an acknowledged forwarding operation.
func (output *RouteViewPublishOutput) UnmarshalJSON(data []byte) error {
	var reply routeViewMutationReply
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	*output = RouteViewPublishOutput{Session: *reply.Session, Operation: *reply.Operation, Cursor: *reply.Cursor}
	return nil
}

// UnmarshalJSON refuses an empty success where scoped cleanup was not proved.
func (output *RouteViewCloseOutput) UnmarshalJSON(data []byte) error {
	var reply routeViewSessionReply
	if err := routeViewDecodeReply(data, &reply); err != nil {
		return err
	}
	if err := reply.validate(); err != nil {
		return err
	}
	*output = RouteViewCloseOutput{Session: *reply.Session}
	return nil
}
