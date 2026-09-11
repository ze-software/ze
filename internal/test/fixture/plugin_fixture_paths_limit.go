// Design: docs/architecture/testing/ci-format.md -- independent OPEN sender facts
// Related: test/plugin/paths-limit-live.ci -- ordered wire assertions
// Related: ui_fixture_send_bgp.go -- the ephemeral SSH operator-CLI lifecycle
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("plugin/paths-limit-live", pathsLimitLive)
}

// pathsLimitLive drives separate operator commands, not an in-process session
// mock. A different-prefix UPDATE fences each suppressed announcement: the peer
// must read all preceding frames before accepting that sentinel. No quiet-time
// interval is used to infer absence, and the final assertion is the peer's exit.
func pathsLimitLive(ctx context.Context, args []string) error {
	session, err := startPathsLimitSession(ctx, args)
	if err != nil {
		return err
	}
	defer session.stop()

	// Inspect while the OPEN-negotiated session is still up, before the wire
	// script's final expected UPDATE lets the independent receiver exit.
	if err := checkPathsLimitPeerOutput(ctx, session); err != nil {
		return err
	}

	commands := []struct {
		line  string
		fence string
	}{
		// IPv4's remote limit is one, despite our own advertised limit of nine.
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 1 add 192.0.2.0/24", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 2 add 192.0.2.0/24", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000010000006318C63364"},
		// Same identity, changed attributes: replacement must pass at the limit.
		{"send bgp * update text origin igp next-hop 10.0.0.2 nlri ipv4/unicast path-information 1 add 192.0.2.0/24", ""},
		{"send bgp * update text nlri ipv4/unicast path-information 1 del 192.0.2.0/24", ""},
		// A withdrawal does not drain a hidden suppressed-path queue.
		{"send bgp * update text origin igp next-hop 10.0.0.5 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000050000006318C63364"},
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 2 add 192.0.2.0/24", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 3 add 192.0.2.0/24", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.2 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000020000006318C63364"},
		// IPv6 independently admits two identities while IPv4 is already full.
		{"send bgp * update text origin igp next-hop 2001:db8::1 nlri ipv6/unicast path-information 1 add 2001:db8:1::/64", ""},
		{"send bgp * update text origin igp next-hop 2001:db8::1 nlri ipv6/unicast path-information 2 add 2001:db8:1::/64", ""},
		{"send bgp * update text origin igp next-hop 2001:db8::1 nlri ipv6/unicast path-information 3 add 2001:db8:1::/64", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.3 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000030000006318C63364"},
		{"send bgp * update text origin igp next-hop 2001:db8::2 nlri ipv6/unicast path-information 2 add 2001:db8:1::/64", ""},
		{"send bgp * update text nlri ipv6/unicast path-information 1 del 2001:db8:1::/64", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.6 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000060000006318C63364"},
		{"send bgp * update text origin igp next-hop 2001:db8::1 nlri ipv6/unicast path-information 3 add 2001:db8:1::/64", ""},
		// Freeing IPv6 capacity must not free IPv4's occupied slot.
		{"send bgp * update text origin igp next-hop 10.0.0.1 nlri ipv4/unicast path-information 3 add 192.0.2.0/24", ""},
		{"send bgp * update text origin igp next-hop 10.0.0.4 nlri ipv4/unicast path-information 99 add 198.51.100.0/24", "4003040A0000040000006318C63364"},
	}
	for _, command := range commands {
		result := session.run(ctx, strings.Fields(command.line)...)
		if result.code != 0 {
			return fmt.Errorf("ze %s exit=%d: %s\npeer: %s", command.line, result.code, result.out, session.peer.output.String())
		}
		if command.fence != "" && !Poll(ctx, 200, 50*time.Millisecond, func() bool {
			return strings.Contains(strings.ToUpper(session.peer.output.String()), command.fence)
		}) {
			return fmt.Errorf("wire sentinel after %q never arrived: %s", command.line, session.peer.output.String())
		}
	}
	if err := waitFixtureProcess(ctx, session.peer, 20*time.Second); err != nil {
		return fmt.Errorf("PATHS-LIMIT wire sequence failed: %w\n%s", err, session.peer.output.String())
	}
	fmt.Println("OK: PATHS-LIMIT live IPv4 and IPv6 admission, replacement, withdrawal and retry")
	return nil
}

type pathsLimitPeerOutput struct {
	Send    map[string]uint16 `json:"send"`
	Receive map[string]uint16 `json:"receive"`
}

// checkPathsLimitPeerOutput drives the real fact producer, SSH command handlers,
// and both renderers. Different remote/local values expose swapped directions;
// different remote family values expose a lost per-family limit.
func checkPathsLimitPeerOutput(ctx context.Context, session *cliWireSession) error {
	wantSend := map[string]uint16{"ipv4/unicast": 1, "ipv6/unicast": 2}
	wantReceive := map[string]uint16{"ipv4/unicast": 9, "ipv6/unicast": 9}
	for _, surface := range []string{"capabilities", "detail"} {
		args := []string{"show", "bgp", "peer", "127.0.0.1", surface, "|", "json"}
		result := session.run(ctx, args...)
		if result.code != 0 {
			return fmt.Errorf("peer %s JSON exit=%d: %s", surface, result.code, result.out)
		}
		var limits *pathsLimitPeerOutput
		if surface == "capabilities" {
			var answer []struct {
				Peer                string `json:"peer"`
				NegotiationComplete bool   `json:"negotiation-complete"`
				Negotiated          struct {
					PathsLimit *pathsLimitPeerOutput `json:"paths-limit"`
				} `json:"negotiated"`
			}
			if err := json.Unmarshal([]byte(result.out), &answer); err != nil {
				return fmt.Errorf("peer capabilities JSON: %w: %s", err, result.out)
			}
			if len(answer) != 1 || answer[0].Peer != "127.0.0.1" || !answer[0].NegotiationComplete {
				return fmt.Errorf("peer capabilities lost established peer: %s", result.out)
			}
			limits = answer[0].Negotiated.PathsLimit
		} else {
			var answer struct {
				Peers map[string]struct {
					Capabilities struct {
						NegotiationComplete bool                  `json:"negotiation-complete"`
						PathsLimit          *pathsLimitPeerOutput `json:"paths-limit"`
					} `json:"capabilities"`
				} `json:"peers"`
			}
			if err := json.Unmarshal([]byte(result.out), &answer); err != nil {
				return fmt.Errorf("peer detail JSON: %w: %s", err, result.out)
			}
			peer, ok := answer.Peers["127.0.0.1"]
			if !ok || !peer.Capabilities.NegotiationComplete {
				return fmt.Errorf("peer detail lost established peer: %s", result.out)
			}
			limits = peer.Capabilities.PathsLimit
		}
		if limits == nil || !maps.Equal(limits.Send, wantSend) || !maps.Equal(limits.Receive, wantReceive) {
			return fmt.Errorf("peer %s lost PATHS-LIMIT send=%v receive=%v: %s", surface, wantSend, wantReceive, result.out)
		}

		args[len(args)-1] = "text"
		result = session.run(ctx, args...)
		if result.code != 0 {
			return fmt.Errorf("peer %s text exit=%d: %s", surface, result.code, result.out)
		}
		// Match the standard nested table's direction, family and value cells,
		// not padding, explanatory wording, or whole output.
		for _, cells := range [][]string{
			{"paths-limit"},
			{"receive", "ipv4/unicast", "9"},
			{"ipv6/unicast", "9"},
			{"send", "ipv4/unicast", "1"},
			{"ipv6/unicast", "2"},
		} {
			found := false
			for line := range strings.SplitSeq(result.out, "\n") {
				fields := strings.Fields(line)
				for i := 0; i+len(cells) <= len(fields); i++ {
					if slices.Equal(fields[i:i+len(cells)], cells) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf("peer %s text lost PATHS-LIMIT cells %v: %s", surface, cells, result.out)
			}
		}
	}
	fmt.Println("OK: PATHS-LIMIT live peer text and JSON show send IPv4=1 IPv6=2 and receive request=9")
	return nil
}

// Keep this fixture's configuration in its .ci: its own limit and the remote
// limit must be visibly different. Existing CLI fixtures remain unchanged.
func startPathsLimitSession(ctx context.Context, args []string) (*cliWireSession, error) {
	if len(args) != 1 {
		return nil, errors.New("paths-limit-live requires the leased BGP port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid BGP port %q", args[0])
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	template, err := os.ReadFile("paths-limit.conf")
	if err != nil {
		return nil, err
	}
	if strings.Count(string(template), "{{PASSWORD_HASH}}") != 1 {
		return nil, errors.New("paths-limit.conf must carry one password hash placeholder")
	}
	workDir, err := os.MkdirTemp("", "ze-paths-limit-")
	if err != nil {
		return nil, err
	}
	session := &cliWireSession{workDir: workDir}
	started := false
	defer func() {
		if !started {
			session.stop()
		}
	}()
	hash, err := cliWirePasswordHash(ctx, workDir)
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(workDir, "paths-limit.conf")
	config := strings.Replace(string(template), "{{PASSWORD_HASH}}", hash, 1)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return nil, err
	}
	session.peer, err = startFixtureProcess(ctx, os.Environ(), "", "ze-test", "peer",
		"--port", args[0], "--asn", strconv.Itoa(cliWirePeerAS), filepath.Join(cwd, "peer-script"))
	if err != nil {
		return nil, err
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		return strings.Contains(session.peer.output.String(), "listening on")
	}) {
		return nil, fmt.Errorf("PATHS-LIMIT peer did not listen: %s", session.peer.output.String())
	}
	sshAddrPath := filepath.Join(workDir, "ssh.addr")
	readyPath := filepath.Join(workDir, "ready")
	daemonEnv := miscEnvironment(map[string]string{
		envSSHEphemeral: sshAddrPath,
		envReadyFile:    readyPath,
		envConfigDir:    workDir,
		envTestBGPPort:  args[0],
		envLogBGP:       logLevelInfo,
	})
	session.daemon, err = startFixtureProcess(ctx, daemonEnv, "", "ze", "-f", configPath)
	if err != nil {
		return nil, err
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		return cliWireFileExists(sshAddrPath) && cliWireFileExists(readyPath)
	}) {
		return nil, fmt.Errorf("PATHS-LIMIT daemon did not become ready: %s", session.daemon.output.String())
	}
	host, sshPort, err := cliWireSSHAddress(sshAddrPath)
	if err != nil {
		return nil, err
	}
	session.cliEnv = miscEnvironment(map[string]string{
		envSSHHost: host, envSSHPort: sshPort, envSSHUsername: "ci",
		envSSHPassword: valueSecret, envConfigDir: workDir,
	})
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		out := strings.ToUpper(session.peer.output.String())
		return strings.Contains(out, cliWireEndOfRIBHex) &&
			strings.Contains(out, "001E:02:00000007900F0003000201")
	}) {
		return nil, fmt.Errorf("PATHS-LIMIT initial IPv4/IPv6 EOR barrier failed: %s\ndaemon: %s", session.peer.output.String(), session.daemon.output.String())
	}
	started = true
	return session, nil
}
