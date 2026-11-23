// Design: third_party/vpp-patches/README.md -- generate both APIs from one pin.
// VALIDATES: Feature bindings share the bounded generator and ownership rules.
package buildvpp

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const apiFeatureOutput = "internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"

// TestVPPAPIGenerationPublishesFeatureBindings checks the native generation
// lifecycle publishes both Feature files and identifies all six outputs.
func TestVPPAPIGenerationPublishesFeatureBindings(t *testing.T) {
	root, scratch := apiFixture(t)
	process := &apiFeatureProcessFixture{}
	docker := &apiDockerFixture{}
	report, err := generateAPI(t.Context(), root, scratch, docker, process, []string{"CGO_ENABLED=0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(process.commands) == 0 {
		t.Fatal("the generator never ran")
	}
	for _, command := range process.commands {
		if !slices.Contains(command.Arguments, "feature") {
			t.Fatal("Feature API was not selected for generation")
		}
	}
	for _, name := range []string{"feature.ba.go", "feature_rpc.ba.go"} {
		path := filepath.Join(apiFeatureOutput, name)
		if !slices.Contains(report.Files, filepath.ToSlash(path)) {
			t.Fatalf("Feature output absent from report: %s", path)
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), apiGeneratedHeader+"\n") {
			t.Fatalf("unowned Feature output: %s", path)
		}
	}
	if len(report.Files) != 6 {
		t.Fatalf("published files = %v", report.Files)
	}
	if !slices.Equal(docker.inputs, []string{"/api/pppoe.api.json", "/api/feature.api.json", "/api/memclnt.api.json", "/api/interface_types.api.json", "/api/ethernet_types.api.json", "/api/ip_types.api.json"}) {
		t.Fatalf("JSON inputs = %v", docker.inputs)
	}
	if process.sourceVersion != "fixture-commit-pppoe-patched\n" {
		t.Fatalf("generator source version = %q", process.sourceVersion)
	}
}

// TestVPPAPIFeatureOwnershipRefusedBeforeBuild protects foreign files and
// symlinked output before any source build or generator can run.
func TestVPPAPIFeatureOwnershipRefusedBeforeBuild(t *testing.T) {
	for _, kind := range []string{"foreign-file", "symlink-file", "symlink-directory"} {
		t.Run(kind, func(t *testing.T) {
			root, scratch := apiFixture(t)
			output := filepath.Join(root, apiFeatureOutput)
			if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink-directory" {
				if err := os.Symlink(t.TempDir(), output); err != nil {
					t.Fatal(err)
				}
			} else {
				path := filepath.Join(output, "feature.ba.go")
				if kind == "foreign-file" {
					apiWriteFixture(t, path, "package feature\n// original\n")
				} else {
					if err := os.MkdirAll(output, 0o750); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Join(t.TempDir(), "original"), path); err != nil {
						t.Fatal(err)
					}
				}
			}
			docker := &apiDockerFixture{}
			process := &apiFeatureProcessFixture{}
			if _, err := generateAPI(t.Context(), root, scratch, docker, process, []string{"CGO_ENABLED=0"}); err == nil {
				t.Fatal("foreign Feature output accepted")
			}
			if docker.build.Context != "" {
				t.Fatal("ownership refusal followed a source build")
			}
			if process.runs != 0 {
				t.Fatal("ownership refusal followed generation")
			}
		})
	}
}

// TestVPPAPIFeatureValidationPrecedesAllPublication checks a malformed second
// package never changes already-owned PPPoE outputs or publishes Feature files.
func TestVPPAPIFeatureValidationPrecedesAllPublication(t *testing.T) {
	for _, mutation := range []string{"wrong-field", "wrong-type", "wrong-package", "missing-rpc", "unowned-rpc"} {
		t.Run(mutation, func(t *testing.T) {
			root, scratch := apiFixture(t)
			path := filepath.Join(root, apiOutput, "pppoe.ba.go")
			original := apiGeneratedHeader + "\npackage pppoe\n// original\n"
			apiWriteFixture(t, path, original)
			process := &apiFeatureProcessFixture{mutation: mutation}
			if _, err := generateAPI(t.Context(), root, scratch, &apiDockerFixture{}, process, []string{"CGO_ENABLED=0"}); err == nil {
				t.Fatal("malformed Feature package accepted")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != original {
				t.Fatal("Feature refusal changed PPPoE output")
			}
			if _, err := os.Stat(filepath.Join(root, apiFeatureOutput, "feature.ba.go")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Feature refusal published output: %v", err)
			}
		})
	}
}

// TestVPPAPIFeatureGenerationRepeatsWithoutChangingFiles checks canonical
// regeneration is stable and preserves unrelated files in both packages.
func TestVPPAPIFeatureGenerationRepeatsWithoutChangingFiles(t *testing.T) {
	root, scratch := apiFixture(t)
	original := "package feature\n// user original\n"
	path := filepath.Join(root, apiFeatureOutput, "ownership_test.go")
	apiWriteFixture(t, path, original)
	report, err := generateAPI(t.Context(), root, scratch, &apiDockerFixture{}, &apiFeatureProcessFixture{}, []string{"CGO_ENABLED=0"})
	if err != nil {
		t.Fatal(err)
	}
	first := make(map[string]string)
	for _, name := range report.Files {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		first[name] = string(data)
	}
	if _, err := generateAPI(t.Context(), root, scratch, &apiDockerFixture{}, &apiFeatureProcessFixture{}, []string{"CGO_ENABLED=0"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range first {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("regeneration changed %s", name)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatal("regeneration changed unrelated Feature source")
	}
}

type apiFeatureProcessFixture struct {
	apiProcessFixture
	mutation      string
	sourceVersion string
}

func (p *apiFeatureProcessFixture) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	result, err := p.apiProcessFixture.Run(ctx, command)
	if err != nil {
		return result, err
	}
	if !slices.Contains(command.Arguments, "feature") {
		return result, nil
	}
	for _, argument := range command.Arguments {
		if input, ok := strings.CutPrefix(argument, "--input="); ok {
			data, err := os.ReadFile(filepath.Join(input, "VPP_VERSION"))
			if err != nil {
				return result, err
			}
			p.sourceVersion = string(data)
		}
		outputDir, ok := strings.CutPrefix(argument, "--output-dir=")
		if !ok {
			continue
		}
		output := filepath.Join(outputDir, "feature")
		if err := os.MkdirAll(output, 0o750); err != nil {
			return result, err
		}
		for _, name := range []string{"feature.ba.go", "feature_rpc.ba.go"} {
			if name == "feature_rpc.ba.go" {
				if p.mutation == "missing-rpc" {
					if err := os.Remove(filepath.Join(output, name)); err != nil {
						return result, err
					}
					continue
				}
			}
			data := apiFeatureFixture()
			if p.mutation == "wrong-field" {
				data = strings.ReplaceAll(data, "IsEnabled bool", "Missing bool")
			}
			if p.mutation == "wrong-type" {
				data = strings.ReplaceAll(data, "Enable bool", "Enable uint8")
			}
			if p.mutation == "wrong-package" {
				data = strings.ReplaceAll(data, "package feature", "package foreign")
			}
			if name == "feature_rpc.ba.go" {
				if p.mutation == "unowned-rpc" {
					data = strings.TrimPrefix(data, apiGeneratedHeader+"\n")
				}
			}
			if err := os.WriteFile(filepath.Join(output, name), []byte(data), 0o600); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// TestVPPAPIGenerationPublishesMemclntBindings checks the dump completion pair
// of Ze's binapi tree is generated and published with the other packages, and
// that a memclnt output without the pair is refused before publication.
func TestVPPAPIGenerationPublishesMemclntBindings(t *testing.T) {
	root, scratch := apiFixture(t)
	process := &apiFeatureProcessFixture{}
	report, err := generateAPI(t.Context(), root, scratch, &apiDockerFixture{}, process, []string{"CGO_ENABLED=0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(process.commands) == 0 {
		t.Fatal("the generator never ran")
	}
	for _, command := range process.commands {
		if !slices.Contains(command.Arguments, "memclnt") {
			t.Fatal("memclnt API was not selected for generation")
		}
	}
	for _, name := range []string{"memclnt.ba.go", "memclnt_rpc.ba.go"} {
		path := filepath.ToSlash(filepath.Join(apiOutputRoot, "memclnt", name))
		if !slices.Contains(report.Files, path) {
			t.Fatalf("memclnt output absent from report: %s", path)
		}
	}
	for name, data := range map[string]string{
		"no-ping":   strings.ReplaceAll(apiMemclntFixture(), "type ControlPing struct{}", ""),
		"no-retval": strings.ReplaceAll(apiMemclntFixture(), "Retval int32", "Missing int32"),
	} {
		parsed, err := parser.ParseFile(token.NewFileSet(), "memclnt.ba.go", data, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := apiCheckPackageSchema("memclnt", parsed); err == nil {
			t.Fatalf("%s: incomplete memclnt schema accepted", name)
		}
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "other.ba.go", "package other\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := apiCheckPackageSchema("other", parsed); err == nil {
		t.Fatal("a package with no schema check was accepted")
	}
}

func apiMemclntFixture() string {
	return apiGeneratedHeader + "\npackage memclnt\n" +
		"type ControlPing struct{}\n" +
		"type ControlPingReply struct { Retval int32; ClientIndex uint32; VpePID uint32 }\n"
}

func apiFeatureFixture() string {
	return apiGeneratedHeader + "\npackage feature\n" +
		"import interface_types \"go.fd.io/govpp/binapi/interface_types\"\n" +
		"type FeatureEnableDisable struct { SwIfIndex interface_types.InterfaceIndex; Enable bool; ArcName string; FeatureName string }\n" +
		"type FeatureEnableDisableReply struct { Retval int32 }\n" +
		"type FeatureIsEnabled struct { SwIfIndex interface_types.InterfaceIndex; ArcName string; FeatureName string }\n" +
		"type FeatureIsEnabledReply struct { Retval int32; IsEnabled bool }\n"
}
