// VALIDATES: native child execution preserves ordinary exits and shell-compatible
// signal exits without routing through a shell process.
// PREVENTS: verify-lock or session seeding flattening a child's status accidentally.
package job

import (
	"io"
	"os"
	"strconv"
	"syscall"
	"testing"
)

func TestRunProcessPreservesExitAndSignalStatus(t *testing.T) {
	if os.Getenv("LEJOB_PROCESS_HELPER") == "1" {
		processHelper()
		return
	}
	environ := append(os.Environ(), "LEJOB_PROCESS_HELPER=1")
	for _, test := range []struct {
		name string
		mode string
		want int
	}{
		{name: "exit", mode: "37", want: 37},
		{name: "signal", mode: "term", want: 128 + int(syscall.SIGTERM)},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, err := RunProcess(
				[]string{os.Args[0], "-test.run=^TestRunProcessPreservesExitAndSignalStatus$", "--", test.mode},
				ProcessIO{Dir: t.TempDir(), Environ: environ, Stdin: nil, Stdout: io.Discard, Stderr: io.Discard},
			)
			if err != nil || code != test.want {
				t.Fatalf("RunProcess = code %d err %v, want code %d", code, err, test.want)
			}
		})
	}
}

func processHelper() {
	mode := os.Args[len(os.Args)-1]
	if mode == "term" {
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			os.Exit(96)
		}
		os.Exit(95)
	}
	code, err := strconv.Atoi(mode)
	if err != nil {
		os.Exit(97)
	}
	os.Exit(code)
}
