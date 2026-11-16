// Design: docs/architecture/container/runtime.md -- the system-call filter

//go:build linux

package seccomp

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	libseccomp "github.com/opencontainers/runc/libcontainer/seccomp"
	"golang.org/x/sys/unix"
)

// sourceCapabilities are the capabilities the source profile's rules name,
// so a bounding set of all of them selects every capability rule.
func sourceCapabilities() []string {
	var capabilities []string
	for _, rule := range sourceRules {
		for _, capability := range []string{rule.capability, rule.withoutCapability} {
			if capability == "" {
				continue
			}
			if !slices.Contains(capabilities, capability) {
				capabilities = append(capabilities, capability)
			}
		}
	}
	return capabilities
}

// boundingSets are the sets the profile tests select under: none, a
// thirteen-capability default without CAP_SYS_ADMIN, and every capability a
// rule names.
func boundingSets() map[string][]string {
	return map[string][]string{
		"no capability": nil,
		"the default thirteen": {
			"CAP_AUDIT_WRITE", "CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_FSETID", "CAP_KILL",
			"CAP_NET_BIND_SERVICE", "CAP_NET_RAW", "CAP_SETFCAP", "CAP_SETGID", "CAP_SETPCAP", "CAP_SETUID", "CAP_SYS_CHROOT",
		},
		"every capability a rule names": sourceCapabilities(),
	}
}

// TestDefaultProfileNamesOnlyKnownCalls proves every name of the default
// profile, for each architecture and bounding set, is a call at least one
// table holds: the profile names nothing the filter would silently skip on
// both architectures.
func TestDefaultProfileNamesOnlyKnownCalls(t *testing.T) {
	if len(sourceCapabilities()) == 0 {
		t.Fatal("no source rule names a capability, so the capability sets below select nothing")
	}
	for _, table := range tables {
		for setName, bounding := range boundingSets() {
			profile := defaultProfileFor(table.goarch, bounding)
			if len(profile.Syscalls) == 0 {
				t.Fatalf("the %s profile under %s names no call", table.goarch, setName)
			}
			for _, call := range profile.Syscalls {
				if !knownToATable(call.Name) {
					t.Errorf("the %s profile under %s names %q, which neither table holds", table.goarch, setName, call.Name)
				}
			}
		}
	}
}

// knownToATable tells whether at least one table holds name.
func knownToATable(name string) bool {
	for _, table := range tables {
		if _, found := table.numbers[name]; found {
			return true
		}
	}
	return false
}

// mobyProfile is the part of the source profile's JSON the comparison reads.
type mobyProfile struct {
	DefaultAction   string `json:"defaultAction"`
	DefaultErrnoRet uint   `json:"defaultErrnoRet"`
	Syscalls        []struct {
		Names    []string `json:"names"`
		Action   string   `json:"action"`
		ErrnoRet *uint    `json:"errnoRet"`
		Args     []struct {
			Index    uint   `json:"index"`
			Value    uint64 `json:"value"`
			ValueTwo uint64 `json:"valueTwo"`
			Op       string `json:"op"`
		} `json:"args"`
		Includes mobyFilter `json:"includes"`
		Excludes mobyFilter `json:"excludes"`
	} `json:"syscalls"`
}

// mobyFilter is a rule's includes or excludes.
type mobyFilter struct {
	Caps      []string `json:"caps"`
	Arches    []string `json:"arches"`
	MinKernel string   `json:"minKernel"`
}

// mobySelected tells whether the source selects a rule on goarch for
// bounding, as Moby's setupSeccomp does: an excluded architecture or any
// excluded capability drops it, and an included architecture list or
// capability list must hold. The kernel version is not read: Ze applies the
// 4.8 rule always.
func mobySelected(includes, excludes mobyFilter, goarch string, bounding []string) bool {
	if slices.Contains(excludes.Arches, goarch) {
		return false
	}
	for _, capability := range excludes.Caps {
		if slices.Contains(bounding, capability) {
			return false
		}
	}
	if len(includes.Arches) > 0 {
		if !slices.Contains(includes.Arches, goarch) {
			return false
		}
	}
	for _, capability := range includes.Caps {
		if !slices.Contains(bounding, capability) {
			return false
		}
	}
	return true
}

// TestDefaultProfileFollowsTheSourceProfile proves profile.go is the source
// profile: the JSON in third_party/moby-seccomp-profile, selected as the
// source selects it and with the names no table holds left out, gives the
// same profile for each architecture and bounding set.
//
// MUTATION: drop a name from the allow rule, or move a rule; the comparison
// fails.
func TestDefaultProfileFollowsTheSourceProfile(t *testing.T) {
	source, err := os.ReadFile("../../../../third_party/moby-seccomp-profile/default.json")
	if err != nil {
		t.Fatalf("reading the source profile: %v", err)
	}
	var moby mobyProfile
	if err := json.Unmarshal(source, &moby); err != nil {
		t.Fatalf("decoding the source profile: %v", err)
	}
	if moby.DefaultAction != "SCMP_ACT_ERRNO" {
		t.Fatalf("the source default is %s, and the profile's is Errno", moby.DefaultAction)
	}
	if moby.DefaultErrnoRet != uint(unix.EPERM) {
		t.Fatalf("the source default error number is %d, and the profile's is EPERM", moby.DefaultErrnoRet)
	}
	for _, table := range tables {
		for setName, bounding := range boundingSets() {
			want := &configs.Seccomp{DefaultAction: configs.Errno, DefaultErrnoRet: errnoOf(uint(unix.EPERM))}
			for _, rule := range moby.Syscalls {
				if !mobySelected(rule.Includes, rule.Excludes, table.goarch, bounding) {
					continue
				}
				action, err := libseccomp.ConvertStringToAction(rule.Action)
				if err != nil {
					t.Fatal(err)
				}
				var arguments []*configs.Arg
				for _, argument := range rule.Args {
					op, err := libseccomp.ConvertStringToOperator(argument.Op)
					if err != nil {
						t.Fatal(err)
					}
					arguments = append(arguments, &configs.Arg{Index: argument.Index, Value: argument.Value, ValueTwo: argument.ValueTwo, Op: op})
				}
				for _, name := range rule.Names {
					if !knownToATable(name) {
						continue
					}
					want.Syscalls = append(want.Syscalls, &configs.Syscall{Name: name, Action: action, ErrnoRet: rule.ErrnoRet, Args: arguments})
				}
			}
			got := defaultProfileFor(table.goarch, bounding)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("on %s under %s the profile differs from the source: %d rules, the source selects %d", table.goarch, setName, len(got.Syscalls), len(want.Syscalls))
			}
		}
	}
}

// TestDefaultProfileConfinesWithoutTheAdminCapability pins what the guest
// test and the closing test rely on, on both tables: without CAP_SYS_ADMIN,
// unshare and a clone into a new namespace answer EPERM and clone3 answers
// ENOSYS (a C library then falls back to clone); with it, unshare is
// allowed. Calls a root entrypoint needs to switch user (setuid, setgid,
// chown) are allowed. Number 470 lies above the largest the profile names,
// so it answers "not implemented" whatever the host kernel knows.
func TestDefaultProfileConfinesWithoutTheAdminCapability(t *testing.T) {
	const cloneNewUser = unix.CLONE_NEWUSER
	const numberAboveTheProfile = 470
	thirteen := boundingSets()["the default thirteen"]
	for _, table := range tables {
		confined := assemble(t, defaultProfileFor(table.goarch, thirteen), table)
		for name, want := range map[string]uint32{
			"unshare": returnPermission,
			"setns":   returnPermission,
			"mount":   returnPermission,
			"clone3":  returnNotImplemented,
			"setuid":  returnAllow,
			"setgid":  returnAllow,
			"fchown":  returnAllow,
			"read":    returnAllow,
		} {
			if got := call(t, confined, table, name); got != want {
				t.Errorf("on %s without CAP_SYS_ADMIN, %s answered %#x, want %#x", table.goarch, name, got, want)
			}
		}
		if got := call(t, confined, table, "clone", uint64(unix.SIGCHLD)); got != returnAllow {
			t.Errorf("on %s, a plain clone answered %#x, want ALLOW", table.goarch, got)
		}
		if got := call(t, confined, table, "clone", uint64(cloneNewUser|unix.SIGCHLD)); got != returnPermission {
			t.Errorf("on %s, a clone into a new user namespace answered %#x, want ERRNO(EPERM)", table.goarch, got)
		}
		if got := runFilter(t, confined, table.audit, numberAboveTheProfile); got != returnNotImplemented {
			t.Errorf("on %s, call %d answered %#x, want ERRNO(ENOSYS)", table.goarch, numberAboveTheProfile, got)
		}

		admin := assemble(t, defaultProfileFor(table.goarch, append(slices.Clone(thirteen), "CAP_SYS_ADMIN")), table)
		if got := call(t, admin, table, "unshare"); got != returnAllow {
			t.Errorf("on %s with CAP_SYS_ADMIN, unshare answered %#x, want ALLOW", table.goarch, got)
		}
		if got := runFilter(t, admin, table.audit, numberAboveTheProfile); got != returnNotImplemented {
			t.Errorf("on %s with CAP_SYS_ADMIN, call %d answered %#x, want ERRNO(ENOSYS)", table.goarch, numberAboveTheProfile, got)
		}
	}
}
