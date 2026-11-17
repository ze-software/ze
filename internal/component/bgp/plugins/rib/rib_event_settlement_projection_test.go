// Design: docs/architecture/plugin/rib-storage-design.md -- bounded settlement failures.
// Related: rib_event_settlement_test.go -- real RIB, SDK and manager fixture.
package rib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestQuiesceFailureProjectsLargeDestinationSet drives the actual SDK receipt
// boundary with more live held writers than its 4096-byte failure field fits.
// The native fixture accepts arbitrary admitted destinations; its healthy writer
// proves the large held set neither blocks another destination nor loses work.
func TestQuiesceFailureProjectsLargeDestinationSet(t *testing.T) {
	// These 160 whole identity lines alone occupy 5797 bytes, so even the
	// bounded name-only projection must honestly omit some identities. The
	// first 96 identities remain the original oversized-cause counterexample.
	const heldCount = 160
	ctx := selectedDispatchContext(t)
	peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.181"), netip.MustParseAddr("192.0.2.182")}
	entered := make(chan struct{}, heldCount)
	healthy := make(chan rpc.SelectRoutesInput, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.Destination == peers[1].String() {
			healthy <- input
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		}
		entered <- struct{}{}
		select {
		case <-release:
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	manager, r := startQuiesceRIB(t, ctx, true, peers, send)
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	route := selectedDispatchRoute(90, 0x7790)
	require.NoError(t, r.selectedDispatch.publish(route))
	held := make([]netip.Addr, heldCount)
	for i := range held {
		held[i] = netip.MustParseAddr(fmt.Sprintf("2001:db8::%x", i+1))
		require.NoError(t, r.selectedDispatch.destinationUp(held[i], uint64(i+1)))
	}
	for range heldCount {
		selectedDispatchAwait(t, ctx, entered)
	}
	require.NoError(t, r.selectedDispatch.destinationUp(peers[1], 182))
	require.NoError(t, r.selectedDispatch.barrier(ctx, peers[1]))
	written := selectedDispatchAwait(t, ctx, healthy)
	require.Equal(t, route.keyNLRI, written.Routes[0].KeyNLRI)

	blocked, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	err := manager.DrainEventDelivery(blocked)
	cancel()
	require.Error(t, err)
	require.ErrorContains(t, err, "destination "+held[0].String()+" Session 1")
	require.ErrorContains(t, err, "fix: ze show bgp rib status")
	require.NotContains(t, err.Error(), "event settlement failure exceeds")
	shown := 0
	for i, peer := range held {
		if strings.Contains(err.Error(), fmt.Sprintf("destination %s Session %d", peer, i+1)) {
			shown++
		}
	}
	omission := regexp.MustCompile(`(\d+) additional destination identities omitted`).FindStringSubmatch(err.Error())
	require.Len(t, omission, 2, "a bounded failure must state how many live identities it omitted")
	omitted, parseErr := strconv.Atoi(omission[1])
	require.NoError(t, parseErr)
	require.Equal(t, heldCount-shown, omitted)
	require.Positive(t, omitted)

	// Exercise the existing status command named by the failure while every
	// writer remains held. Ordinary convergence remains neutral in doctor.
	connection := manager.GetProcess("bgp-rib").Conn()
	status, statusErr := connection.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Serial: "held-selected-status", Command: "show bgp rib status"})
	require.NoError(t, statusErr)
	require.Equal(t, statusDone, status.Status)
	var document struct {
		SelectedOutput []struct {
			Destination netip.Addr `json:"destination"`
			Session     uint64     `json:"session"`
			InFlight    int        `json:"in-flight"`
			Failure     string     `json:"failure"`
		} `json:"selected-output"`
	}
	require.NoError(t, json.Unmarshal(status.Data, &document))
	require.Len(t, document.SelectedOutput, heldCount)
	want := make([]selectedDestinationID, heldCount)
	for i, peer := range held {
		want[i] = selectedDestinationID{peer: peer, session: uint64(i + 1)}
	}
	got := make([]selectedDestinationID, len(document.SelectedOutput))
	for i, destination := range document.SelectedOutput {
		got[i] = selectedDestinationID{peer: destination.Destination, session: destination.Session}
		require.Equal(t, 1, destination.InFlight)
		require.Empty(t, destination.Failure, "a held receipt is not a transport failure")
	}
	require.ElementsMatch(t, want, got)
	doctor, doctorErr := connection.SendDoctorCheck(ctx, advertiseDoctorName)
	require.NoError(t, doctorErr)
	require.Empty(t, doctor.Diagnostics, "ordinary convergence must not raise doctor failures")

	releaseOnce.Do(func() { close(release) })
	require.NoError(t, manager.DrainEventDelivery(ctx))
	status, statusErr = connection.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Serial: "settled-selected-status", Command: "show bgp rib status"})
	require.NoError(t, statusErr)
	require.Equal(t, statusDone, status.Status)
	require.NoError(t, json.Unmarshal(status.Data, &document))
	require.Empty(t, document.SelectedOutput)
}
