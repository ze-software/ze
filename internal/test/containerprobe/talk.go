// Design: docs/architecture/container/runtime.md -- the output of a container
// Related: main.go -- the dispatch of the run-talking and stopped-finding modes
// Related: stages.go -- zeCLI, which the stopped-finding mode asks the daemon through

package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// What the run-talking entrypoint writes and the status it exits with, which
// the stopped-finding mode requires in the daemon's doctor answer.
const (
	talkingStdoutLine = "probe stdout: the service read its configuration"
	talkingStderrLine = "probe stderr: the service cannot start, its port setting is missing"
	talkingExitStatus = 3
)

// talkingFindingCode is the code of the finding of a container stopped after
// a non-zero exit, copied from the container plugin (doctor.go codeStopped):
// the probe links no ze package.
const talkingFindingCode = "doctor-container-stopped"

// commandDoctorJSON is the doctor answer the stopped-finding mode reads.
const commandDoctorJSON = "show doctor | json"

// The spacing of the stopped-finding mode's attempts: each is a whole ze cli
// run.
const doctorStep = time.Second

// talk is the run-talking entrypoint: one line to its standard output, one
// to its standard error, as a service that explains why it cannot start
// does. It answers the status the entrypoint exits with.
func talk() int {
	fmt.Fprintln(os.Stdout, talkingStdoutLine) //nolint:errcheck // the line is what the test reads; a write that failed shows as its absence
	fmt.Fprintln(os.Stderr, talkingStderrLine) //nolint:errcheck // the same
	return talkingExitStatus
}

// stoppedFinding runs on the host beside the daemon: it waits at most wait
// for the daemon's doctor answer to hold the stopped finding of container
// name with both lines the run-talking entrypoint wrote, then prints the
// answer. On a timeout it prints the last answer as a FAIL.
func stoppedFinding(name string, wait time.Duration) bool {
	parts := []string{talkingFindingCode, "container " + name + " ", talkingStdoutLine, talkingStderrLine}
	deadline := time.Now().Add(wait)
	var last string
	for time.Now().Before(deadline) {
		answer, err := zeCLI(commandDoctorJSON)
		if err != nil {
			last = "the doctor command answered an error: " + err.Error()
			time.Sleep(doctorStep)
			continue
		}
		last = string(answer)
		if holdsEvery(last, parts) {
			fmt.Println("probe: the doctor answer holds the stopped finding of " + name)
			fmt.Println(last)
			return true
		}
		time.Sleep(doctorStep)
	}
	fmt.Println("probe: FAIL the doctor answer holds no stopped finding of " + name + " with both lines; last answer: " + oneLine([]byte(last)))
	return false
}

// holdsEvery answers whether text holds every one of parts.
func holdsEvery(text string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
