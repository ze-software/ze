// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: ../../plugins/container/storeseam_zetest.go -- the test store that puts this program in its image
//
// Command ze-containerprobe is the program of the container test store's
// fixture image. It is static, links no ze package, and has three modes.
//
// `ze-containerprobe run` is the container's entrypoint: it writes what it
// sees from inside to /report (its root, so the file lands in the writable
// layer), then waits for a termination signal and exits 0.
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
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The report: written whole to reportStaging, then renamed to reportPath, so
// a reader never sees half of it.
const (
	reportPath    = "/report"
	reportStaging = "/report.staging"
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
		fail("usage: ze-containerprobe run | reset STORE | verify STORE NAME")
	}
	switch os.Args[1] {
	case "run":
		if err := run(); err != nil {
			fail("probe run: " + err.Error())
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

// run writes the report, then waits for SIGTERM. Init of a pid namespace gets
// no signal it set no handler for, so the handler is what makes a stop
// signal end it.
func run() error {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, syscall.SIGTERM)
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
	<-terminate
	return nil
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
