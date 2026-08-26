package qemu

// The hugepage proof's payload.
//
// Goal: prove that the answer is DATA. Thus, `| json`, `| yaml` and `| table`
// render it with no code in the tool. Also prove that its default rendering
// says what the Python original printed. Method: marshal the report and read
// the document back. Then render the text for each verdict.

import (
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: the report encodes as a JSON document carrying what was ASKED for
// beside what was OBSERVED.
// PREVENTS: a tool that answers finished text, which is what AC-7 forbids, and a
// report that carries only the verdict, which a reader cannot tell from a run
// that asked for no reservation at all.
func TestTheHugepagesReportIsStructuredData(t *testing.T) {
	report := HugepagesReport{
		Verdict: VerdictPass, Arch: ArchAMD64, Accelerator: "kvm",
		PageSize: DefaultPageSize, PageToken: "2M", Reservation: DefaultReservation,
		Pages: 64, MemoryMiB: 1024,
		Cmdline: "console=ttyS0 hugepages=64", PagesTotal: 64,
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("the encoded report does not parse: %v", err)
	}

	for _, key := range []string{
		"verdict", "arch", "accelerator", "page-size", "page-token",
		"reservation", "pages", "memory-mib", "cmdline", "hugepages-total",
	} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("the report answered no %q key: %v", key, parsed)
		}
	}
	if parsed["verdict"] != "pass" {
		t.Errorf("the verdict rendered as %v, want the word", parsed["verdict"])
	}
}

// VALIDATES: a verdict reaches JSON as its WORD, and a run that reached no
// conclusion reaches it as "unspecified".
// PREVENTS: the zero value reading as a pass. That is the whole reason the
// verdict is a typed number instead of a `skipped` and a `passed` boolean.
func TestAVerdictRendersAsItsWord(t *testing.T) {
	for verdict, want := range map[Verdict]string{
		VerdictUnspecified: "unspecified",
		VerdictPass:        "pass",
		VerdictSkip:        "skip",
		VerdictFail:        "fail",
	} {
		raw, err := json.Marshal(verdict)
		if err != nil {
			t.Fatalf("marshal %v: %v", verdict, err)
		}
		var got string
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("the encoded verdict is not a string: %s", raw)
		}
		if got != want {
			t.Errorf("verdict %d rendered as %q, want %q", verdict, got, want)
		}
	}
}

// VALIDATES: the default rendering opens with the prefix the functional suite
// greps for, and says the same three things the Python original said.
// PREVENTS: a rendering that no longer carries the page count and the kernel's
// own total, which are the two numbers a reader checks a pass by.
func TestTheHugepagesReportRendersEveryVerdict(t *testing.T) {
	cases := []struct {
		name   string
		report HugepagesReport
		want   []string
	}{
		{
			"a pass names both numbers",
			HugepagesReport{Verdict: VerdictPass, Pages: 64, PagesTotal: 64},
			[]string{ReportPrefix + "PASS cmdline has hugepages=64, hugepages-total=64"},
		},
		{
			"a skip names the reason",
			HugepagesReport{Verdict: VerdictSkip, Reason: "sshpass not found"},
			[]string{ReportPrefix + "SKIP sshpass not found"},
		},
		{
			"a failure carries the console",
			HugepagesReport{
				Verdict: VerdictFail, Reason: "appliance did not answer over SSH",
				ConsoleTail: []string{"kernel panic"},
			},
			[]string{
				ReportPrefix + "FAIL appliance did not answer over SSH",
				"serial console tail:", "kernel panic",
			},
		},
	}
	for _, one := range cases {
		text := one.report.Text()
		for _, want := range one.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s: the rendering does not carry %q:\n%s", one.name, want, text)
			}
		}
	}

	// The console belongs to a failure alone. A run that got an answer has
	// nothing to explain, and printing a boot log would bury the verdict.
	passed := HugepagesReport{Verdict: VerdictPass, ConsoleTail: []string{"should not be printed"}}
	if strings.Contains(passed.Text(), "should not be printed") {
		t.Errorf("a passing run printed its serial console:\n%s", passed.Text())
	}
}
