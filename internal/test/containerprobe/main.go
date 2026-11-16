// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: ../../plugins/container/storeseam_zetest.go -- the test store that puts this program in its image
//
// Command ze-containerprobe is the program of the container test store's
// fixture image. It is static, links no ze package, and has these modes.
//
// `ze-containerprobe run` is the container's entrypoint: it appends a line to
// /starts, writes what it sees from inside to /report (its root, so both land
// in the writable layer), then waits for a termination signal, records it in
// /signals and exits 0.
//
// `ze-containerprobe run-ignoring-stop` is the entrypoint of a container that
// ignores its stop signal: it does what run does, then records every stop
// signal it gets in /signals and keeps running until it is killed. The test
// store gives it to a container whose name starts with "stubborn".
//
// `ze-containerprobe gone STORE NAME [at-least|less-than SECONDS]` runs on
// the host after a stop: it waits a bounded time until container NAME has no
// control group, root mount or runtime library state left, requires a
// recorded stop signal, and with a bound judges the time from that signal to
// the end.
//
// `ze-containerprobe starts STORE NAME COUNT` runs on the host: it waits a
// bounded time for COUNT starts in the record, then requires exactly COUNT
// and exactly one process in the container's control group.
//
// `ze-containerprobe reset STORE` runs on the host before the test daemon
// starts: it removes the test store directory STORE, so a report an earlier
// run left there cannot answer for this one. It refuses a path outside
// /run/ze-container-test-*.
//
// `ze-containerprobe verify STORE NAME` runs on the host, beside the test
// daemon: it waits a bounded time for the report of container NAME in the
// test store directory STORE, compares each fact with what a container of
// its own must see, prints one line per check, and exits 1 when one fails.
//
// `ze-containerprobe confined STORE NAME` runs on the host the same way and
// judges the default confinement of container NAME: only its own process,
// its cgroup as the root, the default capability mask in its effective and
// bounding sets, and the two answers of the system-call filter, which it
// first makes on the host to prove the host answers otherwise
// (confinement_linux.go).
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// drive runs a daemon on config itself, as `ze start CONFIG`, for a scenario
// whose steps a .ci line cannot send: signals and a config change between
// observations of the container. Once container name runs:
//
//   - second-signal: SIGTERM starts the daemon's stop and its drain; once the
//     container recorded its stop signal, so the drain is under way, a second
//     SIGTERM aborts the drain, and the container must then be gone less than
//     20 seconds after its signal, well inside its 30 second stop timeout.
//   - remove: config is overwritten with the content of removed, which holds
//     no such container, and SIGHUP reloads it; the container must be gone,
//     with its stop signal recorded. The daemon then stops.
func drive(store, name, config, action, removed string) bool {
	// Every return below stops the daemon and waits for its exit first, so
	// the cancel the defer runs never ends a daemon drive still observes.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemon := exec.CommandContext(ctx, "ze", "start", config) // #nosec G204 G702 -- the program is ze, found on the PATH the functional runner gives the step, and config is the file the .ci step names in its own work directory.
	daemon.Stdout, daemon.Stderr = os.Stdout, os.Stderr
	if err := daemon.Start(); err != nil {
		return judge("the daemon starts", false, err.Error())
	}
	exited := make(chan error, 1)
	go awaitExit(daemon, exited)
	stopDaemon := func() bool {
		daemon.Process.Signal(syscall.SIGTERM) //nolint:errcheck,gosec // an exited daemon is what is waited for
		select {
		case <-exited:
			return true
		case <-time.After(recordWaitStep * recordWaitSteps):
			daemon.Process.Kill() //nolint:errcheck,gosec // the bound passed: the verdict is the failure below
			<-exited
			return judge("the daemon exits after its stop", false, "still running")
		}
	}
	report := filepath.Join(store, "layer", name, "upper", strings.TrimPrefix(reportPath, "/"))
	if _, err := awaitReport(report); err != nil {
		stopDaemon()
		return judge("the container runs", false, err.Error())
	}
	fmt.Println("probe: the container runs")
	switch action {
	case "second-signal":
		daemon.Process.Signal(syscall.SIGTERM) //nolint:errcheck,gosec // the daemon was just seen running
		if !awaitSignal(store, name) {
			stopDaemon()
			return false
		}
		if !stopDaemon() {
			return false
		}
		return gone(store, name, []string{"less-than", "20"})
	case "remove":
		data, err := os.ReadFile(removed) //nolint:gosec // the test's own config file
		if err != nil {
			stopDaemon()
			return judge("the config without the container is read", false, err.Error())
		}
		if err := os.WriteFile(config, data, 0o644); err != nil { //nolint:gosec // the daemon reads it
			stopDaemon()
			return judge("the config without the container is written", false, err.Error())
		}
		daemon.Process.Signal(syscall.SIGHUP) //nolint:errcheck,gosec // the daemon was just seen running
		removedGone := gone(store, name, nil)
		return stopDaemon() && removedGone
	}
	stopDaemon()
	return judge("the action is second-signal or remove", false, action)
}

// awaitExit is the one waiter of the daemon process drive started, for the
// process's whole life: it hands the exit to exited.
func awaitExit(daemon *exec.Cmd, exited chan<- error) {
	exited <- daemon.Wait()
}

// awaitSignal waits, a bounded time, until container name recorded a stop
// signal.
func awaitSignal(store, name string) bool {
	path := filepath.Join(store, "layer", name, "upper", strings.TrimPrefix(signalsPath, "/"))
	for range recordWaitSteps {
		if lines, err := readLines(path); err == nil && len(lines) > 0 {
			fmt.Println("probe: the container received its stop signal while the daemon drains")
			return true
		}
		time.Sleep(recordWaitStep)
	}
	return judge("the container received its stop signal while the daemon drains", false, "no record at "+path)
}

// The report: written whole to reportStaging, then renamed to reportPath, so
// a reader never sees half of it.
const (
	reportPath    = "/report"
	reportStaging = "/report.staging"
)

// usage names every mode.
const usage = "usage: ze-containerprobe run | run-ignoring-stop | reset STORE | verify STORE NAME | confined STORE NAME" +
	" | gone STORE NAME [at-least|less-than SECONDS] | starts STORE NAME COUNT" +
	" | drive STORE NAME CONFIG second-signal|remove [REMOVED-CONFIG]"

// The records the entrypoint appends to its root, so they land in the
// writable layer and outlive a restart: one line per start, and one line per
// stop signal received.
const (
	startsPath  = "/starts"
	signalsPath = "/signals"
)

// stopSignals are the signals a stop can send (the stop-signal leaf), which
// the stubborn mode records and ignores.
var stopSignals = []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2}

// The plugin's own places on the host, copied from the container plugin
// (describe_linux.go cgroupParent, rootfs_linux.go rootMountDirectory and
// libraryStateRoot): the probe links no ze package, and a test that finds one
// of them still there after a stop has found a leftover.
const (
	pluginControlGroups = "/sys/fs/cgroup/ze-container"
	pluginRootMounts    = "/run/ze-container/root"
	pluginLibraryState  = "/run/ze-container/state"
)

// The bound of the waits of gone and starts: a daemon stop drains for at most
// 37 seconds, and a restarted daemon starts its containers well within it.
const (
	recordWaitStep  = 100 * time.Millisecond
	recordWaitSteps = 450
)

// namespaces are the namespaces a container gets of its own by default.
var namespaces = []string{"pid", "mnt", "net", "ipc", "uts", "cgroup"}

// The bound of the verifier's wait for the report: the daemon starts, the
// store installs the image, and the container starts within it.
const (
	reportWaitStep  = 100 * time.Millisecond
	reportWaitSteps = 300
)

// iffUp is the IFF_UP bit of a network device's flags.
const iffUp = 0x1

func main() {
	if len(os.Args) < 2 {
		fail(usage)
	}
	switch os.Args[1] {
	case "run":
		if err := run(false); err != nil {
			fail("probe run: " + err.Error())
		}
	case "run-ignoring-stop":
		if err := run(true); err != nil {
			fail("probe run-ignoring-stop: " + err.Error())
		}
	case "gone":
		switch len(os.Args) {
		case 4, 6:
		default:
			fail("usage: ze-containerprobe gone STORE NAME [at-least|less-than SECONDS]")
		}
		if !gone(os.Args[2], os.Args[3], os.Args[4:]) {
			os.Exit(1)
		}
	case "drive":
		switch len(os.Args) {
		case 6, 7:
		default:
			fail("usage: ze-containerprobe drive STORE NAME CONFIG second-signal|remove [REMOVED-CONFIG]")
		}
		removed := ""
		if len(os.Args) == 7 {
			removed = os.Args[6]
		}
		if !drive(os.Args[2], os.Args[3], os.Args[4], os.Args[5], removed) {
			os.Exit(1)
		}
	case "starts":
		if len(os.Args) != 5 {
			fail("usage: ze-containerprobe starts STORE NAME COUNT")
		}
		count, err := strconv.Atoi(os.Args[4])
		if err != nil {
			fail("probe starts: COUNT " + strconv.Quote(os.Args[4]) + " is not a number")
		}
		if !starts(os.Args[2], os.Args[3], count) {
			os.Exit(1)
		}
	case "reset":
		if len(os.Args) != 3 {
			fail("usage: ze-containerprobe reset STORE")
		}
		if err := reset(os.Args[2]); err != nil {
			fail("probe reset: " + err.Error())
		}
	case "verify":
		if len(os.Args) != 4 {
			fail("usage: ze-containerprobe verify STORE NAME")
		}
		if !verify(os.Args[2], os.Args[3]) {
			os.Exit(1)
		}
	case "confined":
		if len(os.Args) != 4 {
			fail("usage: ze-containerprobe confined STORE NAME")
		}
		if !confined(os.Args[2], os.Args[3]) {
			os.Exit(1)
		}
	default:
		fail("unknown mode " + strconv.Quote(os.Args[1]))
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message) //nolint:errcheck // the exit status carries the failure
	os.Exit(2)
}

// storePrefix is where every test store lives. The test's work directory is
// no place for one: in the QEMU guest it sits on the 9p share of the
// checkout, and an overlay cannot take a 9p directory as its upper layer.
// /run is tmpfs in the guest and on the appliance.
const storePrefix = "/run/ze-container-test-"

// reset removes the test store directory store, so the report of an earlier
// run cannot pass this one. It refuses any path outside storePrefix, so a
// mistyped argument cannot remove a directory the test does not own.
func reset(store string) error {
	if !strings.HasPrefix(filepath.Clean(store), storePrefix) {
		return errors.New("refusing to remove " + store + ": a test store lives under " + storePrefix + "*")
	}
	return os.RemoveAll(store)
}

// run appends a line to /starts, writes the report, then waits for a stop
// signal and appends "NUMBER UNIX-NANOSECONDS" to /signals for each one it
// gets. It exits 0 at the first SIGTERM, unless ignoreStop is set: then it
// records every signal a stop can send and keeps running until it is killed.
// Init of a pid namespace gets no signal it set no handler for, so the
// handler is what makes a stop signal reach it at all.
func run(ignoreStop bool) error {
	stops := make(chan os.Signal, len(stopSignals))
	if ignoreStop {
		signal.Notify(stops, stopSignals...)
	} else {
		signal.Notify(stops, syscall.SIGTERM)
	}
	if err := appendLine(startsPath, strconv.AppendInt(nil, time.Now().UnixNano(), 10)); err != nil {
		return err
	}
	facts, err := observe()
	if err != nil {
		return err
	}
	if err := os.WriteFile(reportStaging, []byte(facts), 0o644); err != nil { //nolint:gosec // a report the host test reads
		return err
	}
	if err := os.Rename(reportStaging, reportPath); err != nil {
		return err
	}
	// The loop ends only when the process does: by the first SIGTERM, or by
	// the kill a stubborn container waits for.
	for received := range stops {
		number, isSignal := received.(syscall.Signal)
		if !isSignal {
			return errors.New("a stop signal that is no Linux signal")
		}
		line := strconv.AppendInt(nil, int64(number), 10)
		line = append(line, ' ')
		line = strconv.AppendInt(line, time.Now().UnixNano(), 10)
		if err := appendLine(signalsPath, line); err != nil {
			return err
		}
		if !ignoreStop {
			return nil
		}
	}
	return nil
}

// appendLine appends line and a newline to the file at path, creating it.
func appendLine(path string, line []byte) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // a record the host test reads
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		file.Close() //nolint:errcheck,gosec // the write error is the one reported
		return err
	}
	return file.Close()
}

// observe reads, from inside the container, the facts the verifier checks.
func observe() (string, error) {
	var report strings.Builder
	report.WriteString("pid=" + strconv.Itoa(os.Getpid()) + "\n")
	cgroup, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	report.WriteString("cgroup=" + strings.TrimSpace(string(cgroup)) + "\n")
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	report.WriteString("hostname=" + hostname + "\n")
	flags, err := os.ReadFile("/sys/class/net/lo/flags")
	if err != nil {
		return "", err
	}
	report.WriteString("loopback-flags=" + strings.TrimSpace(string(flags)) + "\n")
	devices, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(devices))
	for _, device := range devices {
		names = append(names, device.Name())
	}
	slices.Sort(names)
	report.WriteString("interfaces=" + strings.Join(names, ",") + "\n")
	for _, namespace := range namespaces {
		link, err := os.Readlink("/proc/self/ns/" + namespace)
		if err != nil {
			return "", err
		}
		report.WriteString("ns-" + namespace + "=" + link + "\n")
	}
	confinement, err := observeConfinement()
	if err != nil {
		return "", err
	}
	report.WriteString(confinement)
	return report.String(), nil
}

// verify judges the report of container name in the store directory store.
func verify(store, name string) bool {
	facts, err := awaitReport(filepath.Join(store, "layer", name, "upper", "report"))
	if err != nil {
		fmt.Println("probe: FAIL " + err.Error())
		return false
	}
	passed := true
	check := func(what string, ok bool, seen string) {
		if ok {
			fmt.Println("probe: " + what)
			return
		}
		passed = false
		fmt.Println("probe: FAIL " + what + ": saw " + strconv.Quote(seen))
	}
	check("pid 1 in its own pid namespace", facts["pid"] == "1", facts["pid"])
	check("its own cgroup reads as the root", facts["cgroup"] == "0::/", facts["cgroup"])
	check("host name is the instance name", facts["hostname"] == name, facts["hostname"])
	check("the loopback device is the only device", facts["interfaces"] == "lo", facts["interfaces"])
	flags, err := strconv.ParseUint(strings.TrimPrefix(facts["loopback-flags"], "0x"), 16, 32)
	check("the loopback device is up", err == nil && flags&iffUp != 0, facts["loopback-flags"])
	for _, namespace := range namespaces {
		host, err := os.Readlink("/proc/self/ns/" + namespace)
		seen := facts["ns-"+namespace]
		check("a "+namespace+" namespace other than the host's", err == nil && seen != "" && seen != host, seen)
	}
	return passed
}

// readFacts reads a report's key=value lines by key, and closes file.
func readFacts(file *os.File) (map[string]string, error) {
	defer file.Close() //nolint:errcheck // a read-only file
	facts := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if found {
			facts[key] = value
		}
	}
	return facts, scanner.Err()
}

// awaitReport waits, a bounded time, for the report at path, then answers its
// facts by name.
func awaitReport(path string) (map[string]string, error) {
	for range reportWaitSteps {
		file, err := os.Open(path) //nolint:gosec // the report of the test's own container
		if errors.Is(err, os.ErrNotExist) {
			time.Sleep(reportWaitStep)
			continue
		}
		if err != nil {
			return nil, err
		}
		return readFacts(file)
	}
	return nil, errors.New("no report at " + path + " after " + (reportWaitStep * reportWaitSteps).String() + ": the container did not start")
}

// judge prints one check's line and answers ok.
func judge(what string, ok bool, seen string) bool {
	if ok {
		fmt.Println("probe: " + what)
		return true
	}
	fmt.Println("probe: FAIL " + what + ": saw " + strconv.Quote(seen))
	return false
}

// gone waits, a bounded time, until container name has left nothing on the
// host: no control group, no root mount point, no runtime library state.
// Then it reads the stop signals the container recorded and requires one.
// With a bound, at-least or less-than SECONDS, it also requires the time from
// that first signal to the moment the leftovers were seen gone to be at least,
// or less than, SECONDS: a container killed at its stop timeout outlives its
// signal by that timeout, one killed at once does not.
func gone(store, name string, bound []string) bool {
	places := []struct{ what, path string }{
		{"its control group is gone", filepath.Join(pluginControlGroups, name)},
		{"its root mount is gone", filepath.Join(pluginRootMounts, name)},
		{"its runtime library state is gone", filepath.Join(pluginLibraryState, name)},
	}
	var goneAt time.Time
	for range recordWaitSteps {
		left := 0
		for _, place := range places {
			if _, err := os.Lstat(place.path); err == nil {
				left++
			}
		}
		if left == 0 {
			goneAt = time.Now()
			break
		}
		time.Sleep(recordWaitStep)
	}
	passed := true
	for _, place := range places {
		_, err := os.Lstat(place.path)
		passed = judge(place.what, errors.Is(err, os.ErrNotExist), place.path) && passed
	}
	if !passed {
		return false
	}
	lines, err := readLines(filepath.Join(store, "layer", name, "upper", strings.TrimPrefix(signalsPath, "/")))
	if !judge("the container received a stop signal", err == nil && len(lines) > 0, strings.Join(lines, ";")) {
		return false
	}
	number, nanos, found := strings.Cut(lines[0], " ")
	signaled, parseErr := strconv.ParseInt(nanos, 10, 64)
	if !judge("the stop signal is recorded with its time", found && parseErr == nil, lines[0]) {
		return false
	}
	fmt.Println("probe: the first stop signal was " + number)
	if len(bound) == 0 {
		return true
	}
	seconds, err := strconv.Atoi(bound[1])
	if err != nil {
		return judge("the bound is a number of seconds", false, bound[1])
	}
	lived := goneAt.Sub(time.Unix(0, signaled))
	limit := time.Duration(seconds) * time.Second
	switch bound[0] {
	case "at-least":
		return judge("it ended at least "+bound[1]+"s after its stop signal", lived >= limit, lived.String())
	case "less-than":
		return judge("it ended less than "+bound[1]+"s after its stop signal", lived < limit, lived.String())
	}
	return judge("the bound is at-least or less-than", false, bound[0])
}

// starts waits, a bounded time, until container name recorded count starts
// in its writable layer, then requires exactly count, and exactly one process
// in its control group: a container started again as a new process, alone,
// with nothing of an earlier run beside it.
func starts(store, name string, count int) bool {
	path := filepath.Join(store, "layer", name, "upper", strings.TrimPrefix(startsPath, "/"))
	var lines []string
	for range recordWaitSteps {
		lines, _ = readLines(path) //nolint:errcheck // an absent record is a count of none until the wait ends
		if len(lines) >= count {
			break
		}
		time.Sleep(recordWaitStep)
	}
	passed := judge("the container started "+strconv.Itoa(count)+" times", len(lines) == count, strings.Join(lines, ";"))
	processes, err := readLines(filepath.Join(pluginControlGroups, name, "cgroup.procs"))
	seen := strings.Join(processes, ",")
	if err != nil {
		seen = err.Error()
	}
	return judge("one process runs in its control group", err == nil && len(processes) == 1, seen) && passed
}

// readLines answers the non-empty lines of the file at path.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // a record of the test's own container
	if err != nil {
		return nil, err
	}
	var lines []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
