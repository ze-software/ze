// Design: docs/labs/pppoe-interop.md -- selection coverage of the native checker
//go:build ze_l2tp

package pppoe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// This scripted lab proves the checker asks BOTH peers and revisits the survivor.
// It is an oracle unit test; it does not manufacture or claim native evidence.
func TestSharedAccessCheckerVisitsBothPeers(t *testing.T) {
	for _, breakSecondTraffic := range []bool{false, true} {
		t.Run(fmt.Sprint(breakSecondTraffic), func(t *testing.T) {
			lab, pings := sharedAccessCheckerFixture(t, breakSecondTraffic)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			err := checkSharedAccess(ctx, &interoplab.CheckContext{Lab: lab})
			if breakSecondTraffic {
				if err == nil {
					t.Fatal("checker ignored broken second client's bound traffic")
				}
				if !strings.Contains(err.Error(), "bound PPP traffic failed") {
					t.Fatalf("wrong failure: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pings["client-a"] != 1 {
				t.Fatalf("first-client ping count=%d", pings["client-a"])
			}
			if pings["client-b"] != 2 {
				t.Fatalf("second-client ping count=%d, want before and after first departure", pings["client-b"])
			}
			if pings[zeImageName] != 3 {
				t.Fatal("checker omitted reverse traffic on an actual selected PPP device")
			}
		})
	}
}

func sharedAccessCheckerFixture(t *testing.T, breakSecondTraffic bool) (*fakeLab, map[string]int) {
	t.Helper()
	peers := sharedAccessPeers()
	clients := [2]sharedAccessClient{
		{peer: peers[0], vlan: sharedAccessVLANIdentity{11, "02:00:00:00:00:11"}, row: sharedAccessRow{SID: 11, MAC: "02:00:00:00:00:11", Access: sharedAccessDevice, Service: pppoeService, State: "session", Device: "ppp11", Index: 21}},
		{peer: peers[1], vlan: sharedAccessVLANIdentity{12, "02:00:00:00:00:12"}, row: sharedAccessRow{SID: 12, MAC: "02:00:00:00:00:12", Access: sharedAccessDevice, Service: pppoeService, State: "session", Device: "ppp12", Index: 22}},
	}
	ac := [6]byte{2, 0, 0, 0, 0, 1}
	capture := base64.StdEncoding.EncodeToString(buildCapture(t, sharedAccessTestFrames(t, ac, &clients)...))
	var started, stopped [2]bool
	pings := map[string]int{}
	indexFor := func(peer string) int {
		for i, p := range peers {
			if p.name == peer {
				return i
			}
		}
		return -1
	}
	lab := &fakeLab{}
	lab.logsFn = func(string, int) (interoplab.LogResult, error) {
		return interoplab.LogResult{Available: true, Text: "PPPoE interface configured"}, nil
	}
	lab.detachedFn = func(peer string, args []string) error {
		if peer == zeImageName {
			if !strings.Contains(strings.Join(args, " "), "tcpdump") {
				return errors.New("not original capture")
			}
			return nil
		}
		i := indexFor(peer)
		if i < 0 {
			return errors.New("wrong client selected")
		}
		started[i] = true
		return nil
	}
	lab.execFn = func(peer string, args []string) (interoplab.CommandResult, error) {
		joined := strings.Join(args, " ")
		i := indexFor(peer)
		ok := func(out string) (interoplab.CommandResult, error) { return interoplab.CommandResult{Stdout: out}, nil }
		if strings.HasPrefix(joined, "ip link add ") {
			return ok("")
		}
		if strings.HasPrefix(joined, "ip link set ") {
			return ok("")
		}
		if strings.HasPrefix(joined, "ip -d -j link show dev ") {
			mac := "02:00:00:00:00:01"
			index := 10
			if i >= 0 {
				mac = clients[i].vlan.mac
				index = clients[i].vlan.index
			}
			return ok(fmt.Sprintf(`[{"ifindex":%d,"ifname":"eth0.100","address":%q,"flags":["UP"],"linkinfo":{"info_kind":"vlan","info_data":{"id":100}}}]`, index, mac))
		}
		if args[0] == "sh" {
			if peer == zeImageName {
				return ok("")
			}
			if i < 0 {
				return interoplab.CommandResult{}, errors.New("wrong process peer")
			}
			if strings.Contains(args[2], "kill -TERM") {
				stopped[i] = true
			}
			return ok(fmt.Sprint(200 + i))
		}
		if args[0] == "test" {
			if i < 0 {
				return interoplab.CommandResult{}, errors.New("wrong exit peer")
			}
			if !stopped[i] {
				return interoplab.CommandResult{}, errors.New("process remains")
			}
			return ok("")
		}
		if strings.HasPrefix(joined, "ip -j link show type ppp") {
			var links []map[string]any
			for n := range clients {
				client := &clients[n]
				if !started[n] {
					continue
				}
				if stopped[n] {
					continue
				}
				if peer == zeImageName {
					links = append(links, map[string]any{"ifname": client.row.Device})
					continue
				}
				if n == i {
					links = append(links, map[string]any{"ifname": client.peer.device})
				}
			}
			if links == nil {
				return ok("[]")
			}
			raw, err := json.Marshal(links)
			return interoplab.CommandResult{Stdout: string(raw)}, err
		}
		if strings.HasPrefix(joined, "ip -j addr show dev ") {
			device := args[len(args)-1]
			for n := range clients {
				client := &clients[n]
				if !started[n] {
					continue
				}
				if stopped[n] {
					continue
				}
				local := fmt.Sprintf("10.20.0.%d", n+2)
				remote := zeGateway
				index := 31 + n
				if peer == zeImageName {
					if device != client.row.Device {
						continue
					}
					remote, local = local, zeGateway
					index = int(client.row.Index)
				} else if n != i {
					continue
				}
				return ok(fmt.Sprintf(`[{"ifindex":%d,"ifname":%q,"flags":["UP","POINTOPOINT"],"addr_info":[{"family":"inet","local":%q,"address":%q,"prefixlen":32}]}]`, index, device, local, remote))
			}
			return interoplab.CommandResult{}, errors.New("no selected PPP link")
		}
		if strings.HasPrefix(joined, "ip -j route get ") {
			device := ""
			if i >= 0 {
				device = peers[i].device
			} else {
				for n := range clients {
					client := &clients[n]
					if args[len(args)-1] == fmt.Sprintf("10.20.0.%d", n+2) {
						device = client.row.Device
					}
				}
			}
			return ok(fmt.Sprintf(`[{"dev":%q}]`, device))
		}
		if args[0] == pingCommand {
			pings[peer]++
			if peer == "client-b" {
				if breakSecondTraffic {
					return interoplab.CommandResult{ExitCode: 1}, nil
				}
			}
			return ok("3 received")
		}
		return interoplab.CommandResult{}, errors.New("unexpected exec: " + peer + " " + joined)
	}
	lab.queryFn = func(peer string, args []string) (string, error) {
		if args[0] == "curl" {
			if strings.Contains(args[len(args)-1], "show pppoe sessions") {
				rows := []sharedAccessRow{}
				for i := range clients {
					client := &clients[i]
					if started[i] {
						if !stopped[i] {
							rows = append(rows, client.row)
						}
					}
				}
				raw, err := json.Marshal(rows)
				return `{"status":"done","data":` + string(raw) + `}`, err
			}
			return `{"status":"done","data":{"session-count":0}}`, nil
		}
		if args[0] == commandCat {
			i := indexFor(peer)
			if i < 0 {
				return "", errors.New("wrong log peer")
			}
			return "sent [LCP ConfReq <mru 1492>\nrcvd [LCP ConfAck <mru 1492>\nrcvd [LCP ConfReq <auth chap MD5>\nsent [LCP ConfAck <auth chap MD5>\nsent [CHAP Response name = \"" + peers[i].username + "\"\nrcvd [CHAP Challenge\nrcvd [CHAP Success\nrcvd [IPCP ConfAck\n", nil
		}
		if args[0] == "sh" {
			return capture, nil
		}
		return "", errors.New("unexpected query")
	}
	return lab, pings
}
