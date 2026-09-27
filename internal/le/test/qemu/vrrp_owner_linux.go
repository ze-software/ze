//go:build linux

// Design: docs/architecture/testing/qemu-integration.md -- interop labs need a QEMU path too
// Related: vrrp_keepalived_linux.go -- the lab this scenario runs in

package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The owner scenario is the QEMU path of the Docker interop scenario
// vrrp-v2-owner-keepalived. ze owns vrrpOwnerAddress: it is a real address of
// ze's interface and the group's only virtual address, so ze runs VRID
// vrrpOwnerVRID at priority 255 (RFC 3768 Section 5.3.4). keepalived claims the
// same VRID and address at 255 too, which is a misconfiguration on purpose.
//
// The addresses are ordered so that the scenario goes red if either of two
// defects returns. keepalived sends from vrrpKAAddress (.252), above ze's
// source vrrpZeAddress (.251), so without the owner discard the
// equal-priority tie-break of RFC 3768 Section 6.4.3 demotes ze. The owned
// address (.254) sits above keepalived's, so a tie-break that compared the
// sender with the first virtual address instead of the address ze sends from
// would win and hide the missing discard.
const (
	vrrpOwnerVRID     = 20
	vrrpOwnerAddress  = "192.0.2.254"
	vrrpOwnerOnVMAC   = vrrpOwnerAddress + "/32"
	vrrpOwnerOnPeer   = vrrpOwnerAddress + "/" + vrrpPrefixLength
	vrrpOwnerHold     = 8 * time.Second
	vrrpOwnerDiscards = 3
	// vrrpOwnerDiscardReason is the packet-errors key `show vrrp statistics`
	// counts an owner discard under (packet.ReasonOwner).
	vrrpOwnerDiscardReason = "owner"
)

// The CLI account the scenario reads `show vrrp statistics` through. The
// password is a fixture; its bcrypt hash is computed at run time, so the
// config and the login cannot disagree.
const (
	vrrpCLIUser     = "interop"
	vrrpCLIPassword = "testpass" // #nosec G101 -- fixture-only account inside a throwaway guest namespace
	vrrpCLIPort     = "2222"
)

func vrrpZeOwnerConfig(names vrrpNames, passwordHash string) []byte {
	var tb textbuf.Buffer
	return tb.Str("interface {\n").
		Str("    backend netlink;\n").
		Str("    ethernet ").Str(names.zeVeth).Str(" {\n").
		Str("        unit 0 {\n").
		Str("            ipv4 {\n").
		Str("                address [ ").Str(vrrpZeAddress).Byte('/').Str(vrrpPrefixLength).
		Byte(' ').Str(vrrpOwnerOnPeer).Str(" ];\n").
		Str("                vrrp {\n").
		Str("                    group owner {\n").
		Str("                        vrid ").Int(vrrpOwnerVRID).Str(";\n").
		Str("                        version 2;\n").
		Str("                        virtual-address [ ").Str(vrrpOwnerAddress).Str(" ];\n").
		Str("                        advertise-interval-milliseconds ").Int(vrrpAdvertMS).Str(";\n").
		Str("                    }\n").
		Str("                }\n").
		Str("            }\n").
		Str("        }\n").
		Str("    }\n").
		Str("}\n").
		Str("system {\n    authentication {\n        user ").Str(vrrpCLIUser).
		Str(" {\n            password ").Quoted(passwordHash).Str(";\n        }\n    }\n}\n").
		Str("environment {\n    ssh {\n        enabled true;\n        server main {\n").
		Str("            ip 127.0.0.1;\n            port ").Str(vrrpCLIPort).Str(";\n").
		Str("        }\n    }\n}\n").
		Bytes()
}

// vrrpKeepalivedOwnerConfig makes keepalived a second VRRPv2 router claiming
// priority 255 for the VRID ze owns. It speaks unicast, bound to its own
// address, so it never hears ze: keepalived treats a priority-255
// advertisement heard while it is itself at 255 as a configuration error and
// drops itself to 254, which would end the stimulus after one advertisement.
// ze's raw protocol-112 socket takes a unicast advertisement as it takes a
// multicast one.
func vrrpKeepalivedOwnerConfig(names vrrpNames) []byte {
	var tb textbuf.Buffer
	return tb.Str("global_defs {\n").
		Str("    vrrp_garp_master_refresh 0\n").
		Str("}\n").
		Str("vrrp_instance owner {\n").
		Str("    state MASTER\n").
		Str("    interface ").Str(names.kaVeth).Byte('\n').
		Str("    virtual_router_id ").Int(vrrpOwnerVRID).Byte('\n').
		Str("    priority 255\n").
		Str("    advert_int 1\n").
		Str("    version 2\n").
		Str("    unicast_src_ip ").Str(vrrpKAAddress).Byte('\n').
		Str("    unicast_peer {\n").
		Str("        ").Str(vrrpZeAddress).Byte('\n').
		Str("    }\n").
		Str("    virtual_ipaddress {\n").
		Str("        ").Str(vrrpOwnerOnPeer).Byte('\n').
		Str("    }\n").
		Str("}\n").
		Bytes()
}

// runOwnerKeepsTheAddress proves the VRRPv2 owner discard end to end: ze keeps
// the owned address on its virtual-MAC interface while keepalived advertises a
// conflicting priority 255 from a greater address, and ze counts each discard.
//
// RFC 3768 Section 7.1: "MUST verify that the VRID is configured on the
// receiving interface and the local router is not the IP Address owner
// (Priority equals 255 (decimal))." The owner half of that check is what this
// scenario exercises.
func (l *vrrpLab) runOwnerKeepsTheAddress(ctx context.Context) error {
	var tb textbuf.Buffer
	hash, err := bcrypt.GenerateFromPassword([]byte(vrrpCLIPassword), bcrypt.MinCost)
	if err != nil {
		return err
	}
	if err := l.startCapture(ctx); err != nil {
		return err
	}
	if _, err := l.startZe(ctx, vrrpZeOwnerConfig(l.names, string(hash))); err != nil {
		return err
	}
	if err := l.waitZeMaster(ctx); err != nil {
		return err
	}
	if err := l.waitAddress(ctx, l.names.zeNS, vrrpOwnerOnVMAC, vrrpZeMasterTimeout); err != nil {
		return err
	}
	// Only state changes logged after keepalived starts count, so ze's own
	// start-up transition to master is not read as a demotion.
	logBefore := len(l.zeLines.snapshot())
	if err := l.startKeepalived(ctx, vrrpKeepalivedOwnerConfig(l.names)); err != nil {
		return err
	}
	if err := l.waitAddress(ctx, l.names.kaNS, vrrpOwnerOnPeer, vrrpKAStateTimeout); err != nil {
		return fmt.Errorf("keepalived never claimed the owned address, so it sent no conflicting advertisement: %w", err)
	}
	l.details = append(l.details, tb.Str("  stimulus: keepalived holds ").Str(vrrpOwnerOnPeer).
		Str(" and advertises prio 255 from ").Str(vrrpKAAddress).Str(" to ").Str(vrrpZeAddress).String())
	tb.Reset()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(vrrpOwnerHold):
	}
	if !l.hasAddress(ctx, l.names.zeNS, vrrpOwnerOnVMAC) {
		return fmt.Errorf("ze lost %s after %s of keepalived's prio 255 advertisements", vrrpOwnerOnVMAC, vrrpOwnerHold)
	}
	for _, line := range l.zeLines.snapshot()[logBefore:] {
		if strings.Contains(line, "vrrp: state change") && !strings.Contains(line, "to=master") {
			return errors.New(tb.Str("ze left master while keepalived advertised prio 255: ").
				Str(strings.TrimSpace(line)).String())
		}
	}
	discards, err := l.ownerDiscards(ctx)
	if err != nil {
		return err
	}
	if discards < vrrpOwnerDiscards {
		return fmt.Errorf("show vrrp statistics counts %d owner discard(s), want at least %d", discards, vrrpOwnerDiscards)
	}
	l.details = append(l.details, tb.Str("  owner: ze kept ").Str(vrrpOwnerOnVMAC).
		Str(" on its virtual-MAC interface for ").Str(vrrpOwnerHold.String()).
		Str(" and counted ").Int(int64(discards)).Str(" owner discards").String())
	return nil
}

func (l *vrrpLab) hasAddress(ctx context.Context, namespace, address string) bool {
	result, err := guestRun(ctx, namespace, []string{"ip", "-o", "-f", "inet", ipObjectAddr}, nil)
	if err != nil {
		return false
	}
	var tb textbuf.Buffer
	return result.Code == 0 && strings.Contains(result.Stdout, tb.Byte(' ').Str(address).Byte(' ').String())
}

func (l *vrrpLab) waitAddress(ctx context.Context, namespace, address string, timeout time.Duration) error {
	err := waitGuest(ctx, timeout, guestPollInterval, func() (bool, error) {
		return l.hasAddress(ctx, namespace, address), nil
	})
	if err != nil {
		return fmt.Errorf("%s never appeared in namespace %s: %w", address, namespace, err)
	}
	return nil
}

// ownerDiscards asks the running daemon, through its own CLI, how many
// advertisements the owner group discarded under the owner rule.
func (l *vrrpLab) ownerDiscards(ctx context.Context) (uint64, error) {
	store := filepath.Join(l.work, "cli")
	var tb textbuf.Buffer
	seed := tb.Str("printf '%s\\n%s\\n127.0.0.1\\n%s\\n' ").Str(vrrpCLIUser).Byte(' ').
		Str(vrrpCLIPassword).Byte(' ').Str(vrrpCLIPort).Str(" | ZE_CONFIG_DIR=").Str(store).
		Byte(' ').Str(l.binary).Str(" init >/dev/null").String()
	query := tb.Reset().Str("[ -d ").Str(store).Str(" ] || ").Str(seed).Str("; ZE_CONFIG_DIR=").Str(store).
		Str(" ZE_SSH_PASSWORD=").Str(vrrpCLIPassword).Byte(' ').Str(l.binary).
		Str(" cli -c 'show vrrp statistics' --user ").Str(vrrpCLIUser).Str(" --format json").String()
	result, err := guestRun(ctx, l.names.zeNS, []string{"sh", "-c", query}, os.Environ())
	if err != nil {
		return 0, err
	}
	if result.Code != 0 {
		return 0, fmt.Errorf("show vrrp statistics exited %d: %s%s", result.Code, result.Stdout, result.Stderr)
	}
	// cmd_show.go answers {"vrrp-statistics": [statisticsView]}, and the CLI
	// prints the list under that single key, measured on the runtime kernel.
	var views []struct {
		VRID         int               `json:"vrid"`
		PacketErrors map[string]uint64 `json:"packet-errors"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &views); err != nil {
		return 0, fmt.Errorf("show vrrp statistics is not the expected JSON list (%w): %s", err, result.Stdout)
	}
	for _, view := range views {
		if view.VRID == vrrpOwnerVRID {
			return view.PacketErrors[vrrpOwnerDiscardReason], nil
		}
	}
	return 0, fmt.Errorf("show vrrp statistics names no VRID %d: %s", vrrpOwnerVRID, result.Stdout)
}
