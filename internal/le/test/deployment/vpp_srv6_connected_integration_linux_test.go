//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- real connected-route prerequisites.
// Related: internal/plugins/connected/locrib.go -- connected addresses enter Loc-RIB.
// Related: internal/plugins/iface/netlink/monitor_linux.go -- live address events.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var vppSRv6Connected = flag.Bool("vpp-srv6-connected", false, "synchronize real connected routes before the SRv6 BGP probe")

const vppSRv6ConnectedReady = "/run/vpp/srv6-connected.ready"

// TestVPPSRv6ConnectedProbe runs as a real external SDK plugin in each Ze
// generation. Linux addresses alone are insufficient: the configured interface
// monitor must deliver live events to the configured connected producer.
func TestVPPSRv6ConnectedProbe(t *testing.T) {
	if !*vppSRv6Connected {
		t.Skip("connected-route readiness personality is started by the native harness")
	}
	p, err := sdk.NewFromTLSEnv("srv6-connected")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	ready := make(chan struct{})
	p.OnAllPluginsReady(func() error { close(ready); return nil })
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
		t.Fatalf("connected readiness plugin exited before startup completed: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	host, err := netlink.LinkByName(vppSRv6Host)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"2001:db8:94::3/64", "2001:db8:95::1/64"} {
		address, err := netlink.ParseAddr(prefix)
		if err != nil {
			t.Fatal(err)
		}
		// On Ze restart the old addresses persist, but a new monitor receives
		// changes rather than a snapshot. Re-add through real netlink events.
		if err := netlink.AddrDel(host, address); err != nil && !errors.Is(err, unix.EADDRNOTAVAIL) {
			t.Fatal(err)
		}
		address.Flags = unix.IFA_F_NODAD
		if err := netlink.AddrAdd(host, address); err != nil {
			t.Fatal(err)
		}
	}
	deadline, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	for {
		status, payload, err := p.DispatchCommand(deadline, "show rib")
		if err != nil || status != "done" {
			t.Fatalf("query actual connected RIB: status=%s error=%v payload=%s", status, err, payload)
		}
		var routes []struct {
			Prefix   netip.Prefix `json:"prefix"`
			Protocol string       `json:"protocol"`
		}
		if err := json.Unmarshal(payload, &routes); err != nil {
			t.Fatal(err)
		}
		nextHop, sid := false, false
		for _, route := range routes {
			if route.Protocol != "connected" {
				continue
			}
			nextHop = nextHop || route.Prefix == netip.MustParsePrefix("2001:db8:94::/64")
			sid = sid || route.Prefix == netip.MustParsePrefix("2001:db8:95::/64")
		}
		if nextHop && sid {
			t.Logf("real connected producer populated Loc-RIB/system-RIB before service announcements: %s", payload)
			break
		}
		select {
		case <-deadline.Done():
			t.Fatalf("connected covering routes missing from actual RIB: %s", payload)
		case <-time.After(50 * time.Millisecond):
		}
	}
	// Atomic replacement ties the barrier to this real plugin process. The BGP
	// probe MUST reject the prior generation's readiness after restarting Ze.
	generation := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(vppSRv6ConnectedReady+".next", generation, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(vppSRv6ConnectedReady+".next", vppSRv6ConnectedReady); err != nil {
		t.Fatal(err)
	}
	// Keep the authenticated plugin alive until its owning Ze exits.
	err = <-done
	joined = true
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func vppSRv6AwaitConnected(t *testing.T, previous string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, err := os.ReadFile(vppSRv6ConnectedReady)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		generation := strings.TrimSpace(string(data))
		if generation != "" && generation != previous {
			if pid, err := strconv.Atoi(generation); err != nil || pid <= 1 {
				t.Fatalf("invalid connected readiness generation %q", generation)
			}
			t.Log("connected Loc-RIB readiness established for plugin generation ", generation)
			return generation
		}
		if !time.Now().Before(deadline) {
			log, _ := os.ReadFile("/run/vpp/srv6-connected.log") //nolint:errcheck // failure diagnostics
			t.Fatalf("no fresh connected Loc-RIB readiness after generation %q: %s", previous, log)
		}
		// sleep(poll): await the producer's real command result, not a fixed delay.
		time.Sleep(50 * time.Millisecond)
	}
}
