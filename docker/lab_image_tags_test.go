package docker

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	awkProgram     = `$1 ~ /^ze_/ {print $1}`
	personalityTag = "ze_core"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func readRepositoryFile(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func featureTags(t *testing.T, root string) map[string]bool {
	t.Helper()
	tags := make(map[string]bool)
	for line := range strings.SplitSeq(readRepositoryFile(t, root, "feature-gates.txt"), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 0 && strings.HasPrefix(fields[0], "ze_") {
			tags[fields[0]] = true
		}
	}
	return tags
}

var awkPattern = regexp.MustCompile(`awk\s+'([^']*)'\s*(\S*feature-gates\.txt)`)

func awkPrograms(text string) []string {
	matches := awkPattern.FindAllStringSubmatch(text, -1)
	programs := make([]string, 0, len(matches))
	for _, match := range matches {
		programs = append(programs, strings.ReplaceAll(match[1], "$$", "$"))
	}
	return programs
}

func instructions(text string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

var buildTagsPattern = regexp.MustCompile(`-tags\s+"([^"]*)"`)

func goBuildTagLists(text string) []string {
	matches := buildTagsPattern.FindAllStringSubmatch(text, -1)
	lists := make([]string, 0, len(matches))
	for _, match := range matches {
		lists = append(lists, match[1])
	}
	return lists
}

func TestLabImageRecipesUseTheDefaultFeatureSet(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{"docker/Dockerfile", "docker/Dockerfile.lab"}
	if info, err := os.Stat(filepath.Join(root, "docker", "Dockerfile.lab")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("docker/Dockerfile.lab is missing: netlab and containerlab need a shell and network tools")
	}

	for _, path := range paths {
		text := readRepositoryFile(t, root, path)
		t.Run(path+" derives tags once", func(t *testing.T) {
			if got := awkPrograms(text); !slices.Equal(got, []string{awkProgram}) {
				t.Fatalf("awk programs = %q, want %q", got, []string{awkProgram})
			}
		})
		t.Run(path+" spells no feature tags", func(t *testing.T) {
			gates := featureTags(t, root)
			words := regexp.MustCompile(`\bze_[a-z0-9_]+\b`).FindAllString(instructions(text), -1)
			spelled := make([]string, 0)
			for _, word := range words {
				if gates[word] && !slices.Contains(spelled, word) {
					spelled = append(spelled, word)
				}
			}
			slices.Sort(spelled)
			if len(spelled) != 0 {
				t.Fatalf("hand-written feature tags = %q; they must come from feature-gates.txt", spelled)
			}
		})
		t.Run(path+" builds identical tags", func(t *testing.T) {
			want := personalityTag + " $ZE_FEATURES $ZE_TAGS"
			if got := goBuildTagLists(text); !slices.Equal(got, []string{want}) {
				t.Fatalf("go build tag lists = %q, want %q", got, []string{want})
			}
		})
	}

	for _, path := range []string{"docker/Dockerfile", "docker/Dockerfile.lab", "test/interop/Dockerfile.ze"} {
		if !slices.Contains(awkPrograms(readRepositoryFile(t, root, path)), awkProgram) {
			t.Errorf("%s does not use the shared feature-gates.txt derivation", path)
		}
	}
	if !featureTags(t, root)["ze_bgp"] {
		t.Error("derived feature set does not contain ze_bgp")
	}
}

func TestDeploymentAndLabRecipesRemainDistinct(t *testing.T) {
	root := repositoryRoot(t)
	deploy := readRepositoryFile(t, root, "docker/Dockerfile")
	lab := readRepositoryFile(t, root, "docker/Dockerfile.lab")
	if !strings.Contains(deploy, "FROM scratch") {
		t.Error("deployment recipe no longer ends in the scratch image")
	}
	if strings.Contains(lab, "FROM scratch") || !strings.Contains(lab, "FROM alpine:") {
		t.Error("lab recipe must carry Alpine tools instead of the deployment scratch image")
	}
	if deploy == lab {
		t.Error("deployment and lab recipes became identical")
	}
}
