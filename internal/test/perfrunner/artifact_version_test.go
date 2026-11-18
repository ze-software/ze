// Design: docs/guide/benchmarking.md -- measured Ze binary identity.
package perfrunner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/perf"
)

// TestZePassVersionsNameTheMeasuredBinary reads the running image's binary
// identity through Docker, even when the reusable image and staging file differ.
func TestZePassVersionsNameTheMeasuredBinary(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("measured image executable")))
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reuse-image=%t", reuse), func(t *testing.T) {
			runner, recorder := binaryVersionFixture(t, digest+"  /usr/local/bin/ze\n", 0)
			runner.NoBuild = reuse
			// The staging path is deliberately a different artifact. It cannot
			// establish what a reused image actually executes.
			if err := os.WriteFile(filepath.Join(runner.interopDir, "ze-linux"), []byte("different staged executable"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, pass := range []perf.Pass{perf.PassLiveForwarding, perf.PassRIBPropagation} {
				if _, ok := runner.runPerf(t.Context(), DUTs()[0], pass); !ok {
					t.Fatalf("%s measurement failed with a valid running-image identity", pass)
				}
			}
			calls := versionSenderCalls(recorder.commands)
			if len(calls) != 2 {
				t.Fatalf("sender calls = %d, want both Ze passes", len(calls))
			}
			seen := make(map[perf.Pass]bool)
			for _, argv := range calls {
				pass := perf.PassFor(slices.Contains(argv, "-sender-first"))
				seen[pass] = true
				version := versionArgument(argv, "--dut-version")
				if want := "sha256:" + digest; version != want {
					t.Errorf("%s DUT version = %q, want measured image identity %q", pass, version, want)
				}
			}
			for _, pass := range []perf.Pass{perf.PassLiveForwarding, perf.PassRIBPropagation} {
				if !seen[pass] {
					t.Errorf("no sender call for %s", pass)
				}
			}
			queries := 0
			for _, argv := range recorder.commands {
				at := slices.Index(argv, "sha256sum")
				if at < 0 {
					continue
				}
				queries++
				if at == 0 || argv[at-1] != runner.containerName(dutZE) || !slices.Equal(argv[at:], []string{"sha256sum", "/usr/local/bin/ze"}) {
					t.Errorf("binary identity read targets something other than the running Ze DUT: %q", argv)
				}
			}
			if queries == 0 {
				t.Error("no running-image binary identity was read")
			}
		})
	}
}

// TestZePassRefusesAnUnknownBinaryIdentity prevents a failed or malformed read
// from publishing an empty, guessed or foreign binary version as a measurement.
func TestZePassRefusesAnUnknownBinaryIdentity(t *testing.T) {
	digest := strings.Repeat("a", 64)
	cases := []struct {
		name   string
		output string
		status int
	}{
		{"read failure", digest + "  /usr/local/bin/ze\n", 1},
		{"empty answer", "", 0},
		{"short digest", digest[:63] + "  /usr/local/bin/ze\n", 0},
		{"nonhex digest", strings.Repeat("g", 64) + "  /usr/local/bin/ze\n", 0},
		{"foreign file", digest + "  /usr/local/bin/le\n", 0},
		{"duplicate answer", strings.Repeat(digest+"  /usr/local/bin/ze\n", 2), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, recorder := binaryVersionFixture(t, tc.output, tc.status)
			if _, ok := runner.runPerf(t.Context(), DUTs()[0], perf.PassLiveForwarding); ok {
				t.Fatal("Ze measurement accepted an unavailable or malformed binary identity")
			}
			if calls := versionSenderCalls(recorder.commands); len(calls) != 0 {
				t.Errorf("unknown Ze identity started a measurement: %q", calls)
			}
		})
	}
}

// TestOtherDUTPassesDoNotAcquireAZeBinaryVersion preserves the existing BIRD
// and FRR version behavior; neither is Ze's executable or uses its image path.
func TestOtherDUTPassesDoNotAcquireAZeBinaryVersion(t *testing.T) {
	for _, name := range []string{dutBIRD, dutFRR} {
		t.Run(name, func(t *testing.T) {
			runner, recorder := binaryVersionFixture(t, "", 1)
			var selected DUT
			for _, dut := range DUTs() {
				if dut.Name == name {
					selected = dut
				}
			}
			if selected.Name == "" {
				t.Fatalf("DUT declaration contains no %s", name)
			}
			if _, ok := runner.runPerf(t.Context(), selected, perf.PassLiveForwarding); !ok {
				t.Fatal("non-Ze measurement failed because Ze's binary identity was unavailable")
			}
			calls := versionSenderCalls(recorder.commands)
			if len(calls) != 1 {
				t.Fatalf("sender calls = %d, want one non-Ze measurement", len(calls))
			}
			if version := versionArgument(calls[0], "--dut-version"); version != "" {
				t.Errorf("non-Ze DUT acquired Ze's version: %q", version)
			}
			for _, argv := range recorder.commands {
				if slices.Contains(argv, "sha256sum") {
					t.Errorf("non-Ze measurement queried Ze's image binary: %q", argv)
				}
			}
		})
	}
}

func binaryVersionFixture(t *testing.T, output string, status int) (*Runner, *dockerRecorder) {
	t.Helper()
	runner, recorder := fixtureRunner(t, io.Discard)
	recorder.checksumAnswer = func() (interoplab.ProcessResult, error) {
		return interoplab.ProcessResult{Stdout: output, Stderr: "controlled image identity read", ExitCode: status}, nil
	}
	return runner, recorder
}

func versionSenderCalls(commands [][]string) [][]string {
	var calls [][]string
	for _, argv := range commands {
		at := slices.Index(argv, containerLe)
		if at >= 0 && len(argv) > at+2 && slices.Equal(argv[at+1:at+3], []string{"perf", "send"}) {
			calls = append(calls, argv)
		}
	}
	return calls
}

func versionArgument(argv []string, name string) string {
	if at := slices.Index(argv, name); at >= 0 && at+1 < len(argv) {
		return argv[at+1]
	}
	return ""
}
