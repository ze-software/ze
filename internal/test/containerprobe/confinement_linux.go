// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: main.go -- the modes, the report and the verifier

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// defaultCapabilityMask is the effective and bounding mask of a container
// with no capability grant: the thirteen default capabilities. CAP_MKNOD
// (bit 27) returns with the control group's device allowlist, and the mask
// is then 00000000a80425fb.
const defaultCapabilityMask = "00000000a00425fb"

// callAboveTheProfile is a call number above the largest the default
// profile names on both shipped architectures (listns since Linux 6.18), so
// the filter answers ENOSYS for it whatever the kernel implements.
const callAboveTheProfile = 470

// callAnswer makes call number with zero arguments and answers its error
// number, 0 when it succeeded. unshare(0) and listns with no buffer change
// nothing.
func callAnswer(number uintptr) syscall.Errno {
	_, _, errno := syscall.RawSyscall(number, 0, 0, 0)
	return errno
}

// observeConfinement reads, from inside the container, the facts of its
// confinement: its process list, its capability masks and the filter's
// answers to unshare(0) and to a number above the profile.
func observeConfinement() (string, error) {
	var report strings.Builder
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return "", err
	}
	var processes []string
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			processes = append(processes, entry.Name())
		}
	}
	slices.Sort(processes)
	report.WriteString("processes=" + strings.Join(processes, ",") + "\n")
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(status), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch key {
		case "CapEff":
			report.WriteString("cap-effective=" + strings.TrimSpace(value) + "\n")
		case "CapBnd":
			report.WriteString("cap-bounding=" + strings.TrimSpace(value) + "\n")
		}
	}
	report.WriteString("unshare=" + strconv.FormatUint(uint64(callAnswer(syscall.SYS_UNSHARE)), 10) + "\n")
	report.WriteString("call-above=" + strconv.FormatUint(uint64(callAnswer(callAboveTheProfile)), 10) + "\n")
	return report.String(), nil
}

// confined judges the default confinement of container name from its report
// in store. The filter's two answers count only once the host answers the
// same calls otherwise: there unshare(0) succeeds and the kernel implements
// the number, so EPERM and ENOSYS in the container are the filter's.
func confined(store, name string) bool {
	facts, err := awaitReport(filepath.Join(store, "layer", name, "upper", "report"))
	if err != nil {
		fmt.Println("probe: FAIL " + err.Error())
		return false
	}
	hostUnshare := callAnswer(syscall.SYS_UNSHARE)
	hostAbove := callAnswer(callAboveTheProfile)
	permission := strconv.FormatUint(uint64(syscall.EPERM), 10)
	notImplemented := strconv.FormatUint(uint64(syscall.ENOSYS), 10)
	passed := judge("the host allows unshare(0)", hostUnshare == 0, hostUnshare.Error())
	passed = judge("the host kernel implements call "+strconv.Itoa(callAboveTheProfile), hostAbove != syscall.ENOSYS, hostAbove.Error()) && passed
	passed = judge("its process list holds only its own process", facts["processes"] == "1", facts["processes"]) && passed
	passed = judge("its own cgroup reads as the root", facts["cgroup"] == "0::/", facts["cgroup"]) && passed
	passed = judge("its effective capabilities are the defaults", facts["cap-effective"] == defaultCapabilityMask, facts["cap-effective"]) && passed
	passed = judge("its bounding capabilities are the defaults", facts["cap-bounding"] == defaultCapabilityMask, facts["cap-bounding"]) && passed
	passed = judge("the filter refuses unshare with a permission error", facts["unshare"] == permission, facts["unshare"]) && passed
	passed = judge("the filter answers call "+strconv.Itoa(callAboveTheProfile)+" with not implemented", facts["call-above"] == notImplemented, facts["call-above"]) && passed
	return passed
}
