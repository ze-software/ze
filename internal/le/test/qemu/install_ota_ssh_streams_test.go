// Design: docs/architecture/api/ipc_protocol.md -- SSH answer streams.

package testqemu

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestApplianceOTASSHSeparatesAnswerFrame crosses the real SSH channel with
// the document on stdout and the daemon's answer frame on stderr. The JSON
// consumer must receive the document alone, independent of stream ordering.
func TestApplianceOTASSHSeparatesAnswerFrame(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.mu.Lock()
	fixture.host = otaPersistentHost
	fixture.sshStderr = otaSSHDocumentFrame(1, "")
	fixture.mu.Unlock()
	guest := &otaGuest{installer: fixture.installer, sshPort: fixture.sshPort}
	output, err := guest.ssh(context.Background(), HostnameQuery, "")
	if err != nil {
		t.Fatal(err)
	}
	name, err := hostnameOf(output)
	if err != nil {
		t.Fatalf("SSH document includes non-JSON frame bytes: %v; output=%q", err, output)
	}
	if name != otaPersistentHost {
		t.Fatalf("decoded host = %q, want %q", name, otaPersistentHost)
	}
	if err := guest.assertHost(context.Background()); err != nil {
		t.Fatalf("persisted host check rejected framed SSH answer: %v", err)
	}
}

// TestApplianceOTASSHRetainsStderrFailure requires the remote refusal and exit
// status to survive separation of the stdout answer from stderr diagnostics.
func TestApplianceOTASSHRetainsStderrFailure(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.mu.Lock()
	fixture.sshStderr = "\n" + otaSSHDocumentFrame(0, "command refused")
	fixture.mu.Unlock()
	guest := &otaGuest{installer: fixture.installer, sshPort: fixture.sshPort}
	const command = "show nonexistent proof item"
	_, err := guest.ssh(context.Background(), command, "")
	if err == nil {
		t.Fatal("remote failure became successful SSH output")
	}
	if !strings.Contains(err.Error(), "unknown command: "+command) {
		t.Fatalf("remote stderr diagnostic was lost: %v", err)
	}
	var exit *ssh.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("remote SSH exit cause was lost: %v", err)
	}
	if exit.ExitStatus() != 1 {
		t.Fatalf("remote exit = %d, want 1", exit.ExitStatus())
	}
}

// TestApplianceOTAHostDecodeIdentifiesSSHResponse makes an invalid hostname
// document identify the failed proof step without dumping response bytes.
func TestApplianceOTAHostDecodeIdentifiesSSHResponse(t *testing.T) {
	fixture := newOTAProtocolFixture(t, false)
	fixture.mu.Lock()
	fixture.sshStderr = otaSSHDocumentFrame(1, "")
	fixture.mu.Unlock()
	guest := &otaGuest{installer: fixture.installer, sshPort: fixture.sshPort}
	err := guest.assertHost(context.Background())
	if err == nil {
		t.Fatal("hostname document without a hostname passed the proof")
	}
	if !strings.Contains(err.Error(), "appliance SSH hostname response") {
		t.Fatalf("hostname failure lacks SSH response context: %v", err)
	}
}

// otaSSHDocumentFrame uses the same wire producers as the daemon's
// answerFrame.head and answerFrame.terminator, with one newline per frame line.
func otaSSHDocumentFrame(count uint64, message string) string {
	frame := rpc.AppendAnswerHead(nil, rpc.AnswerNoID, rpc.AnswerTypeDocument, "", nil)
	frame = append(frame, '\n')
	frame = rpc.AppendAnswerTerminator(frame, rpc.AnswerNoID, count, 0, message)
	return string(append(frame, '\n'))
}
