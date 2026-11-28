// Design: docs/architecture/exabgp-bridge.md -- end-to-end wiring test for base64 UPDATE delivery.
// Related: wrapper_test.go -- ExaBGP wrapper unit tests.
// Related: bridgerun/forms.go, bridgerun/fleet.go -- EventFormat and Fleet.Broadcast.

//go:build ze_bgp

package exabgp

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/exabgp/bridge"
	"github.com/ze-software/ze/internal/plugins/exabgp/bridgerun"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExabgpBase64EndToEnd exercises the full base64 wiring path from
// bridgerun.EventFormat through format.AppendMessage, MuxConn.CallBatchRPC,
// Fleet.Broadcast, and script stdin delivery for a received IPv4 unicast UPDATE.
//
// VALIDATES: Wiring Test (AC-1, AC-3, AC-4, AC-5) -- Stage 5 format selection,
// positional `upd ` line formatting, batched MuxConn transport, Fleet base64
// dispatch, and ExaBGP JSON output on script stdin.
// PREVENTS: disconnected format selection, unimplemented formatter or bridge
// dispatch, or broken MuxConn batch delivery along the base64 transport path.
func TestExabgpBase64EndToEnd(t *testing.T) {
	filter := &rpc.BusParticipantFilter{
		ReceiveRules: []rpc.BusFilterRule{{
			Message:   "bgp:update",
			Allowed:   true,
			TargetAll: true,
		}},
	}
	fmtMode := bridgerun.EventFormat(filter, bridgerun.FormSpecs{})
	require.Equal(t, plugin.FormatBase64, fmtMode)

	// IPv4 unicast UPDATE announcing 192.0.2.0/24 with ORIGIN=IGP,
	// 4-byte AS_PATH=[65001], and NEXT_HOP=10.0.0.1.
	updateBody := []byte{
		0x00, 0x00, // Withdrawn Routes Length = 0
		0x00, 0x14, // Total Path Attribute Length = 20
		0x40, 0x01, 0x01, 0x00, // ORIGIN: IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9, // AS_PATH: AS_SEQUENCE [65001]
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP: 10.0.0.1
		0x18, 0xc0, 0x00, 0x02, // NLRI: 192.0.2.0/24
	}
	var wu wireu.WireUpdate
	wireu.InitWireUpdate(&wu, updateBody, bgpctx.APIContextID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)

	peerAddr := netip.MustParseAddr("10.0.0.1")
	localAddr := netip.MustParseAddr("10.0.0.2")
	peer := &plugin.PeerInfo{
		Address:         peerAddr,
		LocalAddress:    localAddr,
		AddressStr:      peerAddr.String(),
		LocalAddressStr: localAddr.String(),
		LocalAS:         65002,
		PeerAS:          rpc.RemoteASOf(65001),
		RouterID:        0x0a000002,
	}
	msg := bgptypes.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   updateBody,
		Direction:  rpc.DirectionReceived,
		WireUpdate: &wu,
		AttrsWire:  attrs,
	}

	rawLine := string(format.AppendMessage(nil, peer, msg, bgptypes.ContentConfig{
		Encoding: plugin.EncodingJSON,
		Format:   fmtMode,
	}))
	rawLine = strings.TrimSuffix(rawLine, "\n")
	require.True(t, strings.HasPrefix(rawLine, "upd "), "base64 output must start with 'upd ', got %q", rawLine)

	// Deliver the base64 line over a MuxConn CallBatchRPC round-trip.
	engineEnd, pluginEnd := net.Pipe()
	t.Cleanup(func() {
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
	})
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		_ = engineMux.Close()
		_ = pluginMux.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deliveredCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		req, ok := <-pluginMux.Requests()
		if !ok {
			errCh <- context.Canceled
			return
		}
		batchEvents, parseErr := rpc.ParseBatchEvents(req.Params)
		if parseErr != nil {
			errCh <- parseErr
			return
		}
		if sendErr := pluginMux.SendOK(ctx, req.ID); sendErr != nil {
			errCh <- sendErr
			return
		}
		var eventStr string
		if unmarshalErr := json.Unmarshal(batchEvents[0], &eventStr); unmarshalErr != nil {
			errCh <- unmarshalErr
			return
		}
		deliveredCh <- eventStr
	}()

	quotedEvent, err := json.Marshal(rawLine)
	require.NoError(t, err)
	_, err = engineMux.CallBatchRPC(ctx, [][]byte{quotedEvent})
	require.NoError(t, err)

	var delivered string
	select {
	case recvErr := <-errCh:
		t.Fatalf("plugin mux batch receive failed: %v", recvErr)
	case delivered = <-deliveredCh:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for batched base64 event: %v", ctx.Err())
	}

	// Feed the delivered base64 line into a real Fleet running a capture script.
	dir := t.TempDir()
	outFile := filepath.Join(dir, "script-stdin.jsonl")
	scriptPath := filepath.Join(dir, "capture.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat > "+outFile+"\n"), 0o700))

	fleet := bridgerun.New(slogutil.DiscardLogger(), []string{"ipv4/unicast"}, []bridgerun.Script{{
		Name:    "capture",
		Argv:    []string{scriptPath},
		Encoder: bridge.EncoderJSON,
	}})
	require.NoError(t, fleet.Start(ctx, nil))
	fleet.Broadcast(delivered)
	fleet.Shutdown()
	cancel()
	fleet.Stop()

	outBytes, err := os.ReadFile(outFile) //nolint:gosec // G304: test-owned temp file.
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(outBytes)), "\n")
	require.NotEmpty(t, lines, "capture script received no lines")

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &doc), "first line is not valid JSON: %q", lines[0])
	assert.Equal(t, "update", doc["type"])

	neighbor, ok := doc["neighbor"].(map[string]any)
	require.True(t, ok, "missing neighbor object in %s", lines[0])
	address, ok := neighbor["address"].(map[string]any)
	require.True(t, ok, "missing neighbor.address object in %s", lines[0])
	assert.Equal(t, "10.0.0.1", address["peer"])

	message, ok := neighbor["message"].(map[string]any)
	require.True(t, ok, "missing neighbor.message object in %s", lines[0])
	update, ok := message["update"].(map[string]any)
	require.True(t, ok, "missing neighbor.message.update object in %s", lines[0])
	attrsMap, ok := update["attribute"].(map[string]any)
	require.True(t, ok, "missing attribute map in %s", lines[0])
	assert.Equal(t, "igp", attrsMap["origin"])
	announce, ok := update["announce"].(map[string]any)
	require.True(t, ok, "missing announce map in %s", lines[0])
	ipv4Uni, ok := announce["ipv4 unicast"].(map[string]any)
	require.True(t, ok, "missing ipv4 unicast map in %s", lines[0])
	nh, ok := ipv4Uni["10.0.0.1"].([]any)
	require.True(t, ok, "missing next-hop 10.0.0.1 list in %s", lines[0])
	require.Len(t, nh, 1)
	nlriEntry, ok := nh[0].(map[string]any)
	require.True(t, ok, "unexpected NLRI entry type in %s", lines[0])
	assert.Equal(t, "192.0.2.0/24", nlriEntry["nlri"])
}
