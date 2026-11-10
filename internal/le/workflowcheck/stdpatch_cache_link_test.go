package workflowcheck

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// stdpatchGenerator is the bootstrap route to the patched GOROOT that runs
// before le exists (internal/le/go/stdpatch/goroot).
const stdpatchGenerator = "go run ./internal/le/go/stdpatch/goroot"

// leInvocation matches a shell command that starts the checkout's launcher.
var leInvocation = regexp.MustCompile(`(^|[\s;&|("'])\./le(\s|$)`)

// TestPatchedGOROOTSurvivesTheLauncherCacheLink pins every workflow job that
// selects the patched GOROOT with the generator to have run le first, when the
// job runs le at all.
//
// The generator places the tree under cache/stdpatch and creates cache/ as a
// real directory when nothing made it the link yet. The launcher's
// ensure_cache_link (./le) then moves a real cache/ aside and links it to
// ~/.cache/ze, so the GOROOT the job exported names a tree that is gone. Run
// first, le makes cache/ the link, and the tree then lives behind it.
//
// VALIDATES: in each job, the first step that runs the generator comes after a
// step that runs ./le, or no step runs ./le.
// PREVENTS: CodeQL run 42915194844, where "Build Go shipped feature
// combinations" ran ./le after the generator had made cache/ real, the
// launcher moved the tree away, and go failed with "verifying module: missing
// GOSUMDB" fetching a toolchain.
func TestPatchedGOROOTSurvivesTheLauncherCacheLink(t *testing.T) {
	for _, name := range workflowNames(t) {
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Name string `yaml:"name"`
					Run  string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(workflowSource(t, name)), &workflow); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for jobName, job := range workflow.Jobs {
			generator := -1
			leBefore := false
			leAfter := ""
			for index, step := range job.Steps {
				run := stripComments(step.Run)
				if generator < 0 && strings.Contains(run, stdpatchGenerator) {
					generator = index
					continue
				}
				if !leInvocation.MatchString(run) {
					continue
				}
				if generator < 0 {
					leBefore = true
					continue
				}
				if leAfter == "" {
					leAfter = step.Name
				}
			}
			if generator < 0 {
				continue
			}
			if leBefore {
				continue
			}
			if leAfter == "" {
				continue
			}
			t.Errorf("%s job %s selects the patched GOROOT with %q before any step runs ./le, and step %q runs ./le after it: the launcher moves the real cache/ the generator made, and the exported GOROOT with it. Fix: select the tree with `./le go stdpatch goroot`, or run ./le in an earlier step", name, jobName, stdpatchGenerator, leAfter)
		}
	}
}
