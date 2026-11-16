package fixture

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const nativeOrphanFix = `ze cli -c "send bgp * update text mp ipv4/flow del destination-ipv4 203.0.113.9/32"`

func nativeOrphanReport(fix string) string {
	return "WARN  [doctor-ddos-flowspec-orphan-announcement] recorded victim. then remove the record with ze data rm meta/ddos-flowspec/announcement, which works while ze runs.\n      fix: " + fix + "\n"
}

// TestDDoSFlowspecReportedFix binds the literal argv to one orphan diagnostic,
// rejecting missing, ambiguous and malformed output rather than choosing a fix
// from a different finding. The quoted withdrawal must remain one -c argument.
func TestDDoSFlowspecReportedFix(t *testing.T) {
	for _, tc := range []struct {
		name, report string
	}{
		{"missing", "all checks passed\n"},
		{"wrong diagnostic", "WARN  [doctor-other] unrelated\n      fix: ze request plugin restart ddos-flowspec\n"},
		{"unfixed orphan", "WARN  [doctor-ddos-flowspec-orphan-announcement] victim\nWARN  [doctor-other] other\n      fix: ze request plugin restart ddos-flowspec\n"},
		{"ambiguous diagnostic", nativeOrphanReport(nativeOrphanFix) + nativeOrphanReport(nativeOrphanFix)},
		{"ambiguous fix", nativeOrphanReport(nativeOrphanFix) + "      fix: ze doctor\n"},
		{"malformed quote", nativeOrphanReport(`ze cli -c "send bgp`)},
		{"empty fix", nativeOrphanReport("")},
		{"runtime unchecked", nativeOrphanReport(nativeOrphanFix) + "WARN  [doctor-runtime-not-checked] disconnected\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ddosFlowspecReportedFix(tc.report)
			require.Error(t, err)
		})
	}
	fix, err := ddosFlowspecReportedFix(nativeOrphanReport(nativeOrphanFix) + "WARN  [doctor-other] other\n      fix: ze doctor\n")
	require.NoError(t, err)
	require.Equal(t, nativeOrphanFix, fix.line)
	require.Equal(t, []string{"ze", "cli", "-c", "send bgp * update text mp ipv4/flow del destination-ipv4 203.0.113.9/32"}, fix.argv)
}

// TestDDoSFlowspecNativeClearOrdersCommands records the exact subprocess argv
// and requires doctor, its quoted withdrawal, its named removal, then doctor.
// Each failure must stop before later commands can hide it.
func TestDDoSFlowspecNativeClearOrdersCommands(t *testing.T) {
	for _, failure := range []string{"", "doctor start", "doctor exit", "withdraw", "remove", "uncleared", "disconnected", "empty answer", "refusal only", "unnamed removal", "wrong fix"} {
		t.Run(failure, func(t *testing.T) {
			var calls [][]string
			run := func(_ context.Context, _ []string, _ string, argv ...string) (int, string, string, error) {
				calls = append(calls, slices.Clone(argv))
				switch len(calls) {
				case 1:
					report := nativeOrphanReport(nativeOrphanFix)
					if failure == "doctor start" {
						return -1, "", "", errors.New("start failed")
					}
					if failure == "doctor exit" {
						return 2, report, "", nil
					}
					if failure == "unnamed removal" {
						report = strings.ReplaceAll(report, "ze data rm meta/ddos-flowspec/announcement", "ze data rm other")
					}
					if failure == "wrong fix" {
						report = nativeOrphanReport("ze doctor")
					}
					return 1, report, "", nil
				case 2:
					if failure == "withdraw" {
						return 1, "", "withdraw refused", nil
					}
				case 3:
					if failure == "remove" {
						return 2, "", "remove refused", nil
					}
					return 0, "removed meta/ddos-flowspec/announcement\n1 entries removed from store\n", "", nil
				case 4:
					if failure == "uncleared" {
						return 1, nativeOrphanReport(nativeOrphanFix), "", nil
					}
					if failure == "disconnected" {
						return 1, "WARN  [doctor-runtime-not-checked] offline", "", nil
					}
					if failure == "empty answer" {
						return 0, "", "", nil
					}
					if failure == "refusal only" {
						return 1, "", "connection refused", nil
					}
				}
				return 0, "all checks passed\n", "", nil
			}
			err := ddosFlowspecClearOrphanUsing(t.Context(), nil, run)
			if failure == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			expected := [][]string{{"ze", "doctor"}, {"ze", "cli", "-c", "send bgp * update text mp ipv4/flow del destination-ipv4 203.0.113.9/32"}, {"ze", "data", "rm", "meta/ddos-flowspec/announcement"}, {"ze", "doctor"}}
			count := 4
			switch failure {
			case "doctor start", "doctor exit", "unnamed removal", "wrong fix":
				count = 1
			case "withdraw":
				count = 2
			case "remove":
				count = 3
			}
			require.Equal(t, expected[:count], calls)
		})
	}
}

// TestDDoSFlowspecNativeRestart executes precisely the restart printed in the
// orphan's stdout block. A matching fix on stderr or another block cannot stand
// in for it, and a failed restart cannot become success.
func TestDDoSFlowspecNativeRestart(t *testing.T) {
	for _, failure := range []string{"", "stderr only", "wrong command", "restart"} {
		t.Run(failure, func(t *testing.T) {
			var calls [][]string
			run := func(_ context.Context, _ []string, _ string, argv ...string) (int, string, string, error) {
				calls = append(calls, slices.Clone(argv))
				if len(calls) == 1 {
					report := nativeOrphanReport("ze request plugin restart ddos-flowspec")
					if failure == "stderr only" {
						return 1, "", report, nil
					}
					if failure == "wrong command" {
						report = nativeOrphanReport("ze request plugin restart other")
					}
					return 1, report, "", nil
				}
				if failure == "restart" {
					return 1, "", "restart refused", nil
				}
				return 0, "restarted", "", nil
			}
			err := ddosFlowspecRunRestartUsing(t.Context(), nil, run)
			if failure == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if failure == "stderr only" {
				require.Equal(t, [][]string{{"ze", "doctor"}}, calls)
				return
			}
			if failure == "wrong command" {
				require.Equal(t, [][]string{{"ze", "doctor"}}, calls)
				return
			}
			require.Equal(t, [][]string{{"ze", "doctor"}, {"ze", "request", "plugin", "restart", "ddos-flowspec"}}, calls)
		})
	}
}

// TestDDoSFlowspecNativeRestoreCause requires the restored file before reload
// and successful SSH/state barrier after it. A failed reload must not await.
func TestDDoSFlowspecNativeRestoreCause(t *testing.T) {
	for _, failure := range []string{"", "copy", "reload", "barrier"} {
		t.Run(failure, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if failure != "copy" {
				require.NoError(t, os.WriteFile("running.conf", []byte("original attach"), 0o600))
			}
			require.NoError(t, os.WriteFile("running2.conf", []byte("missing attach"), 0o600))
			var calls []string
			run := func(_ context.Context, _ []string, _ string, argv ...string) (int, string, string, error) {
				body, err := os.ReadFile("running2.conf")
				require.NoError(t, err)
				require.Equal(t, "original attach", string(body))
				require.Equal(t, []string{"ze", "request", "reload"}, argv)
				calls = append(calls, "reload")
				if failure == "reload" {
					return 1, "", "reload refused", nil
				}
				return 0, "", "", nil
			}
			await := func(_ context.Context, args []string) error {
				calls = append(calls, "barrier")
				require.Equal(t, []string{`"peers-established": 1`, "show bgp | json"}, args)
				if failure == "barrier" {
					return errors.New("not established")
				}
				return nil
			}
			err := ddosFlowspecRestoreCauseUsing(t.Context(), nil, run, await)
			if failure == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			switch failure {
			case "copy":
				require.Empty(t, calls)
			case "reload":
				require.Equal(t, []string{"reload"}, calls)
			default:
				require.Equal(t, []string{"reload", "barrier"}, calls)
			}
		})
	}
}

// TestDoctorPluginFatalChild is a real child of the tests below. It is never
// a shell wrapper, and a cancellation case blocks until CommandContext kills it.
func TestDoctorPluginFatalChild(t *testing.T) {
	if os.Getenv("ZE_FIXTURE_FATAL_CHILD") != "yes" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if _, err := os.Stdout.WriteString("fatal-child-stdout\n"); err != nil {
		os.Exit(9)
	}
	if _, err := os.Stderr.WriteString("fatal-child-stderr\n"); err != nil {
		os.Exit(9)
	}
	switch mode {
	case "one":
		os.Exit(1)
	case "zero":
		os.Exit(0)
	case "other":
		os.Exit(17)
	case "signal":
		process, err := os.FindProcess(os.Getpid())
		if err != nil {
			os.Exit(9)
		}
		if err := process.Kill(); err != nil {
			os.Exit(9)
		}
		<-time.NewTimer(time.Hour).C
	case "cancel":
		<-time.NewTimer(time.Hour).C
	default:
		os.Exit(9)
	}
}

// nativeFatalCancelWriter cancels only after a started child wrote its marker.
// Cmd.Wait joins its output-copy worker before the test reads output.
type nativeFatalCancelWriter struct {
	output *strings.Builder
	cancel context.CancelFunc
}

func (writer *nativeFatalCancelWriter) Write(p []byte) (int, error) {
	n, err := writer.output.Write(p)
	if strings.Contains(writer.output.String(), "fatal-child-stdout\n") {
		writer.cancel()
	}
	return n, err
}

// TestDoctorPluginFatalWait proves the exact actual exit, separate streams and
// reaping after cancellation, signal, normal success and other failure exits.
func TestDoctorPluginFatalWait(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	for _, mode := range []string{"one", "zero", "other", "signal", "cancel", "start"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestDoctorPluginFatalChild$", "--", mode)
			command.Env = append(os.Environ(), "ZE_FIXTURE_FATAL_CHILD=yes")
			if mode == "start" {
				command.Path = filepath.Join(t.TempDir(), "absent")
			}
			var stdout, stderr strings.Builder
			var output io.Writer = &stdout
			if mode == "cancel" {
				output = &nativeFatalCancelWriter{output: &stdout, cancel: cancel}
			}
			err := doctorPluginFatalWait(ctx, command, output, &stderr)
			if mode == "one" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if mode != "start" {
				require.NotNil(t, command.ProcessState, "the wait owner must reap every started child")
				if mode != "cancel" {
					require.Equal(t, "fatal-child-stdout\n", stdout.String())
					require.Equal(t, "fatal-child-stderr\n", stderr.String())
				}
			}
		})
	}
}
