// Design: docs/architecture/testing/qemu-integration.md -- the host QEMU harness
// Overview: run.go -- the run that produces these reports
package qemu

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RunVerdict is the result of a guest run. Zero is invalid so an unassigned
// verdict cannot look like a successful run.
type RunVerdict uint8

const (
	RunVerdictUnspecified RunVerdict = iota
	RunVerdictPass
	RunVerdictFail
)

// String answers the stable word used by text and JSON output.
func (v RunVerdict) String() string {
	switch v {
	case RunVerdictPass:
		return verdictWordPass
	case RunVerdictFail:
		return verdictWordFail
	case RunVerdictUnspecified:
		return verdictWordUnspecified
	default:
		return "invalid"
	}
}

// MarshalJSON writes the verdict as its word instead of its numeric code.
func (v RunVerdict) MarshalJSON() ([]byte, error) {
	if v == RunVerdictUnspecified {
		return nil, errors.New("qemu run verdict is unspecified")
	}
	if v != RunVerdictPass {
		if v != RunVerdictFail {
			return nil, errors.New("qemu run verdict is invalid")
		}
	}
	return json.Marshal(v.String())
}

// RunPlan is every host decision made before QEMU starts.
type RunPlan struct {
	Tree                  string   `json:"tree"`
	AlpineVersion         string   `json:"alpine-version"`
	AlpineMinor           string   `json:"alpine-minor"`
	AlpineArch            string   `json:"alpine-arch"`
	GoVersion             string   `json:"go-version"`
	QEMUBinary            string   `json:"qemu-binary"`
	ISO                   string   `json:"iso"`
	Kernel                string   `json:"kernel,omitempty"`
	Memory                string   `json:"memory"`
	CPUs                  string   `json:"cpus"`
	SSHPort               int      `json:"ssh-port"`
	BootTimeoutSeconds    int64    `json:"boot-timeout-seconds"`
	CommandTimeoutSeconds int64    `json:"command-timeout-seconds"`
	Command               string   `json:"command,omitempty"`
	Packages              []string `json:"packages,omitempty"`
	KeepAlive             bool     `json:"keep-alive"`
	QEMUArgv              []string `json:"qemu-argv"`
	BootstrapCommand      string   `json:"bootstrap-command"`
	SetupCommand          string   `json:"setup-command"`
}

// RunReport is the structured answer from one VM.
type RunReport struct {
	Verdict       RunVerdict `json:"verdict"`
	Plan          RunPlan    `json:"plan"`
	GuestExitCode int        `json:"guest-exit-code"`
	ProofFailure  string     `json:"proof-failure,omitempty"`
}

// Text renders the result for a person. Pipe operators render the same fields
// directly from the report.
func (r *RunReport) Text() string {
	word := "INVALID"
	if r.Verdict == RunVerdictPass {
		word = "PASS"
	}
	if r.Verdict == RunVerdictFail {
		word = "FAIL"
	}
	var b textbuf.Buffer
	b.Str("QEMU VM: ").Str(word)
	if r.ProofFailure != "" {
		b.Str("\nproof failure: ").Str(r.ProofFailure)
	}
	failedGuest := r.Verdict == RunVerdictFail && r.ProofFailure == ""
	if failedGuest {
		b.Str(" (exit code ").Int(int64(r.GuestExitCode)).Byte(')')
	}
	return b.Byte('\n').String()
}
