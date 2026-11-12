// Design: docs/contributing/preview-artifacts.md -- images reuse checked binaries.
package interoplab

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBuildTargetReachesDockerAsOneStageArgument(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch AS lab\n", 0o644)
	recorded := &buildContextRunner{}
	_, err := newDocker(recorded.runner()).Build(t.Context(), ImageBuild{Tag: "target:local", Context: root, Target: "lab"})
	if err != nil {
		t.Fatal(err)
	}
	index := slices.Index(recorded.arguments, "--target")
	if index < 0 || index+1 >= len(recorded.arguments) || recorded.arguments[index+1] != "lab" {
		t.Fatalf("target absent from Docker argv: %v", recorded.arguments)
	}
	for _, target := range []string{"--pull", "lab --pull", "lab\nother"} {
		runner := &recordingRunner{}
		if _, err := newDocker(runner).Build(t.Context(), ImageBuild{Tag: "bad:local", Context: root, Target: target}); err == nil {
			t.Fatalf("invalid stage admitted: %q", target)
		}
		if len(runner.commands) != 0 {
			t.Fatalf("invalid target reached Docker: %v", runner.commands)
		}
	}
}

func TestRuntimeImageContextContainsOnlyRecipeAndStagedBinary(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, filepath.Join(root, "docker", "Dockerfile.runtime"), "FROM scratch AS deployment\nCOPY ze /ze\n", 0o644)
	writeContextFile(t, filepath.Join(root, "test", "interop-stale", "ze-linux"), "unrelated large artifact", 0o755)
	binary := filepath.Join(t.TempDir(), "ze-linux-arm64")
	writeContextFile(t, binary, "already checked arm64 binary", 0o755)
	directory := filepath.Join(t.TempDir(), "image")
	build, err := StageRuntimeImage(root, directory, binary, "deployment")
	if err != nil {
		t.Fatal(err)
	}
	build.Tag = "runtime:local"
	recorded := &buildContextRunner{}
	if _, err := newDocker(recorded.runner()).Build(t.Context(), build); err != nil {
		t.Fatal(err)
	}
	entries := recorded.entries(t)
	if len(entries) != 2 || entries["ze"].body != "already checked arm64 binary" || entries["Dockerfile"].body != "FROM scratch AS deployment\nCOPY ze /ze\n" {
		t.Fatalf("runtime image context = %v", entries)
	}
	if entries["ze"].header.Mode&0o777 != 0o755 || build.Target != "deployment" {
		t.Fatalf("runtime mode=%o target=%q", entries["ze"].header.Mode, build.Target)
	}
	if _, err := StageRuntimeImage(root, directory, binary, "lab"); err == nil {
		t.Fatal("existing context overwritten")
	}
	if err := os.Symlink(binary, filepath.Join(root, "linked-ze")); err != nil {
		t.Fatal(err)
	}
	if _, err := StageRuntimeImage(root, filepath.Join(t.TempDir(), "linked"), filepath.Join(root, "linked-ze"), "lab"); err == nil {
		t.Fatal("symlink binary admitted")
	}
}

func TestRuntimeRecipeKeepsDeploymentAndLabContracts(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "docker", "Dockerfile.runtime"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{"FROM scratch AS deployment", "FROM alpine:3.24.3 AS lab", "ca-certificates", "/etc/ssl/certs/ca-certificates.crt", "COPY ze /ze", "COPY ze /usr/local/bin/ze", "EXPOSE 179 2222 8080", `ENTRYPOINT ["/ze"]`, `ENTRYPOINT ["tini", "--"]`, `CMD ["ze", "start", "/etc/ze/ze.conf"]`} {
		if !strings.Contains(text, required) {
			t.Errorf("runtime recipe lacks %s", required)
		}
	}
	for _, forbidden := range []string{"golang:", "go build", "COPY . ."} {
		if strings.Contains(text, forbidden) {
			t.Errorf("runtime recipe rebuilds or copies source: %s", forbidden)
		}
	}
}
