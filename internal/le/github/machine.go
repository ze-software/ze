// Design: docs/contributing/github-integration.md -- local observations qualify native evidence.

package github

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	changed "github.com/ze-software/ze/internal/le/repo/changed"
)

const codeMachine = "github-machine-evidence"
const machineLabel = "github-machine-observation"
const machineGit = "git"

// machineReport records current local observations. Ready means the observed
// committed inputs and capacity permit a main-thread handoff, not that a test
// ran or that a slot is reserved. It never imports a prior certificate.
type machineReport struct {
	Repository        string        `json:"repository"`
	Root              string        `json:"root"`
	Host              string        `json:"host"`
	OS                string        `json:"os"`
	Architecture      string        `json:"architecture"`
	Revision          string        `json:"revision"`
	RequestedRevision string        `json:"requested-revision"`
	InputDigest       string        `json:"input-digest"`
	Dirty             bool          `json:"dirty"`
	LinkedWorktree    bool          `json:"linked-worktree"`
	SourceRead        bool          `json:"source-read"`
	Ready             bool          `json:"ready"`
	Execution         string        `json:"execution"`
	Capacity          *job.Overview `json:"capacity"`
	Diagnostic        string        `json:"diagnostic"`
	Problems          []string      `json:"problems"`
	Policy            string        `json:"policy"`
}

// machineSources is the already owned local machinery, not a transport API.
// Its zero value is uninitialized; production uses localMachineSources.
type machineSources struct {
	root      func() (string, error)
	head      func(string) string
	inputHash func(string) (string, error)
	dirty     func(string) (bool, error)
	branch    func(string) (string, error)
	linked    func(string) (bool, error)
	hostname  func() (string, error)
	capacity  func(string) (*job.Overview, error)
}

func localMachineSources() machineSources {
	return machineSources{
		root: lepath.Root, head: job.Head, hostname: os.Hostname,
		inputHash: func(root string) (string, error) { return job.InputHash(root, machineLabel) },
		dirty: func(root string) (bool, error) {
			out, err := changed.RunCommand(root, []string{machineGit, "status", "--porcelain=v1", "-z", "--untracked-files=all"})
			return out != "", err
		},
		branch: func(root string) (string, error) {
			out, err := changed.RunCommand(root, []string{machineGit, "symbolic-ref", "--short", "HEAD"})
			return strings.TrimSpace(out), err
		},
		linked: func(root string) (bool, error) {
			directory, err := changed.RunCommand(root, []string{machineGit, "rev-parse", "--path-format=absolute", "--git-dir"})
			if err != nil {
				return false, err
			}
			common, err := changed.RunCommand(root, []string{machineGit, "rev-parse", "--path-format=absolute", "--git-common-dir"})
			if err != nil {
				return false, err
			}
			directory, err = filepath.EvalSymlinks(strings.TrimSpace(directory))
			if err != nil {
				return false, err
			}
			common, err = filepath.EvalSymlinks(strings.TrimSpace(common))
			if err != nil {
				return false, err
			}
			return directory != common, nil
		},
		capacity: func(root string) (*job.Overview, error) {
			admission, err := job.NewIn(root)
			if err != nil {
				return nil, err
			}
			return admission.Overview()
		},
	}
}

func observeMachine(requested string, source machineSources) (machineReport, error) {
	report := machineReport{Repository: Repository, OS: runtime.GOOS, Architecture: runtime.GOARCH,
		Execution: "not-started", Problems: []string{}, Policy: "Current AGENTS hard bans and ai/owner/machines.md govern transport; this observation grants no permission"}
	if requested != "" {
		canonical, err := fullRevision(requested)
		if err != nil {
			report.Diagnostic = codeInput
			return report.machineFailure(err)
		}
		report.RequestedRevision = canonical
	}
	root, err := source.root()
	if err != nil {
		return report.machineFailure(err)
	}
	report.Root = root
	report.Host, err = source.hostname()
	if err != nil {
		return report.machineFailure(err)
	}
	if report.Host == "" {
		return report.machineFailure(errors.New("local hostname was not measured"))
	}
	report.Revision = source.head(root)
	if _, err := fullRevision(report.Revision); err != nil {
		return report.machineFailure(errors.New("local HEAD could not be identified"))
	}
	before, err := source.inputHash(root)
	if err != nil {
		return report.machineFailure(err)
	}
	if before == "" {
		return report.machineFailure(errors.New("local inputs were not fingerprinted"))
	}
	report.InputDigest = before
	report.Dirty, err = source.dirty(root)
	if err != nil {
		return report.machineFailure(err)
	}
	branch, err := source.branch(root)
	if err != nil {
		return report.machineFailure(err)
	}
	if branch != "main" {
		report.Problems = append(report.Problems, "this is not the main checkout branch")
	}
	report.LinkedWorktree, err = source.linked(root)
	if err != nil {
		return report.machineFailure(err)
	}
	if report.LinkedWorktree {
		report.Problems = append(report.Problems, "this is a linked worktree, not the main checkout")
	}
	report.Capacity, err = source.capacity(root)
	if err != nil {
		return report.machineFailure(err)
	}
	if report.Capacity == nil {
		return report.machineFailure(errors.New("local job capacity was not measured"))
	}
	after, err := source.inputHash(root)
	if err != nil {
		return report.machineFailure(err)
	}
	if after != before {
		report.Problems = append(report.Problems, "local source inputs changed during observation")
	}
	if source.head(root) != report.Revision {
		report.Problems = append(report.Problems, "local HEAD changed during observation")
	}
	report.SourceRead = true
	if report.Dirty {
		report.Problems = append(report.Problems, "local inputs are dirty; they are not the committed revision's input set")
	}
	if report.RequestedRevision != "" {
		if report.Revision != report.RequestedRevision {
			report.Problems = append(report.Problems, "local HEAD differs from the requested revision")
		}
	}
	if report.Capacity.ProcessTable != "read" {
		report.Problems = append(report.Problems, "job process liveness is unavailable")
	}
	if report.Capacity.Free < 1 {
		report.Problems = append(report.Problems, "no native job slot is currently free")
	}
	if len(report.Capacity.Waiters) != 0 {
		report.Problems = append(report.Problems, "another native job is already waiting")
	}
	memory, measured := report.Capacity.Machine.MemoryAvailable.Value()
	if !measured {
		report.Problems = append(report.Problems, "available memory could not be measured")
	}
	if measured {
		if memory <= 0 {
			report.Problems = append(report.Problems, "no available memory was reported")
		}
	}
	if len(report.Problems) != 0 {
		return report.machineFailure(nil)
	}
	report.Ready = true
	return report, nil
}

func (report machineReport) machineFailure(err error) (machineReport, error) {
	if err != nil {
		report.Problems = append(report.Problems, err.Error())
	}
	if report.Diagnostic == "" {
		report.Diagnostic = codeMachine
	}
	report.Ready = false
	return report, opererr.New("local machine evidence is not ready for a committed-revision handoff",
		strings.Join(report.Problems, "; ")+". No synchronization or test was started; prior native evidence remains unbound to this observation.", "./le github report machine")
}
