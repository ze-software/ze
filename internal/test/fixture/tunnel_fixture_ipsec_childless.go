// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- a childless IKE SA, and an esp-group reload that keeps it
// Related: register_ipsec_childless.go -- the registration
// Related: tunnel_fixture_ipsec.go -- tunnelIPsecWait, the shared poll

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The responder's esp-group offers tunnelIPsecChildlessRefused, which the
// initiator does not, and the reload swaps it for tunnelIPsecChildlessAccepted.
const (
	tunnelIPsecChildlessRefused  = "encryption aes128"
	tunnelIPsecChildlessAccepted = "encryption aes256"
)

// tunnelIPsecChildlessSA is the part of one `show vpn ipsec sa | json` entry
// the fixture judges: the IKE SA's state, its initiator SPI, and its Child SAs,
// a key the command omits while the IKE SA carries none.
type tunnelIPsecChildlessSA struct {
	State        string          `json:"state"`
	InitiatorSPI string          `json:"initiator-spi"`
	ChildSA      json.RawMessage `json:"child-sa"`
}

// tunnelIPsecChildlessState is one read of the responder's single IKE SA:
// whether it is established, its initiator SPI, and whether it carries a Child SA.
type tunnelIPsecChildlessState struct {
	established bool
	spi         string
	child       bool
}

// tunnelIPsecChildlessRead asks the RESPONDER daemon, the only one that runs an
// SSH server in ipsec-childless-ike-sa-recovers.ci, for its IKE SAs.
func tunnelIPsecChildlessRead(ctx context.Context) (tunnelIPsecChildlessState, error) {
	output, err := tunnelIPsecCommand(ctx, "ze", "cli", "-c", "show vpn ipsec sa | json")
	if err != nil {
		return tunnelIPsecChildlessState{}, err
	}
	var sas []tunnelIPsecChildlessSA
	if err := json.Unmarshal([]byte(output), &sas); err != nil {
		return tunnelIPsecChildlessState{}, fmt.Errorf("show vpn ipsec sa | json is not a JSON array of IKE SAs: %w", err)
	}
	// The established IKE SA is the one judged; while none is established the
	// read reports "not established", never a stale SPI.
	for _, sa := range sas {
		if sa.State != stateEstablished {
			continue
		}
		child := len(sa.ChildSA) > 0 && string(sa.ChildSA) != "null" && string(sa.ChildSA) != "[]"
		return tunnelIPsecChildlessState{established: true, spi: sa.InitiatorSPI, child: child}, nil
	}
	return tunnelIPsecChildlessState{}, nil
}

// tunnelIPsecChildlessWait polls the responder until condition holds. A failed
// read (the SSH server not yet listening) is a "not yet", never an answer, and
// the last failure is named if the budget runs out.
func tunnelIPsecChildlessWait(ctx context.Context, budget time.Duration, condition func(tunnelIPsecChildlessState) bool) (tunnelIPsecChildlessState, error) {
	deadline := time.Now().Add(budget)
	var last tunnelIPsecChildlessState
	var lastErr error
	for time.Now().Before(deadline) {
		state, err := tunnelIPsecChildlessRead(ctx)
		lastErr = err
		if err == nil {
			last = state
			if condition(state) {
				return state, nil
			}
		}
		if waitErr := tunnelIPsecWaitDelay(ctx, 500*time.Millisecond); waitErr != nil {
			return last, waitErr
		}
	}
	if lastErr != nil {
		return last, fmt.Errorf("condition never held within %s, last read failed: %w", budget, lastErr)
	}
	return last, fmt.Errorf("condition never held within %s, last state %+v", budget, last)
}

// tunnelIPsecChildlessRecovers drives ipsec-childless-ike-sa-recovers.ci.
//
// It waits until the responder holds an established IKE SA with NO Child SA,
// records that SA's initiator SPI, reloads the responder's esp-group to one the
// initiator offers, and waits until the SAME IKE SA carries a Child SA. The SPI
// is the identity: a reload that tore the IKE SA down, or an initiator that
// re-established instead of creating, shows the Child SA under a new SPI, which
// this reports as RECONNECTED.
func tunnelIPsecChildlessRecovers(ctx context.Context, _ []string) error {
	if _, err := tunnelIPsecWait(ctx, 200, 50*time.Millisecond, func() ([]string, error) {
		if _, statErr := os.Stat("daemon.pid"); statErr != nil {
			return nil, statErr
		}
		return []string{fieldReady}, nil
	}, func(values []string) bool { return len(values) == 1 }); err != nil {
		return err
	}
	childless, err := tunnelIPsecChildlessWait(ctx, 30*time.Second, func(state tunnelIPsecChildlessState) bool {
		return state.established && state.spi != ""
	})
	if err != nil {
		return fmt.Errorf("NEVER-ESTABLISHED: %w", err)
	}
	if childless.child {
		return errors.New("CHILD-BEFORE-RELOAD: the responder holds a Child SA its esp-group cannot have negotiated")
	}
	fmt.Println("CHILDLESS:", childless.spi)
	if err := tunnelIPsecChildlessReload(); err != nil {
		return err
	}
	// The initiator retries creation on its own schedule, 30 s after the IKE SA
	// came up and then doubling, so the budget covers the first two attempts.
	created, err := tunnelIPsecChildlessWait(ctx, 95*time.Second, func(state tunnelIPsecChildlessState) bool {
		if !state.established {
			return false
		}
		if state.spi != childless.spi {
			return true
		}
		return state.child
	})
	if err != nil {
		return fmt.Errorf("NEVER-CREATED: %w", err)
	}
	if created.spi != childless.spi {
		return fmt.Errorf("RECONNECTED: the IKE SA changed from %s to %s", childless.spi, created.spi)
	}
	fmt.Println("CREATED:", created.spi)
	return nil
}

// tunnelIPsecChildlessReload edits the responder's own config file, the one the
// runner wrote with every $PORT already expanded, so the edit changes the
// esp-group encryption and nothing else, then sends the daemon SIGHUP.
func tunnelIPsecChildlessReload() error {
	pidBytes, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("daemon.pid never appeared: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("parse daemon.pid: %w", err)
	}
	config, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return err
	}
	if count := strings.Count(string(config), tunnelIPsecChildlessRefused); count != 1 {
		return fmt.Errorf("ze-bgp.conf holds %q %d times, want exactly once", tunnelIPsecChildlessRefused, count)
	}
	edited := strings.Replace(string(config), tunnelIPsecChildlessRefused, tunnelIPsecChildlessAccepted, 1)
	if err := os.WriteFile("ze-bgp.conf", []byte(edited), 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	fmt.Println("RELOADED: sent SIGHUP to pid", pid)
	return nil
}
