// Design: docs/architecture/storage-backends.md -- the boot record
// Related: register_stop_boot_record.go -- the registration
// Related: cmd/ze/hub/config_source.go -- awaitBootRecordAtStop, the wait this drives

package fixture

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const (
	// stopBootRecordServing is the line ze prints once it serves, after the
	// boot record has started (runYANGConfig, cmd/ze/hub/main.go). The first
	// signal waits for it, so the stop always finds a record in flight.
	stopBootRecordServing = "Ze running."
	// stopBootRecordWaiting is the line the stop prints when the record is
	// still running (awaitBootRecordAtStop). Every later signal waits for it,
	// so it reaches the daemon inside that wait and nowhere earlier: a second
	// signal during the stop's fence only skips the drain.
	stopBootRecordWaiting = "Stopping: the store is still recording the boot configuration"
	// stopBootRecordTag opens every line the driver writes on stderr.
	stopBootRecordTag = "stop-boot-record: "
)

// stopDuringBootRecord runs `ze start <config>`, relays its stdout line by line,
// signals it, and checks the exit code it ends with.
//
// Arguments: <config> <signals> <exit-code>. <signals> is a comma-separated list
// of `term` or `int`. The first is sent once ze prints that it serves; each
// later one once the stop announces that it waits for the boot record. A
// signal the daemon never gave the cue for is a failure, so a stop that does
// not wait cannot pass for one that does.
func stopDuringBootRecord(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("hub/stop-during-boot-record takes <config> <signals> <exit-code>")
	}
	signals, err := parseStopBootRecordSignals(args[1])
	if err != nil {
		return err
	}
	exitWanted, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("hub/stop-during-boot-record: exit code %q is not a number", args[2])
	}

	command := exec.CommandContext(ctx, "ze", "start", args[0]) //nolint:gosec // the fixture chooses the program; the config path is the test's own
	command.Env = os.Environ()
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}

	sent, relayErr := relayStopBootRecord(command.Process, stdout, signals)
	waitErr := command.Wait()
	if relayErr != nil {
		return relayErr
	}
	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return waitErr
		}
		exitCode = exitErr.ExitCode()
	}
	if sent < len(signals) {
		return fmt.Errorf("hub/stop-during-boot-record: sent %d of %d signals; ze exited %d without printing the cue for the next (%q first, then %q)", sent, len(signals), exitCode, stopBootRecordServing, stopBootRecordWaiting)
	}
	if exitCode != exitWanted {
		return fmt.Errorf("hub/stop-during-boot-record: ze exited %d, want %d", exitCode, exitWanted)
	}
	return nil
}

// parseStopBootRecordSignals reads the comma-separated signal list.
func parseStopBootRecordSignals(list string) ([]syscall.Signal, error) {
	names := strings.Split(list, ",")
	signals := make([]syscall.Signal, 0, len(names))
	for _, name := range names {
		switch name {
		case "term":
			signals = append(signals, syscall.SIGTERM)
		case "int":
			signals = append(signals, syscall.SIGINT)
		default:
			return nil, fmt.Errorf("hub/stop-during-boot-record: signal %q is neither term nor int", name)
		}
	}
	return signals, nil
}

// relayStopBootRecord copies the daemon's stdout to the fixture's own, line by
// line, and sends each signal on its cue. It returns how many it sent once the
// daemon closes its stdout, which it does as it exits.
func relayStopBootRecord(process *os.Process, stdout io.Reader, signals []syscall.Signal) (int, error) {
	sent := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := os.Stdout.WriteString(line + "\n"); err != nil {
			return sent, err
		}
		if sent == len(signals) {
			continue
		}
		cue := stopBootRecordWaiting
		if sent == 0 {
			cue = stopBootRecordServing
		}
		if !strings.HasPrefix(line, cue) {
			continue
		}
		if err := process.Signal(signals[sent]); err != nil {
			return sent, fmt.Errorf("hub/stop-during-boot-record: signal ze: %w", err)
		}
		_, _ = os.Stderr.WriteString(stopBootRecordTag + "sent " + signals[sent].String() + " after " + strconv.Quote(cue) + "\n") //nolint:errcheck // a progress note; the assertions read the daemon's lines
		sent++
	}
	return sent, scanner.Err()
}
