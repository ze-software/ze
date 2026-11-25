// Design: docs/architecture/bgp/egress-attribute-rules.md -- ordinary selected-route MED preservation before iBGP readvertisement.
// Related: register_med_ibgp_ordinary.go -- fixture registration.
// Related: bgp_best_route_transit.go -- real TCP socket harness without bgp-rs.
// Related: plugin_fixture_09_bgp.go -- route-server raw MED drop fixture.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var medOrdinaryCommunity = []byte{0xfd, 0xe8, 0x12, 0x34}

// medOrdinaryHarness owns the filter counters and the single observer worker
// goroutine started on OnAllPluginsReady and stopped when runObserver returns
// and requests daemon shutdown.
// Safe for concurrent use: filter callbacks and the observer synchronize on mu.
type medOrdinaryHarness struct {
	listenAddr string
	mu         sync.Mutex
	removed    int
	rewritten  int
}

func (h *medOrdinaryHarness) onFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	if route.Direction() != directionExport {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	if len(route.Raw()) == 0 {
		return nil, errors.New("RAW-MED-DROP: export callback had no raw body")
	}
	body, droppedMED, err := stripMEDAndSetCommunity(route.Raw(), medOrdinaryCommunity)
	if err != nil {
		return nil, fmt.Errorf("RAW-MED-DROP: %w", err)
	}
	h.mu.Lock()
	h.rewritten++
	if droppedMED {
		h.removed++
	}
	h.mu.Unlock()
	if droppedMED {
		var tb textbuf.Buffer
		tb.Str("RAW-MED-DROP: removed MULTI_EXIT_DISC for ").Str(route.Peer()).Byte('\n')
		_, _ = os.Stderr.WriteString(tb.String())
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: body}, nil
}

func (h *medOrdinaryHarness) runObserver(ctx context.Context, plugin *sdk.Plugin, result chan<- error) {
	scenarioErr := h.runScenario(ctx, plugin)
	if scenarioErr == nil {
		scenarioErr = plugin01Quiesce(ctx, plugin)
	}
	if scenarioErr == nil {
		_, _ = os.Stderr.WriteString("OK: ordinary selected-route iBGP MED preserved alongside raw community rewrite and no-MED control\n")
	}
	ReportFailure(scenarioErr)
	result <- scenarioErr
	_, _, _ = plugin.DispatchCommand(context.Background(), "request shutdown")
}

// medIBGPOrdinaryPostSelectionRemoval runs both the raw export filter (dropping
// MED and rewriting COMMUNITY) and the real-TCP observer over an explicitly
// attached bgp-rib without bgp-rs.
func medIBGPOrdinaryPostSelectionRemoval(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("med-ibgp-ordinary-post-selection-removal requires <BGP listen address>")
	}
	if args[0] == "" {
		return errors.New("med-ibgp-ordinary-post-selection-removal requires a non-empty <BGP listen address>")
	}

	plugin, err := newObserver("raw-med-drop")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown

	h := &medOrdinaryHarness{listenAddr: args[0]}
	plugin.OnFilter(h.onFilter)

	result := make(chan error, 1)
	started := make(chan struct{})
	plugin.OnAllPluginsReady(func() error {
		close(started)
		go h.runObserver(ctx, plugin, result)
		return nil
	})

	reg := &sdk.Registration{
		Filters: []sdk.FilterDecl{{
			Name:      "drop-med-raw",
			Direction: sdk.FilterExport,
			Reads:     []string{sdk.FilterFieldRaw},
			OnError:   sdk.OnErrorReject,
		}},
	}
	runErr := runObserver(ctx, plugin, reg)
	return awaitObserverResult(started, result, runErr)
}

func (h *medOrdinaryHarness) runScenario(ctx context.Context, plugin *sdk.Plugin) error {
	source, target, err := bgpBestRouteConnect(ctx, plugin, h.listenAddr, 65001, 65000)
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck // fixture teardown
	defer target.Close() //nolint:errcheck // fixture teardown

	// 1. Subject route with MED 100 from eBGP source (65001 -> 65000).
	subjectNLRI := []byte{24, 10, 63, 0}
	if err := medOrdinarySendEBGP(source, subjectNLRI, true, 100); err != nil {
		return fmt.Errorf("send subject route: %w", err)
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.63.0.0/24", addrLoopbackSecond); err != nil {
		return fmt.Errorf("subject route did not become best: %w", err)
	}
	subjectUpdate, err := atomicAggregatePolicyRead(target)
	if err != nil {
		return fmt.Errorf("read subject UPDATE on iBGP destination: %w", err)
	}
	if !bytes.Equal(subjectUpdate.NLRI, subjectNLRI) {
		return fmt.Errorf("subject wire NLRI=%x, want %x", subjectUpdate.NLRI, subjectNLRI)
	}
	if len(subjectUpdate.WithdrawnRoutes) != 0 {
		return fmt.Errorf("subject wire carried unexpected withdrawals: %x", subjectUpdate.WithdrawnRoutes)
	}
	if err := medOrdinaryRequireCommunity(subjectUpdate.PathAttributes, medOrdinaryCommunity); err != nil {
		return fmt.Errorf("subject community marker: %w", err)
	}
	_, _, medVal, hasMED := attribute.AttrFind(subjectUpdate.PathAttributes, attribute.AttrMED)
	if !hasMED {
		return errors.New("subject UPDATE to iBGP destination lost MULTI_EXIT_DISC after raw export removal")
	}
	if len(medVal) != 4 {
		return fmt.Errorf("subject UPDATE MULTI_EXIT_DISC length=%d, want 4", len(medVal))
	}
	if binary.BigEndian.Uint32(medVal) != 100 {
		return fmt.Errorf("subject UPDATE MULTI_EXIT_DISC=%x, want 100 (00000064)", medVal)
	}

	// 2. Distinct control route WITHOUT MED from the same eBGP source.
	controlNLRI := []byte{24, 10, 63, 1}
	if err := medOrdinarySendEBGP(source, controlNLRI, false, 0); err != nil {
		return fmt.Errorf("send no-MED control route: %w", err)
	}
	if err := rfc8212ReloadBest(ctx, plugin, "10.63.1.0/24", addrLoopbackSecond); err != nil {
		return fmt.Errorf("control route did not become best: %w", err)
	}
	controlUpdate, err := atomicAggregatePolicyRead(target)
	if err != nil {
		return fmt.Errorf("read control UPDATE on iBGP destination: %w", err)
	}
	if !bytes.Equal(controlUpdate.NLRI, controlNLRI) {
		return fmt.Errorf("control wire NLRI=%x, want %x", controlUpdate.NLRI, controlNLRI)
	}
	if len(controlUpdate.WithdrawnRoutes) != 0 {
		return fmt.Errorf("control wire carried unexpected withdrawals: %x", controlUpdate.WithdrawnRoutes)
	}
	if err := medOrdinaryRequireCommunity(controlUpdate.PathAttributes, medOrdinaryCommunity); err != nil {
		return fmt.Errorf("control community marker: %w", err)
	}
	if _, _, _, controlHasMED := attribute.AttrFind(controlUpdate.PathAttributes, attribute.AttrMED); controlHasMED {
		return errors.New("no-MED control route unexpectedly gained MULTI_EXIT_DISC on iBGP destination")
	}

	h.mu.Lock()
	gotRemoved, gotRewritten := h.removed, h.rewritten
	h.mu.Unlock()
	if gotRemoved < 1 {
		return fmt.Errorf("raw export filter removed MED %d times, want >= 1", gotRemoved)
	}
	if gotRewritten < 2 {
		return fmt.Errorf("raw export filter rewrote %d routes, want >= 2", gotRewritten)
	}
	return policyFinalSessions(ctx, plugin, 2)
}

func medOrdinarySendEBGP(conn net.Conn, nlri []byte, withMED bool, med uint32) error {
	// ORIGIN=IGP, AS_PATH=[65001] (2-byte, matching atomicAggregatePolicyConnect), NEXT_HOP=192.0.2.1.
	attrs := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 4, 2, 1, 0xfd, 0xe9,
		0x40, 3, 4, 192, 0, 2, 1,
	}
	if withMED {
		attrs = append(attrs, 0x80, 4, 4)
		attrs = binary.BigEndian.AppendUint32(attrs, med)
	}
	packet := message.PackTo(&message.Update{PathAttributes: attrs, NLRI: nlri}, nil)
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

func medOrdinaryRequireCommunity(attrs, want []byte) error {
	_, _, got, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	if !found {
		return errors.New("raw export filter community marker is absent")
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("community=%x, want %x", got, want)
	}
	return nil
}

// stripMEDAndSetCommunity parses an UPDATE body, drops MULTI_EXIT_DISC (code 4)
// if present, and writes COMMUNITY (code 8) with value comm in ascending
// attribute type-code order.
func stripMEDAndSetCommunity(body, comm []byte) ([]byte, bool, error) {
	if len(body) < 4 {
		return nil, false, errors.New("raw UPDATE body too short")
	}
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	attrLenOff := 2 + withdrawnLen
	if attrLenOff+2 > len(body) {
		return nil, false, errors.New("withdrawn section overruns body")
	}
	attrLen := int(binary.BigEndian.Uint16(body[attrLenOff : attrLenOff+2]))
	attrStart := attrLenOff + 2
	attrEnd := attrStart + attrLen
	if attrEnd > len(body) {
		return nil, false, errors.New("attribute section overruns body")
	}

	commAttr := make([]byte, 0, 3+len(comm))
	commAttr = append(commAttr, 0xC0, byte(attribute.AttrCommunity), byte(len(comm)))
	commAttr = append(commAttr, comm...)

	outAttrs := make([]byte, 0, attrLen+len(commAttr))
	removedMED := false
	insertedComm := false
	for pos := attrStart; pos < attrEnd; {
		if pos+3 > attrEnd {
			return nil, false, errors.New("truncated attribute header")
		}
		flags := body[pos]
		code := body[pos+1]
		hdrLen := 3
		valLen := int(body[pos+2])
		if flags&0x10 != 0 {
			if pos+4 > attrEnd {
				return nil, false, errors.New("truncated extended attribute header")
			}
			hdrLen = 4
			valLen = int(binary.BigEndian.Uint16(body[pos+2 : pos+4]))
		}
		end := pos + hdrLen + valLen
		if end > attrEnd {
			return nil, false, errors.New("attribute value overruns section")
		}
		if code == byte(attribute.AttrMED) {
			removedMED = true
			pos = end
			continue
		}
		if code == byte(attribute.AttrCommunity) {
			pos = end
			continue
		}
		if !insertedComm && code > byte(attribute.AttrCommunity) {
			outAttrs = append(outAttrs, commAttr...)
			insertedComm = true
		}
		outAttrs = append(outAttrs, body[pos:end]...)
		pos = end
	}
	if !insertedComm {
		outAttrs = append(outAttrs, commAttr...)
	}

	result := make([]byte, 0, attrStart+len(outAttrs)+(len(body)-attrEnd))
	result = append(result, body[:attrLenOff]...)
	result = binary.BigEndian.AppendUint16(result, uint16(len(outAttrs))) //nolint:gosec // bounded by UPDATE max
	result = append(result, outAttrs...)
	result = append(result, body[attrEnd:]...)
	return result, removedMED, nil
}
