// Design: docs/architecture/doctor-and-health-checks.md -- doctor entry-point proof.

//go:build ze_bgp

package bgpconfig_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// referenceDoctorResult reads the same payload the local and daemon commands
// render, without replacing any check or mutating a process-wide output stream.
func referenceDoctorResult(t *testing.T, content string) diagnostic.DoctorResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ze.conf")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	answer := command.AnswerLocal(func(args command.ValidatedArgs) command.LocalAnswer {
		return doctor.Answer(args, nil)
	}, commandtest.Args("config", path))
	if answer.Payload == nil {
		t.Fatalf("doctor returned no readiness payload: %+v", answer)
	}
	data, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var result diagnostic.DoctorResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
