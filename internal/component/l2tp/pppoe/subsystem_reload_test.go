// Design: docs/architecture/l2tp/bng-5-pppoe.md -- reload applies the diff live
//
// Goal: prove that Subsystem.Reload applies every committed `pppoe` change to
// the running access concentrator, refuses one it cannot apply before changing
// anything, and ends the sessions of a removed interface with a PADT.
// Method: build a running subsystem whose servers capture their outgoing
// frames, drive Reload with a real config.Provider holding the root in the
// shape Tree.ToMap produces, and read the servers, session tables and frames.

package pppoe

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// reloadFixture is a running subsystem with one access interface, eth0, whose
// server records every frame it sends.
type reloadFixture struct {
	sub    *Subsystem
	mu     sync.Mutex
	frames [][]byte
}

const (
	reloadEth0Index = 7
	reloadEth1Index = 8
)

func newReloadFixture(t *testing.T) *reloadFixture {
	t.Helper()
	f := &reloadFixture{}
	restore := resolveInterfaceFn
	resolveInterfaceFn = func(name string) (int, [EthALen]byte, int, error) {
		switch name {
		case "eth0":
			return reloadEth0Index, [EthALen]byte{0x02, 0, 0, 0, 0, 0x07}, 1500, nil
		case "eth1":
			return reloadEth1Index, [EthALen]byte{0x02, 0, 0, 0, 0, 0x08}, 1500, nil
		}
		return 0, [EthALen]byte{}, 0, errors.New("no such interface")
	}
	t.Cleanup(func() { resolveInterfaceFn = restore })

	p := Parameters{
		Enabled:           true,
		ACName:            "ze-before",
		CookieTimeout:     DefaultCookieTimeout,
		MaxSessions:       DefaultMaxSessions,
		MaxSessionsPerMAC: DefaultMaxSessionsPerMAC,
		PADIRateLimit:     DefaultPADIRateLimit,
		AuthMethod:        DefaultAuthMethod,
		Interfaces: []InterfaceConfig{{
			Name: "eth0", MaxSessions: DefaultMaxSessions, MaxSessionsPerMAC: DefaultMaxSessionsPerMAC,
		}},
	}
	sub := NewSubsystem(p)
	sub.logger = slog.Default()
	sub.started = true
	sub.servers = make(map[int]*InterfaceServer)
	srv, err := sub.newInterfaceServer(&sub.params.Interfaces[0], &sub.params)
	if err != nil {
		t.Fatalf("build eth0 server: %v", err)
	}
	srv.sendFrameFn = func(frame []byte) {
		f.mu.Lock()
		f.frames = append(f.frames, append([]byte(nil), frame...))
		f.mu.Unlock()
	}
	sub.servers[srv.ifIndex] = srv
	// A running subsystem: the reader has exited, so stopLocked joins at once.
	sub.readDone = make(chan struct{})
	close(sub.readDone)
	sub.stop = make(chan struct{})
	f.sub = sub
	return f
}

// reload drives Reload with a provider holding root as the pppoe config.
func (f *reloadFixture) reload(t *testing.T, root map[string]any) error {
	t.Helper()
	cp := zeconfig.NewProvider()
	if root != nil {
		cp.SetRoot(SubsystemName, root)
	}
	return f.sub.Reload(context.Background(), cp)
}

// padtSIDs returns the session id of every PADT the servers sent.
func (f *reloadFixture) padtSIDs(t *testing.T) []uint16 {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var sids []uint16
	for _, frame := range f.frames {
		pkt, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatalf("captured frame does not parse: %v", err)
		}
		if pkt.Code == CodePADT {
			sids = append(sids, pkt.SID)
		}
	}
	return sids
}

func addReloadSession(t *testing.T, srv *InterfaceServer, sid uint16) {
	t.Helper()
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, byte(sid)}
	if err := srv.sessions.Add(&Session{SID: sid, MAC: mac, IfName: srv.ifName, PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("seed session %d: %v", sid, err)
	}
}

// VALIDATES: a changed ac-name, service-name, max-sessions and padi-rate-limit
// reach the server of a kept interface, while its live session, session table
// and cookie key survive the reload untouched.
// PREVENTS: the accepted-and-ignored reload, where Reload returned nil and the
// AC kept the boot configuration until a restart.
func TestPPPoEReloadAppliesToKeptInterface(t *testing.T) {
	f := newReloadFixture(t)
	old := f.sub.servers[reloadEth0Index]
	addReloadSession(t, old, 42)

	err := f.reload(t, map[string]any{
		"enabled":         "true",
		"ac-name":         "ze-after",
		"service-name":    "internet",
		"max-sessions":    "10",
		"padi-rate-limit": "5",
		"interface":       map[string]any{"eth0": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	srv := f.sub.servers[reloadEth0Index]
	if srv == nil {
		t.Fatal("eth0 is no longer served")
	}
	if srv.acName != "ze-after" {
		t.Errorf("acName = %q, want ze-after", srv.acName)
	}
	if len(srv.serviceNames) != 1 || srv.serviceNames[0] != "internet" {
		t.Errorf("serviceNames = %v, want [internet]", srv.serviceNames)
	}
	if srv.sessions != old.sessions {
		t.Error("the reload replaced the session table of a kept interface")
	}
	if srv.cookieKey != old.cookieKey {
		t.Error("the reload replaced the cookie key, invalidating every outstanding AC-Cookie")
	}
	if srv.sessions.maxSessions != 10 {
		t.Errorf("maxSessions = %d, want 10", srv.sessions.maxSessions)
	}
	if srv.limiter == old.limiter {
		t.Error("padi-rate-limit changed but the PADI limiter was kept")
	}
	if srv.sessions.Lookup(42) == nil {
		t.Error("the reload ended a live session on a kept interface")
	}
	if sids := f.padtSIDs(t); len(sids) != 0 {
		t.Errorf("PADT sent for %v on a kept interface", sids)
	}
	if f.sub.params.ACName != "ze-after" {
		t.Errorf("params.ACName = %q, want ze-after", f.sub.params.ACName)
	}
}

// VALIDATES: an added interface is served, and a removed one is taken out of
// service with a PADT for each of its sessions (RFC 2516 Section 5.5).
// PREVENTS: a removed interface that keeps answering PADI, or whose
// subscribers are left with a session the AC no longer serves.
func TestPPPoEReloadRemovedInterfaceSendsPADT(t *testing.T) {
	f := newReloadFixture(t)
	old := f.sub.servers[reloadEth0Index]
	addReloadSession(t, old, 42)
	addReloadSession(t, old, 43)

	err := f.reload(t, map[string]any{
		"enabled":   "true",
		"interface": map[string]any{"eth1": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if f.sub.servers[reloadEth0Index] != nil {
		t.Error("eth0 is still served after the reload removed it")
	}
	if f.sub.servers[reloadEth1Index] == nil {
		t.Error("eth1 is not served after the reload added it")
	}
	sids := f.padtSIDs(t)
	if len(sids) != 2 {
		t.Fatalf("PADT sent for %v, want sessions 42 and 43", sids)
	}
	if old.sessions.Count() != 0 {
		t.Errorf("removed interface still holds %d sessions", old.sessions.Count())
	}
}

// VALIDATES: a reload adding an interface the kernel does not have is refused
// with a message naming the interface and the command that fixes it, and the
// running configuration is left exactly as it was.
// PREVENTS: a commit accepted while the interface it names is never served.
func TestPPPoEReloadRefusesMissingInterface(t *testing.T) {
	f := newReloadFixture(t)
	old := f.sub.servers[reloadEth0Index]

	err := f.reload(t, map[string]any{
		"enabled":   "true",
		"ac-name":   "ze-after",
		"interface": map[string]any{"eth9": map[string]any{}},
	})
	if err == nil {
		t.Fatal("Reload accepted an access interface that does not exist")
	}
	if !strings.Contains(err.Error(), `"eth9"`) || !strings.Contains(err.Error(), "delete pppoe interface eth9") {
		t.Errorf("error %q does not name the interface and the fix", err)
	}
	if f.sub.servers[reloadEth0Index] != old {
		t.Error("a refused reload changed the served interfaces")
	}
	if f.sub.params.ACName != "ze-before" {
		t.Errorf("a refused reload changed ac-name to %q", f.sub.params.ACName)
	}
}

// VALIDATES: a reload whose auth policy ExtractParameters refuses is refused
// before anything changes.
func TestPPPoEReloadRefusesInvalidConfig(t *testing.T) {
	f := newReloadFixture(t)
	err := f.reload(t, map[string]any{
		"enabled":     "true",
		"auth-method": "none",
		"interface":   map[string]any{"eth0": map[string]any{}},
	})
	if !errors.Is(err, ErrAuthMethodNoneRequiresAllow) {
		t.Fatalf("Reload error = %v, want ErrAuthMethodNoneRequiresAllow", err)
	}
	if f.sub.params.AuthMethod != DefaultAuthMethod {
		t.Error("a refused reload changed auth-method")
	}
}

// VALIDATES: enabled false ends every session with a PADT and stops the
// subsystem, which stays started and idle so a later reload can start it; a
// removed `pppoe` root does the same.
// PREVENTS: a disable that is accepted while the AC keeps serving.
func TestPPPoEReloadDisableStops(t *testing.T) {
	for name, root := range map[string]map[string]any{
		"enabled false": {"enabled": "false", "interface": map[string]any{"eth0": map[string]any{}}},
		"root removed":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			f := newReloadFixture(t)
			addReloadSession(t, f.sub.servers[reloadEth0Index], 42)

			if err := f.reload(t, root); err != nil {
				t.Fatalf("Reload: %v", err)
			}
			if sids := f.padtSIDs(t); len(sids) != 1 || sids[0] != 42 {
				t.Errorf("PADT sent for %v, want session 42", sids)
			}
			if len(f.sub.servers) != 0 {
				t.Errorf("%d interfaces still served", len(f.sub.servers))
			}
			if f.sub.readDone != nil {
				t.Error("the discovery reader is still recorded as running")
			}
			if !f.sub.started {
				t.Error("the subsystem is no longer started, so the next reload is refused")
			}
		})
	}
}

// VALIDATES: a reload of an idle subsystem that leaves it disabled records the
// configuration and starts nothing.
func TestPPPoEReloadIdleStaysIdle(t *testing.T) {
	sub := NewSubsystem(Parameters{})
	sub.started = true
	cp := zeconfig.NewProvider()
	cp.SetRoot(SubsystemName, map[string]any{"enabled": "false", "ac-name": "ze-after"})
	if err := sub.Reload(context.Background(), cp); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if sub.readDone != nil {
		t.Error("a disabled reload started discovery")
	}
	if sub.params.ACName != "ze-after" {
		t.Errorf("params.ACName = %q, want ze-after", sub.params.ACName)
	}
}

// VALIDATES: Reload before Start is refused rather than answered nil.
func TestPPPoEReloadBeforeStartRefused(t *testing.T) {
	sub := NewSubsystem(Parameters{})
	if err := sub.Reload(context.Background(), zeconfig.NewProvider()); !errors.Is(err, errPppoeReloadNotStarted) {
		t.Fatalf("Reload error = %v, want errPppoeReloadNotStarted", err)
	}
}
