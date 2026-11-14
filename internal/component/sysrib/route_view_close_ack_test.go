// Design: docs/architecture/rib/unified-locrib.md -- system-RIB routing-view consumer.
// Related: route_view_client.go -- routeViewClient.Close.
// Related: route_view_client_test.go -- routeViewRuntimePeer, the socket responder.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Goal: a close acknowledgment that names another session is refused, never
// read as the cleanup of this consumer's output. Close returns the operator
// refusal and the doctor check reports the routing view failed with it.
// Method: a socket responder that opens the lease as "close-owner" and answers
// its close for "other-owner", with no worker started.
func TestRouteViewCloseRefusesAcknowledgmentForAnotherSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	closes := 0
	p := routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			return rpc.RouteViewOpenOutput{Session: "close-owner", Epoch: "daemon", Snapshot: "initial"}, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, nil
		case rpc.MethodRouteViewClose:
			closes++
			return rpc.RouteViewCloseOutput{Session: "other-owner"}, nil
		default:
			return nil, fmt.Errorf("unexpected method %q before close", request.Method)
		}
	})
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(ctx, p, newSysRIB(), declared, map[string]bool{"static": true})
	if err != nil {
		t.Fatalf("open routing-view consumer: %v", err)
	}
	err = client.Close(ctx)
	if closes != 1 {
		t.Fatalf("close requests = %d, want 1", closes)
	}
	requireRouteViewRefusal(t, err, "The system RIB close acknowledgment belongs to another session", "ze request reload")
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want the failed routing view", findings)
	}
	if findings[0].Severity != "error" {
		t.Fatalf("a refused close acknowledgment left %q health: %+v", findings[0].Severity, findings[0])
	}
	if !strings.Contains(findings[0].Message, "belongs to another session") {
		t.Fatalf("doctor message %q does not name the refused acknowledgment", findings[0].Message)
	}
}
