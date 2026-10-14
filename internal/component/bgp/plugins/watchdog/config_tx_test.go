package watchdog

import (
	"slices"
	"strings"
	"sync"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const reloadPeer = "127.0.0.1"

// poolSection is a bgp delivery giving peer reloadPeer one update per prefix,
// each in watchdog pool dnsr and withdrawn until announced. med sets the
// first prefix's MED, so a test can change one entry's announce command.
func poolSection(med string, prefixes ...string) []sdk.ConfigSection {
	var b strings.Builder
	b.WriteString(`{"bgp":{"peer":{"peer1":{"connection":{"remote":{"ip":"` + reloadPeer + `"}},"update":{`)
	for i, prefix := range prefixes {
		if i > 0 {
			b.WriteString(",")
		}
		attrs := `"origin":"igp","next-hop":"1.2.3.4"`
		if i == 0 && med != "" {
			attrs += `,"med":"` + med + `"`
		}
		b.WriteString(`"u` + prefix + `":{"attribute":{` + attrs + `},` +
			`"nlri":{"ipv4/unicast":{"content":"add ` + prefix + `"}},` +
			`"watchdog":{"name":"dnsr","withdraw":"true"}}`)
	}
	b.WriteString(`}}}}}`)
	return []sdk.ConfigSection{{Root: configRootBGP, Data: b.String()}}
}

// commandRecorder records every route command the server sends.
type commandRecorder struct {
	mu   sync.Mutex
	sent []string
}

func (r *commandRecorder) send(_, command string) {
	r.mu.Lock()
	r.sent = append(r.sent, command)
	r.mu.Unlock()
}

// take returns the commands sent since the last take.
func (r *commandRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	sent := r.sent
	r.sent = nil
	return sent
}

// announcedPrefixes answers which prefixes of pool dnsr are announced to
// reloadPeer, sorted.
func announcedPrefixes(srv *watchdogServer) []string {
	srv.mu.RLock()
	pools := srv.peerPools[reloadPeer]
	srv.mu.RUnlock()
	var keys []string
	for _, entry := range pools.AnnouncedForPeer("dnsr", reloadPeer) {
		keys = append(keys, entry.Key)
	}
	slices.Sort(keys)
	return keys
}

// countContaining counts the commands that contain every fragment.
func countContaining(commands []string, fragments ...string) int {
	count := 0
	for _, command := range commands {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(command, fragment) {
				matched = false
			}
		}
		if matched {
			count++
		}
	}
	return count
}

// announcedServer configures pool dnsr with the given prefixes, brings the
// peer up and announces the pool, the state a reload then starts from.
func announcedServer(t *testing.T, prefixes ...string) (*watchdogServer, *watchdogConfigTx, *commandRecorder) {
	t.Helper()
	recorder := &commandRecorder{}
	srv := newWatchdogServer(recorder.send, nil)
	tx := &watchdogConfigTx{srv: srv}
	if err := tx.configure(poolSection("", prefixes...)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	srv.mu.Lock()
	srv.peerUp[reloadPeer] = true
	srv.mu.Unlock()
	if _, _, err := srv.handleCommand("request bgp watchdog announce", []string{"dnsr"}, reloadPeer); err != nil {
		t.Fatalf("announce: %v", err)
	}
	if got := len(recorder.take()); got != len(prefixes) {
		t.Fatalf("announce sent %d commands, want %d", got, len(prefixes))
	}
	return srv, tx, recorder
}

// VALIDATES: spec-plugin-reload-applies-or-refuses R-7. A reload that keeps an
// announced entry keeps it announced and sends nothing for it; one that edits
// a kept entry's attributes sends the new announce once.
// PREVENTS: a swap that loses announce state (the kept route silently counted
// as withdrawn) or announces a kept route twice.
func TestWatchdogApplyKeepsAnnounceState(t *testing.T) {
	srv, tx, recorder := announcedServer(t, "10.0.0.0/24", "10.0.1.0/24")

	if err := tx.verify(poolSection("", "10.0.0.0/24", "10.0.1.0/24")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("an unchanged reload sent %q, want nothing", sent)
	}
	if got := announcedPrefixes(srv); len(got) != 2 {
		t.Fatalf("after an unchanged reload announced %v, want both prefixes", got)
	}

	if err := tx.verify(poolSection("50", "10.0.0.0/24", "10.0.1.0/24")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	sent := recorder.take()
	if len(sent) != 1 || countContaining(sent, "10.0.0.0/24", "50") != 1 {
		t.Fatalf("an edited entry sent %q, want one announce of 10.0.0.0/24 carrying the new MED", sent)
	}
	if got := announcedPrefixes(srv); len(got) != 2 {
		t.Fatalf("after an edit announced %v, want both prefixes", got)
	}
}

// VALIDATES: spec-plugin-reload-applies-or-refuses R-7 and AC-4 for watchdog.
// A reload that removes an announced entry withdraws it; a rollback brings it
// back announced; a refused verify leaves an apply nothing to apply.
// PREVENTS: a removed entry left announced on the wire with no pool to
// withdraw it from.
func TestWatchdogApplyWithdrawsRemoved(t *testing.T) {
	srv, tx, recorder := announcedServer(t, "10.0.0.0/24", "10.0.1.0/24")

	if err := tx.verify(poolSection("", "10.0.0.0/24")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sent := recorder.take(); len(sent) != 0 {
		t.Fatalf("verify sent %q, want nothing before apply", sent)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	sent := recorder.take()
	if len(sent) != 1 || countContaining(sent, "del 10.0.1.0/24") != 1 {
		t.Fatalf("removing 10.0.1.0/24 sent %q, want its one withdraw", sent)
	}
	if got := announcedPrefixes(srv); len(got) != 1 {
		t.Fatalf("after the removal announced %v, want only 10.0.0.0/24", got)
	}

	if err := tx.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	sent = recorder.take()
	if len(sent) != 1 || countContaining(sent, "add 10.0.1.0/24") != 1 {
		t.Fatalf("rollback sent %q, want the one re-announce of 10.0.1.0/24", sent)
	}
	if got := announcedPrefixes(srv); len(got) != 2 {
		t.Fatalf("after rollback announced %v, want both prefixes", got)
	}

	if err := tx.verify([]sdk.ConfigSection{{Root: configRootBGP, Data: "{not json"}}); err == nil {
		t.Fatal("verify of invalid JSON: err nil, want a refusal")
	}
	if err := tx.apply(); err == nil {
		t.Fatal("apply after a refused verify: err nil, want sdk.ErrConfigApplyUnverified")
	}
}
