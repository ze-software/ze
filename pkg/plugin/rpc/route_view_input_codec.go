// Design: docs/plugin-development/protocol.md -- routing-view input presence.
// Related: route_view_codec.go -- bounded, strict single-pass decoding.

package rpc

import (
	"encoding/json"
	"errors"
	"math"
)

// RouteViewRequestBudget is the encoded input budget for a complete request
// with any correlation ID. Direct calls MUST enforce the same frame bound.
func RouteViewRequestBudget(method string) int {
	return MaxMessageSize - len(AppendRequest(nil, math.MaxUint64, method, json.RawMessage("0"))) + 1
}

// UnmarshalJSON accepts an optional declared reader and preserves the receiver
// on malformed input. An absent reader selects the ordinary selected feed.
func (input *RouteViewOpenInput) UnmarshalJSON(data []byte) error {
	var decoded *struct {
		Reader json.RawMessage `json:"reader"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewOpen, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		return errors.New("routing-view open input is null; resend an empty input object")
	}
	var reader string
	if len(decoded.Reader) > 0 {
		var value *string
		if err := json.Unmarshal(decoded.Reader, &value); err != nil {
			return err
		}
		if value == nil {
			return errors.New("routing-view reader is null; resend selected-feed or local-authority")
		}
		switch *value {
		case RouteViewReaderSelectedFeed, RouteViewReaderLocalAuthority:
			reader = *value
		default:
			return errors.New("routing-view reader is unknown; resend selected-feed or local-authority")
		}
	}
	*input = RouteViewOpenInput{Reader: reader}
	return nil
}

// UnmarshalJSON distinguishes mandatory zero values from missing fields.
func (input *RouteViewReadInput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Session    *string `json:"session"`
		Cursor     *uint64 `json:"cursor"`
		Limit      *uint32 `json:"limit"`
		WaitMillis *uint32 `json:"wait-ms"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewRead, &decoded); err != nil {
		return err
	}
	if err := routeViewInputSession(decoded.Session); err != nil {
		return err
	}
	if decoded.Cursor == nil {
		return errors.New("routing-view read requires non-null cursor; resend the consumed cursor, including zero")
	}
	if decoded.Limit == nil {
		return errors.New("routing-view read requires non-null limit; resend the requested page limit")
	}
	if decoded.WaitMillis == nil {
		return errors.New("routing-view read requires non-null wait-ms; resend the requested wait, including zero")
	}
	*input = RouteViewReadInput{Session: *decoded.Session, Cursor: *decoded.Cursor, Limit: *decoded.Limit, WaitMillis: *decoded.WaitMillis}
	return nil
}

// UnmarshalJSON preserves optional selectors and requires the scan frontier.
func (input *RouteViewCandidatesInput) UnmarshalJSON(data []byte) error {
	type inputValue RouteViewCandidatesInput
	var decoded struct {
		inputValue
		Session *string `json:"session"`
		Cursor  *uint64 `json:"cursor"`
		Limit   *uint32 `json:"limit"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewCandidates, &decoded); err != nil {
		return err
	}
	if err := routeViewInputSession(decoded.Session); err != nil {
		return err
	}
	if decoded.Cursor == nil {
		return errors.New("routing-view candidates requires non-null cursor; resend the scan cursor, including zero")
	}
	if decoded.Limit == nil {
		return errors.New("routing-view candidates requires non-null limit; resend the requested page limit")
	}
	decoded.inputValue.Session = *decoded.Session
	decoded.inputValue.Cursor = *decoded.Cursor
	decoded.inputValue.Limit = *decoded.Limit
	*input = RouteViewCandidatesInput(decoded.inputValue)
	return nil
}

// UnmarshalJSON bounds the complete table before admitting any declaration.
// An omitted or null distance list retains its empty-list meaning.
func (input *RouteViewDistancesInput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Session   *string                                  `json:"session"`
		Operation *uint64                                  `json:"operation"`
		Distances routeViewReplyEntries[RouteViewDistance] `json:"distances"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewDistances, &decoded); err != nil {
		return err
	}
	if err := routeViewInputSession(decoded.Session); err != nil {
		return err
	}
	if decoded.Operation == nil {
		return errors.New("routing-view distances requires non-null operation; resend the retained operation identity")
	}
	*input = RouteViewDistancesInput{Session: *decoded.Session, Operation: *decoded.Operation, Distances: decoded.Distances}
	return nil
}

// UnmarshalJSON retains the consumed frontier and owns the complete batch.
func (input *RouteViewPublishInput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Session   *string          `json:"session"`
		Cursor    *uint64          `json:"cursor"`
		Operation *uint64          `json:"operation"`
		Batch     *json.RawMessage `json:"batch"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewPublish, &decoded); err != nil {
		return err
	}
	if err := routeViewInputSession(decoded.Session); err != nil {
		return err
	}
	if decoded.Cursor == nil {
		return errors.New("routing-view publish requires non-null cursor; resend the consumed cursor, including zero")
	}
	if decoded.Operation == nil {
		return errors.New("routing-view publish requires non-null operation; resend the retained operation identity")
	}
	if decoded.Batch == nil {
		return errors.New("routing-view publish requires non-null batch; resend the retained forwarding batch")
	}
	*input = RouteViewPublishInput{Session: *decoded.Session, Cursor: *decoded.Cursor, Operation: *decoded.Operation, Batch: *decoded.Batch}
	return nil
}

// UnmarshalJSON refuses an absent close token without changing the receiver.
func (input *RouteViewCloseInput) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Session *string `json:"session"`
	}
	if err := routeViewDecodeInput(data, MethodRouteViewClose, &decoded); err != nil {
		return err
	}
	if err := routeViewInputSession(decoded.Session); err != nil {
		return err
	}
	*input = RouteViewCloseInput{Session: *decoded.Session}
	return nil
}

func routeViewInputSession(session *string) error {
	if session == nil {
		return errors.New("routing-view request requires non-null session; resend the session returned by open")
	}
	if *session == "" {
		return errors.New("routing-view request has an empty session; resend the session returned by open")
	}
	return nil
}
