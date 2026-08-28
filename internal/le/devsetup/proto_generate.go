// Design: api/proto/ze.proto -- the native protobuf generation path.
package devsetup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	protoSourceRel    = "api/proto/ze.proto"
	protoGeneratedRel = "api/proto/ze.pb.go"
	grpcGeneratedRel  = "api/proto/ze_grpc.pb.go"
)

// protoGenerateReport is the generated protobuf file set.
type protoGenerateReport struct {
	Files []string `json:"files"`
}

// protoGenerator builds the pinned protoc plugins, runs protoc, and applies the
// explicit json_name options to the generated Go struct tags.
type protoGenerator struct {
	Root  string
	Shell *Shell
}

// run regenerates both checked-in protobuf Go files without an interpreter.
func (generator *protoGenerator) run() (protoGenerateReport, error) {
	if generator.Root == "" {
		return protoGenerateReport{}, errors.New("protobuf generation needs a repository root")
	}
	shell := generator.Shell
	if shell == nil {
		shell = &Shell{}
	}
	if !shell.Present("protoc") {
		return protoGenerateReport{}, errors.New("protoc not found; install protobuf with ./le setup tools")
	}
	module, err := lepath.Module(generator.Root)
	if err != nil {
		return protoGenerateReport{}, err
	}
	binDir := filepath.Join(generator.Root, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return protoGenerateReport{}, fmt.Errorf("create protobuf tool directory: %w", err)
	}
	environment := replaceEnvironment(os.Environ(), map[string]string{"CGO_ENABLED": "0"})
	for _, tool := range []struct {
		output      string
		packagePath string
	}{
		{filepath.Join(binDir, "protoc-gen-go"), "google.golang.org/protobuf/cmd/protoc-gen-go"},
		{filepath.Join(binDir, "protoc-gen-go-grpc"), "google.golang.org/grpc/cmd/protoc-gen-go-grpc"},
	} {
		result := shell.Run(Cmd{
			Argv: []string{"go", "build", "-mod=vendor", "-o", tool.output, tool.packagePath},
			Dir:  generator.Root,
			Env:  environment,
		})
		if !result.OK() {
			return protoGenerateReport{}, fmt.Errorf("build %s: %s", filepath.Base(tool.output), result.complaint())
		}
	}
	protocEnvironment := replaceEnvironment(environment, map[string]string{
		"PATH": binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	result := shell.Run(Cmd{
		Argv: []string{
			"protoc",
			"--go_out=.", "--go_opt=module=" + module,
			"--go-grpc_out=.", "--go-grpc_opt=module=" + module,
			protoSourceRel,
		},
		Dir: generator.Root,
		Env: protocEnvironment,
	})
	if !result.OK() {
		return protoGenerateReport{}, fmt.Errorf("generate protobuf Go: %s", result.complaint())
	}
	rewriter := &protoJSONTags{
		ProtoPath:     filepath.Join(generator.Root, filepath.FromSlash(protoSourceRel)),
		GeneratedPath: filepath.Join(generator.Root, filepath.FromSlash(protoGeneratedRel)),
	}
	if _, err := rewriter.run(); err != nil {
		return protoGenerateReport{}, err
	}
	return protoGenerateReport{Files: []string{protoGeneratedRel, grpcGeneratedRel}}, nil
}

func runProtoGenerate() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := (&protoGenerator{Root: root}).run()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return report, 0
}
