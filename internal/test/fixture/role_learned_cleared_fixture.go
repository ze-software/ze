// Design: docs/architecture/meta/role.md -- Learned role lifecycle
// Related: plugin_fixture_13_peer_lifecycle.go -- the create and delete commands and the peer-table polls it reuses

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// roleClearedPeer is the address both created peers share, so the second one
// is created under the first one's generated name and learned-role key.
const roleClearedPeer = "127.0.0.2"

// roleClearedCreate creates the peer the way an operator does. It names no
// local role, so the session's Role comes from the test peer alone.
const roleClearedCreate = "create bgp peer " + roleClearedPeer + " asn 65002 local-as 65000 local-address " +
	roleClearedPeer + " router-id 1.2.3.4 accept false"

// roleClearedScript is the test peer's script: ze's End-of-RIB, then the session
// held open until ze closes it. extra carries the OPEN option that adds the Role
// capability, or nothing.
func roleClearedScript(extra string) string {
	return "option=tcp_connections:value=1\n" +
		"option=timeout:value=60s\n" +
		"option=linger:value=true\n" +
		extra +
		"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000\n"
}

// roleLearnedClearedOnPeerDelete creates a peer whose OPEN says it is a
// customer (RFC 9234 Role value 3), deletes it, and creates it again at the
// same address against a router whose OPEN carries no Role.
//
// The verdict is in ze's log, asserted by the .ci: the delete must log the
// role it dropped, and the second OPEN must not log the withdrawal of a role
// only the deleted peer ever advertised. That withdrawal line is what bgp-role
// writes when it still holds the old peer's role, so its absence after an
// established second session is the proof the old role was not read.
//
// The two routers are two test peer processes on one address, started one
// after the other, because one test peer sends the same OPEN on every
// connection.
func roleLearnedClearedOnPeerDelete(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("role-learned-cleared fixture requires the BGP port")
	}
	port := args[0]
	return observe13(ctx, "role-cleared-driver", func(ctx context.Context, plugin *sdk.Plugin) error {
		first, err := roleClearedStartPeer(ctx, port, "with-role", "option=open:value=add-capability:code=9:hex=03\n")
		if err != nil {
			return err
		}
		defer stopFixtureProcess(first, 5*time.Second)
		if err := roleClearedCreateEstablished(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: the created peer established with Role customer")

		if _, err := plugin01RequireDone(ctx, plugin, "delete bgp peer "+roleClearedPeer); err != nil {
			return err
		}
		if !lifecycleWaitGone(ctx, plugin, roleClearedPeer) {
			return fmt.Errorf("%s is still a peer after delete bgp peer", roleClearedPeer)
		}
		fmt.Fprintln(os.Stderr, "OK: the created peer was deleted")
		// The port is freed for the second router.
		stopFixtureProcess(first, 5*time.Second)

		second, err := roleClearedStartPeer(ctx, port, "without-role", "")
		if err != nil {
			return err
		}
		defer stopFixtureProcess(second, 5*time.Second)
		if err := roleClearedCreateEstablished(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: the peer created again established with no Role")
		return nil
	})
}

// roleClearedStartPeer writes the script and starts a test peer on the shared
// address, returning once it listens.
func roleClearedStartPeer(ctx context.Context, port, tag, extra string) (*fixtureProcess, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}
	script := filepath.Join(cwd, "role-cleared-"+tag+".peer")
	if err := os.WriteFile(script, []byte(roleClearedScript(extra)), 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", script, err)
	}
	peer, err := startFixtureProcess(ctx, os.Environ(), "", "le", "test", "peer",
		"--bind", roleClearedPeer, "--port", port, "--asn", "65002", script)
	if err != nil {
		return nil, fmt.Errorf("start the %s test peer: %w", tag, err)
	}
	if !Poll(ctx, 50, 100*time.Millisecond, func() bool {
		return strings.Contains(peer.output.String(), "listening on")
	}) {
		stopFixtureProcess(peer, time.Second)
		return nil, fmt.Errorf("the %s test peer did not listen: %s", tag, peer.output.String())
	}
	return peer, nil
}

// roleClearedCreateEstablished creates the peer and waits until its session is
// established, which means its OPEN went through bgp-role's validation.
func roleClearedCreateEstablished(ctx context.Context, plugin *sdk.Plugin) error {
	if _, err := plugin01RequireDone(ctx, plugin, roleClearedCreate); err != nil {
		return err
	}
	if !lifecycleWaitState(ctx, plugin, roleClearedPeer, "established") {
		return fmt.Errorf("the created peer %s never reached established", roleClearedPeer)
	}
	return nil
}
