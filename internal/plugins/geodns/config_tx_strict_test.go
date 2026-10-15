package geodns

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/dnsserver"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: R-13 for geodns. A reload whose listener cannot be bound (its
// port is in use) fails the install with an error naming the endpoint and
// the fix; the previous snapshot stays published and the listener of the
// running config still answers. The Stage 2 install binds leniently.
// PREVENTS: a SIGHUP moving geodns to a busy port reported as committed while
// DNS is down, the failure only in the log.
// Not parallel: stateP is package state.
func TestInstallReloadToUnbindableListenerIsRefused(t *testing.T) {
	data := `{"service":{"geodns":{"enabled":"true","zone":["test.example."],"nameserver":["127.0.0.1"],
		"host-set":{"external":{"host":{"proxy.test.example.":{"address":["10.7.7.7"]}}}},
		"source":{"0.0.0.0/0":{"host-set":"external"}}}}}`
	running, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	candidate, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	loopback := netip.MustParseAddr("127.0.0.1")
	a, b := freePort(t), freePort(t)
	running.Listeners = []listenerEndpoint{{IP: loopback, Port: a}}
	candidate.Listeners = []listenerEndpoint{{IP: loopback, Port: b}}

	inst := &geodnsInstaller{mgr: newServerManager(testLogger()), log: testLogger()}
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	inst.boot = true
	if err := inst.install(running); err != nil {
		t.Fatalf("boot install: %v", err)
	}
	inst.boot = false
	installed := loadState()

	busy := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(b)))
	holder, err := net.ListenPacket("udp", busy)
	if err != nil {
		t.Fatalf("occupy %s: %v", busy, err)
	}
	t.Cleanup(func() { _ = holder.Close() })

	err = inst.install(candidate)
	if err == nil {
		t.Fatal("reload to an occupied port succeeded")
	}
	for _, want := range []string{"geodns: dnsserver: reload refused", "cannot listen on " + busy, "commit again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if loadState() != installed {
		t.Error("a refused reload left its snapshot published")
	}
	resp := queryA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "proxy.test.example.", "1.2.3.4")
	if got := firstA(resp); got != "10.7.7.7" {
		t.Fatalf("running listener after the refusal: A = %q, want 10.7.7.7", got)
	}
}

// portThief occupies 127.0.0.1:port over UDP on the first listener down-edge
// once armed: the strict reload's Stop, before it binds the previous
// listeners again.
type portThief struct {
	t      *testing.T
	port   uint16
	armed  bool
	stolen net.PacketConn
}

func (p *portThief) onListenerChange(_, _ string, up bool) {
	if up {
		return
	}
	if !p.armed {
		return
	}
	if p.stolen != nil {
		return
	}
	pc, err := net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(p.port))))
	if err != nil {
		p.t.Errorf("steal port %d: %v", p.port, err)
		return
	}
	p.stolen = pc
}

func (p *portThief) release() {
	if p.stolen != nil {
		_ = p.stolen.Close()
		p.stolen = nil
	}
}

func listenerSection(t *testing.T, port uint16) []sdk.ConfigSection {
	t.Helper()
	data := `{"service":{"geodns":{"enabled":"true","zone":["test.example."],"nameserver":["127.0.0.1"],` +
		`"listener":{"main":{"ip":"127.0.0.1","port":"` + strconv.Itoa(int(port)) + `"}},` +
		`"host-set":{"external":{"host":{"proxy.test.example.":{"address":["10.7.7.7"]}}}},` +
		`"source":{"0.0.0.0/0":{"host-set":"external"}}}}}`
	return []sdk.ConfigSection{{Root: configRootService, Data: data}}
}

// VALIDATES: ISSUE-5 for geodns, through the plugin's sdk.ConfigTx. A reload
// to a busy port whose running listener's port is taken in the stop and
// rebind window fails with sdk.ErrRunningConfigLost; the transaction's
// rollback, while the port is still taken, answers an error (the bridge acks
// it CodeBroken and the engine restarts the plugin); once the port is free,
// committing the running config again binds it and DNS answers.
// PREVENTS: DNS down after a refused reload with every later commit of the
// running config answered OK as unchanged.
// Not parallel: stateP is package state.
func TestConfigTxReloadThatLosesTheRunningListener(t *testing.T) {
	thief := &portThief{t: t}
	t.Cleanup(thief.release)
	log := testLogger()
	handler := dnsserver.Authoritative(log, answerQuery, onPanic)
	inst := &geodnsInstaller{mgr: &geodnsServer{mgr: dnsserver.New(log, handler,
		dnsserver.Options{OnListenerChange: thief.onListenerChange})}, log: log}
	t.Cleanup(func() {
		inst.mgr.stopAll()
		stateP.Store(nil)
	})
	tx := sdk.NewConfigTx(geodnsConfig{}, parseGeoDNSSections, inst.install)
	a, b := freePort(t), freePort(t)
	thief.port = a
	if err := inst.configureAtBoot(tx, listenerSection(t, a)); err != nil {
		t.Fatalf("boot configure: %v", err)
	}

	busy := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(b)))
	holder, err := net.ListenPacket("udp", busy)
	if err != nil {
		t.Fatalf("occupy %s: %v", busy, err)
	}
	t.Cleanup(func() { _ = holder.Close() })
	if err := tx.Verify(listenerSection(t, b)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	thief.armed = true
	err = tx.Apply()
	if !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("reload whose running port was taken: %v, want sdk.ErrRunningConfigLost", err)
	}
	if !strings.Contains(err.Error(), "1 of the 1 running listener(s)") {
		t.Errorf("error %q does not count the lost listener", err)
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("rollback while the running port is still taken answered OK")
	}

	thief.release()
	if err := tx.Verify(listenerSection(t, a)); err != nil {
		t.Fatalf("verify the running config: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("commit the running config again: %v", err)
	}
	resp := queryA(t, "udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(a))), "proxy.test.example.", "1.2.3.4")
	if got := firstA(resp); got != "10.7.7.7" {
		t.Fatalf("running listener after the re-commit: A = %q, want 10.7.7.7", got)
	}
}
