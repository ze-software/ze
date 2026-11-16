// Design: docs/labs/pppoe-interop.md -- binding regression and named parser discrimination.
// Related: plan.go -- the full positive mechanism; ../../../build/vpp/build.go -- retained images.
package testvpppppoe

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const (
	pppoeProofCommand    = "pppoe"
	metadataProofVariant = "missing-frame-metadata"
	metadataProofOracle  = "empty PADT: wrong session ID 0, expected 17"
	parserProofLifetime  = 90 * time.Second
)

// proofArtifact pairs each proof's variant with the immutable producer and
// its complete result. No failure of the baseline counts as discrimination.
type proofArtifact struct {
	Variant string                 `json:"variant"`
	Image   string                 `json:"image,omitempty"`
	Result  interoplab.SuiteReport `json:"result"`
}

type proofReport struct {
	Baseline proofArtifact `json:"baseline"`
	Bindings proofArtifact `json:"bindings"`
	Metadata proofArtifact `json:"metadata"`
	Code     int           `json:"code"`
}

func (report *proofReport) Text() string {
	return report.Baseline.text(baselineProofVariant) + "\n" + report.Bindings.text("binding regression") + "\n" + report.Metadata.text("metadata counterfactual")
}

func (artifact proofArtifact) text(name string) string {
	variant := artifact.Variant
	if variant == "" {
		variant = "unspecified"
	}
	image := artifact.Image
	if image == "" {
		if len(artifact.Result.NotRun) > 0 {
			image = "not run"
		} else {
			image = "unavailable"
		}
	}
	return fmt.Sprintf("VPP PPPoE %s:\n  variant: %s\n  image: %s\n%s", name, variant, image, artifact.Result.Text())
}

type proofStages struct {
	baseline func(context.Context, string) interoplab.SuiteReport
	bindings func(context.Context, string, string) interoplab.SuiteReport
	metadata func(context.Context, string) interoplab.SuiteReport
}

// runProof retains one baseline build across all three stages on the same
// daemon. The binding regression uses that image after the full wire-first
// proof; the late metadata mutation layer can reuse its source compilation.
func runProof(ctx context.Context, root string) proofReport {
	docker := interoplab.NewDocker()
	return runProofWithBuilder(ctx, root, docker, func(ctx context.Context) error {
		return prepareMechanismHost(ctx, root, docker)
	}, func(image string) proofStages {
		return proofStages{
			baseline: func(ctx context.Context, root string) interoplab.SuiteReport { return Run(ctx, root, image) },
			bindings: runBindingProofImage,
			metadata: runMetadataProof,
		}
	})
}

// proofImageBuilder exposes the owned image lease to the lifecycle regression.
// A successful Build MUST be followed by exactly one ReleaseImage after all
// dependent stages have finished removing their peers.
type proofImageBuilder interface {
	Build(context.Context, interoplab.ImageBuild) (interoplab.ImageResult, error)
	ReleaseImage(context.Context, interoplab.ImageResult) error
}

// runProofWithBuilder owns the baseline lease instead of letting the baseline
// Suite release it before the dependent binding peer starts. Stages receive
// only its immutable reference and MUST NOT acquire or release a second lease.
func runProofWithBuilder(ctx context.Context, root string, builder proofImageBuilder, prepare func(context.Context) error, stages func(string) proofStages) (report proofReport) {
	report = newProofReport()
	if err := prepare(ctx); err != nil {
		report.Baseline.Result = setupFailure(err)
		return report
	}
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, []string{buildvpp.ImageName})
	if err != nil {
		report.Baseline.Result = setupFailure(err)
		return report
	}
	if len(builds) != 1 {
		report.Baseline.Result = setupFailure(errors.New("the combined proof did not resolve exactly one baseline image"))
		return report
	}
	builds[0].Progress = os.Stderr
	image, err := builder.Build(ctx, builds[0])
	if err != nil {
		report.Baseline.Result = setupFailure(err)
		return report
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), controlCommandBudget)
		defer cancel()
		if err := builder.ReleaseImage(cleanupCtx, image); err != nil {
			report.Baseline.Result.CleanupErrors = append(report.Baseline.Result.CleanupErrors, err.Error())
			report.Baseline.Result.Code = 1
			report.Code = 1
		}
	}()
	if !immutableImage(image.Reference) {
		report.Baseline.Result = setupFailure(errors.New("the combined proof build returned no immutable baseline image identity"))
		return report
	}
	return runProofStages(ctx, root, stages(image.Reference))
}

// prepareMechanismHost owns only the fresh binaries used for the first host
// check. Run stages its own current-source inputs after the held baseline build.
func prepareMechanismHost(ctx context.Context, root string, docker *interoplab.Docker) (err error) {
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Join(root, paths.Scratch), "vpp-pppoe-host-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(work); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove host preflight scratch: %w", cleanupErr))
		}
	}()
	relative, err := filepath.Rel(root, work)
	if err != nil {
		return err
	}
	binaries := []interoplab.LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: filepath.Join(relative, "ze")},
		{Name: "le", Base: repofeaturetags.LEBase, Output: filepath.Join(relative, "le"), StartProbe: []string{"help"}},
	}
	return prepareVPPHost(ctx, docker, interoplab.StageBinaries(root, false, binaries...), interoplab.StagedZePath(root, binaries))
}

// prepareVPPHost preserves Suite's fail-fast order for images built before the
// Suite starts. Its stage MUST freshly prepare the Ze named by stagedZe.
func prepareVPPHost(ctx context.Context, docker *interoplab.Docker, stage interoplab.PreflightCheck, stagedZe string) error {
	if err := docker.Probe(ctx); err != nil {
		return err
	}
	return interoplab.Preflights(stage, interoplab.DockerKernel(stagedZe))(ctx, docker)
}

func runProofStages(ctx context.Context, root string, stages proofStages) (report proofReport) {
	report = newProofReport()
	report.Baseline.Result = stages.baseline(ctx, root)
	if report.Baseline.Result.Code != 0 {
		return report
	}
	image, err := proofImageReference(report.Baseline.Result, buildvpp.ImageName)
	if err != nil {
		report.Baseline.Result = setupFailure(err)
		return report
	}
	report.Baseline.Image = image
	report.Bindings.Image = image
	report.Bindings.Result = stages.bindings(ctx, root, image)
	if report.Bindings.Result.Code != 0 {
		return report
	}
	report.Metadata.Result = stages.metadata(ctx, root)
	if report.Metadata.Result.Code != 0 {
		return report
	}
	image, err = proofImageReference(report.Metadata.Result, buildvpp.MetadataImageName)
	if err != nil {
		report.Metadata.Result = setupFailure(err)
		return report
	}
	report.Metadata.Image = image
	report.Code = 0
	return report
}

func newProofReport() proofReport {
	return proofReport{
		Code:     1,
		Baseline: proofArtifact{Variant: baselineProofVariant},
		Bindings: proofArtifact{Variant: baselineProofVariant, Result: interoplab.SuiteReport{NotRun: []string{"pppoe-vpp-bindings-oracle"}}},
		Metadata: proofArtifact{Variant: metadataProofVariant, Result: interoplab.SuiteReport{NotRun: []string{metadataProofVariant}}},
	}
}

func proofImageReference(report interoplab.SuiteReport, name string) (string, error) {
	for _, image := range report.Images {
		if image.Name != name {
			continue
		}
		if image.Error != "" {
			return "", errors.New(image.Error)
		}
		if !immutableImage(image.Reference) {
			return "", fmt.Errorf("proof image %s has no immutable identity: %s", name, image.Reference)
		}
		return image.Reference, nil
	}
	return "", fmt.Errorf("proof result has no image %s", name)
}

func runMetadataProof(ctx context.Context, root string) interoplab.SuiteReport {
	return runSingleVPPProof(ctx, root, singleVPPProof{name: "metadata", imageName: buildvpp.MetadataImageName, plan: metadataProofPlan})
}

func runBindingProof(ctx context.Context, root string) interoplab.SuiteReport {
	return runBindingProofImage(ctx, root, "")
}

func runBindingProofImage(ctx context.Context, root, image string) interoplab.SuiteReport {
	return runSingleVPPProof(ctx, root, singleVPPProof{name: "bindings", imageName: buildvpp.ImageName, reference: image, plan: bindingProofPlan})
}

type singleVPPProof struct {
	name, imageName string
	// Empty reference requests a build and its unique lease; a supplied
	// immutable reference is reused without acquiring or releasing that lease.
	reference string
	plan      func(string, string) interoplab.ScenarioPlan
}

// runSingleVPPProof owns its scratch and any image lease it builds. The Suite
// MUST finish removing its peer before image release or scratch removal.
// A freshly staged Ze retains Suite's check of the Docker host's runtime kernel.
func runSingleVPPProof(ctx context.Context, root string, proof singleVPPProof) (report interoplab.SuiteReport) {
	if proof.reference != "" {
		if !immutableImage(proof.reference) {
			return setupFailure(errors.New("the isolated proof requires a complete immutable sha256 image identity"))
		}
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		return setupFailure(err)
	}
	work, err := os.MkdirTemp(filepath.Join(root, paths.Scratch), "vpp-pppoe-"+proof.name+"-")
	if err != nil {
		return setupFailure(err)
	}
	defer func() {
		if err := os.RemoveAll(work); err != nil {
			report.CleanupErrors = append(report.CleanupErrors, fmt.Errorf("remove %s proof scratch: %w", proof.name, err).Error())
			report.Code = 1
		}
	}()
	if err := os.WriteFile(filepath.Join(work, "startup.conf"), []byte(startupConfig), 0o600); err != nil {
		return setupFailure(err)
	}
	builds, _, err := interoplab.ResolveImages(interoplab.ImageContext{Root: root}, []string{proof.imageName})
	if err != nil {
		return setupFailure(err)
	}
	if len(builds) != 1 {
		return setupFailure(fmt.Errorf("the %s proof did not resolve exactly one image", proof.name))
	}
	docker := interoplab.NewDocker()
	relative, err := filepath.Rel(root, work)
	if err != nil {
		return setupFailure(err)
	}
	binaries := []interoplab.LabBinary{{Name: "ze", Base: repofeaturetags.DaemonBase, Output: filepath.Join(relative, "ze")}}
	if err := prepareVPPHost(ctx, docker, interoplab.StageBinaries(root, false, binaries...), interoplab.StagedZePath(root, binaries)); err != nil {
		return setupFailure(err)
	}
	builds[0].Progress = os.Stderr
	image := interoplab.ImageResult{Name: proof.imageName, Reference: proof.reference}
	if proof.reference == "" {
		image, err = docker.Build(ctx, builds[0])
		if err != nil {
			return setupFailure(err)
		}
		// ReleaseImage MUST release this successful Build once. Suite's NoBuild
		// declaration carries only the immutable reference and owns no build lease.
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), controlCommandBudget)
			defer cancel()
			if err := docker.ReleaseImage(cleanupCtx, image); err != nil {
				report.CleanupErrors = append(report.CleanupErrors, err.Error())
				report.Code = 1
			}
		}()
	}
	if !immutableImage(image.Reference) {
		return setupFailure(fmt.Errorf("the %s build returned no immutable image identity", proof.name))
	}
	builds[0].Tag = image.Reference
	parserCtx, cancel := context.WithTimeout(ctx, parserProofLifetime)
	defer cancel()
	suite := interoplab.Suite{
		Docker: docker, Checkout: root, NoBuild: true,
		StagedZe: interoplab.StagedZePath(root, binaries), Images: builds,
		Scenarios: []interoplab.ScenarioPlan{proof.plan(work, strconv.Itoa(os.Getpid()))},
	}
	report = suite.Run(parserCtx)
	report.Images = []interoplab.ImageResult{image}
	return report
}

func metadataProofPlan(work, suffix string) interoplab.ScenarioPlan {
	return singleVPPProofPlan(work, suffix, "metadata", buildvpp.MetadataImageName, checkMetadataProof)
}

func bindingProofPlan(work, suffix string) interoplab.ScenarioPlan {
	return singleVPPProofPlan(work, suffix, "bindings", buildvpp.ImageName, checkBindingProof)
}

func singleVPPProofPlan(work, suffix, name, imageName string, checker interoplab.Checker) interoplab.ScenarioPlan {
	return interoplab.ScenarioPlan{
		Source: interoplab.ScenarioSource{Name: "pppoe-vpp-" + name + "-oracle", Directory: work, Checker: checker},
		Network: interoplab.NetworkSpec{Name: "ze-vpp-pppoe-" + name + "-net-" + suffix, Candidates: []interoplab.Subnet{
			{IPv4: netip.MustParsePrefix("198.19.35.0/24")},
			{IPv4: netip.MustParsePrefix("198.19.36.0/24")},
		}},
		Peers: []interoplab.PeerConfig{{
			Name: vppPeer, Container: "ze-vpp-pppoe-" + name + "-" + suffix, Image: imageName, Host: 2,
			Mounts: []interoplab.Mount{
				{Source: work, Target: evidencePath},
				{Source: filepath.Join(work, "startup.conf"), Target: "/run/vpp/startup.conf", ReadOnly: true},
			},
			Command:  []string{"vpp", "-c", "/run/vpp/startup.conf"},
			LogFiles: []string{"/evidence/vpp.log"},
			Ready: &interoplab.ReadyProbe{
				Command:  []string{vppctlCommand, "-s", vppCLIPath, "show", "version"},
				Contains: []string{"vpp"}, Timeout: controlCommandBudget, Interval: 250 * time.Millisecond,
			},
		}},
	}
}

func checkMetadataProof(ctx context.Context, check *interoplab.CheckContext) error {
	if check == nil {
		return errors.New("metadata proof has no checker context")
	}
	if check.Lab == nil {
		return errors.New("metadata proof has no VPP peer")
	}
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	variant, err := query(commandCtx, check.Lab, vppPeer, readFileCommand, "/usr/share/doc/vpp/proof-variant")
	if err != nil {
		return err
	}
	if strings.TrimSpace(variant) != metadataProofVariant {
		return fmt.Errorf("metadata proof image declares variant %q, want %s", strings.TrimSpace(variant), metadataProofVariant)
	}
	return checkMetadataOracle(ctx, check.Lab)
}

// Pinned vppctl main returns zero after CLI error output; only transport and
// local syscall errors change its exit status. The C assertion text decides
// this outcome. A different error is never evidence that this mutation worked.
func checkMetadataOracle(ctx context.Context, lab interoplab.CheckerLab) error {
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	result, err := lab.Exec(commandCtx, vppPeer, []string{vppctlCommand, "-s", vppCLIPath, testCommand, pppoeProofCommand, "frame-bounds"}, nil)
	if err != nil {
		return fmt.Errorf("metadata oracle command did not complete: %w", err)
	}
	if strings.Contains(result.Stdout, "PPPoE frame bounds: 31 cases passed") {
		return errors.New("the missing-metadata parser passed the unchanged runtime test")
	}
	if strings.TrimSpace(result.Stdout) != "test pppoe frame-bounds: "+metadataProofOracle {
		return fmt.Errorf("metadata mutation did not reach its exact named assertion; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		return fmt.Errorf("metadata mutation reported another error; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	return nil
}

func checkBindingProof(ctx context.Context, check *interoplab.CheckContext) error {
	if check == nil {
		return errors.New("binding proof has no checker context")
	}
	if check.Lab == nil {
		return errors.New("binding proof has no VPP peer")
	}
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	variant, err := query(commandCtx, check.Lab, vppPeer, readFileCommand, "/usr/share/doc/vpp/proof-variant")
	if err != nil {
		return err
	}
	if strings.TrimSpace(variant) != baselineProofVariant {
		return fmt.Errorf("binding proof image declares variant %q, want baseline", strings.TrimSpace(variant))
	}
	if err := checkBindingOracle(ctx, check.Lab); err != nil {
		return err
	}
	return checkOwnershipOracle(ctx, check.Lab)
}

// Native ownership runs only after the binding producer passed in this same
// isolated baseline peer. A CLI assertion may still have a zero exit status.
func checkOwnershipOracle(ctx context.Context, lab interoplab.CheckerLab) error {
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	result, err := lab.Exec(commandCtx, vppPeer, []string{vppctlCommand, "-s", vppCLIPath, testCommand, pppoeProofCommand, "ownership"}, nil)
	if err != nil {
		return fmt.Errorf("ownership regression command did not complete: %w; stdout: %s; stderr: %s", err, result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) != "PPPoE ownership: 55 cases passed" {
		return fmt.Errorf("VPP ownership regression failed; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		return fmt.Errorf("VPP ownership regression reported an error; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	return nil
}

// A zero vppctl exit code also carries CLI assertion failures. Require the
// complete success answer and no stderr, while preserving both failure streams.
func checkBindingOracle(ctx context.Context, lab interoplab.CheckerLab) error {
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	result, err := lab.Exec(commandCtx, vppPeer, []string{vppctlCommand, "-s", vppCLIPath, testCommand, pppoeProofCommand, "cp-binding"}, nil)
	if err != nil {
		return fmt.Errorf("binding regression command did not complete: %w; stdout: %s; stderr: %s", err, result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) != "PPPoE cp binding: 10 cases passed" {
		return fmt.Errorf("VPP control-binding regression failed; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		return fmt.Errorf("VPP control-binding regression reported an error; stdout: %s; stderr: %s", result.Stdout, result.Stderr)
	}
	return nil
}
