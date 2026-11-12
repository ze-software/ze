// Design: docs/architecture/mpls/mpls-kernel.md -- `show mpls forwarding` on the VPP data plane
// Related: register_vpp_mpls_ldp_pop.go -- the LDP-on-VPP config and stub-log evidence this driver reuses
// Related: misc_fixture_vpp.go -- the VPP stub, the ze process and the stub log reader
//
// vpp/vpp-mpls-forwarding-show drives `ldp { }` under `interface { backend
// vpp; }` against the VPP stub, waits until the stub log shows fib-vpp
// installed LDP's egress pop in MPLS table 0, then asks the running daemon
// `show mpls forwarding | json` over its SSH CLI. It passes only when the
// answer lists that label as a pop: the command read VPP MPLS table 0 (the
// stub answers mpls_route_dump from the routes fib-vpp installed), not the
// kernel AF_MPLS table, which holds no Ze label on this host.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	Register("vpp/vpp-mpls-forwarding-show", vppMPLSForwardingShowDriver)
}

// vppForwardingShowUser is the login the driver runs the command as.
const vppForwardingShowUser = "ci"

// vppForwardingShowLogin answers the `system` block that lets the driver log
// in over SSH, its password hashed by the ze under test (`ze passwd`).
func vppForwardingShowLogin(ctx context.Context) (string, error) {
	passwd := exec.CommandContext(ctx, "ze", "passwd")
	passwd.Stdin = strings.NewReader(valueSecret + "\n")
	var hash bytes.Buffer
	passwd.Stdout = &hash
	passwd.Stderr = os.Stderr
	if err := passwd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w", err)
	}
	return "\nsystem {\n    authentication {\n        user " + vppForwardingShowUser + " {\n" +
		"            password \"" + strings.TrimSpace(hash.String()) + "\"\n" +
		"            profile [ admin ]\n        }\n    }\n" +
		"    authorization {\n        profile admin { run { default-action allow; } edit { default-action allow; } }\n    }\n}\n", nil
}

// vppForwardingShowEntries runs `show mpls forwarding | json` against the
// daemon whose SSH address is in addressPath and decodes its entries.
func vppForwardingShowEntries(ctx context.Context, addressPath, configDir string) ([]map[string]any, error) {
	address, err := os.ReadFile(addressPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(string(address)))
	if err != nil {
		return nil, fmt.Errorf("SSH address %q: %w", address, err)
	}
	cli := exec.CommandContext(ctx, "ze", "cli", "-c", "show mpls forwarding | json")
	cli.Env = miscEnvironment(map[string]string{
		envSSHHost: host, envSSHPort: port, envSSHUsername: vppForwardingShowUser,
		envSSHPassword: valueSecret, envConfigDir: configDir,
	})
	// The answer is stdout alone: client logs go to stderr.
	var diagnostics bytes.Buffer
	cli.Stderr = &diagnostics
	output, err := cli.Output()
	if err != nil {
		return nil, fmt.Errorf("ze cli -c \"show mpls forwarding | json\": %w: %s%s", err, output, diagnostics.Bytes())
	}
	// The CLI prints the entries as a bare JSON array.
	var entries []map[string]any
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("show mpls forwarding answered no JSON array: %w: %s", err, output)
	}
	return entries, nil
}

// listsPop reports whether entries hold label as a pop.
func listsPop(entries []map[string]any, label uint32) bool {
	for _, entry := range entries {
		if entry["in-label"] == float64(label) && entry["operation"] == "pop" {
			return true
		}
	}
	return false
}

func vppMPLSForwardingShowDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("vpp-mpls-forwarding-show fixture takes no arguments")
	}
	dir, socket, log, err := vppWorkPaths("vpp-fwd-show-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup
	login, err := vppForwardingShowLogin(ctx)
	if err != nil {
		return err
	}
	stub, err := startVPPStub(ctx, socket, log, 30)
	if err != nil {
		return err
	}
	defer stopFixtureProcess(stub, 2*time.Second)
	if !waitForFile(ctx, socket, 100, 50*time.Millisecond) {
		return errors.New("stub socket did not appear")
	}
	addressPath := filepath.Join(dir, "ssh.addr")
	ze, err := startVPPZe(ctx, vppMPLSLDPPopConfig(socket)+login, map[string]string{
		envConfigDir: dir, envSSHEphemeral: addressPath, envLogVPP: logLevelInfo, envLogBGP: logLevelWarn,
	})
	if err != nil {
		return err
	}
	defer stopFixtureProcess(ze, 3*time.Second)

	var evidence mplsLDPPopEvidence
	var scanErr error
	Poll(ctx, 150, 100*time.Millisecond, func() bool {
		evidence, scanErr = readMPLSLDPPopEvidence(log)
		_, found := evidence.poppedLabel()
		return found || scanErr != nil
	})
	if scanErr != nil {
		return scanErr
	}
	label, found := evidence.poppedLabel()
	if !found {
		return errors.New("stub never saw fib-vpp install LDP's egress pop in MPLS table 0, so there is nothing to read back")
	}
	if !waitForFile(ctx, addressPath, 100, 50*time.Millisecond) {
		return fmt.Errorf("ze never wrote its SSH address to %s", addressPath)
	}

	entries, err := vppForwardingShowEntries(ctx, addressPath, dir)
	if err != nil {
		return err
	}
	if !listsPop(entries, label) {
		var dump textbuf.Buffer
		dump.Str("--- ze output ---\n").Str(ze.output.String()).Str("--- end ---\n")
		_ = dump.StdErr() //nolint:errcheck // evidence for a failure already being returned
		return fmt.Errorf("show mpls forwarding on the vpp data plane did not list LDP label %d as a pop: %v", label, entries)
	}
	var ok textbuf.Buffer
	ok.Str("OK: show mpls forwarding read VPP MPLS table 0 and listed LDP label ").Uint32(label).Str(" as a pop\n")
	return ok.StdErr()
}
