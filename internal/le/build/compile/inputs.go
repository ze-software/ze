// Design: docs/contributing/preview-artifacts.md -- compiler-selected provenance.
package buildcompile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// DaemonInputs asks the activated compiler for both actual target populations.
// Standard-library provenance is the exact toolchain pin and stdpatch inputs;
// nonstandard dependencies MUST be local vendored source, so no source read
// from an arbitrary module cache can disappear from the manifest.
func DaemonInputs(ctx context.Context, root string) (paths []string, tags []string, pin string, err error) {
	tc, err := gotoolchain.New(root)
	if err != nil {
		return nil, nil, "", err
	}
	tagText, err := repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
	if err != nil {
		return nil, nil, "", err
	}
	selected := map[string]bool{}
	for _, arch := range []string{"amd64", "arm64"} {
		environ := previewEnvironment(tc.EnvironmentContext(ctx, gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch}))
		command := exec.CommandContext(ctx, "go", "list", "-mod=vendor", "-deps", "-json", "-tags", tagText, "./cmd/ze")
		command.Dir, command.Env = root, environ
		var stderr bytes.Buffer
		command.Stderr = &stderr
		body, err := command.Output()
		if err != nil {
			return nil, nil, "", fmt.Errorf("select Linux %s preview inputs: %w: %s", arch, err, stderr.String())
		}
		if err := selectedInputs(root, body, selected); err != nil {
			return nil, nil, "", err
		}
	}
	for path := range selected {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths, strings.Fields(tagText), tc.GoToolchain, nil
}

func selectedInputs(root string, body []byte, selected map[string]bool) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	count := 0
	for {
		var pkg struct {
			Dir, ImportPath                                                                                                     string
			Standard, Incomplete                                                                                                bool
			Error                                                                                                               *struct{ Err string }
			GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles, EmbedFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("decode preview input population: %w", err)
		}
		if pkg.Incomplete || pkg.Error != nil {
			return fmt.Errorf("incomplete preview dependency %s", pkg.ImportPath)
		}
		count++
		if pkg.Standard {
			continue
		}
		if !filepath.IsAbs(pkg.Dir) {
			return fmt.Errorf("dependency %s has no absolute source directory", pkg.ImportPath)
		}
		for _, files := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.SysoFiles, pkg.EmbedFiles} {
			for _, name := range files {
				rel, err := filepath.Rel(root, filepath.Join(pkg.Dir, name))
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return fmt.Errorf("dependency %s reads source outside checkout", pkg.ImportPath)
				}
				selected[filepath.ToSlash(rel)] = true
			}
		}
	}
	if count == 0 || len(selected) == 0 {
		return errors.New("compiler selected no preview source inputs")
	}
	return nil
}
