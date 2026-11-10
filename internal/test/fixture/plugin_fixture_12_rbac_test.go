// Design: docs/architecture/testing/runner-architecture.md -- port leases for private daemon fixtures.
// Related: plugin_fixture_12_rbac.go -- the rbac-web-config-deny fixture under test.
// Related: plugin_fixture_02.go -- the as112-doh fixture, which shared port 18443.
package fixture

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/web/yang" // registers ze-web-conf.yang, which defines environment/web
	"github.com/ze-software/ze/internal/test/runner"
)

// fixedWebPort02 is the port rbac-web-config-deny and as112-doh both bound
// before they took a runner lease.
const fixedWebPort02 = 18443

// leaseAroundFixedWebPort holds 127.0.0.1:18443, as a concurrent copy of the
// other test would, and leases a span whose preferred start covers it. The
// runner hands a .ci test the same kind of lease, $PORT2 being Start+1.
func leaseAroundFixedWebPort(t *testing.T) *runner.PortReservation {
	t.Helper()
	var listenConfig net.ListenConfig
	holder, err := listenConfig.Listen(context.Background(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(fixedWebPort02)))
	if err == nil {
		t.Cleanup(func() { _ = holder.Close() })
	} else {
		// Another process holds it: the port is occupied either way, which is
		// the collision this test reproduces.
		t.Logf("port %d already held elsewhere: %v", fixedWebPort02, err)
	}
	lease, err := runner.LeaseTestPorts(fixedWebPort02 - 1)
	if err != nil {
		t.Fatalf("lease test ports: %v", err)
	}
	t.Cleanup(lease.Release)
	if lease.Start+1 == fixedWebPort02 {
		t.Fatalf("lease handed out $PORT2=%d while it is held", fixedWebPort02)
	}
	return lease
}

// TestRBACWebFixtureTakesTheLeasedWebPort proves the rbac-web-config-deny
// fixture configures, addresses and waits for its web listener on the port the
// runner leased, and no longer on 18443.
// Method: hold 18443, lease a span the way the runner does, then parse the
// fixture's environment block with the YANG parser and read the web server's
// port from the tree, and check the URL the HTTP checks use and the listener
// line the readiness wait requires.
func TestRBACWebFixtureTakesTheLeasedWebPort(t *testing.T) {
	lease := leaseAroundFixedWebPort(t)
	webPort := lease.Start + 1
	want := strconv.Itoa(webPort)

	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("yang schema: %v", err)
	}
	tree, err := config.NewParser(schema).Parse(p12WebRBACEnvironment(webPort))
	if err != nil {
		t.Fatalf("parse fixture environment: %v", err)
	}
	environment := tree.GetContainer("environment")
	if environment == nil {
		t.Fatal("fixture config has no environment container")
	}
	web := environment.GetContainer("web")
	if web == nil {
		t.Fatal("fixture config has no environment/web container")
	}
	server := web.GetList("server")["main"]
	if server == nil {
		t.Fatal("fixture config has no environment/web/server main")
	}
	// MUTATION: write the fixed "port 18443;" back into the environment block
	// and the parsed port is 18443, not the leased one.
	got, ok := server.Get("port")
	if !ok {
		t.Fatal("web server main has no port")
	}
	if got != want {
		t.Fatalf("web server port = %s, want the leased %s", got, want)
	}
	if !strings.Contains(p12WebRBACConfigFor(webPort), p12WebRBACEnvironment(webPort)) {
		t.Fatal("the daemon config does not carry the leased environment block")
	}

	// MUTATION: restore "https://127.0.0.1:18443" as the request base and the
	// host port below is 18443.
	_, basePort, err := net.SplitHostPort(strings.TrimPrefix(p12WebRBACBase(webPort), "https://"))
	if err != nil {
		t.Fatalf("split base URL: %v", err)
	}
	if basePort != want {
		t.Fatalf("request base port = %s, want the leased %s", basePort, want)
	}

	// MUTATION: drop the second argument and parse a fixed web port instead,
	// and a one-argument call no longer fails before starting anything.
	if err := p12WebRBAC(context.Background(), []string{strconv.Itoa(lease.Start)}); err == nil {
		t.Fatal("fixture accepted a call without the leased web port")
	}
}

// TestRBACWebFixtureWaitsForTheReportedListener proves the readiness wait
// returns on the daemon's report of its listener on the leased port, and
// refuses a daemon that reports another port or exits before it is ready.
// Method: write the ready file and daemon log a daemon would, and call
// p12WaitWebListener over each state.
func TestRBACWebFixtureWaitsForTheReportedListener(t *testing.T) {
	lease := leaseAroundFixedWebPort(t)
	webPort := lease.Start + 1
	dir := t.TempDir()
	readyFile := filepath.Join(dir, "ready")
	logPath := filepath.Join(dir, "daemon.log")
	running := make(chan struct{})

	writeLog := func(port int) {
		t.Helper()
		line := p12WebRBACListenerLine + p12WebRBACBase(port) + "/\n"
		if err := os.WriteFile(logPath, []byte("Starting ze\n"+line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(readyFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	writeLog(webPort)
	if err := p12WaitWebListener(context.Background(), running, readyFile, logPath, webPort); err != nil {
		t.Fatalf("listener reported on the leased port, wait refused: %v", err)
	}

	// MUTATION: accept any "web server listening" line, or compare with the
	// fixed port, and a daemon serving 18443 passes as ready on the lease.
	writeLog(fixedWebPort02)
	if err := p12WaitWebListener(context.Background(), running, readyFile, logPath, webPort); err == nil {
		t.Fatal("wait accepted a listener reported on 18443 instead of the leased port")
	}

	// MUTATION: skip the ready-file wait and read the log at once, and an
	// exited daemon's absent ready file is not what the error names.
	if err := os.Remove(readyFile); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	close(exited)
	err := p12WaitWebListener(context.Background(), exited, readyFile, logPath, webPort)
	if err == nil || !strings.Contains(err.Error(), "exited before it was ready") {
		t.Fatalf("wait over an exited daemon = %v, want the exit named", err)
	}
}

// TestAS112DoHFixtureTakesTheLeasedPort proves as112-doh expects its DoH
// listener on the port the test passes, the leased $PORT2, and no longer on
// 18443. Method: hold 18443, lease a span, and read the address the fixture
// derives from the argument.
func TestAS112DoHFixtureTakesTheLeasedPort(t *testing.T) {
	lease := leaseAroundFixedWebPort(t)
	webPort := lease.Start + 1

	// MUTATION: return "127.0.0.1:18443" again and the address is the fixed one.
	address, err := as112DoHAddress02([]string{strconv.Itoa(webPort)})
	if err != nil {
		t.Fatalf("as112-doh address: %v", err)
	}
	if want := net.JoinHostPort("127.0.0.1", strconv.Itoa(webPort)); address != want {
		t.Fatalf("as112-doh address = %s, want %s", address, want)
	}
	if _, err := as112DoHAddress02(nil); err == nil {
		t.Fatal("as112-doh accepted no port argument")
	}
}
