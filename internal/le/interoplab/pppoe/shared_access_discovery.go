// Design: docs/labs/pppoe-interop.md -- real shared-VLAN discovery evidence
//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const sharedAccessCapture = "/tmp/shared-access-discovery.pcap"
const sharedAccessCapturePID = "/tmp/shared-access-discovery.pid"
const sharedAccessCaptureMax = 1 << 20
const sharedAccessCaptureLabel = "capture"

func sharedAccessCaptureStart(ctx context.Context, lab interoplab.CheckerLab) error {
	// One original capture process is owned by this scenario's Ze container.
	// Suite teardown stops that container even if an earlier assertion fails.
	script := `printf '%s\n' "$$" > "$1"; exec tcpdump -U -i "$2" -w "$3" ether proto 0x8863`
	if err := lab.ExecDetached(ctx, zeImageName, []string{"sh", "-c", script, sharedAccessCaptureLabel, sharedAccessCapturePID, sharedAccessDevice, sharedAccessCapture}, nil); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 200 * time.Millisecond, Description: "shared VLAN capture header"}, func(probe context.Context) (bool, error) {
		script := `read -r pid < "$1" || exit 2
test "$(cat "/proc/$pid/comm")" = tcpdump || exit 3
test "$(wc -c < "$2")" -ge 24`
		result, err := lab.Exec(probe, zeImageName, []string{"sh", "-c", script, sharedAccessCaptureLabel, sharedAccessCapturePID, sharedAccessCapture}, nil)
		if err != nil {
			return false, err
		}
		return result.ExitCode == 0, nil
	}, func(ready bool) bool { return ready })
	return err
}

func sharedAccessCaptureCheck(ctx context.Context, lab interoplab.CheckerLab, clients *[2]sharedAccessClient) error {
	script := `read -r pid < "$1" || exit 2
case "$pid" in ''|*[!0-9]*) exit 3;; esac
test "$(cat "/proc/$pid/comm")" = tcpdump || exit 4
kill -INT "$pid"`
	if _, err := lab.Exec(ctx, zeImageName, []string{"sh", "-c", script, sharedAccessCaptureLabel, sharedAccessCapturePID}, nil); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 200 * time.Millisecond, Description: "original shared VLAN capture exit"}, func(probe context.Context) (bool, error) {
		result, err := lab.Exec(probe, zeImageName, []string{"sh", "-c", `read -r pid < "$1" || exit 2; test ! -d "/proc/$pid"`, sharedAccessCaptureLabel, sharedAccessCapturePID}, nil)
		if err != nil {
			return false, err
		}
		return result.ExitCode == 0, nil
	}, func(exited bool) bool { return exited })
	if err != nil {
		return err
	}
	encoded, err := query(ctx, lab, zeImageName, []string{"sh", "-c", `test "$(wc -c < "$1")" -le "$2" || exit 2; base64 "$1"`, sharedAccessCaptureLabel, sharedAccessCapture, fmt.Sprint(sharedAccessCaptureMax)})
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return err
	}
	access, err := sharedAccessReadVLAN(ctx, lab, zeImageName)
	if err != nil {
		return err
	}
	return sharedAccessDiscoveryEvidence(raw, access.mac, clients)
}

// Both real peer MACs need the four wire legs, including the actual admitted SID.
// Reusing another peer's PADS or one successful client cannot satisfy this.
func sharedAccessDiscoveryEvidence(raw []byte, ac string, clients *[2]sharedAccessClient) error {
	if len(raw) > sharedAccessCaptureMax {
		return errors.New("shared VLAN capture exceeds its bound")
	}
	reader, err := pcap.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return errors.New("shared VLAN discovery capture is not Ethernet")
	}
	var seen [2][4]bool
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if record.OriginalLen != len(record.Data) {
			return errors.New("shared discovery frame was truncated")
		}
		packet, err := discovery.ParseDiscovery(record.Data)
		if err != nil {
			return err
		}
		source := net.HardwareAddr(packet.SrcMAC[:]).String()
		target := net.HardwareAddr(packet.DstMAC[:]).String()
		for i := range clients {
			client := &clients[i]
			if source == client.vlan.mac {
				if packet.Code == discovery.CodePADI {
					seen[i][0] = true
				}
				if packet.Code == discovery.CodePADR {
					if target == ac {
						seen[i][2] = true
					}
				}
			}
			if source == ac {
				if target != client.vlan.mac {
					continue
				}
				if packet.Code == discovery.CodePADO {
					seen[i][1] = true
				}
				if packet.Code == discovery.CodePADS {
					if packet.SID == client.row.SID {
						seen[i][3] = true
					}
				}
			}
		}
	}
	for i, phases := range seen {
		for phase, observed := range phases {
			if !observed {
				return fmt.Errorf("%s lacks discovery leg %d for actual SID %d", clients[i].peer.name, phase+1, clients[i].row.SID)
			}
		}
	}
	return nil
}
