// Design: docs/labs/pppoe-interop.md -- classless native two-client health
//go:build ze_l2tp

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type sharedAccessClient struct {
	peer sharedAccessPeer
	vlan sharedAccessVLANIdentity
	ppp  sharedAccessPPP
	row  sharedAccessRow
	pid  int
}

// checkSharedAccess is only a healthy native fixture foundation. No CoS map,
// original Snapshot, network fence or native retirement capability is asserted.
func checkSharedAccess(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil {
		return errors.New("shared-access checker has no context")
	}
	if check.Lab == nil {
		return errors.New("shared-access checker has no lab")
	}
	lab := check.Lab
	defer func() { err = sharedAccessDiagnostics(ctx, lab, err) }()
	if err := waitZePPPoEConfigured(ctx, lab); err != nil {
		return err
	}
	if err := waitZeRESTReady(ctx, lab); err != nil {
		return err
	}
	access, _, err := interoplab.Wait(ctx, sharedAccessWait("actual Ze VLAN"), func(probe context.Context) (sharedAccessVLANIdentity, error) {
		return sharedAccessReadVLAN(probe, lab, zeImageName)
	}, func(sharedAccessVLANIdentity) bool { return true })
	if err != nil {
		return err
	}
	peers := sharedAccessPeers()
	for _, peer := range peers {
		if err := sharedAccessCreatePeerVLAN(ctx, lab, peer); err != nil {
			return err
		}
	}
	if err := sharedAccessCaptureStart(ctx, lab); err != nil {
		return err
	}
	var clients [2]sharedAccessClient
	for i, peer := range peers {
		if err := sharedAccessDial(ctx, lab, peer); err != nil {
			return err
		}
		client, _, err := interoplab.Wait(ctx, sharedAccessWait("native client "+peer.name), func(probe context.Context) (sharedAccessClient, error) {
			return sharedAccessObserveClient(probe, lab, peer, i+1)
		}, func(sharedAccessClient) bool { return true })
		if err != nil {
			return err
		}
		clients[i] = client
	}
	if clients[0].ppp.local == clients[1].ppp.local {
		return errors.New("two native clients share an IPCP address")
	}
	if clients[0].vlan.mac == clients[1].vlan.mac {
		return errors.New("two clients share an Ethernet source")
	}
	if clients[0].row.SID == clients[1].row.SID {
		return errors.New("two clients share a PPPoE SID on one access")
	}
	if clients[0].row.Index == clients[1].row.Index {
		return errors.New("two clients share a native PPP index")
	}
	if clients[0].row.Device == clients[1].row.Device {
		return errors.New("two clients share a native PPP device")
	}
	if err := sharedAccessCaptureCheck(ctx, lab, &clients); err != nil {
		return err
	}
	for i := range clients {
		client := &clients[i]
		if err := sharedAccessTraffic(ctx, lab, client.peer, client.row, client.ppp.local.String()); err != nil {
			return err
		}
	}
	if _, err := sharedAccessPPPCommand(ctx, lab, clients[0].peer, clients[0].pid, true); err != nil {
		return err
	}
	if err := sharedAccessWaitDeparture(ctx, lab, clients[0], []sharedAccessRow{clients[1].row}); err != nil {
		return err
	}
	survivor, err := sharedAccessObserveClient(ctx, lab, clients[1].peer, 1)
	if err != nil {
		return err
	}
	if survivor.row != clients[1].row {
		return errors.New("surviving original daemon row changed after other peer left")
	}
	if survivor.ppp != clients[1].ppp {
		return errors.New("surviving original kernel assignment changed")
	}
	if survivor.vlan != clients[1].vlan {
		return errors.New("surviving peer VLAN changed")
	}
	if survivor.pid != clients[1].pid {
		return errors.New("surviving original pppd process changed")
	}
	if err := sharedAccessTraffic(ctx, lab, survivor.peer, survivor.row, survivor.ppp.local.String()); err != nil {
		return err
	}
	if _, err := sharedAccessPPPCommand(ctx, lab, survivor.peer, survivor.pid, true); err != nil {
		return err
	}
	if err := sharedAccessWaitDeparture(ctx, lab, survivor, nil); err != nil {
		return err
	}
	after, err := sharedAccessReadVLAN(ctx, lab, zeImageName)
	if err != nil {
		return err
	}
	if after != access {
		return errors.New("configured access VLAN changed across independent session teardown")
	}
	return nil
}

func sharedAccessWait(subject string) interoplab.WaitOptions {
	return interoplab.WaitOptions{Timeout: 75 * time.Second, Interval: time.Second, Description: subject}
}

func sharedAccessObserveClient(ctx context.Context, lab interoplab.CheckerLab, peer sharedAccessPeer, count int) (sharedAccessClient, error) {
	vlan, err := sharedAccessReadVLAN(ctx, lab, peer.name)
	if err != nil {
		return sharedAccessClient{}, err
	}
	pid, err := sharedAccessPPPCommand(ctx, lab, peer, 0, false)
	if err != nil {
		return sharedAccessClient{}, err
	}
	log, err := query(ctx, lab, peer.name, []string{commandCat, peer.logPath()})
	if err != nil {
		return sharedAccessClient{}, err
	}
	if err := sharedAccessAuthEvidence(log, peer); err != nil {
		return sharedAccessClient{}, interoplab.WithSearchedOutput(err, log)
	}
	result, err := lab.Exec(ctx, peer.name, []string{"ip", "-j", commandAddr, commandShow, commandDev, peer.device}, nil)
	if err != nil {
		return sharedAccessClient{}, err
	}
	ppp, err := sharedAccessReadPPP([]byte(result.Stdout), peer.device, zeGateway)
	if err != nil {
		return sharedAccessClient{}, err
	}
	rows, err := sharedAccessRows(ctx, lab)
	if err != nil {
		return sharedAccessClient{}, err
	}
	if len(rows) != count {
		return sharedAccessClient{}, fmt.Errorf("daemon rows=%d, want %d", len(rows), count)
	}
	row, err := sharedAccessRowForMAC(rows, vlan.mac)
	if err != nil {
		return sharedAccessClient{}, err
	}
	server, err := lab.Exec(ctx, zeImageName, []string{"ip", "-j", commandAddr, commandShow, commandDev, row.Device}, nil)
	if err != nil {
		return sharedAccessClient{}, err
	}
	serverPPP, err := sharedAccessReadPPP([]byte(server.Stdout), row.Device, ppp.local.String())
	if err != nil {
		return sharedAccessClient{}, err
	}
	if serverPPP.local.String() != zeGateway {
		return sharedAccessClient{}, errors.New("daemon PPP local address differs from IPCP gateway")
	}
	if uint32(serverPPP.index) != row.Index {
		return sharedAccessClient{}, errors.New("daemon published another native PPP index")
	}
	return sharedAccessClient{peer, vlan, ppp, row, pid}, nil
}

func sharedAccessWaitDeparture(ctx context.Context, lab interoplab.CheckerLab, departed sharedAccessClient, want []sharedAccessRow) error {
	_, _, err := interoplab.Wait(ctx, sharedAccessWait("independent departure of "+departed.peer.name), func(probe context.Context) (bool, error) {
		rows, err := sharedAccessRows(probe, lab)
		if err != nil {
			return false, err
		}
		if len(want) == 1 {
			if err := sharedAccessRequireSurvivor(rows, departed.row, want[0]); err != nil {
				return false, err
			}
		} else if len(rows) != 0 {
			return false, errors.New("last departing client left daemon session state")
		}
		for _, selected := range []struct{ peer, device string }{{departed.peer.name, departed.peer.device}, {zeImageName, departed.row.Device}} {
			result, err := lab.Exec(probe, selected.peer, []string{"ip", "-j", commandLink, commandShow, sharedAccessCommandType, "ppp"}, nil)
			if err != nil {
				return false, err
			}
			links, err := sharedAccessDecodeLinks([]byte(result.Stdout))
			if err != nil {
				return false, err
			}
			for _, link := range links {
				if link.Name == selected.device {
					return false, errors.New("departed original PPP interface remains")
				}
			}
		}
		result, err := lab.Exec(probe, departed.peer.name, []string{"test", "!", "-e", departed.peer.pidPath()}, nil)
		if err != nil {
			return false, err
		}
		if result.ExitCode != 0 {
			return false, errors.New("departed pppd still owns its PID file")
		}
		process, err := lab.Exec(probe, departed.peer.name, []string{"test", "!", "-d", "/proc/" + strconv.Itoa(departed.pid)}, nil)
		if err != nil {
			return false, err
		}
		if process.ExitCode != 0 {
			return false, errors.New("original pppd process has not exited")
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	return err
}

func sharedAccessDiagnostics(ctx context.Context, lab interoplab.CheckerLab, problem error) error {
	if problem == nil {
		return nil
	}
	read, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, peer := range sharedAccessPeers() {
		result, err := lab.Exec(read, peer.name, []string{commandCat, peer.logPath()}, nil)
		if err != nil {
			problem = errors.Join(problem, err)
			continue
		}
		problem = fmt.Errorf("%w\n--- %s pppd trace ---\n%s", problem, peer.name, result.Stdout)
	}
	return appendDiagnostics(read, lab, problem, zeImageName, "client-a", "client-b")
}
