// Design: docs/architecture/testing/runner-architecture.md -- mock cwd and native command execution.

package runner

import (
	"bytes"
	"encoding/hex"
	"flag"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var rtrNativeLE = flag.String("rtr-native-le", "", "absolute path to the named le build for the native RTR release proof")

// TestRTRSyncReleaseNativeRunnerEntry starts the registered mock through the
// runner's actual background-command path. A foreground observer writes the
// release by bare name and reads the complete response, including its VRP and
// End of Data. A cwd decision alone cannot satisfy this assertion.
//
// MUTATION: anchor le mock at baseDir instead of Record.WorkDir. The observer's
// release then cannot unblock the cache, so the response read fails.
func TestRTRSyncReleaseNativeRunnerEntry(t *testing.T) {
	const clientMarker = "rtr-release-native-client"
	if len(os.Args) >= 3 {
		if os.Args[len(os.Args)-2] == clientMarker {
			rtrReleaseNativeClient(t, os.Args[len(os.Args)-1])
			return
		}
	}
	if *rtrNativeLE == "" {
		t.Skip("native entry proof requires -rtr-native-le naming the current named le build")
	}
	if !filepath.IsAbs(*rtrNativeLE) {
		t.Fatal("-rtr-native-le must name an absolute path")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, rec := lastHelperRecord(t, "native-rtr-release", "15s", &RunCommand{
		Mode: modeBackground, Seq: 1,
		Exec:  "le mock rtr --port $PORT2 --sync-release rpki-sync.release --vrp 10.0.0.0/8,24,65001",
		Ready: "le mock rtr: listening on",
	})
	r.lePath = *rtrNativeLE
	zero := 0
	rec.ExpectExitCode = &zero
	rec.RunCommands = append(rec.RunCommands, RunCommand{
		Mode: modeForeground, Seq: 2,
		Exec:         "\"" + executable + "\" -test.run=^TestRTRSyncReleaseNativeRunnerEntry$ -- " + clientMarker + " $PORT2",
		ExpectStdout: []string{"native RTR release received complete VRP response"},
	})
	if !r.runTest(t.Context(), rec, &RunOptions{}) {
		t.Fatalf("native mock did not consume its observer's release: %v\nclient: %s\nmock: %s", rec.Error, rec.ClientOutput, rec.PeerOutput)
	}
	if rec.WorkDir == r.baseDir {
		t.Fatal("native proof did not use a private test directory")
	}
	if _, err := os.Stat(filepath.Join(r.baseDir, "rpki-sync.release")); !os.IsNotExist(err) {
		t.Fatalf("the observer wrote its release into the repository anchor: %v", err)
	}
}

// rtrReleaseNativeClient is the observer process of the native runner record.
// It exercises the cache over TCP rather than trusting a readiness line or a
// successful file write as evidence that synchronization completed.
func rtrReleaseNativeClient(t *testing.T, port string) {
	t.Helper()
	if err := os.WriteFile("rpki-sync.release", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dialer := net.Dialer{Timeout: time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte{1, 2, 0, 0, 0, 0, 0, 8}); err != nil {
		t.Fatal(err)
	}
	// The fixed mock inputs yield Cache Response, one IPv4 VRP and End of
	// Data with serial 1, refresh 3600, retry 600 and expiry 7200.
	want, err := hex.DecodeString("01030001000000080104000000000014010818000a0000000000fde901070001000000180000000100000e100000025800001c20")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("cache did not synchronize after its observer released it: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("cache response %X, want %X", got, want)
	}
	if _, err := os.Stdout.WriteString("native RTR release received complete VRP response\n"); err != nil {
		t.Fatal(err)
	}
}
