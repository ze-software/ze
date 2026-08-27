// Design: ai/rules/cli.md -- one structured answer for every renderer
// Overview: docker_run.go -- the run that fills this report
// Related: actions.go -- the evidence action table

package evidence

import (
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DockerRunVerdict is the result of the inner evidence script. Zero is invalid,
// so an operating failure cannot serialize as a successful proof.
type DockerRunVerdict uint8

const (
	DockerRunVerdictUnspecified DockerRunVerdict = iota
	DockerRunVerdictPass
	DockerRunVerdictFail
	DockerRunVerdictSignal
)

// String answers the stable word used by the text and JSON renderings.
func (v DockerRunVerdict) String() string {
	switch v {
	case DockerRunVerdictPass:
		return "pass"
	case DockerRunVerdictFail:
		return "fail"
	case DockerRunVerdictSignal:
		return "signal"
	case DockerRunVerdictUnspecified:
		return "unspecified"
	default:
		return "invalid"
	}
}

// MarshalJSON writes the verdict as a word and refuses every invalid value.
func (v DockerRunVerdict) MarshalJSON() ([]byte, error) {
	if v == DockerRunVerdictUnspecified {
		return nil, errors.New("docker run verdict is unspecified")
	}
	if v != DockerRunVerdictPass && v != DockerRunVerdictFail && v != DockerRunVerdictSignal {
		return nil, errors.New("docker run verdict is invalid")
	}
	return json.Marshal(v.String())
}

// DockerRunCommand records one external command in the order it ran.
type DockerRunCommand struct {
	Program     string   `json:"program"`
	Arguments   []string `json:"arguments"`
	Directory   string   `json:"directory,omitempty"`
	Environment []string `json:"environment,omitempty"`
}

// DockerRunPlan is the complete host-side plan and the commands attempted.
type DockerRunPlan struct {
	Tree          string             `json:"tree"`
	Script        string             `json:"script"`
	Packages      []string           `json:"packages"`
	Environment   []string           `json:"environment"`
	Image         string             `json:"image"`
	Platform      string             `json:"platform"`
	Goarch        string             `json:"goarch"`
	ZeBinary      string             `json:"ze-binary"`
	Container     string             `json:"container"`
	BuildTags     string             `json:"build-tags"`
	ModuleMounted bool               `json:"module-mounted"`
	Commands      []DockerRunCommand `json:"commands"`
}

// DockerRunReport is one completed inner-script run.
type DockerRunReport struct {
	Verdict       DockerRunVerdict `json:"verdict"`
	Plan          DockerRunPlan    `json:"plan"`
	InnerExitCode int              `json:"inner-exit-code"`
	Code          int              `json:"code"`
	Signal        int              `json:"signal,omitempty"`
	Cleanup       bool             `json:"cleanup"`
}

// Text renders the same verdict a person reads from the Python producer's exit.
func (r DockerRunReport) Text() string {
	var tb textbuf.Buffer
	switch r.Verdict {
	case DockerRunVerdictPass:
		tb.Str("PASS: evidence script exited 0")
	case DockerRunVerdictFail:
		tb.Str("FAIL: evidence script exited ").Int(int64(r.InnerExitCode))
	case DockerRunVerdictSignal:
		tb.Str("SIGNAL: evidence run received signal ").Int(int64(r.Signal))
	default:
		tb.Str("INVALID: evidence run has no verdict")
	}
	return tb.Byte('\n').String()
}
