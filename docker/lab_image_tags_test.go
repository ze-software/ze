package docker

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const personalityTag = "ze_core"

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

// derivationPattern matches a recipe calling the one shell reader of the feature
// manifest: `. ./feature-tags` followed by a feature_tags call naming it.
//
// It replaced a match on the awk program each recipe used to inline. That
// assertion pinned the SHAPE of the walk, so collapsing nineteen copies of it
// into feature-tags and calling the survivor here read as a regression. What the
// recipe owes is that its feature set is DERIVED from feature-gates.txt rather
// than spelled out, which the shared reader satisfies and an inline copy no
// longer does.
var derivationPattern = regexp.MustCompile(`\.\s+\./feature-tags\b[\s\S]{0,200}?feature_tags\s+feature-gates\.txt`)

func derivesFeatureSet(text string) bool {
	return derivationPattern.MatchString(instructions(text))
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
		t.Run(path+" derives tags through the shared reader", func(t *testing.T) {
			if !derivesFeatureSet(text) {
				t.Fatalf("%s does not source feature-tags and read feature-gates.txt through it", path)
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

	// test/interop/Dockerfile.ze is not in that set and must not be: it compiles
	// nothing. `./le integration interop` cross-compiles both binaries through
	// internal/le/featuretags and the recipe copies them, so its feature set is
	// derived by the Go reader before the image build starts. The row asserting an
	// inline awk here could never pass, and said nothing about the image when it
	// failed.
	if strings.Contains(instructions(readRepositoryFile(t, root, "test/interop/Dockerfile.ze")), "go build") {
		t.Error("test/interop/Dockerfile.ze now compiles, so it owes the feature-gates.txt derivation the other recipes carry")
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
