//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- managed VPP and tenant-table evidence.
// Related: internal/component/plugin/server/dispatch.go -- existing EmitEvent ingress.
// Related: internal/component/vpp/vpp.go -- the real managed process/reconnect owner.
// RFC: rfc/full/rfc9252.txt -- service SID remains the segment, not a BSID.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/sdk"

	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/core"
)

var vppSRv6Plugin = flag.Bool("vpp-srv6-plugin", false, "run the managed VPP proof as an external Ze plugin")

// TestVPPSRv6ManagedProbe exercises the existing external EmitEvent contract,
// not a production test hook. The separate BGP probe covers the main-table
// producer; this probe covers tenant identity and production reconnect replay.
func TestVPPSRv6ManagedProbe(t *testing.T) {
	if !*vppSRv6Plugin {
		t.Skip("managed VPP plugin personality is started by the native harness")
	}
	p, err := sdk.NewFromTLSEnv("srv6-evidence")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ready := make(chan struct{})
	requests := make(chan struct{}, 16)
	p.SetStartupSubscriptionsIn(sysribevents.Namespace, []string{sysribevents.EventReplayRequest}, nil, "json")
	p.OnAllPluginsReady(func() error { close(ready); return nil })
	p.OnEvent(func(_ string) error {
		select {
		case requests <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			return errors.New("SRv6 evidence replay request queue overflow")
		}
	})
	// This goroutine has one owner: cancellation plus the joined Run result.
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, sdk.Registration{}) }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	select {
	case <-ready:
	case err := <-done:
		joined = true
		t.Fatalf("external plugin exited before readiness: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	conn, err := core.Connect(socketclient.NewVppClient("/run/vpp/api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { conn.Disconnect() }()
	api := sr.NewServiceClient(conn)
	vppSRv6AwaitState(t, api, nil)
	vppSRv6PGUnderlay(t)

	prefix := netip.MustParsePrefix("10.94.3.0/24")
	a := vppSRv6RouteKey{prefix: prefix, table: 10}
	b := vppSRv6RouteKey{prefix: prefix, table: 20}
	sidOne, sidTwo := netip.MustParseAddr(vppSRv6SIDOne), netip.MustParseAddr(vppSRv6SIDTwo)
	want := map[vppSRv6RouteKey]netip.Addr{a: sidOne, b: sidTwo}
	vppSRv6Publish(t, ctx, p, want, routeaction.Update, 0)
	vppSRv6AwaitTables(t, api, want)
	vppSRv6PGPacket(t, 0, prefix.Addr().Next(), sidOne, 21)
	vppSRv6PGPacket(t, 1, prefix.Addr().Next(), sidTwo, 22)
	before := vppSRv6PolicyIDs(t, api)
	vppSRv6Publish(t, ctx, p, want, routeaction.Update, replay.Broadcast)
	vppSRv6AwaitTables(t, api, want)
	vppSRv6SamePolicyIDs(t, before, vppSRv6PolicyIDs(t, api))

	// Empty startup notifications before killing the actual managed child.
	// No connected/reconnected notification is manufactured by the fixture.
	for len(requests) != 0 {
		<-requests
	}
	pid := vppSRv6ManagedPID(t)
	if output, err := exec.CommandContext(ctx, "kill", "-KILL", strconv.Itoa(pid)).CombinedOutput(); err != nil {
		t.Fatalf("kill managed VPP pid=%d: %v: %s", pid, err, output)
	}
	conn.Disconnect()
	select {
	case <-requests:
		t.Log("received production system-rib replay-request after killing managed VPP")
	case err := <-done:
		joined = true
		t.Fatalf("plugin exited during VPP reconnect: %v", err)
	case <-ctx.Done():
		t.Fatal("no production replay request after managed VPP exit: ", ctx.Err())
	}
	newPID := vppSRv6ManagedPID(t)
	if newPID == pid {
		t.Fatal("managed VPP PID did not change")
	}
	newConn, err := core.Connect(socketclient.NewVppClient("/run/vpp/api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	conn = newConn
	api = sr.NewServiceClient(conn)
	vppSRv6AwaitState(t, api, nil)
	// Interface/table/underlay fixtures are process-local prerequisites. Restore
	// them before answering the real replay request, never install SR state.
	vppSRv6PGUnderlay(t)
	vppSRv6Publish(t, ctx, p, want, routeaction.Update, replay.Broadcast)
	vppSRv6AwaitTables(t, api, want)
	vppSRv6PGPacket(t, 0, prefix.Addr().Next(), sidOne, 23)
	vppSRv6PGPacket(t, 1, prefix.Addr().Next(), sidTwo, 24)
	t.Logf("managed VPP restarted pid=%d -> %d; production replay restored both tenant tables", pid, newPID)

	// Replacement in one table MUST NOT overwrite the identical prefix in the
	// other table. Sharing a segment afterward still requires both references.
	want[a] = sidTwo
	vppSRv6Publish(t, ctx, p, map[vppSRv6RouteKey]netip.Addr{a: sidTwo}, routeaction.Update, 0)
	vppSRv6AwaitTables(t, api, want)
	vppSRv6PGPacket(t, 0, prefix.Addr().Next(), sidTwo, 25)
	vppSRv6PGPacket(t, 1, prefix.Addr().Next(), sidTwo, 26)
	vppSRv6Publish(t, ctx, p, map[vppSRv6RouteKey]netip.Addr{a: sidTwo}, routeaction.Withdraw, 0)
	delete(want, a)
	vppSRv6AwaitTables(t, api, want)
	vppSRv6PGPacket(t, 0, prefix.Addr().Next(), netip.Addr{}, 27)
	vppSRv6PGPacket(t, 1, prefix.Addr().Next(), sidTwo, 28)
	vppSRv6Publish(t, ctx, p, want, routeaction.Withdraw, 0)
	vppSRv6AwaitState(t, api, nil)
	vppSRv6PGPacket(t, 1, prefix.Addr().Next(), netip.Addr{}, 29)
	cancel()
	err = <-done
	joined = true
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal("external plugin shutdown: ", err)
	}
	t.Log("srv6 managed lifecycle passed")
}

func vppSRv6ManagedPID(t *testing.T) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "pgrep", "-f", "^/usr/bin/vpp -c /etc/vpp/startup.conf$").CombinedOutput()
	if err != nil {
		t.Fatalf("find managed VPP child: %v: %s", err, output)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || pid <= 1 {
		t.Fatalf("expected exactly one managed VPP child, got %q: %v", output, err)
	}
	return pid
}

func vppSRv6Publish(t *testing.T, ctx context.Context, p *sdk.Plugin, want map[vppSRv6RouteKey]netip.Addr, action routeaction.Action, replayID uint64) {
	t.Helper()
	batch := sysribevents.BestChangeBatch{Family: family.IPv4Unicast, ReplayID: replayID}
	for key, sid := range want {
		batch.Changes = append(batch.Changes, sysribevents.BestChangeEntry{
			Action: action, Prefix: key.prefix, TableID: key.table, SRv6SID: sid,
			NextHop: netip.MustParseAddr(vppSRv6NextHop), Protocol: "bgp",
		})
	}
	payload, err := json.Marshal(&batch)
	if err != nil {
		t.Fatal(err)
	}
	// Assert the very bytes sent through the runtime ingress, not a separate
	// wiring fixture. A value marshal would bypass BestChangeBatch.MarshalJSON.
	var published struct {
		Replay bool `json:"replay"`
	}
	if err := json.Unmarshal(payload, &published); err != nil {
		t.Fatal(err)
	}
	if published.Replay != replay.IsReplay(replayID) {
		t.Fatalf("published replay marker lost: replay-id=%d payload=%s", replayID, payload)
	}
	// The returned count measures external recipients, NOT in-process FIB
	// subscribers. Actual API dumps and packets establish eventual delivery.
	if _, err := p.EmitEvent(ctx, sysribevents.Namespace, sysribevents.EventBestChange, "", "", string(payload)); err != nil {
		t.Fatal(fmt.Errorf("publish tenant best-change: %w", err))
	}
	t.Logf("existing plugin ingress: %s", payload)
}
