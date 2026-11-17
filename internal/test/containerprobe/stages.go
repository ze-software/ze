// Design: docs/architecture/container/image-store.md -- the fetch, the image listing and the store totals these modes read
// Related: main.go -- the dispatch of the fetch, running, listed and diagnose modes
// Related: resp.go -- the resp mode a test runs after these

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The modes in this file drive and observe a daemon through `ze cli`, the
// way an operator does, for a container whose image comes from a real
// registry: they cannot read a report the probe wrote, because the image is
// not the probe's. Every failure line names its stage (fetch, start), and a
// failure prints the diagnosis the daemon and the kernel can give.

// The bounds of the waits. Each listing attempt is a whole `ze cli` process
// and an SSH session, so the attempts are spaced, and the progress of a
// running fetch is printed at most once every progressEvery.
const (
	cliTimeout    = 60 * time.Second // one ze cli run: the fetch command alone answers within 30 s
	listingStep   = 2 * time.Second
	progressEvery = 10 * time.Second
	runningStep   = 250 * time.Millisecond
	runningSteps  = 240 // 60 s from the image's arrival to the service's process
	listedSteps   = 30  // 60 s of listingStep, and each attempt's ze cli run, until the row reads running
)

// The commands the modes run, as an operator types them.
const (
	commandFetchPrefix = "create container image reference "
	commandImageList   = "show container image list | json"
	commandList        = "show container list | json"
	commandStore       = "show container store | json"
	commandDoctor      = "show doctor"
)

// The listing's states the fetch mode judges, as its JSON writes them
// (storeseam_operations.go, ImageState.MarshalText). Every other state,
// fetching among them, is a fetch still running.
const (
	listingPresent = "present"
	listingFailed  = "failed"
	// listedRunning is the state of a running container in show container
	// list (supervise.go StateRunning).
	listedRunning = "running"
)

// listingRow is the part of one image listing row the fetch mode reads,
// copied from the plugin's row (storeseam_operations.go): the probe links no
// ze package. The sizes stay as the daemon wrote them: the mode only prints
// them.
type listingRow struct {
	Reference string      `json:"reference"`
	Digest    string      `json:"digest"`
	Size      json.Number `json:"size"`
	State     string      `json:"state"`
	Received  json.Number `json:"received"`
	UsedBy    []string    `json:"used-by"`
	Error     string      `json:"error"`
	Code      string      `json:"code"`
}

// fetchVerdict is what one listing says of a fetch.
type fetchVerdict int

const (
	fetchVerdictUnspecified fetchVerdict = iota
	// fetchAbsent: the listing holds no row of the reference, or could not
	// be read.
	fetchAbsent
	// fetchRunning: a row of the reference, neither present nor failed.
	fetchRunning
	// fetchPresent: a row of the reference is present.
	fetchPresent
	// fetchFailed: no row of the reference is present, and one failed.
	fetchFailed
)

// fetch runs on the host beside the daemon: it runs the operator's fetch
// command for typed, prints its answer whatever its exit (a fetch longer
// than the command's wait answers an error that names the listing), then
// waits at most wait for the listing's row of listed, the reference typed
// completes to, to be present. A failed row is a FAIL at once, quoting its
// error and its code.
func fetch(typed, listed string, wait time.Duration) bool {
	answer, err := zeCLI(commandFetchPrefix + typed)
	if err != nil {
		fmt.Println("probe: the fetch command answered an error: " + err.Error())
	}
	if err == nil {
		fmt.Println("probe: the fetch command answered: " + oneLine(answer))
	}
	start := time.Now()
	deadline := start.Add(wait)
	var last string
	var progressed time.Time
	for time.Now().Before(deadline) {
		verdict, row, seen := readListing(listed)
		last = seen
		switch verdict {
		case fetchPresent:
			fmt.Println("probe: image " + listed + " is present after " + waited(start) + ": digest " + row.Digest +
				", " + row.Size.String() + " bytes, used by [" + strings.Join(row.UsedBy, ",") + "]")
			return true
		case fetchFailed:
			fmt.Println("probe: FAIL fetch: image " + listed + " failed after " + waited(start) + ": code " +
				strconv.Quote(row.Code) + ": " + row.Error)
			diagnoseDaemon()
			return false
		case fetchRunning, fetchAbsent:
			if time.Since(progressed) >= progressEvery {
				fmt.Println("probe: after " + waited(start) + ": " + seen)
				progressed = time.Now()
			}
		case fetchVerdictUnspecified:
			panic("BUG: readListing answered no verdict")
		}
		time.Sleep(listingStep)
	}
	fmt.Println("probe: FAIL fetch: image " + listed + " was not present after " + waited(start) + "; last seen: " + last)
	diagnoseDaemon()
	return false
}

// readListing runs the listing once and judges the rows of reference. seen
// says what it found, for the progress and failure lines: the row's state
// and bytes received, or why there is no verdict yet.
func readListing(reference string) (fetchVerdict, listingRow, string) {
	answer, err := zeCLI(commandImageList)
	if err != nil {
		return fetchAbsent, listingRow{}, "the listing answered an error: " + err.Error()
	}
	rows, err := listingRows(answer, reference)
	if err != nil {
		return fetchAbsent, listingRow{}, "the listing could not be read: " + err.Error() + ": " + oneLine(answer)
	}
	verdict, row := judgeRows(rows)
	if verdict == fetchAbsent {
		return verdict, row, "the listing holds no row of " + reference
	}
	return verdict, row, "state " + strconv.Quote(row.State) + ", " + row.Received.String() + " bytes received"
}

// listingRows answers the rows of reference in one listing answer: the JSON
// list the listing writes, one object a row.
func listingRows(answer []byte, reference string) ([]listingRow, error) {
	var all []listingRow
	if err := json.Unmarshal(answer, &all); err != nil {
		return nil, err
	}
	var rows []listingRow
	for _, row := range all {
		if row.Reference == reference {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// judgeRows answers the verdict of the rows of one reference and the row it
// rests on. A present row wins over a failed one: the listing keeps a failed
// row beside the present image when only the binding failed.
func judgeRows(rows []listingRow) (fetchVerdict, listingRow) {
	if len(rows) == 0 {
		return fetchAbsent, listingRow{}
	}
	for _, row := range rows {
		if row.State == listingPresent {
			return fetchPresent, row
		}
	}
	for _, row := range rows {
		if row.State == listingFailed {
			return fetchFailed, row
		}
	}
	return fetchRunning, rows[0]
}

// running runs on the host beside the daemon: it waits a bounded time until
// container name's control group holds a process whose command name is
// command, printing each change in what the group holds (each process's
// command name and user), and prints the user that process runs as. A
// process that never appears is a FAIL of the start stage, followed by the
// diagnosis.
func running(name, command string) bool {
	start := time.Now()
	var last string
	for range runningSteps {
		processes, seen := groupProcesses(name)
		if seen != last {
			fmt.Println("probe: after " + waited(start) + ": " + seen)
			last = seen
		}
		for _, process := range processes {
			if process.command == command {
				fmt.Println("probe: " + command + " runs in container " + name + " after " + waited(start) +
					": pid " + process.pid + ", uid " + process.uid)
				return true
			}
		}
		time.Sleep(runningStep)
	}
	fmt.Println("probe: FAIL start: no process named " + command + " ran in container " + name + " after " +
		waited(start) + "; last seen: " + last)
	diagnose(name)
	return false
}

// containerRow is the part of one row of show container list the listed
// mode judges (list_cmd.go ContainerRow): the probe links no ze package.
type containerRow struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Digest string `json:"digest"`
	Since  string `json:"since"`
}

// listed runs on the host beside the daemon: it waits a bounded time until
// show container list holds container name as running, then requires the
// digest of that row to be the digest the image listing gives reference, the
// whole reference of the container's image. It prints the row's since time,
// so a reader sees the start the row stands for.
func listed(name, reference string) bool {
	start := time.Now()
	last := "no listing was read"
	for range listedSteps {
		row, err := readContainerRow(name)
		if err != nil {
			last = err.Error()
			time.Sleep(listingStep)
			continue
		}
		if row.State != listedRunning {
			last = "state " + strconv.Quote(row.State) + ", reason " + strconv.Quote(row.Reason)
			time.Sleep(listingStep)
			continue
		}
		verdict, image, seen := readListing(reference)
		if verdict != fetchPresent {
			fmt.Println("probe: FAIL start: container " + name + " is listed running and image " + reference + " is not present: " + seen)
			return false
		}
		if err := sameDigest(row, image); err != nil {
			fmt.Println("probe: FAIL start: container " + name + " is listed running: " + err.Error())
			return false
		}
		fmt.Println("probe: container " + name + " is listed running after " + waited(start) + " with digest " + row.Digest +
			", the digest of image " + reference + ", since " + row.Since)
		return true
	}
	fmt.Println("probe: FAIL start: container " + name + " was not listed running after " + waited(start) + "; last seen: " + last)
	diagnose(name)
	return false
}

// readContainerRow runs show container list once and answers the row of
// name.
func readContainerRow(name string) (containerRow, error) {
	answer, err := zeCLI(commandList)
	if err != nil {
		return containerRow{}, errors.New("the container listing answered an error: " + err.Error())
	}
	return containerRowOf(answer, name)
}

// containerRowOf answers the row of name in one answer of show container
// list: the JSON list the listing writes, one object a row. An answer that
// holds no row of name, or two, is an error.
func containerRowOf(answer []byte, name string) (containerRow, error) {
	var all []containerRow
	if err := json.Unmarshal(answer, &all); err != nil {
		return containerRow{}, errors.New("the container listing could not be read: " + err.Error() + ": " + oneLine(answer))
	}
	var rows []containerRow
	for _, row := range all {
		if row.Name == name {
			rows = append(rows, row)
		}
	}
	if len(rows) != 1 {
		return containerRow{}, errors.New("the container listing holds " + strconv.Itoa(len(rows)) + " rows of " + name + ": " + oneLine(answer))
	}
	return rows[0], nil
}

// sameDigest requires the running container's row to carry a digest, and
// that digest to be the one of the image's listing row.
func sameDigest(row containerRow, image listingRow) error {
	if row.Digest == "" {
		return errors.New("its row carries no digest")
	}
	if row.Digest != image.Digest {
		return errors.New("its row carries digest " + row.Digest + ", and image " + image.Reference + " is listed with digest " + image.Digest)
	}
	return nil
}

// groupProcess is one process of a container's control group.
type groupProcess struct {
	pid     string
	command string
	uid     string
}

// groupProcesses answers the processes of container name's control group,
// and a line saying what the group holds.
func groupProcesses(name string) ([]groupProcess, string) {
	pids, err := readLines(filepath.Join(pluginControlGroups, name, "cgroup.procs"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "container " + name + " has no control group"
	}
	if err != nil {
		return nil, "container " + name + "'s control group cannot be read: " + err.Error()
	}
	if len(pids) == 0 {
		return nil, "container " + name + "'s control group holds no process"
	}
	processes := make([]groupProcess, 0, len(pids))
	shown := make([]string, 0, len(pids))
	for _, pid := range pids {
		process := groupProcess{pid: pid, command: procField(pid, "comm"), uid: processUID(pid)}
		processes = append(processes, process)
		shown = append(shown, pid+" "+process.command+" (uid "+process.uid+")")
	}
	return processes, "container " + name + "'s control group holds " + strings.Join(shown, ", ")
}

// procField answers the file field of process pid, trimmed, or what kept it
// from being read: the process may have exited between the two reads.
func procField(pid, field string) string {
	data, err := os.ReadFile("/proc/" + pid + "/" + field) //nolint:gosec // a process of the test's own container
	if err != nil {
		return "unreadable: " + err.Error()
	}
	return strings.TrimSpace(string(data))
}

// processUID answers the real user of process pid, from its status file.
func processUID(pid string) string {
	for line := range strings.SplitSeq(procField(pid, "status"), "\n") {
		ids, found := strings.CutPrefix(line, "Uid:")
		if !found {
			continue
		}
		fields := strings.Fields(ids)
		if len(fields) == 0 {
			return "unreadable: an empty Uid line"
		}
		return fields[0]
	}
	return "unreadable: no Uid line"
}

// diagnose prints the whole diagnosis for container name: the daemon's and
// the kernel's. The diagnose mode runs it as a step of its own, which always
// succeeds: its lines are the evidence a failure elsewhere is read against.
func diagnose(name string) {
	diagnoseDaemon()
	for _, file := range groupFiles {
		diagnoseFile(filepath.Join(pluginControlGroups, name, file))
	}
	_, seen := groupProcesses(name)
	fmt.Println("probe: diagnosis: " + seen)
}

// diagnoseDaemon prints what the daemon says of the store and of itself, and
// what the host offers a fetch over the internet: its resolver and the
// trust roots Go's TLS reads.
func diagnoseDaemon() {
	for _, command := range []string{commandImageList, commandStore, commandDoctor} {
		answer, err := zeCLI(command)
		if err != nil {
			fmt.Println("probe: diagnosis: `" + command + "` answered an error: " + err.Error())
			continue
		}
		fmt.Println("probe: diagnosis: `" + command + "` answered: " + oneLine(answer))
	}
	diagnoseFile("/etc/resolv.conf")
	for _, path := range trustRootFiles {
		if _, err := os.Stat(path); err != nil {
			fmt.Println("probe: diagnosis: trust roots " + path + ": " + err.Error())
			continue
		}
		fmt.Println("probe: diagnosis: trust roots " + path + " exist")
	}
}

// trustRootFiles are the bundles Go's crypto/x509 reads on Linux
// (root_linux.go certFiles), the first that exists being the one it uses.
var trustRootFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/ca-bundle.pem",
	"/etc/pki/tls/cacert.pem",
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
	"/etc/ssl/cert.pem",
}

// groupFiles are the control group files that say why a container ended:
// its processes, and the events of its memory and process limits (an OOM
// kill, a fork refused at the process limit).
var groupFiles = []string{"cgroup.procs", "memory.events", "memory.max", "pids.events", "pids.max"}

// diagnoseFile prints the content of the file at path, on one line.
func diagnoseFile(path string) {
	data, err := os.ReadFile(path) //nolint:gosec // a host file the diagnosis names
	if err != nil {
		fmt.Println("probe: diagnosis: " + path + ": " + err.Error())
		return
	}
	fmt.Println("probe: diagnosis: " + path + ": " + oneLine(data))
}

// oneLine answers text with its line ends shown as " | " and its runs of
// space collapsed, so one probe line carries a whole answer.
func oneLine(text []byte) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(text)), "\n", " | ")), " ")
}

// zeCLI runs `ze cli -c command`, bounded by cliTimeout, and answers its
// standard output. A run that exits nonzero answers an error quoting its
// exit status and both outputs.
func zeCLI(command string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()
	run := exec.CommandContext(ctx, "ze", "cli", "-c", command) // #nosec G204 G702 -- the program is ze, found on the PATH the functional runner gives the step, and command is one of this file's constants or the reference the .ci step names.
	output, err := run.Output()
	if exited, isExit := errors.AsType[*exec.ExitError](err); isExit {
		return nil, errors.New("exit " + strconv.Itoa(exited.ExitCode()) + ": " + oneLine(exited.Stderr) + " " + oneLine(output))
	}
	if err != nil {
		return nil, err
	}
	return output, nil
}
