// Design: docs/architecture/testing/ci-format.md -- routed linger at the stop fence.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

// ownedPeerFence expects a specific failure from an intentionally incomplete
// real BGP exchange. Without routed linger classification, the fence stops ze
// before that check and reports a different outcome. No elapsed-time assertion
// supplies the verdict: it must name the pending peer at the stop fence.
// MUTATION: pass nil instead of routed stdinContent to peerLingers in
// runner_exec.go. Both routes report premature shutdown instead of this fence.
func ownedPeerFence(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("owned-peer-fence needs file or pipe")
	}
	if args[0] != fieldFile && args[0] != pipeCommand {
		return fmt.Errorf("unknown fence route %q", args[0])
	}
	ze, err := uiZEBinary()
	if err != nil {
		return err
	}
	previous := env.Get("ze.bin")
	if err := env.Set("ze.bin", ze); err != nil {
		return err
	}
	defer env.Set("ze.bin", previous) //nolint:errcheck // restore the same registered key
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	base, err := os.MkdirTemp(sessionpath.EnsureScratchRoot(root), "owned-peer-fence-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base) //nolint:errcheck // fixture-owned scratch
	tests := runner.NewEncodingTests(base)
	rec := tests.Add("owned-peer-fence-" + args[0])
	rec.Activate()
	rec.Extra = map[string]string{fieldTimeout: "20s"}
	rec.Conf = map[string]any{}
	rec.EnvVars = []string{"ze.log.bgp.reactor.peer=info"}
	rec.AwaitStderr = []string{"session established"}
	rec.AwaitThenStop = true
	rec.AwaitStderrTimeout = "3s"
	rec.StdinBlocks = map[string][]byte{
		fieldPeer: []byte("option=linger:value=true\n" +
			"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000\n" +
			"expect=bgp:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020000000018C63364\n"),
		sourceConfig: []byte(`plugin { internal bgp-rib { use bgp-rib; } }
bgp {
 peer fence {
  connection { remote { ip 127.0.0.1; } local { ip 127.0.0.1; accept false; } }
  session {
   asn { local 1; remote 1; }
   router-id 10.20.30.40;
   family { ipv4/unicast { prefix { maximum 10000; } } }
   capability { graceful-restart { restart-time 120; } }
  }
  attach process bgp-rib { receive [ update state refresh ]; send [ update ]; }
 }
}
`),
	}
	peerExec := "le test peer --port $PORT"
	if args[0] == pipeCommand {
		peerExec += " -"
	}
	rec.RunCommands = []runner.RunCommand{
		{Mode: "background", Seq: 1, Exec: peerExec, Stdin: fieldPeer},
		{Mode: "foreground", Seq: 2, Exec: "ze -", Stdin: sourceConfig},
	}
	r, err := runner.NewRunner(tests, base)
	if err != nil {
		return err
	}
	defer r.Cleanup()
	passed := r.Run(ctx, &runner.RunOptions{Timeout: 25 * time.Second, Parallel: 1, Quiet: true, SkipTimings: true})
	if passed {
		return errors.New("fence passed with an unmet lingering peer expectation")
	}
	want := "await=stderr:then=stop: lingering check peer(s) stdin=peer never completed their expectations"
	if rec.Error == nil {
		return errors.New("fence failed without its pending-peer diagnostic")
	}
	if !strings.Contains(rec.Error.Error(), want) {
		return fmt.Errorf("lost routed linger classification: got %v; want %q; peer=%s; daemon=%s", rec.Error, want, rec.PeerOutput, rec.ClientStderr)
	}
	if !strings.Contains(rec.ClientStderr, "session established") {
		return errors.New("fence did not observe a real established BGP session")
	}
	if !strings.Contains(rec.PeerOutput, ":0017:02:00000000") {
		return fmt.Errorf("peer did not receive the daemon's EOR before the pending expectation: %s", rec.PeerOutput)
	}
	if strings.Contains(rec.PeerOutput, "successful") {
		return errors.New("incomplete peer announced success")
	}
	if err := ownedFenceReleased(rec); err != nil {
		return err
	}
	fmt.Println(args[0] + ": pending BGP peer held the stop fence; children reaped; lease released")
	return nil
}

func ownedFenceReleased(rec *runner.Record) error {
	children := 0
	pidPattern := regexp.MustCompile(`s pid ([0-9]+),`)
	for _, step := range rec.StepTrace {
		match := pidPattern.FindStringSubmatch(step.Assert)
		if match == nil {
			continue
		}
		pid, err := strconv.Atoi(match[1])
		if err != nil {
			return err
		}
		var status syscall.WaitStatus
		got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if !errors.Is(err, syscall.ECHILD) {
			return fmt.Errorf("fence child %d not reaped: wait4=%d, %v", pid, got, err)
		}
		children++
	}
	if children != 2 {
		return fmt.Errorf("fence checked %d children, want peer and daemon", children)
	}
	if _, err := os.Stat(rec.WorkDir); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fence work directory not released: %v", err)
	}
	lease, err := runner.LeaseTestPorts(rec.Port)
	if err != nil {
		return err
	}
	defer lease.Release()
	if lease.Start != rec.Port {
		return fmt.Errorf("fence lease %d still held; got %d", rec.Port, lease.Start)
	}
	return nil
}
