// Design: docs/functional-tests.md -- policy admission through real daemon sockets and both filter transports.
// Related: test/plugin/atomic-aggregate-policy-admission.ci -- placement and daemon configuration.
// RFC: rfc/short/rfc4271.md -- Sections 4.2, 4.3 and 5.1.6.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/atomic-aggregate-policy-admission", atomicAggregatePolicyDriver, StopsDaemon)
	Register("plugin/atomic-aggregate-policy-forwarder", atomicAggregatePolicyForwarder)
}

// atomicAggregatePolicyDriver judges import and raw export through the forked
// SDK. The second destination's ordinary MED filter runs in the placement the
// configuration names. Socket assertions require useful output from each transport.
func atomicAggregatePolicyDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("atomic policy admission requires <internal|forked> <BGP listen address>")
	}
	if args[0] != sysribPlacementInternal && args[0] != sysribPlacementForked {
		return fmt.Errorf("atomic policy admission: unknown placement %q", args[0])
	}
	registration := sdk.Registration{Filters: []sdk.FilterDecl{
		{Name: rfc8212DirectionImport, Direction: sdk.FilterImport, Reads: []string{sdk.FilterFieldRaw}, OnError: sdk.OnErrorReject},
		{Name: directionExport, Direction: sdk.FilterExport, Reads: []string{sdk.FilterFieldRaw}, OnError: sdk.OnErrorReject},
	}}
	var rawCalls atomic.Int64
	setup := func(plugin *sdk.Plugin) error {
		plugin.OnFilter(func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
			rawCalls.Add(1)
			return atomicAggregatePolicyAnswer(route)
		})
		return nil
	}
	return observeConfigured(ctx, "atomic-policy", &registration, setup,
		func(ctx context.Context, plugin *sdk.Plugin) error {
			return atomicAggregatePolicyScenario(ctx, plugin, args[0], args[1], &rawCalls)
		})
}

// atomicAggregatePolicyForwarder owns cached forwarding on a different SDK
// connection from the raw filters. The SDK serializes callbacks: a forward
// issued inside the filter process's event handler would wait for an export
// callback that the same blocked event loop cannot answer.
func atomicAggregatePolicyForwarder(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("atomic policy forwarder: unexpected arguments: %v", args)
	}
	plugin, err := newObserver("atomic-forwarder")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // The run result carries the useful transport error.
	plugin.SetStartupSubscriptions([]string{eventUpdate}, []string{addrLoopbackSecond}, "parsed")
	plugin.OnEvent(func(raw string) error {
		var event struct {
			BGP struct {
				Message struct {
					ID        uint64 `json:"id"`
					Direction string `json:"direction"`
				} `json:"message"`
				Peer struct {
					Name string `json:"name"`
				} `json:"peer"`
				Update map[string]json.RawMessage `json:"update"`
			} `json:"bgp"`
		}
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return err
		}
		if event.BGP.Message.Direction != directionReceived {
			return nil
		}
		id := []uint64{event.BGP.Message.ID}
		if _, eor := event.BGP.Update["eor"]; eor {
			return plugin.ReleaseCached(ctx, id)
		}
		if event.BGP.Peer.Name != peerNameSource {
			return plugin.ReleaseCached(ctx, id)
		}
		if err := plugin.ForwardCached(ctx, rpc.ForwardCachedInput{IDs: id, Destinations: []string{addrLoopbackThird, addrLoopbackFourth}}); err != nil {
			return fmt.Errorf("atomic policy forward cached source update %d: %w", id[0], err)
		}
		return nil
	})
	return plugin.Run(ctx, &sdk.Registration{CacheConsumer: true})
}

// atomicAggregatePolicyAnswer deliberately proposes the unsafe answer. A
// refusal inside this fixture would test the fixture rather than the daemon.
func atomicAggregatePolicyAnswer(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	body := route.Raw()
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nil, err
	}
	nlri := sections.NLRI(body)
	if len(nlri) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	attrs := sections.Attrs(body)
	_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
	if !found {
		return nil, errors.New("atomic policy subject has no MED")
	}
	if len(med) != 4 {
		return nil, errors.New("atomic policy subject has malformed MED")
	}
	metric := binary.BigEndian.Uint32(med)
	if route.Direction() == rfc8212DirectionImport {
		if metric == 60 {
			if len(nlri) != 8 {
				return nil, fmt.Errorf("atomic policy subset expected two /24 routes, got %x", nlri)
			}
			if nlri[0] != 24 {
				return nil, fmt.Errorf("atomic policy subset expected two /24 routes, got %x", nlri)
			}
			if nlri[4] != 24 {
				return nil, fmt.Errorf("atomic policy subset expected two /24 routes, got %x", nlri)
			}
			return &sdk.FilterUpdateOutput{Action: sdk.FilterModify,
				Raw: atomicAggregatePolicyBody(nil, attrs, nlri[4:])}, nil
		}
		if metric == 80 {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterModify,
				Raw: atomicAggregatePolicyBody(nil, nil, nil)}, nil
		}
	}
	strip := route.Direction() == rfc8212DirectionImport && metric == 90
	if route.Direction() == directionExport && metric == 20 {
		strip = true
	}
	if strip {
		var kept []byte
		iterator := attribute.NewAttrIterator(attrs)
		for {
			start := iterator.Offset()
			code, _, _, present := iterator.Next()
			if !present {
				break
			}
			if code != attribute.AttrAtomicAggregate {
				kept = append(kept, attrs[start:iterator.Offset()]...)
			}
		}
		return &sdk.FilterUpdateOutput{Action: sdk.FilterModify,
			Raw: atomicAggregatePolicyBody(sections.Withdrawn(body), kept, nlri)}, nil
	}
	if route.Direction() == directionExport && metric == 30 {
		if len(nlri) != 4 {
			return nil, fmt.Errorf("atomic policy expected one /24, got %x", nlri)
		}
		if nlri[0] != 24 {
			return nil, fmt.Errorf("atomic policy expected one /24, got %x", nlri)
		}
		specific := append([]byte{25}, nlri[1:]...)
		specific = append(specific, 0)
		kept := bytes.Clone(attrs)
		_, _, _, marked := attribute.AttrFind(kept, attribute.AttrAtomicAggregate)
		if !marked {
			kept = append(kept, 0x40, 6, 0)
		}
		return &sdk.FilterUpdateOutput{Action: sdk.FilterModify,
			Raw: atomicAggregatePolicyBody(sections.Withdrawn(body), kept, specific)}, nil
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}

// A later permitted UPDATE on the same socket bounds every silence assertion.
// The original sockets remain open until all wire and actual RIB checks finish.
func atomicAggregatePolicyScenario(ctx context.Context, plugin *sdk.Plugin, placement, address string, rawCalls *atomic.Int64) (err error) {
	stage := "connect peers"
	defer func() {
		if err != nil {
			err = fmt.Errorf("atomic policy %s %s: %w", placement, stage, err)
		}
	}()
	var sockets [3]net.Conn
	// The scenario MUST keep these sockets open through all wire and RIB
	// assertions. This cleanup MUST close every opened socket on any return.
	defer func() {
		for _, conn := range sockets {
			if conn != nil {
				_ = conn.Close() //nolint:errcheck // Assertions are complete or the scenario already failed.
			}
		}
	}()
	// The source is external and both destinations are internal. That keeps
	// received MED visible to the raw wire oracle under RFC 4271 Section 5.1.4.
	for index, asn := range []uint16{65001, 65000, 65000} {
		peer := netip.AddrFrom4([4]byte{127, 0, 0, byte(index + 2)})
		conn, err := atomicAggregatePolicyConnect(ctx, address, peer, asn)
		if err != nil {
			return err
		}
		sockets[index] = conn
	}
	stage = "initial End-of-RIB"
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource, "raw-destination", "typed-destination"); err != nil {
		return err
	}
	for _, conn := range sockets {
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return err
		}
		if len(update.NLRI)+len(update.WithdrawnRoutes)+len(update.PathAttributes) != 0 {
			return fmt.Errorf("atomic policy initial sync was not EOR: %+v", update)
		}
	}
	stage = "transport baseline"
	before, err := filterWireTransportsCounts(ctx, plugin)
	if err != nil {
		return err
	}
	rawBefore := rawCalls.Load()
	source, raw, typed := sockets[0], sockets[1], sockets[2]
	a, b, c, fence := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}, []byte{24, 10, 44, 0}, []byte{24, 10, 45, 0}
	stage = "initial marked A"
	if err := atomicAggregatePolicySend(source, a, true, 10); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(raw, a, nil, true, 10); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, a, nil, true, 77); err != nil {
		return err
	}
	stage = "export marker removal of advertised A"
	if err := atomicAggregatePolicySend(source, a, true, 20); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(raw, nil, a, false, 0); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, a, nil, true, 77); err != nil {
		return err
	}
	stage = "fresh marked B refusal and unmarked C control"
	if err := atomicAggregatePolicySend(source, b, true, 30); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, b, nil, true, 77); err != nil {
		return err
	}
	if err := atomicAggregatePolicySend(source, c, false, 30); err != nil {
		return err
	}
	// The B refusal had no ownership: the next raw socket UPDATE must be C.
	if err := atomicAggregatePolicyExpect(raw, []byte{25, 10, 44, 0, 0}, nil, true, 30); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, c, nil, false, 77); err != nil {
		return err
	}
	stage = "import marker removal of advertised A"
	if err := atomicAggregatePolicySend(source, a, true, 90); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, nil, a, false, 0); err != nil {
		return err
	}
	stage = "import refusal socket fence"
	if err := atomicAggregatePolicySend(source, fence, true, 40); err != nil {
		return err
	}
	// The A import refusal cannot create another withdrawal at raw: it no
	// longer owns A. This same-socket fence detects that spurious withdrawal.
	if err := atomicAggregatePolicyExpect(raw, fence, nil, true, 40); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, fence, nil, true, 77); err != nil {
		return err
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	stage = "received RIB after refusals"
	for _, row := range []struct {
		prefix  string
		present bool
	}{
		{"10.42.0.0/24", false}, {"10.43.0.0/24", true}, {"10.44.0.0/24", true}, {"10.45.0.0/24", true},
	} {
		if err := atomicAggregatePolicyRIB(ctx, plugin, row.prefix, row.present); err != nil {
			return err
		}
	}
	stage = "accepted subset and empty subset retirement"
	if err := atomicAggregatePolicySubsetScenario(ctx, plugin, source, raw, typed); err != nil {
		return err
	}
	stage = "transport judgment"
	after, err := filterWireTransportsCounts(ctx, plugin)
	if err != nil {
		return err
	}
	slotRoutes, batchRoutes := after[0]-before[0], after[1]-before[1]
	rawRoutes := float64(rawCalls.Load() - rawBefore)
	if rawRoutes <= 0 {
		return errors.New("atomic policy raw callbacks were never invoked")
	}
	if placement == sysribPlacementInternal {
		if slotRoutes <= 0 {
			return errors.New("atomic policy internal MED filter never used the typed slot")
		}
		if batchRoutes != rawRoutes {
			return fmt.Errorf("atomic policy internal MED filter escaped into encoded transport: batch=%v raw=%v", batchRoutes, rawRoutes)
		}
	} else {
		if slotRoutes != 0 {
			return fmt.Errorf("atomic policy forked MED filter unexpectedly used the typed slot: %v", slotRoutes)
		}
		if batchRoutes <= rawRoutes {
			return fmt.Errorf("atomic policy forked MED filter never used encoded transport: batch=%v raw=%v", batchRoutes, rawRoutes)
		}
	}
	_, err = fmt.Fprintf(os.Stderr, "OK: atomic admission raw refusal withdraws exact source, fresh refusal stays quiet, unmarked rewrite survives, import clears old RIB, and %s typed MED output survives (slot=%v batch=%v raw=%v)\n", placement, slotRoutes, batchRoutes, rawRoutes)
	return err
}

// atomicAggregatePolicySubsetScenario proves the receive store removes omitted
// source keys after an accepted raw subset or EOR. Socket fences require that
// both destinations agree with that store and retain every useful survivor.
func atomicAggregatePolicySubsetScenario(ctx context.Context, plugin *sdk.Plugin, source, raw, typed net.Conn) error {
	removed, kept := []byte{24, 10, 46, 0}, []byte{24, 10, 47, 0}
	pair := append(bytes.Clone(removed), kept...)
	if err := atomicAggregatePolicySend(source, pair, true, 50); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(raw, pair, nil, true, 50); err != nil {
		return err
	}
	if err := atomicAggregatePolicyExpect(typed, pair, nil, true, 77); err != nil {
		return err
	}
	fence := []byte{24, 10, 48, 0}
	if err := atomicAggregatePolicySend(source, pair, true, 60); err != nil {
		return err
	}
	if err := atomicAggregatePolicySend(source, fence, true, 70); err != nil {
		return err
	}
	if err := atomicAggregatePolicyChangesThroughFence(raw, removed, kept, fence, 60, 70); err != nil {
		return fmt.Errorf("accepted subset raw destination: %w", err)
	}
	if err := atomicAggregatePolicyChangesThroughFence(typed, removed, kept, fence, 77, 77); err != nil {
		return fmt.Errorf("accepted subset typed destination: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	if err := atomicAggregatePolicyRIB(ctx, plugin, "10.46.0.0/24", false); err != nil {
		return err
	}
	if err := atomicAggregatePolicyRIB(ctx, plugin, "10.47.0.0/24", true); err != nil {
		return err
	}
	if err := atomicAggregatePolicySend(source, pair, true, 80); err != nil {
		return err
	}
	fence = []byte{24, 10, 49, 0}
	if err := atomicAggregatePolicySend(source, fence, true, 85); err != nil {
		return err
	}
	if err := atomicAggregatePolicyChangesThroughFence(raw, kept, nil, fence, 0, 85); err != nil {
		return fmt.Errorf("empty subset raw destination: %w", err)
	}
	if err := atomicAggregatePolicyChangesThroughFence(typed, kept, nil, fence, 0, 77); err != nil {
		return fmt.Errorf("empty subset typed destination: %w", err)
	}
	if err := quiesce(ctx, plugin); err != nil {
		return err
	}
	for _, prefix := range []string{"10.46.0.0/24", "10.47.0.0/24"} {
		if err := atomicAggregatePolicyRIB(ctx, plugin, prefix, false); err != nil {
			return err
		}
	}
	return nil
}

// atomicAggregatePolicyChangesThroughFence accepts merged or separate removal
// and survivor messages. All required output must be observed within the bounded
// reads, but an unrelated fence need not wait for cold recovery's withdrawal.
func atomicAggregatePolicyChangesThroughFence(conn net.Conn, removed, kept, fence []byte, keptMED, fenceMED uint32) (resultErr error) {
	var observed [8]string
	observedCount := 0
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("expected removed=%x kept=%x fence=%x; observed UPDATEs=%v: %w", removed, kept, fence, observed[:observedCount], resultErr)
		}
	}()
	sawRemoval := false
	sawFence := false
	// No kept key is expected for an empty raw subset.
	sawKept := len(kept) == 0
	for range 4 {
		update, err := atomicAggregatePolicyRead(conn)
		if err != nil {
			return err
		}
		if observedCount < len(observed) {
			withdrawn, announced, attrs := update.WithdrawnRoutes, update.NLRI, update.PathAttributes
			observed[observedCount] = fmt.Sprintf("withdrawn(%d)=%x nlri(%d)=%x attrs(%d)=%x", len(withdrawn), withdrawn[:min(32, len(withdrawn))], len(announced), announced[:min(32, len(announced))], len(attrs), attrs[:min(32, len(attrs))])
			observedCount++
		}
		if len(update.WithdrawnRoutes) != 0 {
			if !bytes.Equal(update.WithdrawnRoutes, removed) {
				return fmt.Errorf("atomic policy subset withdrawal=%x, want one removal of %x", update.WithdrawnRoutes, removed)
			}
			if sawRemoval {
				return fmt.Errorf("atomic policy subset withdrawal=%x, want one removal of %x", update.WithdrawnRoutes, removed)
			}
			sawRemoval = true
		}
		if len(update.NLRI) > 0 {
			isFence := bytes.Equal(update.NLRI, fence)
			metric := keptMED
			if isFence {
				if sawFence {
					return fmt.Errorf("atomic policy subset repeated fence=%x", fence)
				}
				metric = fenceMED
			} else {
				if !bytes.Equal(update.NLRI, kept) {
					return fmt.Errorf("atomic policy subset announced=%x, want one retained key %x or fence %x", update.NLRI, kept, fence)
				}
				if sawKept {
					return fmt.Errorf("atomic policy subset announced=%x, want one retained key %x or fence %x", update.NLRI, kept, fence)
				}
				sawKept = true
			}
			_, _, _, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
			if !marked {
				return errors.New("atomic policy subset survivor lost its marker")
			}
			_, _, med, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMED)
			if !present {
				return errors.New("atomic policy subset survivor MED absent")
			}
			if len(med) != 4 {
				return fmt.Errorf("atomic policy subset survivor MED malformed or absent: %x", med)
			}
			if binary.BigEndian.Uint32(med) != metric {
				return fmt.Errorf("atomic policy subset survivor MED=%d, want %d", binary.BigEndian.Uint32(med), metric)
			}
			if isFence {
				sawFence = true
			}
		}
		if sawRemoval && sawKept && sawFence {
			return nil
		}
	}
	return fmt.Errorf("atomic policy subset incomplete after four UPDATEs: removed=%t kept=%t fence=%t", sawRemoval, sawKept, sawFence)
}

// atomicAggregatePolicyConnect returns one established socket. The caller MUST
// keep it open through the final assertions and MUST close it afterward, as
// atomicAggregatePolicyScenario does; this function closes a failed handshake.
func atomicAggregatePolicyConnect(ctx context.Context, address string, source netip.Addr, asn uint16) (net.Conn, error) {
	dialer := net.Dialer{Timeout: time.Second, LocalAddr: net.TCPAddrFromAddrPort(netip.AddrPortFrom(source, 0))}
	var conn net.Conn
	var dialErr error
	if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
		conn, dialErr = dialer.DialContext(ctx, "tcp4", address)
		// refuse-by-default: guards Poll -- false retries readiness; dialErr is retained and returned if the barrier fails.
		return dialErr == nil
	}) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("atomic policy listener %s did not accept %s: %w", address, source, dialErr)
	}
	failed := true
	defer func() {
		if failed {
			_ = conn.Close()
		}
	}() //nolint:errcheck // Failed handshake owns this socket.
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	identifier := source.As4()
	open := &message.Open{Version: 4, MyAS: asn, HoldTime: 0, BGPIdentifier: binary.BigEndian.Uint32(identifier[:])}
	if _, err := conn.Write(message.PackTo(open, nil)); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 1); err != nil {
		return nil, err
	}
	if err := delayOpenFixtureRead(conn, 4); err != nil {
		return nil, err
	}
	if _, err := conn.Write(message.PackTo(&message.Keepalive{}, nil)); err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	failed = false
	return conn, nil
}

func atomicAggregatePolicySend(conn net.Conn, prefix []byte, marker bool, med uint32) error {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 1, 0x80, 4, 4}
	// Changing ORIGIN makes the second useful typed A generation observable
	// even though the destination's MED rewrite makes both metrics 77.
	if med == 20 {
		attrs[3] = 2
	}
	// The subset scenario also needs an observable typed survivor after both
	// source generations have their different metrics rewritten to MED77.
	if med == 60 {
		attrs[3] = 2
	}
	attrs = binary.BigEndian.AppendUint32(attrs, med)
	if marker {
		attrs = append(attrs, 0x40, 6, 0)
	}
	update := &message.Update{PathAttributes: attrs, NLRI: prefix}
	packet := message.PackTo(update, nil)
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return conn.SetWriteDeadline(time.Time{})
}

func atomicAggregatePolicyRead(conn net.Conn) (*message.Update, error) {
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, err
	}
	var header [message.HeaderLen]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint16(header[16:18]))
	if size < message.HeaderLen {
		return nil, fmt.Errorf("atomic policy invalid frame size %d", size)
	}
	if size > message.MaxMsgLen {
		return nil, fmt.Errorf("atomic policy invalid frame size %d", size)
	}
	if header[18] != 2 {
		return nil, fmt.Errorf("atomic policy expected UPDATE, got type %d", header[18])
	}
	body := make([]byte, size-message.HeaderLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return message.UnpackUpdate(body)
}

func atomicAggregatePolicyExpect(conn net.Conn, announced, withdrawn []byte, marker bool, med uint32) error {
	update, err := atomicAggregatePolicyRead(conn)
	if err != nil {
		return err
	}
	if !bytes.Equal(update.NLRI, announced) {
		return fmt.Errorf("atomic policy wire NLRI=%x withdrawn=%x; want NLRI=%x withdrawn=%x", update.NLRI, update.WithdrawnRoutes, announced, withdrawn)
	}
	if !bytes.Equal(update.WithdrawnRoutes, withdrawn) {
		return fmt.Errorf("atomic policy wire NLRI=%x withdrawn=%x; want NLRI=%x withdrawn=%x", update.NLRI, update.WithdrawnRoutes, announced, withdrawn)
	}
	_, _, _, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
	if marked != marker {
		return fmt.Errorf("atomic policy wire marker=%t, want %t", marked, marker)
	}
	if len(announced) == 0 {
		return nil
	}
	_, _, metric, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMED)
	if !present {
		return errors.New("atomic policy wire MED absent")
	}
	if len(metric) != 4 {
		return fmt.Errorf("atomic policy wire malformed MED=%x", metric)
	}
	if binary.BigEndian.Uint32(metric) != med {
		return fmt.Errorf("atomic policy wire MED=%d, want %d", binary.BigEndian.Uint32(metric), med)
	}
	return nil
}

func atomicAggregatePolicyBody(withdrawn, attrs, announced []byte) []byte {
	body := binary.BigEndian.AppendUint16(nil, uint16(len(withdrawn)))
	body = append(body, withdrawn...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	return append(body, announced...)
}

func atomicAggregatePolicyRIB(ctx context.Context, plugin *sdk.Plugin, prefix string, present bool) error {
	var stored struct {
		Peers map[string][]struct {
			Key string `json:"key"`
		} `json:"adj-rib-in"`
	}
	status, err := Dispatch(ctx, plugin, "show bgp adj-rib-in", &stored)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("atomic policy Adj-RIB-In status=%s", status)
	}
	found := false
	for _, route := range stored.Peers[addrLoopbackSecond] {
		if strings.HasSuffix(route.Key, ":"+prefix) {
			found = true
		}
	}
	if found != present {
		return fmt.Errorf("atomic policy received %s present=%t, want %t: %+v", prefix, found, present, stored)
	}
	return nil
}
