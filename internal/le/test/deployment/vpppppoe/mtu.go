// Design: docs/labs/pppoe-interop.md -- distinguish the VPP MTU from the peer MTU.
// Related: check.go -- preserve the separate 1492-byte session assertions.
package testvpppppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// A smaller VPP limit separates its enforcement from the client's 1492-byte
// limit. Restore even a failed set: its transport may have lost the answer
// after VPP applied the change.
func checkIndependentMTU(ctx context.Context, lab interoplab.CheckerLab, device string) (err error) {
	ctx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	output, err := query(ctx, lab, clientPeer, "ip", "-j", "link", showCommand, "dev", device)
	if err != nil {
		return err
	}
	if err := independentClientMTU(output, device); err != nil {
		return err
	}
	if err := requireVPPIP4MTU(ctx, lab, 1492); err != nil {
		return err
	}
	defer func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.WithoutCancel(ctx), controlCommandBudget)
		defer restoreCancel()
		_, restoreErr := cli(restoreCtx, lab, "set", interfaceKeyword, mtuToken, "ip4", "1492", sessionInterface)
		if restoreErr == nil {
			restoreErr = requireVPPIP4MTU(restoreCtx, lab, 1492)
		}
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore VPP session MTU to 1492: %w", restoreErr))
		}
	}()
	if _, err = cli(ctx, lab, "set", interfaceKeyword, mtuToken, "ip4", "1400", sessionInterface); err != nil {
		return err
	}
	if err := requireVPPIP4MTU(ctx, lab, 1400); err != nil {
		return err
	}
	if _, err = query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "1", "-W", "3", "-M", "do", "-s", "1372", peerAddress); err != nil {
		return fmt.Errorf("1400-byte positive-control traffic failed: %w", err)
	}
	oversize, oversizeErr := lab.Exec(ctx, trafficPeer, []string{pingCommand, "-n", "-c", "1", "-W", "3", "-M", "do", "-s", "1373", peerAddress}, nil)
	if oversizeErr == nil {
		return errors.New("VPP forwarded a 1401-byte DF packet through a 1400-byte session")
	}
	code, exited := interoplab.ExecExitCode(oversizeErr)
	if !exited {
		return oversizeErr
	}
	if code != 1 {
		return oversizeErr
	}
	// iputils prints the discovered boundary as "mtu = N" or "mtu=N".
	// Match its field, not a packet count or a longer MTU such as 14000.
	if !regexp.MustCompile(`\bmtu\s*=\s*1400\b`).MatchString(oversize.Stdout + oversize.Stderr) {
		return fmt.Errorf("1401-byte ping failed without a measured MTU of 1400: %w", oversizeErr)
	}
	return nil
}

func independentClientMTU(output, device string) error {
	var links []struct {
		Name string `json:"ifname"`
		MTU  int    `json:"mtu"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return err
	}
	if len(links) != 1 {
		return fmt.Errorf("client PPP MTU is not uniquely measured: %s", output)
	}
	if links[0].Name != device {
		return fmt.Errorf("client MTU measurement does not name %s: %s", device, output)
	}
	if links[0].MTU <= 1400 {
		return fmt.Errorf("client PPP MTU must exceed the independent 1400-byte VPP probe: %s", output)
	}
	return nil
}

// Pinned format_vnet_sw_interface_mtu prints L3/IP4/IP6/MPLS in one field.
// Exactly one named row must contain the requested IP4 value.
func requireVPPIP4MTU(ctx context.Context, lab interoplab.CheckerLab, wanted int) error {
	output, err := cli(ctx, lab, showCommand, interfaceKeyword, sessionInterface)
	if err != nil {
		return err
	}
	found := false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != sessionInterface {
			continue
		}
		if found {
			return fmt.Errorf("VPP session MTU readback is ambiguous: %s", output)
		}
		found = true
		if len(fields) < 4 {
			return fmt.Errorf("VPP session MTU row is incomplete: %s", output)
		}
		mtus := strings.Split(fields[3], "/")
		if len(mtus) != 4 {
			return fmt.Errorf("VPP session MTU tuple is incomplete: %s", output)
		}
		for _, mtu := range mtus {
			if _, err = strconv.ParseUint(mtu, 10, 32); err != nil {
				return fmt.Errorf("VPP session MTU tuple is invalid: %s: %w", output, err)
			}
		}
		if mtus[1] != strconv.Itoa(wanted) {
			return fmt.Errorf("VPP session IPv4 MTU is not %d: %s", wanted, output)
		}
	}
	if !found {
		return fmt.Errorf("VPP session MTU row is absent: %s", output)
	}
	return nil
}
