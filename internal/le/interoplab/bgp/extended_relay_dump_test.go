package bgp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// relayDumpLab answers the extended relay dump query for the speakers from
// dump or dumpErr, and every other read like recordingLab.
type relayDumpLab struct {
	*recordingLab
	dump    string
	dumpErr error
}

func (r *relayDumpLab) Query(ctx context.Context, peer string, argv []string, environ []interoplab.EnvironmentVariable) (string, error) {
	if (peer == peerSpeaker || peer == peerSpeaker2) && len(argv) != 0 && argv[0] == "sh" {
		r.reads++
		return r.dump, r.dumpErr
	}
	return r.recordingLab.Query(ctx, peer, argv, environ)
}

// runRelayDumpScript runs extendedRelayDumpScript with sh against base, as
// extendedRelayDump runs it in the speaker container.
func runRelayDumpScript(t *testing.T, base string) string {
	t.Helper()
	output, err := exec.CommandContext(t.Context(), "sh", "-c", extendedRelayDumpScript, "sh", base).CombinedOutput()
	if err != nil {
		t.Fatalf("dump script failed: %v\n%s", err, output)
	}
	return string(output)
}

// TestExtendedRelayDumpScriptReportsMissingFiles proves the dump never skips
// an absent file in silence. Method: run the script on a prefix with no
// files, then with only the verdict file, then with a capture row longer
// than the cut; each absent file must be named missing, and the long row
// must be cut to 512 characters.
func TestExtendedRelayDumpScriptReportsMissingFiles(t *testing.T) {
	base := filepath.Join(t.TempDir(), "extended")
	if got := runRelayDumpScript(t, base); !strings.Contains(got, "no extended relay ran under this prefix") {
		t.Fatalf("a prefix with no files was not reported as such:\n%s", got)
	}

	verdict := `{"transport-error":"LOCAL_PREF appeared on the actual external destination wire"}`
	if err := os.WriteFile(base+"-result.json", []byte(verdict+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := runRelayDumpScript(t, base)
	if !strings.Contains(got, verdict) {
		t.Fatalf("the verdict file is absent from the dump:\n%s", got)
	}
	for _, suffix := range []string{"ze.jsonl", "frr.jsonl"} {
		section := "== " + base + "-" + suffix + " (last 8 rows, each cut to 512 characters)\nmissing\n"
		if !strings.Contains(got, section) {
			t.Fatalf("absent %s was not reported missing:\n%s", suffix, got)
		}
	}

	row := strings.Repeat("a", 2000)
	if err := os.WriteFile(base+"-ze.jsonl", []byte(row+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got = runRelayDumpScript(t, base)
	if !strings.Contains(got, "\n"+strings.Repeat("a", 512)+"\n") || strings.Contains(got, strings.Repeat("a", 513)) {
		t.Fatalf("the capture row was not cut to 512 characters:\n%s", got)
	}
	if !strings.Contains(got, base+"-frr.jsonl (last 8 rows, each cut to 512 characters)\nmissing\n") {
		t.Fatalf("absent frr.jsonl was not reported missing beside a present capture:\n%s", got)
	}
}

// TestCheckerFailureDumpsSpeakerRelayFiles proves a relay's verdict reaches
// the failure even when the speaker logged nothing. Method: every peer log is
// empty, the speaker dump query answers the relay's transport error; the
// failure must carry the named section and the error. A failed dump query
// must be printed, not skipped.
func TestCheckerFailureDumpsSpeakerRelayFiles(t *testing.T) {
	cause := errors.New("LOCAL_PREF wire interval is incomplete")
	verdict := `{"transport-error":"LOCAL_PREF appeared on the actual external destination wire"}`
	err := checkerFailure(t.Context(), &relayDumpLab{recordingLab: &recordingLab{}, dump: verdict}, "bgp-local-pref-strip-gobgp", 0, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("failure lost its cause: %v", err)
	}
	if !strings.Contains(err.Error(), "--- "+peerSpeaker+" extended relay files ("+extendedCaptureBase+"-*) ---\n"+verdict) {
		t.Fatalf("failure omitted the speaker relay verdict:\n%v", err)
	}

	err = checkerFailure(t.Context(), &relayDumpLab{recordingLab: &recordingLab{}, dumpErr: errors.New("container is gone")}, "bgp-local-pref-strip-gobgp", 0, cause)
	if !strings.Contains(err.Error(), peerSpeaker+" extended relay files") || !strings.Contains(err.Error(), "query error: container is gone") {
		t.Fatalf("failed relay dump query was not reported:\n%v", err)
	}
}

// TestExtendedRelayDumpBoundsItsOutput proves one speaker's section cannot
// grow past extendedRelayDumpOctetsMax. Method: answer twice the bound and
// require the tail kept, the cut named, and the head dropped.
func TestExtendedRelayDumpBoundsItsOutput(t *testing.T) {
	answer := strings.Repeat("h", extendedRelayDumpOctetsMax) + strings.Repeat("t", extendedRelayDumpOctetsMax)
	got := extendedRelayDump(t.Context(), &relayDumpLab{recordingLab: &recordingLab{}, dump: answer}, peerSpeaker)
	if strings.Contains(got, "h") {
		t.Fatalf("the dump kept octets past its bound (%d octets)", len(got))
	}
	if !strings.Contains(got, "[first 8192 octets cut]\n"+strings.Repeat("t", extendedRelayDumpOctetsMax)) {
		t.Fatalf("the dump did not name its cut or lost the tail (%d octets)", len(got))
	}
}
