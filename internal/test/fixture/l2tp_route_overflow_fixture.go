// Design: docs/guide/l2tp.md -- real subscriber authorization reaches route refusal.
// Related: tunnel_fixture_l2tp_ppp.go -- actual CHAP and IPCP over kernel PPPoL2TP.
// Related: tunnel_fixture_l2tp_session_clear.go -- owned, joined live-peer serving.
// Related: fixture.go -- observer startup, event application barrier and shutdown.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/subscriberid"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	routeOverflowNeighbor = "10.99.9.10"
	routeOverflowRefused  = "10.99.9.11"
	routeOverflowCode     = "doctor-l2tp-framed-route-limit"
	routeOverflowFix      = "ze show l2tp sessions"
	routeOverflowReplay   = uint64(6401)
)

// This is a decoded external event DTO, not a synthetic admission. Birth and
// SubscriberID are supplied by the real reactor and never reconstructed here.
type routeOverflowAssignment struct {
	Birth        subscriberid.L2TPBirth
	SubscriberID string
	TunnelID     uint16
	SessionID    uint16
	PeerAddr     string
	FramedRoutes []struct {
		Prefix netip.Prefix
		Metric uint32
	}
}

// The SDK event callback and the scenario share this bounded observation only.
// mu protects every field; no lock survives an RPC or a peer operation.
type routeOverflowObservation struct {
	mu          sync.Mutex
	assignments [2]routeOverflowAssignment
	incremental []redistevents.RouteChangeEntry
	replayed    []redistevents.RouteChangeEntry
	failure     error
}

func routeOverflowDriver(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	state := new(routeOverflowObservation)
	setup := func(plugin *sdk.Plugin) error {
		plugin.SetStartupSubscriptionsIn(namespaceL2TP, []string{"route-change", "session-ip-assigned"}, nil, "full")
		plugin.SetEnvelope(true)
		plugin.OnEvent(state.receive)
		return nil
	}
	return observeConfigured(ctx, "route-overflow-test", &sdk.Registration{}, setup, func(ctx context.Context, plugin *sdk.Plugin) error {
		work, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		return state.run(work, plugin, port)
	})
}

func (state *routeOverflowObservation) receive(raw string) (err error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	defer func() {
		if err != nil {
			state.failure = err
		}
	}()
	envelope, err := rpc.ParseEventEnvelope(raw)
	if err != nil {
		return err
	}
	if envelope.Namespace != namespaceL2TP {
		return errors.New("route observer received a different namespace")
	}
	switch envelope.Event {
	case "session-ip-assigned":
		var assigned routeOverflowAssignment
		if err := json.Unmarshal(envelope.Payload, &assigned); err != nil {
			return err
		}
		index := slices.Index([]string{routeOverflowNeighbor, routeOverflowRefused}, assigned.PeerAddr)
		if index < 0 {
			return fmt.Errorf("unexpected assigned address %q", assigned.PeerAddr)
		}
		if state.assignments[index].Birth.IsValid() {
			return errors.New("duplicate IPv4 assignment in the single-family scenario")
		}
		state.assignments[index] = assigned
	case "route-change":
		var batch redistevents.RouteChangeBatch
		if err := json.Unmarshal(envelope.Payload, &batch); err != nil {
			return err
		}
		if batch.Protocol == redistevents.ProtocolUnspecified {
			return errors.New("route batch has no producer identity")
		}
		if batch.AFI != 1 {
			return errors.New("single-family scenario received a non-IPv4 batch")
		}
		if batch.SAFI != 1 {
			return errors.New("single-family scenario received a non-unicast batch")
		}
		var target *[]redistevents.RouteChangeEntry
		switch batch.ReplayID {
		case 0:
			target = &state.incremental
		case routeOverflowReplay:
			target = &state.replayed
		default:
			return fmt.Errorf("unexpected replay token %d", batch.ReplayID)
		}
		if len(*target)+len(batch.Entries) > 256 {
			return errors.New("route observation exceeded its bounded two-subscriber budget")
		}
		// JSON decoding owns these values; append copies the entries before
		// the callback returns, independent of the daemon's batch pool.
		*target = append(*target, batch.Entries...)
	default:
		return fmt.Errorf("unexpected l2tp event %q", envelope.Event)
	}
	return nil
}

func (state *routeOverflowObservation) run(ctx context.Context, plugin *sdk.Plugin, port int) error {
	first, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer first.conn.Close() //nolint:errcheck // owned fixture socket
	if err := first.negotiateIPCP(ctx); err != nil {
		return err
	}
	firstLink := startSessionClearLink(ctx, first)
	defer firstLink.stop()
	neighbor, err := state.waitAssignment(ctx, 0, first, 64)
	if err != nil {
		return err
	}
	if err := routeOverflowQuiesce(ctx, plugin); err != nil {
		return err
	}
	before, err := state.usefulNeighbor()
	if err != nil {
		return err
	}
	second, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer second.conn.Close() //nolint:errcheck // owned fixture socket
	if err := second.negotiateIPCP(ctx); err != nil {
		return err
	}
	secondLink := startSessionClearLink(ctx, second)
	defer secondLink.stop()
	refused, err := state.waitAssignment(ctx, 1, second, 65)
	if err != nil {
		return err
	}
	if refused.Birth == neighbor.Birth {
		return errors.New("the two real subscribers share a Birth")
	}
	if refused.SubscriberID == neighbor.SubscriberID {
		return errors.New("the two real subscribers share a subscriber ID")
	}
	// The actual observer runs before SessionIPAssigned publication. This
	// registered barrier then joins application of earlier external events
	// (ProcessManager.DrainEventDelivery), so an absent emit is not a quiet-time guess.
	if err := routeOverflowQuiesce(ctx, plugin); err != nil {
		return err
	}
	if err := state.unchanged(before, false); err != nil {
		return err
	}
	if err := routeOverflowDiagnostics(ctx, plugin, refused); err != nil {
		return err
	}
	request, err := json.Marshal(redistevents.ReplayRequest{ReplayID: routeOverflowReplay})
	if err != nil {
		return err
	}
	if _, err := plugin.EmitEvent(ctx, redistevents.ReplayNamespace, redistevents.ReplayRequestEventType, "", "", string(request)); err != nil {
		return err
	}
	// EmitEvent returns after the synchronous native replay producer. Its
	// delivery count is not a completion receipt. Quiesce separately joins
	// the resulting event callbacks before the complete replay is compared.
	if err := routeOverflowQuiesce(ctx, plugin); err != nil {
		return err
	}
	if err := state.unchanged(before, true); err != nil {
		return err
	}
	if err := firstLink.failure(); err != nil {
		return err
	}
	if err := secondLink.failure(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: real RADIUS 65-route refusal preserved the useful 64-route Birth and exact replay tokens; daemon errors and doctor retained original context")
	return nil
}

func routeOverflowQuiesce(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := requireDone08(ctx, plugin, "request quiesce")
	return err
}

func (state *routeOverflowObservation) waitAssignment(ctx context.Context, index int, peer *tunnelAccountingPeer, routes int) (routeOverflowAssignment, error) {
	var assigned routeOverflowAssignment
	var failure error
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		state.mu.Lock()
		defer state.mu.Unlock()
		failure = state.failure
		assigned = state.assignments[index]
		if failure != nil {
			return true
		}
		return assigned.Birth.IsValid()
	}) {
		return assigned, errors.New("real daemon did not publish the expected original IPv4 assignment")
	}
	if failure != nil {
		return assigned, failure
	}
	if assigned.SubscriberID == "" {
		return assigned, errors.New("original assignment has no subscriber ID")
	}
	if assigned.TunnelID != peer.localTID {
		return assigned, errors.New("assignment does not name the actual negotiated tunnel")
	}
	if assigned.SessionID != peer.zeSID {
		return assigned, errors.New("assignment does not name the actual negotiated session")
	}
	if len(assigned.FramedRoutes) != routes {
		return assigned, fmt.Errorf("real RADIUS metadata reached the reactor with %d routes, want %d", len(assigned.FramedRoutes), routes)
	}
	return assigned, nil
}

func (state *routeOverflowObservation) usefulNeighbor() ([]redistevents.RouteChangeEntry, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure != nil {
		return nil, state.failure
	}
	if len(state.incremental) != 65 {
		return nil, fmt.Errorf("useful 64-route neighbor produced %d entries, want host plus 64", len(state.incremental))
	}
	instance := state.incremental[0].SourceInstance
	if instance == 0 {
		return nil, errors.New("useful neighbor has no native source token")
	}
	expected := []redistevents.RouteChangeEntry{{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(routeOverflowNeighbor + "/32"), SourceInstance: instance}}
	for index := range 64 {
		expected = append(expected, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: routeOverflowPrefix(0, index), Metric: 7, SourceInstance: instance})
	}
	if !slices.Equal(expected, state.incremental) {
		return nil, fmt.Errorf("useful neighbor entries differ: got %+v, want %+v", state.incremental, expected)
	}
	return slices.Clone(expected), nil
}

func (state *routeOverflowObservation) unchanged(before []redistevents.RouteChangeEntry, replay bool) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure != nil {
		return state.failure
	}
	if !slices.Equal(before, state.incremental) {
		return fmt.Errorf("65-route assignment changed accepted incremental entries: %+v", state.incremental)
	}
	if replay {
		if !slices.Equal(before, state.replayed) {
			return fmt.Errorf("65-route assignment changed the complete accepted replay: %+v", state.replayed)
		}
	}
	return nil
}

func routeOverflowDiagnostics(ctx context.Context, plugin *sdk.Plugin, original routeOverflowAssignment) error {
	var errorsAnswer any
	raw, err := requireDone08(ctx, plugin, "show errors source l2tp")
	if err != nil {
		return err
	}
	if err := decode08(raw, &errorsAnswer); err != nil {
		return err
	}
	var retained map[string]any
	for _, row := range jsonObjectsWith(errorsAnswer, "code") {
		if row["code"] == routeOverflowCode {
			if row["subject"] == original.Birth.String() {
				retained = row
			}
		}
	}
	if retained == nil {
		return fmt.Errorf("daemon errors omit the original coded refusal: %s", describeJSON(errorsAnswer))
	}
	if retained["fix"] != routeOverflowFix {
		return errors.New("retained refusal does not provide the safe read-only next command")
	}
	detail, ok := retained["detail"].(map[string]any)
	if !ok {
		return errors.New("retained refusal has no original context")
	}
	for key, value := range map[string]any{
		"subscriber-id": original.Birth.String(), fieldTunnelID: float64(original.TunnelID),
		fieldSessionID: float64(original.SessionID), "framed-routes-at-least": float64(65), "limit": float64(64),
	} {
		if detail[key] != value {
			return fmt.Errorf("retained refusal %s=%v, want %v", key, detail[key], value)
		}
	}
	cause, ok := detail["cause"].(string)
	if !ok {
		return errors.New("retained refusal has no cause")
	}
	if !strings.Contains(cause, "at least 65") {
		return fmt.Errorf("retained refusal overstates its bounded count: %q", cause)
	}
	if err := routeOverflowWarning(ctx, plugin, original, cause); err != nil {
		return err
	}
	var doctorAnswer any
	raw, err = requireDone08(ctx, plugin, "show doctor")
	if err != nil {
		return err
	}
	if err := decode08(raw, &doctorAnswer); err != nil {
		return err
	}
	for _, row := range jsonObjectsWith(doctorAnswer, "code") {
		if row["code"] != routeOverflowCode {
			continue
		}
		message, ok := row["message"].(string)
		if !ok {
			continue
		}
		if !strings.Contains(message, original.Birth.String()) {
			continue
		}
		if !strings.Contains(message, cause) {
			continue
		}
		if !strings.Contains(message, fmt.Sprintf("(tunnel %d, session %d)", original.TunnelID, original.SessionID)) {
			continue
		}
		if row["fix"] == routeOverflowFix {
			return nil
		}
	}
	return fmt.Errorf("daemon doctor omits the original refusal and safe command: %s", describeJSON(doctorAnswer))
}

// Read the daemon's frozen structured warning, so a right error event cannot
// hide a warning carrying another Birth, wire label, cause or fix.
func routeOverflowWarning(ctx context.Context, plugin *sdk.Plugin, original routeOverflowAssignment, cause string) error {
	raw, err := requireDone08(ctx, plugin, "show log recent level warn count 64")
	if err != nil {
		return err
	}
	var answer any
	if err := decode08(raw, &answer); err != nil {
		return err
	}
	for _, row := range jsonObjectsWith(answer, "attributes") {
		if row["level"] != "warn" {
			continue
		}
		attributes, ok := row["attributes"].(map[string]any)
		if !ok {
			continue
		}
		if attributes["code"] != routeOverflowCode {
			continue
		}
		if attributes["subscriber-id"] != original.Birth.String() {
			continue
		}
		for key, expected := range map[string]any{
			fieldTunnelID: float64(original.TunnelID), fieldSessionID: float64(original.SessionID),
			"error": cause, "fix": routeOverflowFix,
		} {
			if attributes[key] != expected {
				return fmt.Errorf("original warning %s=%v, want %v", key, attributes[key], expected)
			}
		}
		return nil
	}
	return fmt.Errorf("daemon log ring omits the original structured warning: %s", describeJSON(answer))
}
