package procfs

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseProcessStatusLines(t *testing.T) {
	lines := []string{
		"Name:\tze-plugin",
		"Umask:\t0022",
		"State:\tS (sleeping)",
		"Tgid:\t4242",
		"Pid:\t4242",
		"VmPeak:\t  123456 kB",
		"VmSize:\t  112233 kB",
		"VmRSS:\t   45678 kB",
		"VmData:\t   34567 kB",
		"VmStk:\t     136 kB",
		"VmSwap:\t     512 kB",
		"Threads:\t9",
		"voluntary_ctxt_switches:\t12345",
		"nonvoluntary_ctxt_switches:\t678",
		"malformed-line-without-colon",
		"",
	}

	got := parseProcessStatusLines(lines)
	if got.State != "S (sleeping)" {
		t.Errorf("State = %q, want %q", got.State, "S (sleeping)")
	}
	if got.Threads != 9 {
		t.Errorf("Threads = %d, want 9", got.Threads)
	}
	if got.VmPeakKB != 123456 {
		t.Errorf("VmPeakKB = %d, want 123456", got.VmPeakKB)
	}
	if got.VmSizeKB != 112233 {
		t.Errorf("VmSizeKB = %d, want 112233", got.VmSizeKB)
	}
	if got.VmRSSKB != 45678 {
		t.Errorf("VmRSSKB = %d, want 45678", got.VmRSSKB)
	}
	if got.VmDataKB != 34567 {
		t.Errorf("VmDataKB = %d, want 34567", got.VmDataKB)
	}
	if got.VmStackKB != 136 {
		t.Errorf("VmStackKB = %d, want 136", got.VmStackKB)
	}
	if got.VmSwapKB != 512 {
		t.Errorf("VmSwapKB = %d, want 512", got.VmSwapKB)
	}
	if got.VoluntaryCtxtSwitches != 12345 {
		t.Errorf("VoluntaryCtxtSwitches = %d, want 12345", got.VoluntaryCtxtSwitches)
	}
	if got.NonvoluntaryCtxtSwitches != 678 {
		t.Errorf("NonvoluntaryCtxtSwitches = %d, want 678", got.NonvoluntaryCtxtSwitches)
	}
	if got.FDCount != nil {
		t.Errorf("FDCount = %v, want nil", got.FDCount)
	}
}

func TestParseProcessStatusLinesEmpty(t *testing.T) {
	got := parseProcessStatusLines(nil)
	if got != (ProcessInspect{}) {
		t.Errorf("parseProcessStatusLines(nil) = %+v, want zero value", got)
	}
}

func TestReadProcessInspectInvalidPID(t *testing.T) {
	for _, pid := range []int{0, -1, -999} {
		_, err := ReadProcessInspect(pid)
		if err == nil {
			t.Errorf("ReadProcessInspect(%d) succeeded, want error", pid)
		}
	}
}

func TestReadProcessInspectSelf(t *testing.T) {
	info, err := ReadProcessInspect(os.Getpid())
	if runtime.GOOS != "linux" {
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("ReadProcessInspect(self) error = %v, want ErrUnsupported", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("ReadProcessInspect(self) error = %v", err)
	}
	if info.State == "" {
		t.Error("State is empty for live self process")
	}
	if info.Threads <= 0 {
		t.Errorf("Threads = %d, want > 0", info.Threads)
	}
	if info.VmRSSKB <= 0 {
		t.Errorf("VmRSSKB = %d, want > 0", info.VmRSSKB)
	}
	if info.VmSizeKB <= 0 {
		t.Errorf("VmSizeKB = %d, want > 0", info.VmSizeKB)
	}
	if info.FDCount == nil || *info.FDCount <= 0 {
		t.Errorf("FDCount = %v, want > 0", info.FDCount)
	}
}

func TestProcessInspectJSONTags(t *testing.T) {
	zero := 0
	info := ProcessInspect{
		State:                    "R (running)",
		Threads:                  4,
		VmRSSKB:                  100,
		VmSizeKB:                 200,
		VmSwapKB:                 10,
		VmPeakKB:                 300,
		VmDataKB:                 50,
		VmStackKB:                8,
		VoluntaryCtxtSwitches:    11,
		NonvoluntaryCtxtSwitches: 22,
		Wchan:                    "ep_poll",
		FDCount:                  &zero,
	}
	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(raw)
	for _, key := range []string{
		`"state":"R (running)"`,
		`"threads":4`,
		`"vm-rss-kb":100`,
		`"vm-size-kb":200`,
		`"vm-swap-kb":10`,
		`"vm-peak-kb":300`,
		`"vm-data-kb":50`,
		`"vm-stack-kb":8`,
		`"voluntary-ctxt-switches":11`,
		`"nonvoluntary-ctxt-switches":22`,
		`"wchan":"ep_poll"`,
		`"fd-count":0`,
	} {
		if !strings.Contains(s, key) {
			t.Errorf("JSON %s missing %s", s, key)
		}
	}

	info.FDCount = nil
	rawWithoutFD, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal without FDCount: %v", err)
	}
	if strings.Contains(string(rawWithoutFD), `"fd-count"`) {
		t.Errorf("JSON %s unexpectedly included fd-count when nil", string(rawWithoutFD))
	}
}
