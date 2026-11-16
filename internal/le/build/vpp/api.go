// Design: third_party/vpp-patches/README.md -- generation from the patched API.
// Related: api_recipe.go -- pinned source inputs and canonical output ownership.
package buildvpp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	apiOutputRoot = "internal/component/l2tp/pppoe/dataplane/vpp/binapi"
	apiOutput     = apiOutputRoot + "/pppoe"
	apiGenerator  = "go.fd.io/govpp/cmd/binapi-generator@v0.13.0"
	apiImageName  = "ze-vpp-pppoe-api"
)

// apiPackageNames is the bounded publication set. JSON dependencies are read
// separately; they remain GoVPP imports rather than additional Ze outputs.
func apiPackageNames() []string { return []string{"pppoe", "feature"} }

// APIReport identifies the generator, source and published files. Generation
// proves the API shape; it proves no compatibility with a running VPP process.
type APIReport struct {
	Image        interoplab.ImageResult `json:"image"`
	Generator    string                 `json:"generator"`
	SourceCommit string                 `json:"source-commit"`
	SourceSHA256 string                 `json:"source-sha256"`
	Files        []string               `json:"files"`
}

type apiDocker interface {
	imageBuilder
	RunOneShot(context.Context, interoplab.OneShotContainer) (interoplab.CommandResult, error)
}

// generateAPI is synchronous and owns only its MkdirTemp directory, image
// retention lease and four canonical generated files. Callers MUST serialize
// writes to the generated packages through the build action's job admission.
func generateAPI(ctx context.Context, root, scratch string, docker apiDocker,
	process interoplab.ProcessRunner, environ []string) (report APIReport, retErr error) {
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := apiCheckOutput(root); err != nil {
		return report, err
	}
	pins, err := apiSourcePins(root)
	if err != nil {
		return report, err
	}
	work, err := os.MkdirTemp(scratch, "vpp-api-")
	if err != nil {
		return report, err
	}
	defer func() { retErr = errors.Join(retErr, os.RemoveAll(work)) }()
	build, err := apiBuildRecipe(root, work, pins)
	if err != nil {
		return report, err
	}
	image, err := docker.Build(ctx, build)
	if err != nil {
		return report, err
	}
	defer func() {
		retErr = errors.Join(retErr, docker.ReleaseImage(context.WithoutCancel(ctx), image))
	}()
	input := filepath.Join(work, "api")
	if err := os.Mkdir(input, 0o750); err != nil {
		return report, err
	}
	// RPC generation resolves control_ping and its reply from memclnt even
	// when only PPPoE and Feature are selected for publication.
	for _, name := range append(apiPackageNames(), "interface_types", "ethernet_types", "ip_types", "memclnt") {
		if err := apiReadJSON(ctx, docker, image.Reference, input, name); err != nil {
			return report, err
		}
	}
	// GoVPP otherwise probes the enclosing Ze repository or VPP_VERSION from
	// the operator's environment. This source identity is independent of both.
	if err := os.WriteFile(filepath.Join(input, "VPP_VERSION"), []byte(pins.commit+"-pppoe-patched\n"), 0o600); err != nil {
		return report, err
	}
	output := filepath.Join(work, "generated")
	command := interoplab.ProcessCommand{
		Arguments: []string{"go", "run", "-mod=mod", apiGenerator,
			"--input=" + input, "--output-dir=" + output,
			"--import-prefix=go.fd.io/govpp/binapi", "--no-source-path-info"},
		Directory: root, Environment: environ, Timeout: 10 * time.Minute,
		Stdout: os.Stderr, Stderr: os.Stderr,
	}
	command.Arguments = append(command.Arguments, apiPackageNames()...)
	result, err := process.Run(ctx, command)
	if err != nil {
		return report, err
	}
	if result.ExitCode != 0 {
		return report, fmt.Errorf("the GoVPP generator exited %d", result.ExitCode)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	files, err := apiPublish(root, output)
	if err != nil {
		return report, err
	}
	return APIReport{Image: image, Generator: apiGenerator, SourceCommit: pins.commit,
		SourceSHA256: pins.sha256, Files: files}, nil
}

func apiReadJSON(ctx context.Context, docker apiDocker, image, input, name string) (retErr error) {
	file, err := os.OpenFile(filepath.Join(input, name+".api.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	_, err = docker.RunOneShot(ctx, interoplab.OneShotContainer{
		Image: image, Network: "none", Timeout: time.Minute,
		Command: []string{"cat", "/api/" + name + ".api.json"}, Stdout: file, Stderr: os.Stderr,
	})
	return err
}
