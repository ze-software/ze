// Design: docs/labs/l2tp-interop.md -- live subscriber state from independent peers.
// Related: scale.go -- ordered scale assertions and teardown.
package l2tp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scaleAccountingPath = "/var/log/freeradius/ze-detail"
	scaleRequestPath    = "/run/scale-disconnect.req"
	scaleFixtureSecret  = "ze-subscriber-fixture-secret" //nolint:gosec // fixed synthetic RADIUS credential shared only by the isolated interop fixtures
	scaleConfigPath     = "/etc/kl2tpd/kl2tpd.toml"
	scaleIPAddr         = "addr"
)

type scaleObservation struct {
	population scalePopulation
	links      map[string]string
}

func observeScalePopulation(ctx context.Context, lab labOperations, users []string) (scaleObservation, error) {
	raw, err := lnsPolicyCommand(ctx, lab, "show l2tp sessions")
	if err != nil {
		return scaleObservation{}, err
	}
	population, err := readScaleSessions(raw, users)
	if err != nil {
		return scaleObservation{}, interoplab.WithSearchedOutput(err, string(raw))
	}
	addresses, err := execOutput(ctx, lab, peerZe, []string{"ip", "-j", "-4", scaleIPAddr, commandShow})
	if err != nil {
		return scaleObservation{}, err
	}
	routes, err := execOutput(ctx, lab, peerZe, []string{"ip", "-j", "-4", "route", commandShow})
	if err != nil {
		return scaleObservation{}, err
	}
	links, err := requireScaleKernel([]byte(addresses), []byte(routes), population)
	if err != nil {
		return scaleObservation{}, interoplab.WithSearchedOutput(err, addresses+"\n"+routes)
	}
	if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-KERNEL", addresses, routes); err != nil {
		return scaleObservation{}, err
	}
	return scaleObservation{population, links}, nil
}

func waitScalePopulation(ctx context.Context, lab labOperations, users []string, timeout time.Duration) (scaleObservation, error) {
	observed, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: 100 * time.Millisecond, Description: fmt.Sprintf("%d established subscribers and distinct kernel PPP routes", len(users)),
	}, func(probe context.Context) (scaleObservation, error) {
		return observeScalePopulation(probe, lab, users)
	}, func(scaleObservation) bool { return true })
	return observed, err
}

func requireScaleTopology(population scalePopulation) error {
	tunnels := make(map[uint16]bool, scaleLACCount)
	for lac := 1; lac <= scaleLACCount; lac++ {
		var tunnel uint16
		localIDs := make(map[uint16]bool, scaleSessionsPerLAC)
		for session := 1; session <= scaleSessionsPerLAC; session++ {
			user := scaleUsername(lac, session)
			actual, exists := population.byUser[user]
			if !exists {
				return fmt.Errorf("scale topology lacks %s", user)
			}
			if actual.remote != uint16(session) {
				return fmt.Errorf("%s remote SID %d, want configured SID%d", user, actual.remote, session)
			}
			if tunnel == 0 {
				tunnel = actual.tunnel
			}
			if actual.tunnel != tunnel {
				return fmt.Errorf("LAC%d spans multiple Ze tunnels", lac)
			}
			if actual.local > scaleSessionsPerLAC || localIDs[actual.local] {
				return fmt.Errorf("LAC%d lacks unique lowest-unused local SIDs: %d", lac, actual.local)
			}
			localIDs[actual.local] = true
		}
		if tunnels[tunnel] {
			return fmt.Errorf("LAC%d shares Ze tunnel %d with another LAC", lac, tunnel)
		}
		tunnels[tunnel] = true
	}
	return nil
}

func requireScaleFRR(ctx context.Context, lab labOperations, original, current scalePopulation) error {
	users := scaleUsers(scaleLACCount)
	arguments := make([]string, 1, 1+2*len(users))
	arguments[0] = commandVTYSH
	for _, user := range users {
		session, exists := original.byUser[user]
		if !exists {
			return fmt.Errorf("FRR baseline lacks %s", user)
		}
		arguments = append(arguments, "-c", "show bgp ipv4 unicast "+netip.PrefixFrom(session.address, 32).String()+" json")
	}
	output, err := execOutput(ctx, lab, peerFRR, arguments)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewBufferString(output))
	for _, user := range users {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return interoplab.WithSearchedOutput(fmt.Errorf("FRR answer for %s: %w", user, err), output)
		}
		session := original.byUser[user]
		present, err := readScaleRoute(raw, netip.PrefixFrom(session.address, 32))
		if err != nil {
			return interoplab.WithSearchedOutput(fmt.Errorf("FRR subscriber %s: %w", user, err), string(raw))
		}
		_, expected := current.byUser[user]
		if present != expected {
			return interoplab.WithSearchedOutput(fmt.Errorf("FRR subscriber %s %s present=%v, want %v", user, session.address, present, expected), string(raw))
		}
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("FRR returned unexpected trailing route evidence")
	}
	if err := requireFRRSession(ctx, lab, zeAddress); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "L2TP-SCALE-FRR", output)
	return err
}

func waitScaleFRR(ctx context.Context, lab labOperations, original, current scalePopulation, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "actual subscriber /32 paths from Ze at FRR with BGP up"}, func(probe context.Context) (bool, error) {
		if err := requireScaleFRR(probe, lab, original, current); err != nil {
			return false, err
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	return err
}

func requireScaleTraffic(ctx context.Context, lab labOperations, observed scaleObservation, ipcp bool) error {
	for lac := 1; lac <= scaleLACCount; lac++ {
		peer := "kl2tpd-" + strconv.Itoa(lac)
		addresses, err := execOutput(ctx, lab, peer, []string{"ip", "-j", "-4", scaleIPAddr, commandShow})
		if err != nil {
			return err
		}
		if err := requireScalePeerPopulation([]byte(addresses), observed.population, lac); err != nil {
			return interoplab.WithSearchedOutput(err, addresses)
		}
		for number := 1; number <= scaleSessionsPerLAC; number++ {
			user := scaleUsername(lac, number)
			session, exists := observed.population.byUser[user]
			if !exists {
				continue
			} // Only the explicitly removed users are absent.
			name := scaleInterface(lac, number)
			if err := requireScalePeerLink([]byte(addresses), session, name); err != nil {
				return interoplab.WithSearchedOutput(err, addresses)
			}
			if ipcp {
				log, err := execOutput(ctx, lab, peer, []string{"cat", "/run/" + user + ".log"})
				if err != nil {
					return err
				}
				if err := requireScaleIPCP(log, session); err != nil {
					return interoplab.WithSearchedOutput(err, log)
				}
				if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-IPCP", user, log); err != nil {
					return err
				}
			}
			for _, probe := range []struct{ peer, device, destination string }{{peer, name, localPPPAddress}, {peerZe, observed.links[user], session.address.String()}} {
				output, err := execOutput(ctx, lab, probe.peer, []string{commandPing, "-n", "-I", probe.device, "-c", "1", "-W", "2", probe.destination})
				if err != nil {
					return fmt.Errorf("session %s traffic from %s via %s: %w", user, probe.peer, probe.device, err)
				}
				if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-TRAFFIC", user, probe.peer, probe.device, output); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func requireScalePeerPopulation(raw []byte, population scalePopulation, lac int) error {
	links, err := readScalePPPLinks(raw)
	if err != nil {
		return err
	}
	expected := 0
	for number := 1; number <= scaleSessionsPerLAC; number++ {
		actual, exists := population.byUser[scaleUsername(lac, number)]
		if !exists {
			continue
		}
		expected++
		if err := requireScalePeerLink(raw, actual, scaleInterface(lac, number)); err != nil {
			return err
		}
	}
	if len(links) != expected {
		return fmt.Errorf("LAC%d has %d PPP IPv4 links, want %d", lac, len(links), expected)
	}
	return nil
}

func waitScalePeers(ctx context.Context, lab labOperations, population scalePopulation, timeout time.Duration) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 100 * time.Millisecond, Description: "exact survivor PPP interfaces at every LAC"}, func(probe context.Context) (bool, error) {
		for lac := 1; lac <= scaleLACCount; lac++ {
			output, err := execOutput(probe, lab, "kl2tpd-"+strconv.Itoa(lac), []string{"ip", "-j", "-4", scaleIPAddr, commandShow})
			if err != nil {
				return false, err
			}
			if err := requireScalePeerPopulation([]byte(output), population, lac); err != nil {
				return false, interoplab.WithSearchedOutput(err, output)
			}
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	return err
}

func waitScaleAccounting(ctx context.Context, lab labOperations, population scalePopulation, status string, timeout time.Duration) (scaleAccounting, error) {
	accounting, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: time.Second, Description: "FreeRADIUS " + status + " joined to every subscriber"}, func(probe context.Context) (scaleAccounting, error) {
		output, err := execOutput(probe, lab, peerRadius, []string{"cat", scaleAccountingPath})
		if err != nil {
			return scaleAccounting{}, err
		}
		result, err := readScaleAccounting(output, population, status)
		if err != nil {
			return scaleAccounting{}, interoplab.WithSearchedOutput(err, output)
		}
		if _, err := fmt.Fprintln(os.Stdout, "L2TP-SCALE-ACCOUNTING", status, output); err != nil {
			return scaleAccounting{}, err
		}
		return result, nil
	}, func(scaleAccounting) bool { return true })
	return accounting, err
}

func scaleDisconnect(ctx context.Context, lab labOperations, id string) error {
	request := fmt.Sprintf("Acct-Session-Id = %q\nEvent-Timestamp = %d\nMessage-Authenticator = 0x00\n", id, time.Now().Unix())
	if _, err := lab.Exec(ctx, peerRadius, []string{"sh", "-c", `printf '%s' "$1" > "$2"`, "request", request, scaleRequestPath}, nil); err != nil {
		return err
	}
	result, err := lab.Exec(ctx, peerRadius, []string{"radclient", "-x", "-r", "1", "-t", "5", "-f", scaleRequestPath, zeAddress + ":3799", "disconnect", scaleFixtureSecret}, nil)
	output := result.Stdout + result.Stderr
	if err != nil {
		return interoplab.WithSearchedOutput(err, output)
	}
	if !strings.Contains(output, "Received Disconnect-ACK ") {
		return interoplab.WithSearchedOutput(errors.New("authenticated exact-ID request received no Disconnect-ACK"), output)
	}
	_, err = fmt.Fprintln(os.Stdout, "L2TP-SCALE-DISCONNECT", id, output)
	return err
}

func scaleStopPeerPPP(ctx context.Context, lab labOperations, user string) error {
	for lac := 1; lac <= scaleLACCount; lac++ {
		for number := 1; number <= scaleSessionsPerLAC; number++ {
			if scaleUsername(lac, number) != user {
				continue
			}
			// The PID file is pppd 2.5.5's create_linkpidfile output: PID,
			// then actual ifname. Validate both before signaling this process.
			script := `read -r pid < "$1" || exit 2
case "$pid" in ''|*[!0-9]*) exit 3;; esac
test "$(sed -n '2p' "$1")" = "$2" || exit 4
test "$(cat "/proc/$pid/comm")" = pppd || exit 5
kill -TERM "$pid"`
			_, err := lab.Exec(ctx, "kl2tpd-"+strconv.Itoa(lac), []string{"sh", "-c", script, "stop-session", "/var/run/ppp-" + user + ".pid", scaleInterface(lac, number)}, nil)
			return err
		}
	}
	return fmt.Errorf("unknown peer PPP identity %s", user)
}
