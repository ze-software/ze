// Design: docs/architecture/testing/verify-freshness-scope.md -- verification debt at commit preparation
package commit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/verify/engine"
)

// VerificationState records what the latest native verify status proves for
// the prospective commit's explicit paths.
type VerificationState struct {
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Mode      string `json:"mode,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

const trackedBuildStage = "repository tracked-build/check"

var structuralStages = map[string]bool{
	"verify lint/run": true, "tier/check": true,
	"iface-resolution": true, "plugin boundary/check": true,
	"doc wiring": true, "verify deps/evidence-vet": true,
	"staticcheck-feature-matrix/check": true,
	trackedBuildStage:                  true,
}

type structuralReds struct {
	Charged      []string
	Unattributed []string
	Foreign      []string
}

type failureIndex struct {
	Stages []json.RawMessage `json:"stages"`
}

type failureStage struct {
	Stage    string          `json:"stage"`
	ExitCode int             `json:"exit-code"`
	Groups   json.RawMessage `json:"groups"`
}

type failureGroup struct {
	ID      any `json:"group-id"`
	Kind    any `json:"kind"`
	Related any `json:"related"`
}

func structuralGateReds(root string, paths []string) structuralReds {
	content, err := os.ReadFile(filepath.Join(root, "tmp", "ze-verify-failures.json")) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
	if err != nil {
		return structuralReds{}
	}
	var index failureIndex
	if json.Unmarshal(content, &index) != nil {
		return structuralReds{}
	}
	scope := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			scope = append(scope, path)
		}
	}
	var result structuralReds
	for _, rawStage := range index.Stages {
		var stage failureStage
		if json.Unmarshal(rawStage, &stage) != nil {
			continue
		}
		if stage.ExitCode == 0 || !structuralStages[stage.Stage] {
			continue
		}
		var groups []failureGroup
		groupsReadable := json.Unmarshal(stage.Groups, &groups) == nil
		blind := make([]string, 0)
		owned := false
		if !groupsReadable || len(groups) == 0 {
			blind = append(blind, stage.Stage)
		}
		if groupsReadable {
			for _, group := range groups {
				named := groupRelatedPaths(root, group)
				if len(named) == 0 {
					id, _ := group.ID.(string)
					if id == "" {
						id = stage.Stage
					}
					blind = append(blind, id)
					continue
				}
				for _, related := range named {
					if relatedInCommit(related, scope) {
						owned = true
						break
					}
				}
			}
		}
		if len(blind) == 0 && !owned {
			result.Foreign = append(result.Foreign, stage.Stage)
			continue
		}
		result.Charged = append(result.Charged, stage.Stage)
		if !owned {
			result.Unattributed = append(result.Unattributed,
				stage.Stage+" ("+strings.Join(blind, ", ")+")")
		}
	}
	return result
}

func groupRelatedPaths(root string, group failureGroup) []string {
	kind, _ := group.Kind.(string)
	if kind != "files" && kind != "lint" && kind != "package" {
		return nil
	}
	values, ok := group.Related.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0)
	for _, value := range values {
		related, ok := value.(string)
		if !ok {
			continue
		}
		related = strings.TrimSpace(related)
		related = strings.TrimSuffix(related, "/...")
		related = strings.TrimPrefix(related, "./")
		if related == "" || related == "." || strings.HasPrefix(related, "/") {
			continue
		}
		valid := true
		for component := range strings.SplitSeq(related, "/") {
			if component == "" || component == "." || component == ".." {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(related))); err == nil {
			result = append(result, related)
		}
	}
	return result
}

func relatedInCommit(related string, paths []string) bool {
	for _, path := range paths {
		if related == path || strings.HasPrefix(related, path+"/") ||
			strings.HasPrefix(path, related+"/") {
			return true
		}
	}
	return false
}

func verificationState(root string, paths []string) VerificationState {
	freshness := verifyengine.CheckCertificate(root, paths)
	state := VerificationState{
		State:     "stale",
		Detail:    freshness.Reason,
		Mode:      freshness.Mode,
		Timestamp: freshness.Timestamp,
		Commit:    freshness.GitSHA,
	}
	if freshness.Fresh {
		state.State = verifyFresh
	}
	return state
}

func carriesGo(paths []string) bool {
	for _, path := range paths {
		if strings.HasSuffix(path, ".go") || path == "go.mod" || path == "go.sum" || strings.HasPrefix(path, "vendor/") {
			return true
		}
	}
	return false
}
