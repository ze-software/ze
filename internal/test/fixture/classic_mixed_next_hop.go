// Design: docs/architecture/bgp/structural-forwarding.md -- classic presence after raw export policy.
// Related: classic_mixed_next_hop_wire.go -- independent recipient socket oracle.
// Related: register_classic_mixed_next_hop.go -- compiled fixture registration.
package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const classicMixedNextHopRailCached = "cached" // The forwarding rail driven through ForwardCached.

var classicMixedNextHopPrefix = [4]byte{24, 10, 99, 0}

// Only this scenario's observer and SDK callback share the receipts. The mutex
// protects their three-element history; the filter counters are atomic because
// the scenario waits for actual export judgments before its writer fence.
type classicMixedNextHopState struct {
	mu       sync.Mutex
	receipts []bgptypes.UpdateReceipt
	calls    [4]atomic.Uint32
}

// This driver proves the actual configured raw-export path over daemon sockets.
// Fresh refused classic routes have no owner and emit no withdrawal. Seeded
// classic withdrawal/recovery is a distinct recording-writer unit assertion.
func classicMixedNextHopDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("classic-mixed-next-hop takes <cached|native> <listen-address>")
	}
	if args[0] != classicMixedNextHopRailCached {
		if args[0] != "native" {
			return fmt.Errorf("unknown classic mixed forwarding rail %q", args[0])
		}
	}
	state := new(classicMixedNextHopState)
	registration := &sdk.Registration{CacheConsumer: args[0] == classicMixedNextHopRailCached, Filters: []sdk.FilterDecl{{
		Name: "rewrite", Direction: sdk.FilterExport, Reads: []string{sdk.FilterFieldRaw}, OnError: sdk.OnErrorReject,
	}}}
	setup := func(plugin *sdk.Plugin) error {
		plugin.SetStartupSubscriptions([]string{eventUpdate}, []string{addrLoopbackSecond}, "parsed")
		plugin.SetEnvelope(true)
		plugin.OnEvent(state.receive)
		plugin.OnFilter(state.rewrite)
		return nil
	}
	return observeConfigured(ctx, "classic-mixed-nh", registration, setup, func(ctx context.Context, plugin *sdk.Plugin) error {
		return state.run(ctx, plugin, args[0], args[1])
	})
}

func (state *classicMixedNextHopState) receive(raw string) error {
	envelope, err := rpc.ParseEventEnvelope(raw)
	if err != nil {
		return err
	}
	if envelope.Namespace != namespaceBGP {
		return nil
	}
	if envelope.Event != eventUpdate {
		return nil
	}
	var payload struct {
		BGP struct {
			PeerOwner     uint64 `json:"peer-owner"`
			Epoch         uint64 `json:"source-policy-epoch"`
			SourceReceipt uint64 `json:"source-receipt"`
			Message       struct {
				ID        uint64 `json:"id"`
				Direction string `json:"direction"`
			} `json:"message"`
			Peer struct {
				Remote struct {
					Address string `json:"address"`
				} `json:"remote"`
			} `json:"peer"`
			Update map[string]json.RawMessage `json:"update"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return err
	}
	if payload.BGP.Message.Direction != directionReceived {
		return nil
	}
	if payload.BGP.Peer.Remote.Address != addrLoopbackSecond {
		return nil
	}
	if _, eor := payload.BGP.Update["eor"]; eor {
		return nil
	}
	receipt, err := bgptypes.ParseUpdateReceipt(envelope.Metadata)
	if err != nil {
		return err
	}
	if err := receipt.Validate(payload.BGP.Peer.Remote.Address, payload.BGP.PeerOwner, payload.BGP.Message.ID, payload.BGP.Epoch, payload.BGP.SourceReceipt); err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.receipts) == 3 {
		return errors.New("classic mixed source emitted more than its three subject generations")
	}
	state.receipts = append(state.receipts, receipt)
	return nil
}

func (state *classicMixedNextHopState) rewrite(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	body := route.Raw()
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return nil, err
	}
	if len(sections.NLRI(body)) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	if !bytes.Equal(sections.NLRI(body), classicMixedNextHopPrefix[:]) {
		return nil, fmt.Errorf("unexpected classic input %x", sections.NLRI(body))
	}
	attrs := sections.Attrs(body)
	_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
	if !found {
		return nil, errors.New("classic mixed source omitted MED phase")
	}
	if len(med) != 4 {
		return nil, errors.New("classic mixed MED phase is not four octets")
	}
	phase := binary.BigEndian.Uint32(med)
	if phase < 1 {
		return nil, fmt.Errorf("invalid classic mixed phase %d", phase)
	}
	if phase > 3 {
		return nil, fmt.Errorf("invalid classic mixed phase %d", phase)
	}
	state.calls[phase].Add(1)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: classicMixedNextHopBody(attrs, byte(phase))}, nil
}

// Preserve ORIGIN/AS_PATH/MED exactly and omit only code 3. The two surrounding
// MP-only generations bound the harmful middle generation on the same sockets.
func classicMixedNextHopBody(attrs []byte, phase byte) []byte {
	var kept []byte
	iterator := attribute.NewAttrIterator(attrs)
	for { // The source attribute section bounds this walk.
		start := iterator.Offset()
		code, _, _, found := iterator.Next()
		if !found {
			break
		}
		if code != attribute.AttrNextHop {
			kept = append(kept, attrs[start:iterator.Offset()]...)
		}
	}
	flow := classicMixedNextHopFlow(phase)
	kept = append(kept, 0x80, 14, byte(len(flow)))
	kept = append(kept, flow...)
	var classic []byte
	if phase == 2 {
		classic = classicMixedNextHopPrefix[:]
	}
	return atomicAggregatePolicyBody(nil, kept, classic)
}

// The literal RFC8955 destination-prefix rule is six octets including its
// one-octet rule length. AFI1/SAFI133 carries a zero next-hop length and reserved0.
func classicMixedNextHopFlow(phase byte) []byte {
	return []byte{0, 1, 133, 0, 0, 5, 1, 24, 198, 51, phase}
}

func (state *classicMixedNextHopState) run(ctx context.Context, plugin *sdk.Plugin, rail, address string) error {
	var sockets [3]net.Conn
	defer func() {
		for _, conn := range sockets {
			if conn != nil {
				_ = conn.Close()
			} //nolint:errcheck // Scenario outcome already carries the useful failure.
		}
	}()
	for index, asn := range []uint16{65001, 65000, 65000} {
		peer := netip.AddrFrom4([4]byte{127, 0, 0, byte(index + 2)})
		conn, err := classicMixedNextHopConnect(ctx, address, peer, asn)
		if err != nil {
			return err
		}
		sockets[index] = conn
	}
	if err := p12WaitPeersEOR(ctx, plugin, peerNameSource, "harmful", "useful"); err != nil {
		return err
	}
	for _, conn := range sockets {
		if err := classicMixedNextHopInitial(conn); err != nil {
			return err
		}
	}
	for phase := byte(1); phase <= 3; phase++ {
		if err := atomicAggregatePolicySend(sockets[0], classicMixedNextHopPrefix[:], false, uint32(phase)); err != nil {
			return err
		}
		if !Poll(ctx, 80, 100*time.Millisecond, func() bool {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.receipts) == int(phase)
		}) {
			return fmt.Errorf("%s source receipt %d never arrived", rail, phase)
		}
		state.mu.Lock()
		receipt := state.receipts[phase-1]
		state.mu.Unlock()
		if receipt.ReactorForwarded != (rail == "native") {
			return fmt.Errorf("%s phase%d reactor-forwarded=%t", rail, phase, receipt.ReactorForwarded)
		}
		if rail == classicMixedNextHopRailCached {
			if err := plugin.ForwardCached(ctx, rpc.ForwardCachedInput{IDs: []uint64{receipt.MessageID}, Destinations: []string{addrLoopbackThird, addrLoopbackFourth}}); err != nil {
				return err
			}
		}
		if !Poll(ctx, 80, 100*time.Millisecond, func() bool { return state.calls[phase].Load() >= 2 }) {
			return fmt.Errorf("%s phase%d did not reach both export judgments", rail, phase)
		}
		if err := quiesce(ctx, plugin); err != nil {
			return err
		}
		for index, peer := range []string{addrLoopbackThird, addrLoopbackFourth} {
			if err := fixture10Call(ctx, plugin, "send bgp "+peer+" update text nlri eor").requireDone("classic mixed socket fence"); err != nil {
				return err
			}
			if err := classicMixedNextHopReadPhase(sockets[index+1], phase, index == 1); err != nil {
				return fmt.Errorf("%s phase%d peer%s: %w", rail, phase, peer, err)
			}
		}
	}
	if err := policyFinalSessions(ctx, plugin, 3); err != nil {
		return err
	}
	filterSubjectSaid("OK: classic mixed missing-code3 "+rail+" refusal and useful sibling/repair recipients verified", nil)
	return nil
}
