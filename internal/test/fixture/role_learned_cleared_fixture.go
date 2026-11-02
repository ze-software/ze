// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: plugin_fixture_13_peer_lifecycle.go -- the delete command and the peer-table polls it reuses
// Related: reload_fixture_live_stages.go -- liveStagesReload, the SIGHUP reload it drives

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// roleClearedPeer is the address of both routers: the one whose OPEN says Role
// customer, which the .ci starts, and the one whose OPEN carries no Role, which
// this fixture starts once the first has gone. The configured peer keeps its
// address and name across the delete and the reload, so the second session is
// looked up under the first one's learned-role key, and bgp-role sends the
// same Role capability it sent at start (it refuses a reload that changes one).
const roleClearedPeer = "127.0.0.2"

// roleClearedNextConfig is the tmpfs file the .ci declares: the configuration
// the daemon started with, which still names the deleted peer.
const roleClearedNextConfig = "role-cleared-next.conf"

// roleClearedScript is the second router's script: ze's End-of-RIB, then the
// session held open until ze closes it. Its OPEN carries no Role capability.
const roleClearedScript = "option=tcp_connections:value=1\n" +
	"option=timeout:value=90s\n" +
	"option=linger:value=true\n" +
	"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000\n"

// roleLearnedClearedOnPeerDelete deletes a configured peer whose OPEN said it
// is a customer (RFC 9234 Role value 3), then brings the same peer name back
// through a config reload against a router whose OPEN carries no Role, at
// the same address.
//
// The verdict is in ze's log, asserted by the .ci: the delete must log the
// role it dropped, and the second OPEN must not log the withdrawal of a role
// only the deleted peer ever advertised. That withdrawal line is what bgp-role
// writes when it still holds the old peer's role, so its absence after an
// established second session is the proof the old role was not read.
func roleLearnedClearedOnPeerDelete(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("role-learned-cleared fixture requires the BGP port")
	}
	port := args[0]
	return observe13(ctx, "role-cleared-driver", func(ctx context.Context, plugin *sdk.Plugin) error {
		if !lifecycleWaitState(ctx, plugin, roleClearedPeer, "established") {
			return fmt.Errorf("the configured peer %s never reached established", roleClearedPeer)
		}
		fmt.Fprintln(os.Stderr, "OK: the configured peer established with Role customer")

		if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer "+roleClearedPeer); err != nil {
			return err
		}
		if !lifecycleWaitGone(ctx, plugin, roleClearedPeer) {
			return fmt.Errorf("%s is still a peer after delete bgp peer", roleClearedPeer)
		}
		fmt.Fprintln(os.Stderr, "OK: the configured peer was deleted")

		second, err := roleClearedStartPeer(ctx, port)
		if err != nil {
			return err
		}
		defer stopFixtureProcess(second, 5*time.Second)
		if err := liveStagesReload(ctx, plugin, roleClearedNextConfig); err != nil {
			return fmt.Errorf("reload the peer back: %w", err)
		}
		if !lifecycleWaitState(ctx, plugin, roleClearedPeer, "established") {
			return fmt.Errorf("the peer the reload brought back at %s never reached established", roleClearedPeer)
		}
		fmt.Fprintln(os.Stderr, "OK: the peer the reload brought back established with no Role")
		return nil
	})
}

// roleClearedStartPeer starts the router whose OPEN carries no Role, once the
// first router has released the address (it exits when ze's Cease ends its one
// connection), and returns once it listens.
func roleClearedStartPeer(ctx context.Context, port string) (*fixtureProcess, error) {
	address := net.JoinHostPort(roleClearedPeer, port)
	var probe net.ListenConfig
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		listener, err := probe.Listen(ctx, "tcp", address)
		if err != nil {
			return false
		}
		return listener.Close() == nil
	}) {
		return nil, fmt.Errorf("the first router never released %s", address)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}
	script := filepath.Join(cwd, "role-cleared-without-role.peer")
	if err := os.WriteFile(script, []byte(roleClearedScript), 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", script, err)
	}
	peer, err := startFixtureProcess(ctx, os.Environ(), "", "le", "test", "peer",
		"--bind", roleClearedPeer, "--port", port, "--asn", "65002", script)
	if err != nil {
		return nil, fmt.Errorf("start the test peer without Role: %w", err)
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		return strings.Contains(peer.output.String(), "listening on")
	}) {
		stopFixtureProcess(peer, time.Second)
		return nil, fmt.Errorf("the test peer without Role did not listen: %s", peer.output.String())
	}
	return peer, nil
}
