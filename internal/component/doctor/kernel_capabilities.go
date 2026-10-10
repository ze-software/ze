// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: doctor.go -- the argument parsing that selects this mode
//
// `ze doctor kernel-capabilities` probes EVERY enrolled kernel capability with
// the configuration ignored (kernelcap.ProbeAll) and answers whether this
// kernel holds every feature Ze can use. It is the answer a Docker host is
// checked by before Ze runs on it: the check runs this mode inside a container
// on the Docker daemon's kernel and reads the JSON.

package doctor

import (
	"encoding/json"
	"io"
	"os"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// kernelCapabilitiesKeyword selects the mode. It is a closed keyword in the
// position a config path otherwise takes (ai/rules/cli.md).
const kernelCapabilitiesKeyword = "kernel-capabilities"

// kernelCapabilitiesAnswer is the JSON answer. Capabilities is never null, so
// a consumer reads an empty list rather than a missing one.
type kernelCapabilitiesAnswer struct {
	Ready        bool            `json:"ready"`
	Capabilities []kernelcap.Row `json:"capabilities"`
}

// runKernelCapabilities probes every enrolment, writes the answer to stdout,
// and returns the exit code: 0 only when the kernel holds every feature.
func runKernelCapabilities(jsonOutput bool) int {
	ready, err := writeKernelCapabilities(os.Stdout, kernelcap.ProbeAll(), jsonOutput)
	if err != nil {
		var msg textbuf.Buffer
		msg.Str("error: write kernel capabilities: ").Err(err).Byte('\n')
		os.Stderr.WriteString(msg.String()) //nolint:errcheck // the exit code carries the failure; a stderr write error has no other channel
		return 1
	}
	if !ready {
		return 1
	}
	return 0
}

// writeKernelCapabilities writes rows to w and reports whether the kernel is
// ready. Ready means at least one capability is enrolled and every one is
// present: an unknown row is not a pass, and an empty enrolment (a build whose
// owners enroll nothing, as off Linux) says nothing about the kernel, so it is
// not a pass either.
func writeKernelCapabilities(w io.Writer, rows []kernelcap.Row, jsonOutput bool) (bool, error) {
	ready := len(rows) > 0
	for i := range rows {
		if rows[i].State != kernelcap.StatePresent.String() {
			ready = false
		}
	}

	if jsonOutput {
		answer := kernelCapabilitiesAnswer{Ready: ready, Capabilities: rows}
		if answer.Capabilities == nil {
			answer.Capabilities = []kernelcap.Row{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return ready, enc.Encode(answer)
	}

	var text textbuf.Buffer
	if len(rows) == 0 {
		text.Str("no kernel capability is enrolled in this build, so nothing about the kernel was asked\n")
	}
	for i := range rows {
		text.Str(rows[i].State).Byte(' ').Str(rows[i].Subsystem).Byte(' ').Str(rows[i].Kernel)
		if rows[i].Reason != "" {
			text.Str(": ").Str(rows[i].Reason)
		}
		text.Byte('\n')
	}
	if ready {
		text.Str("ready\n")
	} else {
		text.Str("not ready\n")
	}
	_, err := io.WriteString(w, text.String())
	return ready, err
}
