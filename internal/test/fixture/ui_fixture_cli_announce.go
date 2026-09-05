// Design: docs/architecture/bgp/on-demand-origination.md -- the announce and withdraw verbs this drives
// Related: ui_fixture_cli_verb_daemon_dispatch.go -- the daemon-over-ephemeral-SSH half this reuses
// Related: misc_fixture_vpp.go -- startFixtureProcess and Poll, the ze-test peer half this reuses
//
// The announce verbs are the only command family in the tree whose whole
// grammar lives in a handler and whose seven registered handlers no functional
// test reaches. A .ci cannot drive them on its own: the daemon publishes its
// ephemeral SSH address into the file ZE_SSH_EPHEMERAL names, and only a
// compiled fixture can read that file and put ZE_SSH_HOST and ZE_SSH_PORT on
// the client. So this fixture starts BOTH halves, a daemon over ephemeral SSH
// and a ze-test peer, and drives `ze announce` as argv against them.
//
// The peer is the assertion. Its script states the wire bytes it must receive,
// and ze-test peer exits zero only when every one of them arrived, so the
// fixture proves the announcement reached the wire rather than only that the
// command exited zero.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	registerFixture("ui/cli-announce-reaches-the-wire", cliAnnounceReachesTheWire)
	registerFixture("ui/cli-announce-tag-round-trip", cliAnnounceTagRoundTrip)
}

// cliAnnouncePrefix is the prefix every announce in this file originates. Its
// wire form, one length octet of 24 and the three significant octets RFC 4271
// Section 4.3 asks for, is what each .ci peer script expects.
const cliAnnouncePrefix = "10.0.0.0/24"

// cliAnnounceNextHop is the next hop every announce in this file carries.
//
// It is stated rather than left to default, and the reason is RFC 4271 Section
// 5.1.3: a speaker MUST NOT advertise a route whose NEXT_HOP is the receiving
// peer's own address, and ze withholds one that is. Both ends of this session
// sit on the loopback, so next-hop self IS the peer's address and the route
// never reaches the wire.
const cliAnnounceNextHop = "10.0.0.1"

// cliWireEndOfRIBHex is the body of the End-of-RIB marker RFC 4724
// Section 2 defines for IPv4 unicast: an UPDATE of 23 octets with no withdrawn
// route and no path attribute. Ze writes it once the session is Established and
// its initial RIB is sent, so its arrival at the peer is the barrier an
// announce has to wait for.
const cliWireEndOfRIBHex = "0017:02:00000000"

// argAnnounce is the verb every command in this file types first, and
// argUnicast, argBlackhole and argFlowspec are its three forms.
const (
	argAnnounce  = "announce"
	argUnicast   = "unicast"
	argBlackhole = "blackhole"
	argFlowspec  = "flowspec"
	argNextHop   = "next-hop"
	argTag       = "tag"
)

// cliAnnounceTagKey and cliAnnounceTagValue name the announcement the
// round-trip test lists and then withdraws.
const (
	cliAnnounceTagKey   = "maint"
	cliAnnounceTagValue = "window"
)

// cliAnnounceUnclaimedToken is the word every refusal case types where no
// keyword claims it. It is one word rather than a literal in each case so the
// argv and the expectation cannot drift apart.
const cliAnnounceUnclaimedToken = "bogus"

// cliWireSession is one run's two processes and the environment a client
// reaches the daemon with: a daemon over ephemeral SSH, and a ze-test peer
// scripted by the .ci that launched the fixture. It carries no announce
// vocabulary, so every fixture that drives a command at a peer's wire uses it
// (ui_fixture_send_raw.go is the second).
//
// The caller MUST call stop for every session startCLIWireSession
// returned, and stop MUST NOT be called before that function returns.
type cliWireSession struct {
	workDir string
	cliEnv  []string
	daemon  *fixtureProcess
	peer    *fixtureProcess
}

// cliWireResult is one `ze` invocation's exit status and the whole text it
// wrote, both streams joined in the order an operator reads them.
type cliWireResult struct {
	code int
	out  string
}

// cliAnnounceReachesTheWire proves that `ze announce unicast <prefix>` typed as
// argv reaches the handler and puts the prefix on a peer's wire, and that a
// trailing token no keyword claims is refused by name rather than discarded.
func cliAnnounceReachesTheWire(ctx context.Context, args []string) error {
	session, err := startCLIWireSession(ctx, args)
	if err != nil {
		return err
	}
	defer session.stop()

	announced := session.run(ctx, argAnnounce, argUnicast, cliAnnouncePrefix, argNextHop, cliAnnounceNextHop)
	if announced.code != 0 {
		return fmt.Errorf("ze announce unicast %s exit=%d, want 0: %s", cliAnnouncePrefix, announced.code, announced.out)
	}

	// The refusals share this daemon because each one is a single command
	// launch and a second daemon would buy nothing. Each drives a different
	// announce form, so the fix reaches all three rather than flowspec alone.
	refusals := []struct {
		argv  []string
		token string
	}{
		// A second action after the first. This is the case that used to be
		// discarded in silence: the rule went out as a plain discard and the
		// rate limit was lost without a word.
		// The keyword names the family: a bare "destination" reaches
		// parseComponentText, which knows destination-ipv4 and destination-ipv6
		// and refuses the bare form, so the run would die on the keyword before
		// it reached the trailing token this case exists to prove.
		{[]string{argAnnounce, argFlowspec, "destination-ipv4", "1.1.1.1/32", "discard", "rate-limit", "500"}, "rate-limit"},
		// A token after a complete option, on both of the other two forms. This
		// is the same silent discard reached through the two handlers that share
		// the option parser.
		{[]string{argAnnounce, argUnicast, cliAnnouncePrefix, argNextHop, cliAnnounceNextHop, argTag, "k", "v", cliAnnounceUnclaimedToken}, cliAnnounceUnclaimedToken},
		{[]string{argAnnounce, argBlackhole, cliAnnouncePrefix, argTag, "k", "v", cliAnnounceUnclaimedToken}, cliAnnounceUnclaimedToken},
		// A bare token where an option keyword belongs. Each handler's own
		// keyword loop refuses this one, and it is named here so both refusals
		// stay proven from the operator's side.
		{[]string{argAnnounce, argUnicast, cliAnnouncePrefix, cliAnnounceUnclaimedToken}, cliAnnounceUnclaimedToken},
		{[]string{argAnnounce, argBlackhole, cliAnnouncePrefix, cliAnnounceUnclaimedToken}, cliAnnounceUnclaimedToken},
	}
	for _, refusal := range refusals {
		line := strings.Join(refusal.argv, " ")
		result := session.run(ctx, refusal.argv...)
		if result.code == 0 {
			return fmt.Errorf("ze %s exit=0: an unclaimed token was accepted: %s", line, result.out)
		}
		if !strings.Contains(result.out, refusal.token) {
			return fmt.Errorf("ze %s did not name the unclaimed token %q: %s", line, refusal.token, result.out)
		}
	}

	// The peer holds the assertion: it exits zero only when every expectation
	// in its script arrived, so this is what proves the prefix reached the wire.
	if err := waitFixtureProcess(ctx, session.peer, 20*time.Second); err != nil {
		return fmt.Errorf("the peer did not receive the announced %s: %w\n%s", cliAnnouncePrefix, err, session.peer.output.String())
	}

	fmt.Println("OK")
	return nil
}

// cliAnnounceTagRoundTrip proves that an announcement carrying a tag is listed
// while it is live, that withdrawing the tag reports one removal, and that the
// peer receives the withdrawal.
func cliAnnounceTagRoundTrip(ctx context.Context, args []string) error {
	session, err := startCLIWireSession(ctx, args)
	if err != nil {
		return err
	}
	defer session.stop()

	announced := session.run(ctx, argAnnounce, argUnicast, cliAnnouncePrefix, argNextHop, cliAnnounceNextHop, argTag, cliAnnounceTagKey, cliAnnounceTagValue)
	if announced.code != 0 {
		return fmt.Errorf("ze announce unicast %s tag %s %s exit=%d, want 0: %s",
			cliAnnouncePrefix, cliAnnounceTagKey, cliAnnounceTagValue, announced.code, announced.out)
	}

	listed := session.run(ctx, "show", "announcements")
	if listed.code != 0 {
		return fmt.Errorf("ze show announcements exit=%d, want 0: %s", listed.code, listed.out)
	}
	if !strings.Contains(listed.out, cliAnnounceTagKey) {
		return fmt.Errorf("ze show announcements does not list the live announcement: %s", listed.out)
	}

	withdrawn := session.run(ctx, "withdraw", argTag, cliAnnounceTagKey)
	if withdrawn.code != 0 {
		return fmt.Errorf("ze withdraw tag %s exit=%d, want 0: %s", cliAnnounceTagKey, withdrawn.code, withdrawn.out)
	}
	if !strings.Contains(withdrawn.out, "1") {
		return fmt.Errorf("ze withdraw tag %s did not report one removal: %s", cliAnnounceTagKey, withdrawn.out)
	}

	if err := waitFixtureProcess(ctx, session.peer, 20*time.Second); err != nil {
		return fmt.Errorf("the peer did not receive the withdrawal of %s: %w\n%s", cliAnnouncePrefix, err, session.peer.output.String())
	}

	fmt.Println("OK")
	return nil
}

// startCLIWireSession starts the peer and the daemon and answers the
// session a caller drives commands through. The caller MUST call stop on a
// session this returns.
//
// args carries one value, the BGP port the .ci reserved. The peer listens on
// it and ze_test_bgp_port points the daemon's peer block at the same number,
// which is the route every .ci-launched peer in this tree already takes.
func startCLIWireSession(ctx context.Context, args []string) (*cliWireSession, error) {
	if len(args) != 1 {
		return nil, errors.New("the cli-announce fixture requires the BGP port")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid BGP port %q: %w", args[0], err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve the test directory: %w", err)
	}
	workDir, err := os.MkdirTemp("", "ze-ui-cli-announce-")
	if err != nil {
		return nil, fmt.Errorf("create fixture directory: %w", err)
	}

	session := &cliWireSession{workDir: workDir}
	started := false
	defer func() {
		if started {
			return
		}
		session.stop()
	}()

	passwordHash, err := cliWirePasswordHash(ctx, workDir)
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(workDir, "announce.conf")
	if err := os.WriteFile(configPath, []byte(cliWireConfig(passwordHash)), 0o600); err != nil {
		return nil, fmt.Errorf("write announce.conf: %w", err)
	}

	// The peer listens first: ze dials it, and a daemon that starts against a
	// closed port waits out its connect retry before the session establishes.
	peerScript := filepath.Join(cwd, "peer-script")
	session.peer, err = startFixtureProcess(ctx, os.Environ(), "", "ze-test", "peer", "--port", strconv.Itoa(port), peerScript)
	if err != nil {
		return nil, fmt.Errorf("start the peer: %w", err)
	}
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		return strings.Contains(session.peer.output.String(), "listening on")
	}) {
		return nil, fmt.Errorf("the peer did not report listening: %s", session.peer.output.String())
	}

	sshAddrPath := filepath.Join(workDir, "ssh.addr")
	readyPath := filepath.Join(workDir, "ready")
	daemonEnv := miscEnvironment(map[string]string{
		envSSHEphemeral: sshAddrPath,
		envReadyFile:    readyPath,
		envConfigDir:    workDir,
		envTestBGPPort:  strconv.Itoa(port),
		envLogBGP:       logLevelInfo,
	})
	session.daemon, err = startFixtureProcess(ctx, daemonEnv, "", "ze", "-f", configPath)
	if err != nil {
		return nil, fmt.Errorf("start the daemon: %w", err)
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		return cliWireFileExists(sshAddrPath) && cliWireFileExists(readyPath)
	}) {
		return nil, fmt.Errorf("the daemon did not become ready: %s", session.daemon.output.String())
	}

	host, sshPort, err := cliWireSSHAddress(sshAddrPath)
	if err != nil {
		return nil, err
	}
	session.cliEnv = miscEnvironment(map[string]string{
		envSSHHost:     host,
		envSSHPort:     sshPort,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   workDir,
	})

	// The ready file says the daemon is up, not that the session is. An
	// announce made before the peer reaches Established matches no peer and is
	// lost, which is a green command and a silent wire.
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		return strings.Contains(session.peer.output.String(), cliWireEndOfRIBHex)
	}) {
		return nil, fmt.Errorf("the session did not establish: %s", session.peer.output.String())
	}

	started = true
	return session, nil
}

// stop ends both processes and removes the working directory. It MUST be
// called once for every session startCLIWireSession returned, and it is
// safe to call when either process never started.
func (s *cliWireSession) stop() {
	stopFixtureProcess(s.daemon, 3*time.Second)
	stopFixtureProcess(s.peer, 2*time.Second)
	_ = os.RemoveAll(s.workDir) //nolint:errcheck // fixture cleanup, and the run is over
}

// run drives one `ze` invocation as argv against this session's daemon.
func (s *cliWireSession) run(ctx context.Context, args ...string) cliWireResult {
	command := exec.CommandContext(ctx, "ze", args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = s.workDir
	command.Env = s.cliEnv
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := cliWireResult{out: stdout.String() + stderr.String()}
	if err == nil {
		return result
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result
	}
	// A launch that never produced an exit status is reported as a failure the
	// caller can read, rather than as the zero exit it would otherwise take for
	// success (ai/rules/evidence.md).
	result.code = -1
	result.out += err.Error()
	return result
}

// cliWirePasswordHash answers the stored form of the fixture's password,
// which the config needs before the daemon starts.
func cliWirePasswordHash(ctx context.Context, workDir string) (string, error) {
	command := exec.CommandContext(ctx, "ze", "passwd")
	command.Dir = workDir
	command.Env = os.Environ()
	command.Stdin = strings.NewReader(valueSecret + "\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// cliWireConfig is the daemon's whole configuration: one peer this daemon
// dials on the loopback, and one operator the CLI authenticates as.
//
// The peer's port is not written here. ze_test_bgp_port overrides the remote
// port of every peer (applyPortOverride, internal/component/bgp/config/peers.go),
// which is how every .ci-launched peer in this tree reaches a dynamic port.
func cliWireConfig(passwordHash string) string {
	return `bgp {
    router-id 10.0.0.2
    peer peer1 {
        connection {
            remote { ip 127.0.0.1; }
            local { ip 127.0.0.1; accept false; }
        }
        session {
            asn { local 65533; remote 65000; }
            router-id 10.0.0.2
            family { ipv4/unicast { prefix { maximum 10000; } } }
            capability { graceful-restart disable; }
        }
        behavior { group-updates disable; }
        timer { receive-hold-time 180; }
    }
}

system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
}

// cliWireSSHAddress splits the address the daemon published into the host
// and the port a client needs.
func cliWireSSHAddress(path string) (host, port string, err error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return "", "", fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(data))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return "", "", fmt.Errorf("invalid ssh.addr %q", addr)
	}
	return addr[:colon], addr[colon+1:], nil
}

// cliWireFileExists reports whether a startup barrier has appeared.
func cliWireFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
