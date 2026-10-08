package testqemu

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	leaction "github.com/ze-software/ze/internal/le/le/action"
	testintegration "github.com/ze-software/ze/internal/le/test/integration"
)

// VALIDATES: `le test qemu stress` boots the guest, runs the stress harness as
// root, and copies the report, the profiles and the DUT binary to a host path.
// PREVENTS: a measurement read from an earlier run, or from a file the guest
// report never named.

// TestStressActionIsRegistered proves `le test qemu stress` is reachable and
// declares its keywords. Method: read the area's own action table, the data
// the dispatcher and the help line are built from.
func TestStressActionIsRegistered(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != stressVerb {
			continue
		}
		var keywords []string
		for _, parameter := range action.Parameters {
			keywords = append(keywords, parameter.Keyword)
		}
		for _, want := range []string{keywordScenario, keywordPrefixes, keywordPprof, keywordOutput, keywordTimeout} {
			if !slices.Contains(keywords, want) {
				t.Errorf("stress declares %v, missing %q", keywords, want)
			}
		}
		return
	}
	t.Fatalf("le test qemu has no %q action", stressVerb)
}

// TestStressArgumentsParse proves a full invocation becomes the request the
// guest command is built from, with an absolute output directory. Method:
// parse typed arguments as the dispatcher hands them over.
func TestStressArgumentsParse(t *testing.T) {
	base := t.TempDir()
	request, err := parseStressArguments(leaction.Arguments{
		keywordScenario: {"05-profile-1m"}, keywordPrefixes: {"20000"},
		keywordPprof: {""}, keywordOutput: {"out"}, keywordTimeout: {"900s"},
	}, base)
	if err != nil {
		t.Fatal(err)
	}
	want := stressRequest{
		Scenario: "05-profile-1m", Prefixes: 20000, Pprof: true,
		Output: filepath.Join(base, "out"), Timeout: 900 * time.Second,
	}
	if request != want {
		t.Fatalf("got %+v, want %+v", request, want)
	}
}

// TestStressArgumentsRefuse proves every malformed invocation is refused on the
// host, before a guest boots for minutes only to fail. Method: one case for
// each refusal, each asserting the error names the cause.
func TestStressArgumentsRefuse(t *testing.T) {
	base := t.TempDir()
	full := func() leaction.Arguments {
		return leaction.Arguments{keywordScenario: {"05-profile-1m"}, keywordOutput: {"out"}}
	}
	cases := []struct {
		name string
		edit func(leaction.Arguments)
		want string
	}{
		{"no scenario", func(a leaction.Arguments) { delete(a, keywordScenario) }, "scenario"},
		{"unknown scenario", func(a leaction.Arguments) { a[keywordScenario] = []string{"99-nope"} }, "99-nope"},
		{"no output", func(a leaction.Arguments) { delete(a, keywordOutput) }, "output"},
		{"zero prefixes", func(a leaction.Arguments) { a[keywordPrefixes] = []string{"0"} }, "prefixes"},
		{"negative prefixes", func(a leaction.Arguments) { a[keywordPrefixes] = []string{"-1"} }, "prefixes"},
		{"word prefixes", func(a leaction.Arguments) { a[keywordPrefixes] = []string{"many"} }, "prefixes"},
		{"zero timeout", func(a leaction.Arguments) { a[keywordTimeout] = []string{"0"} }, "timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := full()
			tc.edit(args)
			_, err := parseStressArguments(args, base)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestStressPrefixesLowerBound proves one prefix is the smallest smoke a run
// accepts. Method: the boundary value beside the refused zero.
func TestStressPrefixesLowerBound(t *testing.T) {
	request, err := parseStressArguments(leaction.Arguments{
		keywordScenario: {"05-profile-1m"}, keywordOutput: {"out"}, keywordPrefixes: {"1"},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if request.Prefixes != 1 || request.Timeout != stressDefaultTimeout {
		t.Fatalf("got %+v", request)
	}
}

// TestStressScenarioPlaceholderIsTheRegistry proves the help placeholder is
// derived from the harness registry, so a new scenario is discoverable here
// with no edit. Method: compare the declared value with StressScenarios.
func TestStressScenarioPlaceholderIsTheRegistry(t *testing.T) {
	want := strings.Join(testintegration.StressScenarios(), ",")
	for _, action := range Actions().Actions {
		if action.Verb != stressVerb {
			continue
		}
		for _, parameter := range action.Parameters {
			if parameter.Keyword == keywordScenario && parameter.Value != want {
				t.Fatalf("scenario placeholder %q, want %q", parameter.Value, want)
			}
		}
	}
}

// TestStressGuestCommand proves the guest runs the harness through the guest
// le, as root, with the scenario and knobs it was given, and writes the JSON
// report where the host reads it. Method: build the command for both
// requests a measurement and a smoke make.
func TestStressGuestCommand(t *testing.T) {
	measure := stressGuestCommand(stressRequest{Scenario: "05-profile-1m"}, ArchARM64)
	wantMeasure := "STRESS_SCENARIO='05-profile-1m' tmp/qemu/linux-arm64/le test integration stress '|' json > tmp/qemu/stress-report.json"
	if measure != wantMeasure {
		t.Errorf("measure command\n got %s\nwant %s", measure, wantMeasure)
	}
	smoke := stressGuestCommand(stressRequest{Scenario: "05-profile-1m", Prefixes: 20000, Pprof: true}, ArchAMD64)
	wantSmoke := "STRESS_SCENARIO='05-profile-1m' STRESS_PREFIXES=20000 ZE_PPROF=1 tmp/qemu/linux-amd64/le test integration stress '|' json > tmp/qemu/stress-report.json"
	if smoke != wantSmoke {
		t.Errorf("smoke command\n got %s\nwant %s", smoke, wantSmoke)
	}
}

// TestStressEvidenceCopiesWhatTheReportNames proves the host copies the report,
// every profile it lists and the DUT binary into the output directory, reading
// the paths from the guest report rather than from a second list. Method: a
// fake checkout holding the files at the guest's /workspace paths.
func TestStressEvidenceCopiesWhatTheReportNames(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "tmp", "stress-profile-cpu.pb.gz"), "cpu")
	writeTestFile(t, filepath.Join(root, "tmp", "stress", "ze"), "elf")
	report := `{"action":"stress","scenarios":[{"name":"05-profile-1m","passed":true,` +
		`"profiles":[{"name":"cpu","path":"/workspace/tmp/stress-profile-cpu.pb.gz","bytes":3}],` +
		`"binary":"/workspace/tmp/stress/ze","exit-code":0}],"passed":1,"failed":0,"code":0}`
	output := filepath.Join(t.TempDir(), "evidence")

	files, err := copyStressEvidence(root, []byte(report), output)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(output, stressReportName),
		filepath.Join(output, "stress-profile-cpu.pb.gz"),
		filepath.Join(output, "ze"),
	}
	if !slices.Equal(files, want) {
		t.Fatalf("copied %v, want %v", files, want)
	}
	for i, content := range []string{report, "cpu", "elf"} {
		got, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != content {
			t.Errorf("%s holds %q, want %q", files[i], got, content)
		}
	}
}

// TestStressEvidenceRefuses proves the copy never invents evidence: a report
// that is not the harness JSON, a path outside the shared checkout, a listed
// file that is missing, and an output directory holding an earlier run are
// each an error. Method: one case for each.
func TestStressEvidenceRefuses(t *testing.T) {
	outside := `{"scenarios":[{"name":"x","profiles":[{"name":"cpu","path":"/tmp/stress-profile-cpu.pb.gz"}]}]}`
	missing := `{"scenarios":[{"name":"x","profiles":[{"name":"cpu","path":"/workspace/tmp/absent.pb.gz"}]}]}`
	cases := []struct {
		name   string
		report string
		dirty  bool
		want   string
	}{
		{"empty report", "", false, "report"},
		{"no scenarios", `{"scenarios":[]}`, false, "no scenario"},
		{"outside the checkout", outside, false, "/tmp/stress-profile-cpu.pb.gz"},
		{"missing file", missing, false, "absent.pb.gz"},
		{"earlier run in output", `{"scenarios":[{"name":"x"}]}`, true, "not empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "evidence")
			if tc.dirty {
				writeTestFile(t, filepath.Join(output, "old.pb.gz"), "old")
			}
			_, err := copyStressEvidence(t.TempDir(), []byte(tc.report), output)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
