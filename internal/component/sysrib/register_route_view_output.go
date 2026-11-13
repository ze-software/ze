// Design: docs/architecture/rib/unified-locrib.md -- authenticated forwarding publication.
// Related: route_view_output.go -- the common publication evaluator.

package sysrib

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"io"
	"net/netip"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	if err := pluginserver.RegisterEngineOp(pluginserver.EngineOperation{
		Method: rpc.MethodRouteViewPublish, Handler: routeViewPublishHandler,
		Native: routeViewPublishNative, Startup: pluginserver.EngineOperationRuntimeOnly,
	}); err != nil {
		panic("BUG: routing output registration failed: " + err.Error())
	}
}

func routeViewPublishHandler(host *pluginserver.Server, ctx context.Context, owner *process.Process, params json.RawMessage) (any, error) {
	if err := routeViews.revokePublicationReset(ctx, owner); err != nil {
		return nil, err
	}
	var input rpc.RouteViewPublishInput
	if err := routeViewDecode(params, &input); err != nil {
		return nil, err
	}
	batch, err := routeViewDecodeOutputBatch(input.Batch)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, routeViewRefusal("The daemon forwarding event bus is unavailable", "This publication cannot be delivered or cleaned up", "Call ze-sysrib:route-view-publish on the authenticated daemon connection")
	}
	return routeViews.publish(ctx, owner, routeViewPublication{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: batch}, host)
}

func routeViewPublishNative(host *pluginserver.Server, ctx context.Context, owner *process.Process, input any) (any, error) {
	if err := routeViews.revokePublicationReset(ctx, owner); err != nil {
		return nil, err
	}
	publication, ok := input.(routeViewPublication)
	if !ok {
		return nil, routeViewRefusal("Native routing publication has an invalid value type", "Publication accepts owned batch values only", "Send a routeViewPublication value through the system-RIB owner adapter")
	}
	if host == nil {
		return nil, routeViewRefusal("The daemon forwarding event bus is unavailable", "This publication cannot be delivered or cleaned up", "Call ze-sysrib:route-view-publish on the authenticated daemon connection")
	}
	return routeViews.publish(ctx, owner, publication, host)
}

// The wire adapter requires the fields the historical event encoder always
// emits. Native values use the same evaluator without passing through JSON.
func routeViewDecodeOutputBatch(data json.RawMessage) (sysribevents.BestChangeBatch, error) {
	if !utf8.Valid(data) {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication JSON contains invalid UTF-8", "Text must retain the same identity through both transports", routeViewOutputFix)
	}
	if len(data) > rpc.MaxMessageSize {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication exceeds the message size limit", "Direct and external requests have the same frame bound", routeViewOutputFix)
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(data), json.DefaultOptionsV1())
	start, err := decoder.ReadToken()
	if err != nil {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch is malformed", "A complete system best-change object is required", routeViewOutputFix)
	}
	if start.Kind() != '{' {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch is not an object", "A complete system best-change object is required", routeViewOutputFix)
	}
	var fam *family.Family
	var replayed bool
	var changes []json.RawMessage
	for decoder.PeekKind() != '}' {
		name, err := decoder.ReadToken()
		if err != nil {
			return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication field is malformed", "No part of the batch was applied", routeViewOutputFix)
		}
		switch name.String() {
		case "family":
			var raw jsontext.Value
			raw, err = decoder.ReadValue()
			if err == nil {
				err = routeViewDecode(json.RawMessage(raw), &fam)
			}
		case "replay":
			var raw jsontext.Value
			raw, err = decoder.ReadValue()
			if err == nil {
				err = routeViewDecode(json.RawMessage(raw), &replayed)
			}
		case "changes":
			changes, err = routeViewOutputRows(decoder)
		default: // Field names are open at the JSON boundary.
			return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication has an undeclared field", "Only family, replay and changes are accepted", routeViewOutputFix)
		}
		if err != nil {
			var refusal *opererr.Error
			if errors.As(err, &refusal) {
				return sysribevents.BestChangeBatch{}, err
			}
			return sysribevents.BestChangeBatch{}, opererr.Wrap(err, "Routing publication field is malformed", "No part of the batch was applied", routeViewOutputFix)
		}
	}
	if _, err := decoder.ReadToken(); err != nil {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch is incomplete", "No part of the batch was applied", routeViewOutputFix)
	}
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch contains trailing JSON", "Exactly one complete batch is accepted", routeViewOutputFix)
	}
	if fam == nil {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch has no family", "A forwarding batch must identify its registered family", routeViewOutputFix)
	}
	if changes == nil {
		return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication batch has no changes array", "An explicit complete change list is required", routeViewOutputFix)
	}
	if err := routeViewOutputRejectNull(data); err != nil {
		return sysribevents.BestChangeBatch{}, err
	}
	batch := sysribevents.BestChangeBatch{Family: *fam, Changes: make([]sysribevents.BestChangeEntry, len(changes))}
	if replayed {
		batch.ReplayID = replay.Broadcast
	}
	for i, raw := range changes {
		type entryFields sysribevents.BestChangeEntry
		value := sysribevents.BestChangeEntry{}
		entry := struct {
			*entryFields
			Action   *routeaction.Action `json:"action"`
			Prefix   *netip.Prefix       `json:"prefix"`
			Protocol *string             `json:"protocol"`
		}{entryFields: (*entryFields)(&value)}
		if err := routeViewDecode(raw, &entry); err != nil {
			return sysribevents.BestChangeBatch{}, err
		}
		if entry.Action == nil {
			return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication change has no action", "Every change must state add, update or withdraw", routeViewOutputFix)
		}
		if entry.Prefix == nil {
			return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication change has no prefix", "Every forwarding change needs its destination key", routeViewOutputFix)
		}
		if entry.Protocol == nil {
			return sysribevents.BestChangeBatch{}, routeViewRefusal("Routing publication change has no protocol field", "Installations name a protocol and withdrawals retain the event's explicit empty field", routeViewOutputFix)
		}
		value.Action, value.Prefix, value.Protocol = *entry.Action, *entry.Prefix, *entry.Protocol
		batch.Changes[i] = value
	}
	return batch, nil
}

// Read rows from the outer stream so an excess-row refusal never scans or
// materializes the rest of the array before enforcing its declared bound.
func routeViewOutputRows(decoder *jsontext.Decoder) ([]json.RawMessage, error) {
	start, err := decoder.ReadToken()
	if err != nil {
		return nil, routeViewRefusal("Routing publication changes is malformed", "An explicit complete change list is required", routeViewOutputFix)
	}
	if start.Kind() != '[' {
		return nil, routeViewRefusal("Routing publication changes is not an array", "An explicit complete change list is required", routeViewOutputFix)
	}
	rows := make([]json.RawMessage, 0)
	for decoder.PeekKind() != ']' {
		if len(rows) == rpc.RouteViewRecordsMax {
			return nil, routeViewRefusal("Routing publication exceeds 4096 changes", "A publication must fit the bounded batch contract", "Split route-view-publish into batches of at most 4096 changes")
		}
		row, err := decoder.ReadValue()
		if err != nil {
			return nil, routeViewRefusal("Routing publication change is malformed", "No part of the batch was applied", routeViewOutputFix)
		}
		// ReadValue lends its bytes only until the next decoder operation.
		rows = append(rows, json.RawMessage(row.Clone()))
	}
	if _, err := decoder.ReadToken(); err != nil {
		return nil, routeViewRefusal("Routing publication changes is incomplete", "No part of the batch was applied", routeViewOutputFix)
	}
	return rows, nil
}

// Null is not a declared scalar, label or forwarding member. Token inspection
// distinguishes it from text containing "null" without inventing zero values.
func routeViewOutputRejectNull(data []byte) error {
	decoder := jsontext.NewDecoder(bytes.NewReader(data), json.DefaultOptionsV1())
	for {
		token, err := decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return routeViewRefusal("Routing publication JSON is malformed", "No part of the batch was applied", routeViewOutputFix)
		}
		if token.Kind() == 'n' {
			return routeViewRefusal("Routing publication contains a null value", "Null cannot replace a declared forwarding value or member", routeViewOutputFix)
		}
	}
}
