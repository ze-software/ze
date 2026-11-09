// Design: docs/architecture/testing/interop.md -- native identical-config reload fence.
// Related: check_gobgp_reload.go -- retains the ordinary source/receiver barriers.
package bgp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// requireIdenticalRepeat checks byte identity before overwriting the path. No
// target-file reader may run after the subsequent read watch has been armed.
func (proof *gobgpReloadProof) requireIdenticalRepeat(ctx context.Context, mounted string) error {
	output, err := proof.query(ctx, "repeat-identical-config-sha256", peerGoBGP, []string{"sha256sum", gobgpReloadRunning, mounted})
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		return errors.New("repeat identity did not return exactly two file digests")
	}
	var digests [2]string
	for index, path := range []string{gobgpReloadRunning, mounted} {
		fields := strings.Fields(lines[index])
		if len(fields) != 2 {
			return errors.New("repeat file identity has an unknown digest schema")
		}
		if fields[1] != path {
			return errors.New("repeat file identity named a different file")
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil {
			return err
		}
		if len(digest) != 32 {
			return errors.New("repeat file identity is not a SHA256 digest")
		}
		digests[index] = fields[0]
	}
	if digests[0] != digests[1] {
		return errors.New("repeat config is not byte-identical to the already consumed simultaneous config")
	}
	return nil
}

// repeatReload observes two actual loader invocations over unchanged bytes. The
// second close proves the first UpdateConfig returned through the serial signal
// loop. It proves second-read consumption, NOT second-UpdateConfig completion.
func (proof *gobgpReloadProof) repeatReload(ctx context.Context) error {
	for sequence := 1; sequence <= 2; sequence++ {
		if err := proof.repeatRead(ctx, sequence); err != nil {
			return err
		}
	}
	return nil
}

// gobgpReloadReadWatch names one one-shot observer in this fresh owned container.
// repeatRead MUST pair an attempted start with stopAndJoin even on failure.
// stopAndJoin MUST run before another watch is started or the checker returns.
// This descriptor is not safe for concurrent use.
type gobgpReloadReadWatch struct {
	proof *gobgpReloadProof
	base  string
}

func (watch *gobgpReloadReadWatch) event() string { return watch.base + ".event" }
func (watch *gobgpReloadReadWatch) log() string   { return watch.base + ".log" }
func (watch *gobgpReloadReadWatch) exit() string  { return watch.base + ".exit" }

func (proof *gobgpReloadProof) repeatRead(ctx context.Context, sequence int) (resultErr error) {
	watch := gobgpReloadReadWatch{proof: proof, base: "/tmp/gobgp-repeat-read-" + strconv.Itoa(sequence)}
	// There is at most one inotifywait process in this fresh GoBGP container.
	// Nothing reads /run/gobgp.toml between watch activation and its close event:
	// identification reads mounted inputs, and snapshots use gRPC/FRR/proc only.
	defer func() {
		diagnostic, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, watch.stopAndJoin(diagnostic))
	}()
	command := "LC_ALL=C; export LC_ALL; if /usr/bin/timeout --signal=TERM --kill-after=5 15 inotifywait --event close_nowrite --format '%w%f|%e' " + gobgpReloadRunning +
		" >" + watch.event() + " 2>" + watch.log() + "; then status=0; else status=$?; fi; printf '%s\\n' \"$status\" >" + watch.exit()
	proof.evidence[watch.base+"-command"] = command
	if err := proof.check.Lab.ExecDetached(ctx, peerGoBGP, []string{"sh", "-c", command}, nil); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 5 * time.Second, Interval: 100 * time.Millisecond, Description: "native repeat configuration read watch armed"}, func(probe context.Context) (string, error) {
		return proof.query(probe, watch.base+"-readiness", peerGoBGP, []string{cmdCat, watch.log()})
	}, gobgpReloadReadWatchReady)
	if err != nil {
		return err
	}
	if err := proof.check.Lab.Signal(ctx, peerGoBGP, "HUP"); err != nil {
		return err
	}
	status, err := watch.join(ctx)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("native repeat read observer exited %d", status)
	}
	output, err := proof.query(ctx, watch.base+"-event", peerGoBGP, []string{cmdCat, watch.event()})
	if err != nil {
		return err
	}
	return requireGoBGPReloadReadEvent(output)
}

func gobgpReloadReadWatchReady(output string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		if line == "Watches established." {
			return true
		}
	}
	return false
}

func requireGoBGPReloadReadEvent(output string) error {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		return errors.New("native repeat observer did not retain exactly one event")
	}
	path, events, ok := strings.Cut(lines[0], "|")
	if !ok {
		return errors.New("native repeat read event has an unknown schema")
	}
	if path != gobgpReloadRunning {
		return errors.New("native repeat event names a different config file")
	}
	// inotify-tools4.23.9.0 emits both the specific and aggregate close names.
	if events != "CLOSE_NOWRITE,CLOSE" {
		return fmt.Errorf("native repeat event is %q, want CLOSE_NOWRITE,CLOSE", events)
	}
	return nil
}

func (watch *gobgpReloadReadWatch) join(ctx context.Context) (int, error) {
	status, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 16 * time.Second, Interval: 100 * time.Millisecond, Description: "joined native configuration read observer"}, func(probe context.Context) (int, error) {
		output, err := watch.proof.query(probe, watch.base+"-exit", peerGoBGP, []string{cmdCat, watch.exit()})
		if err != nil {
			return -1, err
		}
		status, err := strconv.Atoi(strings.TrimSpace(output))
		if err != nil {
			return -1, err
		}
		if status < 0 {
			return -1, errors.New("negative native observer exit status")
		}
		if status > 255 {
			return -1, errors.New("out-of-range native observer exit status")
		}
		return status, nil
	}, func(status int) bool { return status >= 0 })
	return status, err
}

// stopAndJoin MUST finish before the next repeatRead starts. A pkill miss is
// permitted only because join still requires the actual terminal marker; it is
// never interpreted as proof that the process was absent or reaped.
func (watch *gobgpReloadReadWatch) stopAndJoin(ctx context.Context) error {
	command := "if test ! -f " + watch.exit() + "; then pkill -TERM -x inotifywait; status=$?; test \"$status\" -le 1; fi"
	stopErr := watch.proof.exec(ctx, watch.base+"-stop", peerGoBGP, []string{"sh", "-c", command})
	status, joinErr := watch.join(ctx)
	if joinErr != nil {
		return errors.Join(stopErr, joinErr)
	}
	if status != 0 {
		return errors.Join(stopErr, fmt.Errorf("native read observer joined with unsuccessful status %d", status))
	}
	return stopErr
}
