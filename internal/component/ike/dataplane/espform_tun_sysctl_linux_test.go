// Design: plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md -- the complete path re-presents on a policy-checked device
// Related: espform_tun_linux.go -- espFormTUNSysctls, readESPFormTUNSysctl and the sysctl refusals under test
//
// VALIDATES: ze-espform's IPv4 sysctls are written only where the device does not
// already hold Ze's value (disable_policy 0, rp_filter 2). Where the write is refused
// (EROFS, a read-only /proc/sys as in a container; EACCES), the value the kernel applies
// decides: disable_policy left at 1 on the device is refused, and rp_filter is refused
// only when max(conf/all, device) is 1 (IN_DEV_MAXCONF). Every refusal is an operator
// error naming the setting, the value the kernel applies, and the --sysctl fix. A setting
// that cannot be read is refused, never assumed.
// PREVENTS: every UDP-4500 Child SA refused in a container whose /proc/sys is read-only
// even though conf/default already gave the device the right values; an SA accepted on
// a device whose policy check is off, or whose strict reverse-path filter drops every
// re-presented datagram; a read failure taken for "the value is fine".
// METHOD: the reads and writes run through an injected espFormTUNSysctlOps, with no
// privilege. The real read is driven over files in t.TempDir.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The four /proc/sys files espFormTUNSysctls reads.
const (
	sysctlDevPolicy = espFormTUNConf4 + "disable_policy"
	sysctlAllPolicy = espFormTUNConf4All + "disable_policy"
	sysctlDevRPF    = espFormTUNConf4 + "rp_filter"
	sysctlAllRPF    = espFormTUNConf4All + "rp_filter"
)

// fakeSysctls answers reads from values and records each write and each writability
// test. A nil writeErr is a writable /proc/sys, where a write lands in values; otherwise
// every write, and every writability test, fails with it.
type fakeSysctls struct {
	values   map[string]int
	readErr  map[string]error
	writeErr error

	writes []string // "path=value", in order
	probes []string // the paths whose writability was tested, in order
}

func (f *fakeSysctls) ops() espFormTUNSysctlOps {
	return espFormTUNSysctlOps{
		read: func(path string) (int, error) {
			if err, ok := f.readErr[path]; ok {
				return 0, err
			}
			value, ok := f.values[path]
			if !ok {
				return 0, fmt.Errorf("fakeSysctls: a read of %s nobody scripted", path)
			}
			return value, nil
		},
		write: func(path, value string) error {
			f.writes = append(f.writes, path+"="+value)
			if f.writeErr != nil {
				// The shape writeESPFormTUNSysctl returns: the open's PathError, wrapped.
				return fmt.Errorf("dataplane: open %s to set it to %s: %w", path, value,
					&os.PathError{Op: "open", Path: path, Err: f.writeErr})
			}
			number, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("fakeSysctls: write %q to %s: %w", value, path, err)
			}
			f.values[path] = number
			return nil
		},
		writable: func(path string) error {
			f.probes = append(f.probes, path)
			if f.writeErr != nil {
				// The shape probeESPFormTUNSysctlWritable returns: the open's PathError, wrapped.
				return fmt.Errorf("dataplane: open %s to learn whether Ze may set it: %w", path,
					&os.PathError{Op: "open", Path: path, Err: f.writeErr})
			}
			return nil
		},
	}
}

// TestESPFormTUNSysctlsWriteOnlyWhatTheKernelNeeds drives each combination of device
// value, conf/all value and writability, and checks the writes Ze attempts, the values a
// writable /proc/sys ends with, and the outcome: success, the refusal for the setting
// that blocks, or the plain error of a write that failed for another reason.
func TestESPFormTUNSysctlsWriteOnlyWhatTheKernelNeeds(t *testing.T) {
	cases := []struct {
		name     string
		devPol   int
		allPol   int
		devRPF   int
		allRPF   int
		readErr  map[string]error
		writeErr error

		wantWrites []string
		wantValues map[string]int // checked when non-nil: the values after the call
		problem    string         // non-empty: an operator error whose problem holds it
		fix        string
		errno      unix.Errno // in the error chain when non-zero
		plainErr   bool       // an error that is not an operator error
	}{
		{
			name:   "read-only, values already right: no write",
			devPol: 0, allPol: 0, devRPF: 2, allRPF: 1,
			writeErr: unix.EROFS,
		},
		{
			name:   "read-only, conf/all disable_policy 1 is the operator's choice",
			devPol: 0, allPol: 1, devRPF: 2, allRPF: 0,
			writeErr: unix.EROFS,
		},
		{
			name:   "read-only, device disable_policy 1 is refused",
			devPol: 1, allPol: 0, devRPF: 2, allRPF: 0,
			writeErr:   unix.EROFS,
			wantWrites: []string{sysctlDevPolicy + "=0"},
			problem:    "interface has disable_policy 1 and conf/all has 0, so the kernel applies 1, and Ze cannot set the interface to 0: /proc/sys is mounted read-only",
			fix:        "docker run --sysctl net.ipv4.conf.default.disable_policy=0",
			errno:      unix.EROFS,
		},
		{
			name:   "read-only, conf/all rp_filter 1 over device 0 is refused",
			devPol: 0, allPol: 0, devRPF: 0, allRPF: 1,
			writeErr:   unix.EROFS,
			wantWrites: []string{sysctlDevRPF + "=2"},
			problem:    "interface has rp_filter 0 and conf/all has 1, so the kernel applies strict reverse-path filtering (1)",
			fix:        "docker run --sysctl net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EROFS,
		},
		{
			name:   "read-only, device rp_filter 1 is refused",
			devPol: 0, allPol: 0, devRPF: 1, allRPF: 0,
			writeErr:   unix.EROFS,
			wantWrites: []string{sysctlDevRPF + "=2"},
			problem:    "interface has rp_filter 1 and conf/all has 0",
			fix:        "sysctl -w net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EROFS,
		},
		{
			name:   "read-only, applied rp_filter 0 passes",
			devPol: 0, allPol: 0, devRPF: 0, allRPF: 0,
			writeErr:   unix.EROFS,
			wantWrites: []string{sysctlDevRPF + "=2"},
		},
		{
			name:   "read-only, conf/all rp_filter 2 over device 1 passes",
			devPol: 0, allPol: 0, devRPF: 1, allRPF: 2,
			writeErr:   unix.EROFS,
			wantWrites: []string{sysctlDevRPF + "=2"},
		},
		{
			name:   "no permission, device rp_filter 1 is refused",
			devPol: 0, allPol: 0, devRPF: 1, allRPF: 1,
			writeErr:   unix.EACCES,
			wantWrites: []string{sysctlDevRPF + "=2"},
			problem:    "Ze has no permission to write " + sysctlDevRPF,
			fix:        "net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EACCES,
		},
		{
			name:   "writable, both written",
			devPol: 1, allPol: 1, devRPF: 1, allRPF: 1,
			wantWrites: []string{sysctlDevPolicy + "=0", sysctlDevRPF + "=2"},
			wantValues: map[string]int{sysctlDevPolicy: 0, sysctlAllPolicy: 1, sysctlDevRPF: 2, sysctlAllRPF: 1},
		},
		{
			name:   "writable, applied rp_filter 0 is still written to 2",
			devPol: 0, allPol: 0, devRPF: 0, allRPF: 0,
			wantWrites: []string{sysctlDevRPF + "=2"},
			wantValues: map[string]int{sysctlDevPolicy: 0, sysctlAllPolicy: 0, sysctlDevRPF: 2, sysctlAllRPF: 0},
		},
		// MUTATION: in settleESPFormTUNSysctls, return the write failure as it is instead
		// of espFormTUNSysctlWriteRefusal: the error is no operator error and names no fix.
		{
			name:   "a write failing for another reason is refused with the conf/default fix",
			devPol: 0, allPol: 0, devRPF: 0, allRPF: 0,
			writeErr:   unix.EIO,
			wantWrites: []string{sysctlDevRPF + "=2"},
			problem:    "Ze cannot set " + sysctlDevRPF + " to 2",
			fix:        "sudo sysctl -w net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EIO,
		},
		{
			name:   "device disable_policy unreadable is refused",
			devPol: 0, allPol: 0, devRPF: 2, allRPF: 0,
			readErr: map[string]error{sysctlDevPolicy: &os.PathError{Op: "open", Path: sysctlDevPolicy, Err: unix.ENOENT}},
			problem: "Ze cannot read " + sysctlDevPolicy,
			fix:     "mount -t proc proc /proc",
			errno:   unix.ENOENT,
		},
		{
			name:   "conf/all rp_filter unreadable is refused",
			devPol: 0, allPol: 0, devRPF: 2, allRPF: 0,
			readErr: map[string]error{sysctlAllRPF: &os.PathError{Op: "open", Path: sysctlAllRPF, Err: unix.EACCES}},
			problem: "Ze cannot read " + sysctlAllRPF,
			fix:     "mount -t proc proc /proc",
			errno:   unix.EACCES,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sysctls := &fakeSysctls{
				values: map[string]int{
					sysctlDevPolicy: tc.devPol, sysctlAllPolicy: tc.allPol,
					sysctlDevRPF: tc.devRPF, sysctlAllRPF: tc.allRPF,
				},
				readErr:  tc.readErr,
				writeErr: tc.writeErr,
			}
			err := espFormTUNSysctls(sysctls.ops())
			requireCalls(t, sysctls.writes, tc.wantWrites...)
			for path, want := range tc.wantValues {
				if got := sysctls.values[path]; got != want {
					t.Errorf("%s = %d after the call, want %d", path, got, want)
				}
			}

			if tc.problem != "" {
				requireTUNRefusal(t, err, tc.problem, tc.fix, tc.errno)
				return
			}
			if tc.plainErr {
				if err == nil {
					t.Fatalf("err = nil, want the %v write failure", tc.errno)
				}
				if _, ok := errors.AsType[*opererr.Error](err); ok {
					t.Errorf("err = %v, want the write failure as it is, not a sysctl refusal: only EROFS and EACCES let the applied value decide", err)
				}
				if !errors.Is(err, tc.errno) {
					t.Errorf("err = %v, want its chain to hold %v", err, tc.errno)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want success", err)
			}
		})
	}
}

// TestESPFormTUNSysctlReadParsesTheValue drives the real read over files: a value with
// the kernel's trailing newline is read as its integer, a file that holds no integer is
// an error naming what it holds, and a missing file is an error carrying ENOENT.
func TestESPFormTUNSysctlReadParsesTheValue(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return path
	}

	value, err := readESPFormTUNSysctl(write("rp_filter", "2\n"))
	if err != nil {
		t.Fatalf("read a file holding \"2\\n\": %v", err)
	}
	if value != 2 {
		t.Errorf("read %d, want 2", value)
	}

	if value, err := readESPFormTUNSysctl(write("junk", "loose\n")); err == nil {
		t.Errorf("a file holding \"loose\" read as %d, want an error", value)
	}

	_, err = readESPFormTUNSysctl(filepath.Join(dir, "absent"))
	if !errors.Is(err, unix.ENOENT) {
		t.Errorf("err = %v, want its chain to hold ENOENT for a missing file", err)
	}
}
