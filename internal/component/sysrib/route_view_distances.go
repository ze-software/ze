// Design: docs/architecture/rib/unified-locrib.md -- authoritative distances.
// Related: register.go -- the declaration seam the daemon publishes.

package sysrib

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// routeViewDistanceState is empty until this session acknowledges a declaration.
// Its table and the authority's current table are immutable once published.
// The session lease protects acknowledgement and declaration replacement.
type routeViewDistanceState struct {
	operation uint64
	table     map[string]int
	ack       rpc.RouteViewDistancesOutput
}

func (authority *routeViewAuthority) applyDistances(ctx context.Context, owner *process.Process, input rpc.RouteViewDistancesInput) (rpc.RouteViewDistancesOutput, error) {
	var output rpc.RouteViewDistancesOutput
	err := authority.withSession(ctx, owner, input.Session, func(session *routeViewSession) error {
		if session.distances.operation == math.MaxUint64 {
			if input.Operation != math.MaxUint64 {
				return routeViewRefusal("Distance operation sequence is exhausted", "The acknowledged maximum identity has no positive successor", "Close this view with ze-sysrib:route-view-close, then open a fresh view with ze-sysrib:route-view-open before applying another declaration")
			}
		}
		if input.Operation == 0 {
			return routeViewRefusal("Distance operation is zero", "An unset identity cannot establish ordering", "Resend route-view-distances with a positive operation greater than the previous operation")
		}
		if input.Operation < session.distances.operation {
			return routeViewRefusal("Distance operation precedes the acknowledged operation", "An old declaration cannot replace the current declaration", "Resend route-view-distances with an operation greater than the previous operation")
		}
		defaults, err := parseAdminDistanceConfig("{}")
		if err != nil {
			return opererr.Wrap(err, "Distance defaults could not be loaded", "The daemon cannot validate a complete declaration", "Restart Ze with a build containing the rib/distance schema")
		}
		table, err := routeViewDistanceTable(input.Distances, defaults)
		if err != nil {
			return err
		}
		if input.Operation == session.distances.operation {
			if !maps.Equal(table, session.distances.table) {
				return routeViewRefusal("Distance operation was reused with different values", "An acknowledged operation has one immutable declaration", "Resend the original values for this acknowledged operation")
			}
			output = session.distances.ack
			return nil
		}
		if err := session.checkRequest(ctx); err != nil {
			return routeViewStopped(err, session.owner)
		}
		if _, fenceErr := session.resetStatus(); fenceErr != nil {
			return fenceErr
		}
		if authority.distanceTable == nil {
			authority.distanceTable = defaults
		}
		previous := authority.distanceTable
		publishDistances(table)
		authority.distanceTable = table
		session.authoritativeRIB().Reselect()
		cursor, err := session.distanceBarrier(ctx)
		if err != nil {
			// The published map is immutable, so the original declaration remains
			// available across both failed applies and owner replacement.
			publishDistances(previous)
			authority.distanceTable = previous
			session.authoritativeRIB().Reselect()
			return err
		}
		output = rpc.RouteViewDistancesOutput{Session: session.token, Operation: input.Operation, Cursor: cursor}
		session.distances = routeViewDistanceState{operation: input.Operation, table: table, ack: output}
		return nil
	})
	return output, err
}

// distanceBarrier distinguishes a required resnapshot from failed application.
// A normal large-table change may exceed the feed retention bound. Its output
// has already been fenced; a reset marker orders the successful declaration
// without pretending the old selected view can still be used for publication.
func (session *routeViewSession) distanceBarrier(ctx context.Context) (uint64, error) {
	cursor, err := session.synchronize(ctx)
	if err == nil {
		return cursor, nil
	}
	if stopped := session.checkRequest(ctx); stopped != nil {
		return 0, routeViewStopped(stopped, session.owner)
	}
	required, fenceErr := session.resetStatus()
	if fenceErr != nil {
		return 0, fenceErr
	}
	if required {
		return session.resetBarrier(ctx)
	}
	return 0, err
}

// routeViewDistanceTable validates every row before publishing any row. Names
// and ranges come from the existing declaration, including ebgp/ibgp classes.
func routeViewDistanceTable(rows []rpc.RouteViewDistance, defaults map[string]int) (map[string]int, error) {
	const fix = "Resend route-view-distances with every rib/distance leaf exactly once and its schema-valid value"
	if len(rows) != len(defaults) {
		return nil, routeViewRefusal("Distance declaration has an incomplete or oversized table", "A partial declaration would change the meaning of absent protocols", fix)
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, opererr.Wrap(err, "Distance schema could not be loaded", "No declaration was applied", "Restart Ze with a build containing the rib/distance schema")
	}
	table := make(map[string]int, len(rows))
	for _, row := range rows {
		if _, exists := defaults[row.Protocol]; !exists {
			return nil, opererr.Wrap(fmt.Errorf("protocol %q is not a rib/distance leaf", row.Protocol), "Distance declaration names an unknown protocol", "Only the schema's declared distance classes may be changed", fix)
		}
		if _, exists := table[row.Protocol]; exists {
			return nil, opererr.Wrap(fmt.Errorf("protocol %q appears twice", row.Protocol), "Distance declaration repeats a protocol", "Every class must have exactly one value", fix)
		}
		node, err := schema.Lookup(adminDistanceSchemaPath + "/" + row.Protocol)
		if err != nil {
			return nil, opererr.Wrap(err, "Distance protocol has no schema leaf", "The declaration cannot be validated", fix)
		}
		leaf, ok := node.(*config.LeafNode)
		if !ok {
			return nil, routeViewRefusal("Distance protocol is not a schema leaf", "The declaration cannot be validated", fix)
		}
		if err := config.ValidateLeafValue(leaf, []string{"rib", "distance", row.Protocol}, strconv.Itoa(int(row.Distance))); err != nil {
			return nil, opererr.Wrap(err, "Distance declaration contains an invalid value", "No part of the declaration was applied", fix)
		}
		table[row.Protocol] = int(row.Distance)
	}
	return table, nil
}
