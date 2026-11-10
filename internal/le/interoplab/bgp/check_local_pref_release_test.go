// Design: docs/architecture/testing/interop.md -- bgp-local-pref-strip-gobgp injector release
// Related: check_rfc.go -- releaseLocalPrefInjector, the code under test

package bgp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// releaseLab answers Ze's peer list from states, by peer address, and records
// every command run in the injector.
type releaseLab struct {
	states   map[string]string
	exitCode int
	queries  []string
	injected [][]string
}

func (r *releaseLab) Exec(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer != peerInject {
		return interoplab.CommandResult{}, errors.New("unexpected exec in " + peer)
	}
	r.injected = append(r.injected, argv)
	return interoplab.CommandResult{ExitCode: r.exitCode, Stderr: "refused"}, nil
}

func (r *releaseLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	return errors.New("unexpected detached exec")
}

func (r *releaseLab) Query(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (string, error) {
	if peer != "ze" {
		return "", errors.New("unexpected query in " + peer)
	}
	r.queries = append(r.queries, strings.Join(argv, " "))
	var peers []string
	for address, state := range r.states {
		peers = append(peers, `"`+address+`":{"state":"`+state+`"}`)
	}
	return `{"peers":{` + strings.Join(peers, ",") + `}}`, nil
}

func (r *releaseLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, errors.New("no logs")
}
func (r *releaseLab) PeerPID(context.Context, string) (int, error) { return 0, errors.New("no pid") }
func (r *releaseLab) Signal(context.Context, string, string) error { return errors.New("no signal") }
func (r *releaseLab) Pause(context.Context, string) error          { return errors.New("no pause") }
func (r *releaseLab) Unpause(context.Context, string) error        { return errors.New("no unpause") }
func (r *releaseLab) Start(context.Context, string) error          { return errors.New("no start") }
func (r *releaseLab) Stop(context.Context, string, int) error      { return errors.New("no stop") }

// mustReadInjectMsg reads the scenario's inject.msg from the checkout, so the
// test fails when the fixture stops naming the file the checker writes.
func mustReadInjectMsg(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", "bgp-local-pref-strip-gobgp", "inject.msg")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return body
}

// TestReleaseLocalPrefInjectorWaitsForBothDestinations proves the injector of
// bgp-local-pref-strip-gobgp is released only once Ze holds BOTH destination
// peers Established, and that the release is the file its inject.msg awaits.
//
// Method: a fake lab answers Ze's peer list. With both destinations
// established the release must be one write of localPrefRelease in the
// injector, after Ze was read. With either destination short of Established,
// or with the write refused, it must fail, and nothing must be written while
// a destination is down.
//
// PREVENTS: the burst leaving before a destination is up, when Ze's forward
// rail skips that destination and the route reaches it only through the route
// server's peer-up replay, which is not the rail the scenario measures.
func TestReleaseLocalPrefInjectorWaitsForBothDestinations(t *testing.T) {
	const (
		frr   = "172.30.0.3"
		relay = "172.30.0.10"
	)
	up := &releaseLab{states: map[string]string{frr: peerStateEstablished, relay: peerStateEstablished}}
	if err := releaseLocalPrefInjector(t.Context(), up, time.Second); err != nil {
		t.Fatalf("both destinations established: %v", err)
	}
	if len(up.queries) < 2 {
		t.Fatalf("released after %d reads of Ze's peer list, want one per destination", len(up.queries))
	}
	want := []string{"sh", "-c", ": > " + localPrefRelease}
	if len(up.injected) != 1 || !slices.Equal(up.injected[0], want) {
		t.Fatalf("injector commands %q, want exactly %q", up.injected, want)
	}
	if !strings.Contains(string(mustReadInjectMsg(t)), "option=await-file:path="+localPrefRelease+":") {
		t.Fatalf("inject.msg does not await %s, so the release gates nothing", localPrefRelease)
	}

	for _, down := range []string{frr, relay} {
		lab := &releaseLab{states: map[string]string{frr: peerStateEstablished, relay: peerStateEstablished}}
		lab.states[down] = "connect"
		err := releaseLocalPrefInjector(t.Context(), lab, 10*time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), down) {
			t.Fatalf("destination %s in connect: err=%v, want a failure naming it", down, err)
		}
		if len(lab.injected) != 0 {
			t.Fatalf("destination %s in connect, and the injector was released: %q", down, lab.injected)
		}
	}

	refused := &releaseLab{states: map[string]string{frr: peerStateEstablished, relay: peerStateEstablished}, exitCode: 1}
	if err := releaseLocalPrefInjector(t.Context(), refused, time.Second); err == nil {
		t.Fatal("a release write that exited 1 was read as a release")
	}
}
