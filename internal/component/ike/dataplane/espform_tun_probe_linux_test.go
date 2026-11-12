// Design: plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md -- the complete path re-presents on a policy-checked device
// Related: espform_tun_probe_linux.go -- predictESPFormTUNSysctls, the sysctl half of the probe under test
// Related: espform_tun_linux.go -- settleESPFormTUNSysctls and probeESPFormTUNSysctlWritable
//
// VALIDATES: the ze doctor probe predicts the sysctl refusal a dual-form Child SA would
// meet, before any interface exists. It reads net.ipv4.conf.default, which a new
// ze-espform copies, and conf/all; it tests whether each write the receiver would make
// is refused, and writes nothing; and it applies the receiver's own rule, so it answers
// exactly what espFormTUNSysctls answers for a device holding the same values. Each
// refusal speaks of the interface Ze would create, names the setting and the value the
// kernel would apply, and gives the --sysctl fix.
// PREVENTS: a doctor that reports the TUN device available while every dual-form SA is
// refused because /proc/sys is read-only and conf/default holds disable_policy 1 or a
// strict rp_filter; a probe that writes a sysctl; a prediction that drifts from the
// install because it copies the rule.
// METHOD: the reads and the writability tests run through the injected
// espFormTUNSysctlOps (fakeSysctls), with no privilege, and a read of any path the probe
// must not read fails the case. The real writability test runs over files in t.TempDir.

//go:build linux

package dataplane

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
)

// The two conf/default files the probe reads where the receiver reads ze-espform's own:
// a new ze-espform copies conf/default. conf/all is sysctlAllPolicy and sysctlAllRPF.
const (
	sysctlDefaultPolicy = espFormTUNConf4Default + "disable_policy"
	sysctlDefaultRPF    = espFormTUNConf4Default + "rp_filter"
)

// TestESPFormTUNProbePredictsTheSysctlRefusal drives the probe's sysctl half over each
// writability and each conf/default and conf/all combination that decides it, and
// checks the outcome (available or the operator refusal for the failed setting), the
// files whose writability it tested, and that it wrote nothing.
func TestESPFormTUNProbePredictsTheSysctlRefusal(t *testing.T) {
	cases := []struct {
		name     string
		defPol   int
		allPol   int
		defRPF   int
		allRPF   int
		readErr  map[string]error
		writeErr error

		wantProbes []string
		problem    string // non-empty: an operator error whose problem holds it
		why        string
		fix        string
		errno      unix.Errno // in the error chain when non-zero
	}{
		{
			name:   "writable, every value wrong: available",
			defPol: 1, allPol: 1, defRPF: 1, allRPF: 1,
			wantProbes: []string{sysctlDefaultPolicy, sysctlDefaultRPF},
		},
		{
			name:   "read-only, default values already right: available",
			defPol: 0, allPol: 1, defRPF: 2, allRPF: 1,
			writeErr: unix.EROFS,
		},
		{
			name:   "read-only, applied rp_filter 0: available",
			defPol: 0, allPol: 0, defRPF: 0, allRPF: 0,
			writeErr:   unix.EROFS,
			wantProbes: []string{sysctlDefaultRPF},
		},
		{
			name:   "read-only, default disable_policy 1 is refused",
			defPol: 1, allPol: 0, defRPF: 2, allRPF: 0,
			writeErr:   unix.EROFS,
			wantProbes: []string{sysctlDefaultPolicy},
			problem: "the ze-espform interface Ze would create takes disable_policy 1 from conf/default, and conf/all has 0, " +
				"so the kernel would apply 1, and Ze could not set the interface to 0: /proc/sys is mounted read-only",
			why:   "so it would refuse every such SA",
			fix:   "docker run --sysctl net.ipv4.conf.default.disable_policy=0",
			errno: unix.EROFS,
		},
		{
			name:   "read-only, conf/all rp_filter 1 over default 0 is refused",
			defPol: 0, allPol: 0, defRPF: 0, allRPF: 1,
			writeErr:   unix.EROFS,
			wantProbes: []string{sysctlDefaultRPF},
			problem: "the ze-espform interface Ze would create takes rp_filter 0 from conf/default, and conf/all has 1, " +
				"so the kernel would apply strict reverse-path filtering (1), and Ze could not set the interface to 2",
			why:   "so it would refuse every such SA",
			fix:   "docker run --sysctl net.ipv4.conf.default.rp_filter=2",
			errno: unix.EROFS,
		},
		{
			name:   "no permission, default rp_filter 1 is refused",
			defPol: 0, allPol: 0, defRPF: 1, allRPF: 0,
			writeErr:   unix.EACCES,
			wantProbes: []string{sysctlDefaultRPF},
			problem:    "Ze could not set the interface to 2: Ze has no permission to write " + sysctlDefaultRPF,
			why:        "so it would refuse every such SA",
			fix:        "sysctl -w net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EACCES,
		},
		{
			name:   "a writability test failing for another reason names its fix",
			defPol: 0, allPol: 0, defRPF: 0, allRPF: 0,
			writeErr:   unix.EIO,
			wantProbes: []string{sysctlDefaultRPF},
			problem:    "Ze cannot set " + sysctlDefaultRPF + " to 2",
			why:        "Ze then could not receive bare ESP",
			fix:        "sudo sysctl -w net.ipv4.conf.default.rp_filter=2",
			errno:      unix.EIO,
		},
		{
			name:   "default disable_policy unreadable is refused",
			defPol: 0, allPol: 0, defRPF: 2, allRPF: 0,
			readErr: map[string]error{sysctlDefaultPolicy: &os.PathError{Op: "open", Path: sysctlDefaultPolicy, Err: unix.ENOENT}},
			problem: "Ze cannot read " + sysctlDefaultPolicy,
			why:     "Without the value it cannot tell whether such an SA would be refused",
			fix:     "mount -t proc proc /proc",
			errno:   unix.ENOENT,
		},
		{
			name:   "conf/all rp_filter unreadable is refused",
			defPol: 0, allPol: 0, defRPF: 2, allRPF: 0,
			readErr: map[string]error{sysctlAllRPF: &os.PathError{Op: "open", Path: sysctlAllRPF, Err: unix.EACCES}},
			problem: "Ze cannot read " + sysctlAllRPF,
			why:     "Without the value it cannot tell whether such an SA would be refused",
			fix:     "mount -t proc proc /proc",
			errno:   unix.EACCES,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Only conf/default and conf/all are scripted: a read of ze-espform's own
			// directory, which does not exist before the receiver creates it, fails.
			sysctls := &fakeSysctls{
				values: map[string]int{
					sysctlDefaultPolicy: tc.defPol, sysctlAllPolicy: tc.allPol,
					sysctlDefaultRPF: tc.defRPF, sysctlAllRPF: tc.allRPF,
				},
				readErr:  tc.readErr,
				writeErr: tc.writeErr,
			}
			err := predictESPFormTUNSysctls(sysctls.ops())
			if len(sysctls.writes) != 0 {
				t.Errorf("the probe wrote %q, want no write at all", sysctls.writes)
			}
			if strings.Join(sysctls.probes, ", ") != strings.Join(tc.wantProbes, ", ") {
				t.Errorf("writability tested for %q, want %q", sysctls.probes, tc.wantProbes)
			}

			if tc.problem != "" {
				requireTUNRefusal(t, err, tc.problem, tc.fix, tc.errno)
				refusal, _ := errors.AsType[*opererr.Error](err)
				if refusal != nil && !strings.Contains(refusal.Why(), tc.why) {
					t.Errorf("why = %q, want it to contain %q", refusal.Why(), tc.why)
				}
				if tc.errno == unix.EIO && refusal != nil && refusal.Fix() != tc.fix {
					t.Errorf("the I/O refusal's fix = %q, want %q", refusal.Fix(), tc.fix)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want the interface available", err)
			}
		})
	}
}

// TestESPFormTUNProbeAgreesWithTheReceiver runs the receiver (espFormTUNSysctls) over a
// device holding each combination of values and writability, and the probe
// (predictESPFormTUNSysctls) over a conf/default holding the same values. Goal: the two
// give the same verdict, the same fix and the same errno, and the probe tests exactly
// the files the receiver writes, in the same order, mapped to conf/default. A rule
// copied into the probe, rather than shared, drifts from the receiver's here.
func TestESPFormTUNProbeAgreesWithTheReceiver(t *testing.T) {
	values := []int{0, 1, 2}
	writeErrs := []error{nil, unix.EROFS, unix.EACCES, unix.EIO}
	for _, pol := range []int{0, 1} {
		for _, allPol := range []int{0, 1} {
			for _, rpf := range values {
				for _, allRPF := range values {
					for _, writeErr := range writeErrs {
						receiver := &fakeSysctls{
							values: map[string]int{
								sysctlDevPolicy: pol, sysctlAllPolicy: allPol,
								sysctlDevRPF: rpf, sysctlAllRPF: allRPF,
							},
							writeErr: writeErr,
						}
						probe := &fakeSysctls{
							values: map[string]int{
								sysctlDefaultPolicy: pol, sysctlAllPolicy: allPol,
								sysctlDefaultRPF: rpf, sysctlAllRPF: allRPF,
							},
							writeErr: writeErr,
						}
						installErr := espFormTUNSysctls(receiver.ops())
						predictErr := predictESPFormTUNSysctls(probe.ops())

						name := "disable_policy " + itoa(pol) + "/" + itoa(allPol) +
							", rp_filter " + itoa(rpf) + "/" + itoa(allRPF) + ", write " + errName(writeErr)
						requireSameVerdict(t, name, installErr, predictErr, writeErr)

						var wantProbes []string
						for _, write := range receiver.writes {
							path, _, _ := strings.Cut(write, "=")
							wantProbes = append(wantProbes, espFormTUNConf4Default+strings.TrimPrefix(path, espFormTUNConf4))
						}
						if strings.Join(probe.probes, ", ") != strings.Join(wantProbes, ", ") {
							t.Errorf("%s: the probe tested %q, want the receiver's writes %q mapped to conf/default",
								name, probe.probes, wantProbes)
						}
						if len(probe.writes) != 0 {
							t.Errorf("%s: the probe wrote %q, want no write", name, probe.writes)
						}
						if len(receiver.probes) != 0 {
							t.Errorf("%s: the receiver only tested %q, want it to write", name, receiver.probes)
						}
					}
				}
			}
		}
	}
}

// requireSameVerdict checks the install and the prediction agree: both nil, or both an
// operator error with the same fix, or both a plain error, each holding writeErr.
func requireSameVerdict(t *testing.T, name string, installErr, predictErr, writeErr error) {
	t.Helper()
	if installErr == nil {
		if predictErr != nil {
			t.Errorf("%s: the install accepts the device, the probe answers %v", name, predictErr)
		}
		return
	}
	if predictErr == nil {
		t.Errorf("%s: the install refuses with %v, the probe answers available", name, installErr)
		return
	}
	installRefusal, installIsRefusal := errors.AsType[*opererr.Error](installErr)
	predictRefusal, predictIsRefusal := errors.AsType[*opererr.Error](predictErr)
	if installIsRefusal != predictIsRefusal {
		t.Errorf("%s: install %v and probe %v differ in kind", name, installErr, predictErr)
		return
	}
	if installIsRefusal && installRefusal.Fix() != predictRefusal.Fix() {
		t.Errorf("%s: install fix %q, probe fix %q", name, installRefusal.Fix(), predictRefusal.Fix())
	}
	// Every value is scripted, so only a refused write can make either side refuse.
	if writeErr == nil {
		t.Errorf("%s: both refuse a writable /proc/sys: install %v, probe %v", name, installErr, predictErr)
		return
	}
	if !errors.Is(installErr, writeErr) {
		t.Errorf("%s: the install's %v does not hold %v", name, installErr, writeErr)
	}
	if !errors.Is(predictErr, writeErr) {
		t.Errorf("%s: the probe's %v does not hold %v", name, predictErr, writeErr)
	}
}

// itoa writes a sysctl value into a case name.
func itoa(v int) string {
	return strconv.Itoa(v)
}

func errName(err error) string {
	if err == nil {
		return "allowed"
	}
	return err.Error()
}

// TestESPFormTUNSharedSysctlRule reads the source and checks that the receiver and the
// probe each reach the sysctl decision through settleESPFormTUNSysctls, and that it is
// the one caller of settleESPFormTUNUnwritable. Goal: the rule exists once; a copy in
// either caller fails here even before its verdicts drift.
func TestESPFormTUNSharedSysctlRule(t *testing.T) {
	calls := map[string]map[string]bool{} // function name -> names it calls
	fset := token.NewFileSet()
	for _, file := range []string{"espform_tun_linux.go", "espform_tun_probe_linux.go"} {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			called := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok {
					called[ident.Name] = true
				}
				return true
			})
			calls[fn.Name.Name] = called
		}
	}
	for _, caller := range []string{"espFormTUNSysctls", "predictESPFormTUNSysctls"} {
		if !calls[caller]["settleESPFormTUNSysctls"] {
			t.Errorf("%s does not call settleESPFormTUNSysctls: the receiver and the probe must share one rule", caller)
		}
	}
	if !calls["ESPFormTUNAvailable"]["predictESPFormTUNSysctls"] {
		t.Errorf("ESPFormTUNAvailable does not call predictESPFormTUNSysctls: the doctor probe would miss the sysctl refusal")
	}
	for name, called := range calls {
		if called["settleESPFormTUNUnwritable"] && name != "settleESPFormTUNSysctls" {
			t.Errorf("%s calls settleESPFormTUNUnwritable: only settleESPFormTUNSysctls may apply the rule", name)
		}
	}
	if !calls["settleESPFormTUNSysctls"]["settleESPFormTUNUnwritable"] {
		t.Errorf("settleESPFormTUNSysctls does not call settleESPFormTUNUnwritable")
	}
}

// TestESPFormTUNSysctlWritableWritesNothing drives the real writability test over files:
// a file the caller may write answers nil and keeps its content byte for byte, a file it
// may not write answers EACCES, and a missing file answers ENOENT and is not created.
func TestESPFormTUNSysctlWritableWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disable_policy")
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	if err := probeESPFormTUNSysctlWritable(path); err != nil {
		t.Fatalf("a writable file answered %v, want nil", err)
	}
	content, err := os.ReadFile(path) //nolint:gosec // the test's own file in t.TempDir
	if err != nil {
		t.Fatalf("read %s back: %v", path, err)
	}
	if string(content) != "1\n" {
		t.Errorf("content after the test = %q, want \"1\\n\": the probe changed the file", content)
	}

	absent := filepath.Join(dir, "absent")
	if err := probeESPFormTUNSysctlWritable(absent); !errors.Is(err, unix.ENOENT) {
		t.Errorf("a missing file answered %v, want ENOENT", err)
	}
	if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lstat %s = %v, want it absent: the probe created the file", absent, err)
	}

	t.Run("a file the mode refuses", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root holds CAP_DAC_OVERRIDE, so a file mode refuses it nothing; run as a user that is not root")
		}
		readOnly := filepath.Join(dir, "rp_filter")
		if err := os.WriteFile(readOnly, []byte("2\n"), 0o400); err != nil {
			t.Fatalf("write %s: %v", readOnly, err)
		}
		if err := probeESPFormTUNSysctlWritable(readOnly); !errors.Is(err, unix.EACCES) {
			t.Errorf("a file of mode 0400 answered %v, want EACCES", err)
		}
	})
}
