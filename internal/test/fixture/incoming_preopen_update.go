// Design: docs/architecture/behavior/peer-lifecycle.md -- private incoming connections cannot publish UPDATEs.
// Related: role_incoming_open.go -- useful original-socket and role export assertions.
// Related: atomic_aggregate_policy.go -- configured-listener wire helpers.
package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const incomingPreopenPrefix = "198.51.0.0/24"

func init() {
	Register("plugin/incoming-preopen-update-isolation", incomingPreopenUpdateIsolation, StopsDaemon)
}

// incomingPreopenUpdateIsolation sends a withdrawal on an actual competing TCP
// connection before its OPEN. The original source's subsequent announcement
// fences the same source's received-message order at the original destination.
// Neither original socket closes before the real received RIB and wire checks.
func incomingPreopenUpdateIsolation(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("incoming-preopen-update-isolation requires the runner's listener address")
	}
	return Observe(ctx, "incoming-update-watch", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := roleIncomingWaitActive(ctx, plugin); err != nil {
			return err
		}
		source, err := roleIncomingConnect(ctx, args[0], roleIncomingSource, 65001)
		if err != nil {
			return fmt.Errorf("establish original source Customer: %w", err)
		}
		defer source.Close() //nolint:errcheck // All assertions finish before this owned socket is released.
		dest, err := roleIncomingConnect(ctx, args[0], roleIncomingDest, 65002)
		if err != nil {
			return fmt.Errorf("establish original destination Customer: %w", err)
		}
		defer dest.Close() //nolint:errcheck // All assertions finish before this owned socket is released.
		if err := p12WaitPeersEOR(ctx, plugin, "source-peer", "dest-peer"); err != nil {
			return err
		}
		for _, conn := range []net.Conn{source, dest} {
			initial, err := atomicAggregatePolicyRead(conn)
			if err != nil {
				return fmt.Errorf("read original socket's End-of-RIB: %w", err)
			}
			if len(initial.NLRI)+len(initial.WithdrawnRoutes)+len(initial.PathAttributes) != 0 {
				return errors.New("original socket's initial sync was not an empty IPv4 End-of-RIB")
			}
		}
		if err := roleIncomingUseful(source, dest, 0); err != nil {
			return fmt.Errorf("original source's useful route baseline: %w", err)
		}
		if err := incomingPreopenStored(ctx, plugin, true); err != nil {
			return err
		}
		if err := incomingPreopenWithdraw(ctx, args[0]); err != nil {
			return err
		}
		// A leaked withdrawal comes before this fresh announcement on the same
		// source's forwarding stream. Require the next UPDATE itself to be the
		// useful announcement: a delay or an empty query cannot prove absence.
		if err := roleIncomingUseful(source, dest, 1); err != nil {
			return fmt.Errorf("original source's FIFO fence after pre-OPEN withdrawal: %w", err)
		}
		if err := incomingPreopenStored(ctx, plugin, true); err != nil {
			return fmt.Errorf("private connection changed the original source's received RIB: %w", err)
		}
		if err := roleIncomingUnchanged(ctx, plugin); err != nil {
			return err
		}
		if err := incomingPreopenWriteWithdrawal(source); err != nil {
			return fmt.Errorf("send the original source's legitimate withdrawal: %w", err)
		}
		withdrawal, err := atomicAggregatePolicyRead(dest)
		if err != nil {
			return fmt.Errorf("read the original source's legitimate withdrawal: %w", err)
		}
		if !bytes.Equal(withdrawal.WithdrawnRoutes, []byte{24, 198, 51, 0}) {
			return fmt.Errorf("original destination received withdrawal %x, want 198.51.0.0/24", withdrawal.WithdrawnRoutes)
		}
		if len(withdrawal.NLRI) != 0 {
			return fmt.Errorf("legitimate withdrawal unexpectedly announced %x", withdrawal.NLRI)
		}
		if err := incomingPreopenStored(ctx, plugin, false); err != nil {
			return fmt.Errorf("legitimate withdrawal did not remove the original source's received route: %w", err)
		}
		if err := roleIncomingUseful(source, dest, 2); err != nil {
			return fmt.Errorf("original source's FIFO fence after legitimate withdrawal: %w", err)
		}
		if err := roleIncomingUnchanged(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: pre-OPEN withdrawal received EOF without notification; original route stayed in the received RIB and on its original socket; original source alone withdrew it once; both original peers stayed established")
		return nil
	})
}

// incomingPreopenStored observes actual Adj-RIB-In rows through the daemon's
// command surface. Dispatch and row-decoding failures remain errors, never an
// empty table interpreted as successful removal.
func incomingPreopenStored(ctx context.Context, plugin *sdk.Plugin, want bool) error {
	var last []string
	var queryErr error
	reached := Poll(ctx, WaitAttempts(20, 50*time.Millisecond, 80), 50*time.Millisecond, func() bool {
		last, queryErr = incomingPreopenStoredKeys(ctx, plugin)
		if queryErr != nil {
			return true // End the bounded wait and return the command failure below.
		}
		return slices.Contains(last, "ipv4/unicast:"+incomingPreopenPrefix) == want
	})
	if queryErr != nil {
		return queryErr
	}
	if reached {
		return nil
	}
	return fmt.Errorf("received RIB presence for %s=%t was not observed; held source keys=%v", incomingPreopenPrefix, want, last)
}

// incomingPreopenStoredKeys reads the declared Adj-RIB-In view, rather than the
// separate Loc-RIB view. A missing source is an empty table; a malformed table
// or row is an error and cannot satisfy the legitimate-withdrawal assertion.
func incomingPreopenStoredKeys(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp adj-rib-in")
	if err != nil {
		return nil, err
	}
	rib, ok := data["adj-rib-in"].(map[string]any)
	if !ok {
		return nil, errors.New("show bgp adj-rib-in did not return its peer map")
	}
	value, present := rib[roleIncomingSource]
	if !present {
		return nil, nil
	}
	rows, ok := value.([]any)
	if !ok {
		return nil, errors.New("show bgp adj-rib-in did not return source routes as rows")
	}
	keys := make([]string, 0, len(rows))
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("show bgp adj-rib-in returned a source route that is not an object")
		}
		key, ok := row["key"].(string)
		if !ok {
			return nil, errors.New("show bgp adj-rib-in returned a source route without a textual key")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// incomingPreopenWithdraw owns only the competing connection. DelayOpen keeps
// its OPEN unsent, so RFC 4271's early-state catchall closes with zero bytes.
func incomingPreopenWithdraw(ctx context.Context, address string) error {
	conn, err := roleIncomingDial(ctx, address, roleIncomingSource)
	if err != nil {
		return fmt.Errorf("connect source-side competitor: %w", err)
	}
	defer conn.Close() //nolint:errcheck // The peer's EOF must be observed before local release.
	if err := incomingPreopenWriteWithdrawal(conn); err != nil {
		return fmt.Errorf("send competing source withdrawal before OPEN: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(WaitBudget(5, time.Second))); err != nil {
		return err
	}
	var first [1]byte
	n, err := conn.Read(first[:])
	if n != 0 {
		return fmt.Errorf("pre-OPEN connection sent an unexpected octet %x before EOF", first[:n])
	}
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("pre-OPEN connection did not end with EOF: %w", err)
	}
	return nil
}

func incomingPreopenWriteWithdrawal(conn net.Conn) error {
	if err := conn.SetWriteDeadline(time.Now().Add(WaitBudget(5, time.Second))); err != nil {
		return err
	}
	packet := message.PackTo(&message.Update{WithdrawnRoutes: []byte{24, 198, 51, 0}}, nil)
	n, err := conn.Write(packet)
	if err != nil {
		return err
	}
	if n != len(packet) {
		return io.ErrShortWrite
	}
	return conn.SetWriteDeadline(time.Time{})
}
