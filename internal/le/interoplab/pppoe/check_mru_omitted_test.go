//go:build ze_l2tp

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const omittedMRUTrace = `Connect: ppp0 <--> eth0
sent [LCP ConfReq id=0x1 <magic 0x1111>]
rcvd [LCP ConfReq id=0x1 <mru 1492> <auth chap MD5> <magic 0x2222>]
sent [LCP ConfRej id=0x1 <mru 1492>]
rcvd [LCP ConfAck id=0x1 <magic 0x1111>]
rcvd [LCP ConfReq id=0x2 <auth chap MD5> <magic 0x2222>]
sent [LCP ConfAck id=0x2 <auth chap MD5> <magic 0x2222>]
rcvd [CHAP Challenge id=0x3]
sent [CHAP Response id=0x3 name = "alice"]
rcvd [CHAP Success id=0x3]
rcvd [IPCP ConfAck id=0x4]
`

func TestOmittedMRULCPEvidence(t *testing.T) {
	if err := omittedMRULCPEvidence(omittedMRUTrace); err != nil {
		t.Fatalf("valid omitted-MRU session refused: %v", err)
	}
	for name, trace := range map[string]string{
		"client option present": strings.Replace(omittedMRUTrace, "sent [LCP ConfReq id=0x1 ", "sent [LCP ConfReq id=0x1 <mru 1492> ", 1),
		"missing client Ack":    strings.Replace(omittedMRUTrace, "rcvd [LCP ConfAck", "ignored [LCP ConfAck", 1),
		"wrong client Ack ID":   strings.Replace(omittedMRUTrace, "rcvd [LCP ConfAck id=0x1", "rcvd [LCP ConfAck id=0x9", 1),
		"changed client Ack":    strings.Replace(omittedMRUTrace, "rcvd [LCP ConfAck id=0x1 <magic 0x1111>", "rcvd [LCP ConfAck id=0x1 <magic 0x9999>", 1),
		"missing Ze Ack":        strings.Replace(omittedMRUTrace, "sent [LCP ConfAck", "ignored [LCP ConfAck", 1),
		"second attempt":        omittedMRUTrace + "Connect: ppp1 <--> eth0\n",
		"PAP downgrade":         omittedMRUTrace + "sent [PAP AuthReq id=0x5]\n",
		"malformed Identifier":  strings.Replace(omittedMRUTrace, "sent [LCP ConfReq id=0x1", "sent [LCP ConfReq id=invalid", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := omittedMRULCPEvidence(trace); err == nil {
				t.Fatal("incomplete or incorrect wire evidence accepted")
			}
		})
	}
}

func TestOmittedMRUZeLink(t *testing.T) {
	if err := omittedMRUZeLink(`[{"ifname":"ppp7","mtu":1492}]`, "ppp7"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`[{"ifname":"ppp7","mtu":1500}]`,
		`[{"ifname":"ppp8","mtu":1492}]`,
		`[{"ifname":"ppp7"}]`,
		`[]`,
		`null`,
		`not JSON`,
	} {
		if err := omittedMRUZeLink(body, "ppp7"); err == nil {
			t.Fatalf("unmeasured or incorrect interface accepted: %s", body)
		}
	}
}

// The control fails with Ze's uncapped1500 MTU even though the independent
// peer completed CHAP/IPCP. A working capped case still must exercise traffic.
func TestOmittedMRUScenarioMeasuresZeAndTraffic(t *testing.T) {
	for _, mtu := range []int{1492, 1500} {
		t.Run(fmt.Sprint(mtu), func(t *testing.T) {
			started, stopped := false, false
			pinged := make(map[string]bool)
			lab := &fakeLab{}
			lab.logsFn = func(string, int) (interoplab.LogResult, error) {
				return interoplab.LogResult{Available: true, Text: "PPPoE interface configured"}, nil
			}
			lab.detachedFn = func(peer string, argv []string) error {
				if peer != clientImageName || len(argv) != 3 || !strings.Contains(argv[2], "default-mru nopersist noipv6") {
					return errors.New("dial did not explicitly suppress the peer MRU option")
				}
				started = true
				return nil
			}
			lab.execFn = func(peer string, argv []string) (interoplab.CommandResult, error) {
				command := strings.Join(argv, " ")
				switch {
				case command == "sh -c rm -f /var/log/ppp/dial.log":
					return interoplab.CommandResult{}, nil
				case command == "ip -o link show type ppp":
					if started && !stopped {
						return interoplab.CommandResult{Stdout: "9: ppp0: <POINTOPOINT> mtu 1492\n"}, nil
					}
					return interoplab.CommandResult{}, nil
				case command == "ip -o addr show dev ppp0":
					if peer == zeImageName {
						return interoplab.CommandResult{Stdout: "9: ppp0 inet 10.20.0.1 peer 10.20.0.2/32\n"}, nil
					}
					return interoplab.CommandResult{Stdout: "9: ppp0 inet 10.20.0.2 peer 10.20.0.1/32\n"}, nil
				case command == "ip -o route show 10.20.0.1":
					return interoplab.CommandResult{Stdout: "10.20.0.1 dev ppp0 scope link src 10.20.0.2\n"}, nil
				case strings.HasPrefix(command, "ping -c 3 -W 3 -I ppp0 "):
					pinged[peer] = true
					return interoplab.CommandResult{}, nil
				case command == "pkill -TERM -x pppd":
					stopped = true
					return interoplab.CommandResult{}, nil
				case command == "pgrep -x pppd":
					if stopped {
						return interoplab.CommandResult{ExitCode: 1}, errors.New("exit 1")
					}
					return interoplab.CommandResult{}, nil
				case command == "sh -c cat /var/log/ppp/dial.log 2>/dev/null":
					return interoplab.CommandResult{Stdout: omittedMRUTrace}, nil
				default:
					return interoplab.CommandResult{}, fmt.Errorf("unexpected Exec: %s %s", peer, command)
				}
			}
			lab.queryFn = func(peer string, argv []string) (string, error) {
				command := strings.Join(argv, " ")
				switch {
				case peer == zeImageName && strings.HasPrefix(command, "curl "):
					if started && !stopped {
						return `{"status":"done","data":[{"sid":7,"service-name":"internet","interface":"eth0"}]}`, nil
					}
					return `{"status":"done","data":[]}`, nil
				case peer == clientImageName && command == "sh -c cat /var/log/ppp/dial.log 2>/dev/null":
					if stopped {
						return omittedMRUTrace + "Sent PADT\n", nil
					}
					return omittedMRUTrace, nil
				case peer == zeImageName && command == "ip -j link show dev ppp0":
					return fmt.Sprintf(`[{"ifname":"ppp0","mtu":%d}]`, mtu), nil
				case peer == zeImageName && command == "cat /proc/net/pppoe":
					return "Id Address Device\n", nil
				default:
					return "", fmt.Errorf("unexpected Query: %s %s", peer, command)
				}
			}
			err := checkOmittedPeerMRU(context.Background(), &interoplab.CheckContext{Lab: lab})
			if mtu == 1500 {
				if err == nil || !strings.Contains(err.Error(), "1492 transmit cap") {
					t.Fatalf("uncapped Ze interface accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !pinged[zeImageName] || !pinged[clientImageName] || !stopped {
				t.Fatalf("traffic/cleanup not exercised: pinged=%v stopped=%v", pinged, stopped)
			}
		})
	}
}
