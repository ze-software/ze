// Related: evidence.go -- the release-candidate run these tests call as a function

package evidence

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// recorder is a Runner whose two outside effects are captured rather than made:
// what git was asked, and what docker was started with.
type recorder struct {
	asked   [][]string
	started [][]string
	missing string
	status  string
	fail    error
	code    int
}

// checkout is the tree every case in this file judges. Its value never
// matters; what matters is that both halves of a comparison use one.
const checkout = "/checkout"

func (rec *recorder) runner() *Runner {
	return &Runner{
		Tree:     checkout,
		Image:    "golang:1.27",
		Platform: "linux/amd64",
		Look: func(name string) error {
			if name == rec.missing {
				return errors.New("not on PATH")
			}
			return nil
		},
		Ask: func(args ...string) (string, error) {
			rec.asked = append(rec.asked, args)
			return rec.status, rec.fail
		},
		Start: func(args ...string) int {
			rec.started = append(rec.started, args)
			return rec.code
		},
	}
}

// VALIDATES: a clean tree starts one container, mounting the tree read-only and
// handing bash the container script.
// PREVENTS: the gate running against the developer's own tree, which is the one
// thing this check exists to make impossible.
func TestACleanTreeStartsOneContainer(t *testing.T) {
	rec := &recorder{}

	report, err := rec.runner().Run()
	if err != nil {
		t.Fatalf("a clean tree answered an error: %v", err)
	}
	if !report.Passed {
		t.Error("a container that exited 0 was not reported as passed")
	}
	if len(rec.started) != 1 {
		t.Fatalf("the run started %d containers, want 1", len(rec.started))
	}

	argv := rec.started[0]
	line := strings.Join(argv, " ")
	for _, want := range []string{
		"run --rm", "--privileged", "--platform linux/amd64",
		"-v " + checkout + ":/host:ro", "golang:1.27", "bash -lc",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the docker argv does not carry %q:\n%s", want, line)
		}
	}
	if argv[len(argv)-1] != ContainerScript {
		t.Error("the last argument is not the container script")
	}
}

// VALIDATES: a dirty tree refuses BEFORE anything is started, and names every
// path that made it dirty.
// PREVENTS: the refusal reaching the operator without the paths, which is the
// one thing they need in order to act on it.
func TestADirtyTreeStartsNothing(t *testing.T) {
	rec := &recorder{status: " M internal/a.go\n?? tmp/b.txt\n"}

	report, err := rec.runner().Run()
	if !errors.Is(err, ErrDirtyTree) {
		t.Fatalf("a dirty tree answered %v, want ErrDirtyTree", err)
	}
	if len(rec.started) != 0 {
		t.Errorf("a dirty tree started %d containers, want 0", len(rec.started))
	}
	if len(report.Dirty) != 2 {
		t.Fatalf("the report names %v, want both dirty paths", report.Dirty)
	}
	if report.Dirty[0] != " M internal/a.go" || report.Dirty[1] != "?? tmp/b.txt" {
		t.Errorf("the report names %v, want git's own two lines", report.Dirty)
	}
	if report.Passed {
		t.Error("a dirty tree was reported as passed")
	}

	text := report.Text()
	if !strings.Contains(text, "clean git worktree") {
		t.Errorf("the rendering does not say what is required:\n%s", text)
	}
	if !strings.Contains(text, "internal/a.go") {
		t.Errorf("the rendering does not name the dirty path:\n%s", text)
	}
}

// VALIDATES: a missing external command stops the run before it asks git
// anything.
// PREVENTS: a confusing git or docker failure standing in for the plain fact
// that the tool is not installed.
func TestAMissingCommandStopsTheRun(t *testing.T) {
	for _, name := range requiredCommandNames() {
		t.Run(name, func(t *testing.T) {
			rec := &recorder{missing: name}

			_, err := rec.runner().Run()
			if err == nil {
				t.Fatal("a missing command answered no error")
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("the error is %q, want it to name %q", err, name)
			}
			if len(rec.asked) != 0 || len(rec.started) != 0 {
				t.Error("a missing command still ran something")
			}
		})
	}
}

// VALIDATES: the container's own exit status is the command's exit status.
// PREVENTS: a flattened 1, which would tell a caller that the gate failed
// without saying which stage of it did (AC-8).
func TestTheContainerExitStatusIsTheAnswer(t *testing.T) {
	for _, code := range []int{0, 1, 2, 3, 125} {
		rec := &recorder{code: code}

		report, err := rec.runner().Run()
		if err != nil {
			t.Fatalf("the run answered an error: %v", err)
		}
		if report.Code != code {
			t.Errorf("the report carries %d, want the container's own %d", report.Code, code)
		}
		if report.Passed != (code == 0) {
			t.Errorf("a container that exited %d reports passed=%v", code, report.Passed)
		}
	}
}

// VALIDATES: git failing to answer is an error rather than a clean tree.
// PREVENTS: the fail-open where git cannot read the tree, answers nothing, and
// the empty answer is read as "no changes".
func TestGitFailingToAnswerIsNotACleanTree(t *testing.T) {
	rec := &recorder{fail: errors.New("not a git repository")}

	if _, err := rec.runner().Run(); err == nil {
		t.Fatal("git failing to answer was read as a clean tree")
	}
	if len(rec.started) != 0 {
		t.Error("a container was started after git failed")
	}
}

// VALIDATES: the container script clones the mounted tree and runs the verify
// gate inside the clone.
// PREVENTS: a rewrite that runs the gate over the read-only mount, which is the
// developer's own tree by another name.
func TestTheContainerScriptClonesAndVerifies(t *testing.T) {
	for _, want := range []string{
		"set -euo pipefail",
		"git clone --no-local /host /work/src",
		"cd /work/src",
		"./le verify current mode full",
	} {
		if !strings.Contains(ContainerScript, want) {
			t.Errorf("the container script does not carry %q", want)
		}
	}
}

// VALIDATES: the answer is structured data with kebab-case keys, so | json,
// | yaml and | table each render it (AC-7).
// PREVENTS: a tool that answers finished text, which no operator can pipe.
func TestTheReportIsStructuredData(t *testing.T) {
	rec := &recorder{status: " M a.go\n", code: 1}

	report, _ := rec.runner().Run()

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the encoded report does not decode: %v", err)
	}
	for _, key := range []string{"image", "platform", "tree", "dirty", "passed", "code"} {
		if _, ok := back[key]; !ok {
			t.Errorf("the report has no %q key: %s", key, raw)
		}
	}
	for key := range back {
		if strings.ContainsAny(key, "_ ") || strings.ToLower(key) != key {
			t.Errorf("the key %q is not kebab-case", key)
		}
	}
}
