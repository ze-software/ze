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
	// apiGoVPPImports is GoVPP's binapi tree. Its shared types packages
	// (interface_types, ethernet_types, ip_types) are the ones Ze's messages
	// use, so a PPPoE message and GoVPP's own interface or ip messages carry
	// one InterfaceIndex type.
	apiGoVPPImports = "go.fd.io/govpp/binapi"
	// apiZeImports is Ze's binapi tree: the import path of apiOutputRoot.
	apiZeImports = "github.com/ze-software/ze/" + apiOutputRoot
)

// apiPackageNames is the bounded publication set: Ze's binapi tree. JSON
// dependencies are read separately; they remain GoVPP imports rather than
// additional Ze outputs. memclnt is published because GoVPP's Stream.RecvMsg
// resolves a reply only in the binapi tree of the last message sent
// (Connection.GetMessagePath, LookupByID): a dump of a PPPoE request MUST end
// with the control ping of Ze's own tree, never GoVPP's, or its details
// replies are unknown. That holds for the hand-written dump in the PPPoE
// backend and for every dump helper the generator writes (apiPasses).
func apiPackageNames() []string { return []string{"pppoe", "feature", "memclnt"} }

// apiPass is one generator run and the file of every package it publishes.
// GoVPP v0.13.0 resolves every binapi import of a run under its one
// --import-prefix (binapigen.New: path.Join(ImportPrefix, baseName)), and no
// single prefix is right for both files of a package. A message file imports
// the shared types packages, which stay GoVPP's (apiGoVPPImports). A service
// file imports memclnt alone, for the control ping that ends each dump helper,
// and that ping MUST be Ze's (apiPackageNames). So the unmodified generator
// runs once per prefix and each file is published from the run whose prefix
// is right for it; apiCheckImports refuses a file that imports either tree's
// package from the other, so the split cannot publish a wrong import.
type apiPass struct {
	name         string // the run's output directory in the generation scratch
	suffix       string // the file of each package this run publishes
	importPrefix string
}

func apiPasses() []apiPass {
	return []apiPass{
		{name: "messages", suffix: ".ba.go", importPrefix: apiGoVPPImports},
		{name: "services", suffix: "_rpc.ba.go", importPrefix: apiZeImports},
	}
}

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
// retention lease and six canonical generated files. Callers MUST serialize
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
	for _, name := range append(apiPackageNames(), "interface_types", "ethernet_types", "ip_types") {
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
	for _, pass := range apiPasses() {
		if err := apiRunGenerator(ctx, process, root, input, filepath.Join(output, pass.name), pass, environ); err != nil {
			return report, err
		}
	}
	files, err := apiPublish(root, output)
	if err != nil {
		return report, err
	}
	return APIReport{Image: image, Generator: apiGenerator, SourceCommit: pins.commit,
		SourceSHA256: pins.sha256, Files: files}, nil
}

// apiRunGenerator runs the pinned generator once, for one pass, over the whole
// publication set, so both runs read the same input and emit the same messages.
func apiRunGenerator(ctx context.Context, process interoplab.ProcessRunner,
	root, input, output string, pass apiPass, environ []string) error {
	command := interoplab.ProcessCommand{
		Arguments: []string{"go", "run", "-mod=mod", apiGenerator,
			"--input=" + input, "--output-dir=" + output,
			"--import-prefix=" + pass.importPrefix, "--no-source-path-info"},
		Directory: root, Environment: environ, Timeout: 10 * time.Minute,
		Stdout: os.Stderr, Stderr: os.Stderr,
	}
	command.Arguments = append(command.Arguments, apiPackageNames()...)
	result, err := process.Run(ctx, command)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("the GoVPP generator exited %d while generating the %s files", result.ExitCode, pass.name)
	}
	return ctx.Err()
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
