// Design: docs/architecture/firewall/firewall-domain-group.md -- the three functional-test fixtures
// Related: internal/test/mock/dns/server.go -- the DNS stub they serve;
// internal/component/firewall/plugins/domain/command.go -- the commands they dispatch;
// register_domain_group.go -- where the three names are registered

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
)

const (
	// domainGroupStubAddress is where all three fixtures serve DNS. The port is
	// 53 because `system name-server` is declared `type zt:ip-address`
	// (internal/component/config/system/yang/ze-system-conf.yang) and carries
	// no port, so a daemon pointed at a stub reaches it there or nowhere. All
	// three `.ci` files declare option=needs-linux:caps=net-bind for it.
	//
	// These fixtures may bind it in every suite they run in, the per-test
	// namespace `firewall` suite included. runOrchestrated
	// (internal/test/runner/runner_exec.go) drops only the `ze` binary to an
	// unprivileged uid, so a fixture the runner launches itself with
	// cmd=foreground:exec=ze-test keeps the credentials the bind needs. That is
	// the difference from dnsStubAddress (plugin_fixture_dns_stub.go), whose
	// fixture is FORKED BY ze as an external plugin and so inherits the dropped
	// uid without cap_net_bind_service.
	domainGroupStubAddress = "127.0.0.1:53"

	// domainGroupName is the group all three `.ci` files declare, and
	// domainGroupDNSName the single name it holds. web.example.test is in the
	// stub's default zone (defaultZone, internal/test/mock/dns/zone.go) with an
	// A and an AAAA record at TTL 0, so no fixture has to seed it.
	domainGroupName    = "web"
	domainGroupDNSName = "web.example.test"

	// domainGroupSetV4 is the IPv4 set the plugin builds for that group. The
	// spelling comes from firewall.DomainGroupSetNames, which is what the
	// firewall's own config parser matches a rule against.
	domainGroupSetV4 = "domain_v4_" + domainGroupName

	// The two addresses the stub answers with. The first is what its default
	// zone already holds; the second is what the update scenario moves the name
	// to. Both are in the RFC 5737 documentation range.
	domainGroupAddressFirst  = "203.0.113.10"
	domainGroupAddressSecond = "203.0.113.11"

	// One table per scenario. The three tests serialize against each other on
	// option=exclusive:group=dns-stub-port-53, and a distinct name per scenario
	// means a table a killed daemon left behind cannot be read as this test's
	// own.
	domainGroupTableUpdate = "fw_dgu"
	domainGroupTableClear  = "fw_dgc"
	domainGroupTableShow   = "fw_dg"

	// domainGroupTablePrefix is the ownership prefix RegisterTables puts on
	// every table name (internal/component/firewall/registry.go). A `.ci`
	// declaring `table fw_dg` is `ze_fw_dg` in the kernel, while
	// `show firewall ruleset` takes the bare name back
	// (firewall.StripZeTablePrefix).
	domainGroupTablePrefix = "ze_"

	// domainGroupUser is the account each `.ci` file declares and `ze init`
	// provisions client credentials for, with domainGroupPassword as its
	// plaintext form. The bcrypt hash in each file is of this password.
	domainGroupUser     = "operator"
	domainGroupPassword = "testpass"
)

// domainGroupCLIAttempts bounds the retry of one `ze cli` invocation. A port
// that accepts is not yet a daemon that serves, so a refusal early in startup
// is retried rather than reported.
const domainGroupCLIAttempts = 10

// domainGroupCLITimeout bounds one of those attempts.
const domainGroupCLITimeout = 3 * time.Second

// domainGroupPollAttempts bounds a kernel read-back. At netfilterPollDelay it
// is five seconds.
const domainGroupPollAttempts = 100

// domainGroupSession is one fixture's connection to the daemon under test: the
// daemon's pid, so the fixture can stop it, and the environment `ze cli` needs
// to reach it over SSH.
//
// The three scenarios drive the daemon over the real operator path rather than
// over a plugin channel, which is what Wiring rows 4 and 5 of
// plan/spec-firewall-domain-group.md ask for. A `plugin { external ... }` block
// would give a fixture a channel to dispatch on, and would change what the
// daemon under test is running.
//
// Not safe for concurrent use. One scenario drives one daemon from one
// goroutine, and the assertions read an order the `.ci` file states.
type domainGroupSession struct {
	pid int
	env []string
}

// openDomainGroupSession waits for the daemon, provisions client credentials
// for it, and returns a session that can run `ze cli` against it. args carries
// the one argument every `.ci` file passes: the SSH port the runner leased as
// $PORT2.
func openDomainGroupSession(ctx context.Context, args []string) (*domainGroupSession, error) {
	if len(args) != 1 {
		return nil, errors.New("usage: <fixture> <ssh-port>")
	}
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("ssh port %q is not a number: %w", args[0], err)
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("ssh port %d is outside the range 1-65535", port)
	}

	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort("127.0.0.1", args[0])
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		dialer := net.Dialer{Timeout: 200 * time.Millisecond}
		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			return false
		}
		_ = conn.Close() //nolint:errcheck // the probe asks only whether the port accepts
		return true
	}) {
		return nil, fmt.Errorf("the SSH CLI server never listened on %s", address)
	}

	// The client's own config directory, inside the per-test work directory, so
	// nothing has to clean it up and `ze init` never writes into the daemon's.
	// The daemon's directory is what the change-log assertions read.
	clientDir, err := filepath.Abs("cli-client")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(clientDir, 0o750); err != nil {
		return nil, err
	}

	// envWith REPLACES rather than appends, which is load-bearing here: the
	// update scenario sets ze.config.dir for the daemon through option=env, so
	// this process inherits it, and Go's os.Getenv answers with the FIRST
	// match in the environment. An appended override would be the second.
	env := envWith(os.Environ(), "ze.config.dir", clientDir)
	env = envWith(env, "ze.ssh.insecure", "true")

	// `ze init` answers its prompts from this script, so it runs BEFORE
	// ze.ssh.password is set: that variable suppresses the password prompt, and
	// the rest of the script would then answer the wrong questions.
	initCmd := exec.CommandContext(ctx, "ze", "init")
	initCmd.Env = env
	initCmd.Stdin = strings.NewReader(domainGroupUser + "\n" + domainGroupPassword + "\n127.0.0.1\n" + args[0] + "\n")
	if out, runErr := initCmd.CombinedOutput(); runErr != nil {
		return nil, fmt.Errorf("ze init failed: %s: %w", out, runErr)
	}

	env = envWith(env, "ze.ssh.password", domainGroupPassword)
	return &domainGroupSession{pid: pid, env: env}, nil
}

// cli runs one command over the real SSH `ze cli` path and returns what the
// client printed.
//
// Every caller appends `| json`, because the assertions in the `.ci` files read
// quoted JSON keys while the daemon's default render format is whatever its
// configuration says. The pipe operator is the call site's to state
// (ai/rules/cli.md).
func (s *domainGroupSession) cli(ctx context.Context, command string) (string, error) {
	var last string
	for attempt := range domainGroupCLIAttempts {
		attemptCtx, cancel := context.WithTimeout(ctx, domainGroupCLITimeout)
		cmd := exec.CommandContext(attemptCtx, "ze", "cli", "--user", domainGroupUser, "-c", command) //nolint:gosec // the fixture chooses the program and its arguments
		cmd.Env = s.env
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr == nil {
			return string(out), nil
		}

		var tb textbuf.Buffer
		tb.Str("attempt ").Int(int64(attempt)).Str(": ").Str(string(out)).Str(": ").Err(runErr)
		last = tb.String()

		if !sleepContext(ctx, 100*time.Millisecond) {
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("ze cli %q never succeeded: %s", command, last)
}

// startDomainGroupStub serves the stub's default zone on port 53, where the
// `system name-server` leaf in each `.ci` file points the daemon. The caller
// MUST call closeDomainGroupStub, which releases the port the next scenario in
// this exclusive group needs.
func startDomainGroupStub() (*dnsmock.Server, error) {
	server, err := dnsmock.Start(domainGroupStubAddress)
	if err != nil {
		return nil, err
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns stub listening on ").Str(server.Addr().String()).Byte('\n').StdErr() //nolint:errcheck // fixture progress output

	return server, nil
}

// closeDomainGroupStub stops the stub and releases port 53. It MUST be called
// after startDomainGroupStub. A close failure is reported rather than returned:
// the scenario's own result is what the test is about, and the port is released
// by the process exiting either way.
func closeDomainGroupStub(server *dnsmock.Server) {
	err := server.Close()
	if err == nil {
		return
	}

	var tb textbuf.Buffer
	tb.Str("dns stub close: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // fixture diagnostic output
}

// domainGroupSetRead reads the group's IPv4 set out of the kernel. The error is
// the answer when the set, or the table holding it, is not there.
func domainGroupSetRead(ctx context.Context, table string) (string, error) {
	return netfilterCommandOutput(ctx, "nft", "list", "set", "inet", domainGroupTablePrefix+table, domainGroupSetV4)
}

// domainGroupSetHolds reports whether the group's IPv4 set holds one address,
// read from the kernel rather than from the daemon's own memory.
func domainGroupSetHolds(ctx context.Context, table, address string) bool {
	out, err := domainGroupSetRead(ctx, table)
	if err != nil {
		return false
	}
	return strings.Contains(out, address)
}

// domainGroupUpdate proves the operator path: `update firewall domain-group`
// resolves the group's names now, programs what they answer into the kernel,
// and records the move from the old addresses to the new ones.
//
// The stub runs IN this process so the second resolution can be given a
// different answer with no cache-clearing command in between. Its zone answers
// with TTL 0, and cache.put (internal/component/resolve/dns/cache.go) returns
// before storing a zero-TTL answer, so the second lookup reaches the stub
// rather than the daemon's own cache.
func domainGroupUpdate(ctx context.Context, args []string) error {
	session, err := openDomainGroupSession(ctx, args)
	if err != nil {
		return err
	}

	server, err := startDomainGroupStub()
	if err != nil {
		return err
	}
	defer closeDomainGroupStub(server)

	command := "update firewall domain-group " + domainGroupName + " | json"

	first, err := session.cli(ctx, command)
	if err != nil {
		return err
	}
	fmt.Print(first)

	// The answer says what the plugin cached. The kernel is what this test is
	// about, so the first addresses are read back from nftables before the stub
	// is changed. Nothing is PRINTED here: the file asserts on the FINAL
	// contents of the set, and an intermediate dump would put the replaced
	// address into the same buffer.
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		return domainGroupSetHolds(ctx, domainGroupTableUpdate, domainGroupAddressFirst)
	}) {
		return errors.New("the first update never reached the kernel, so the change this test asserts would have nothing to replace")
	}

	server.Set(domainGroupDNSName, mdns.TypeA, dnsmock.Answer{
		Addresses: []netip.Addr{netip.MustParseAddr(domainGroupAddressSecond)},
	})

	second, err := session.cli(ctx, command)
	if err != nil {
		return err
	}
	fmt.Print(second)

	// The set holds what the name resolves to NOW, so the replaced address must
	// be gone from it. A product that added the new address beside the old one
	// keeps filtering on an address the name no longer answers with, and the
	// `.ci` assertion on the printed dump alone would not see it.
	var final string
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		out, readErr := domainGroupSetRead(ctx, domainGroupTableUpdate)
		if readErr != nil {
			return false
		}
		final = out
		return strings.Contains(out, domainGroupAddressSecond) && !strings.Contains(out, domainGroupAddressFirst)
	}) {
		return fmt.Errorf("the second update did not replace the group's address in the kernel; the set reads:\n%s", final)
	}
	fmt.Print(final)

	return signalProcess(session.pid, syscall.SIGTERM)
}

// domainGroupClear proves the deliberate exit from last-known-good: the cached
// addresses go, and the set they were programmed into leaves the kernel with
// them.
//
// It prints no address of its own. The `.ci` file asserts that one is ABSENT
// from the whole accumulated output, so an address printed anywhere for any
// reason would fail it.
func domainGroupClear(ctx context.Context, args []string) error {
	session, err := openDomainGroupSession(ctx, args)
	if err != nil {
		return err
	}

	server, err := startDomainGroupStub()
	if err != nil {
		return err
	}
	defer closeDomainGroupStub(server)

	// The update answer reports counts and no address, which is what makes it
	// safe to print in a file that asserts an absence.
	updated, err := session.cli(ctx, "update firewall domain-group "+domainGroupName+" | json")
	if err != nil {
		return err
	}
	fmt.Print(updated)

	// Silent, and from the kernel: without it the absence asserted at the end
	// would also pass against a daemon that programmed nothing, which is the
	// vacuity this read closes. The failure names neither the address nor the
	// set, so a fixture that fails here does not also trip the absence
	// assertions and send the reader to the wrong diagnosis.
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		return domainGroupSetHolds(ctx, domainGroupTableClear, domainGroupAddressFirst)
	}) {
		return errors.New("the group's IPv4 set never held its resolved address before the clear, so this test would have proved nothing")
	}

	cleared, err := session.cli(ctx, "clear firewall domain-group "+domainGroupName+" | json")
	if err != nil {
		return err
	}
	fmt.Print(cleared)

	// The whole table goes. With no cached address the plugin registers no set
	// (buildGroupSets, internal/component/firewall/plugins/domain/sets.go), and
	// dropTablesMissingAProvidedSet (internal/component/firewall/registry.go)
	// then holds back the operator's table whose term names it.
	var remaining string
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		out, readErr := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", domainGroupTablePrefix+domainGroupTableClear)
		if readErr != nil {
			return true
		}
		remaining = out
		return false
	}) {
		// Printed on purpose: a table still carrying the addresses is exactly
		// what the `.ci` file's absence assertions are written to catch, and
		// this is the evidence they read.
		fmt.Print(remaining)
		return errors.New("the table the group's addresses were programmed into is still in the kernel after the clear")
	}
	fmt.Println("after the clear: the kernel holds no table for this test")

	return signalProcess(session.pid, syscall.SIGTERM)
}

// domainGroupShow proves the enrichment reaches an operator: `show firewall
// ruleset` over the real SSH `ze cli` path names the DNS name that supplied
// each address in the group's set.
//
// This is the only assertion made from outside the daemon. enrichShow
// (internal/component/firewall/plugins/domain/enrich.go) returning the right
// map says nothing about whether the registration key matches the command
// string the show handler passes, whether the merge survives the plugin
// boundary, or whether the name reaches the client at all.
func domainGroupShow(ctx context.Context, args []string) error {
	session, err := openDomainGroupSession(ctx, args)
	if err != nil {
		return err
	}

	server, err := startDomainGroupStub()
	if err != nil {
		return err
	}
	defer closeDomainGroupStub(server)

	if _, err := session.cli(ctx, "update firewall domain-group "+domainGroupName+" | json"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: the group resolved")

	// Silent, and from the kernel. handleShowFirewallRuleset
	// (internal/plugins/firewall/nft/cmd_show.go) renders
	// firewall.LastApplied(), which is the daemon's own merged snapshot, so a
	// set that never reached nftables would render all the same. This read is
	// what makes the answer below evidence about a live filter.
	if !Poll(ctx, domainGroupPollAttempts, netfilterPollDelay, func() bool {
		return domainGroupSetHolds(ctx, domainGroupTableShow, domainGroupAddressFirst)
	}) {
		return errors.New("the group's addresses never reached the kernel, so the rendered answer would not describe a live filter")
	}

	answer, err := session.cli(ctx, "show firewall ruleset "+domainGroupTableShow+" | json")
	if err != nil {
		return err
	}
	fmt.Print(answer)

	return signalProcess(session.pid, syscall.SIGTERM)
}
