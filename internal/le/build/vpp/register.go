// Design: docs/labs/pppoe-interop.md -- the VPP lab build is a registered action.
// Related: build.go -- the image identity and build lifecycle.
package buildvpp

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const (
	area           = "build vpp"
	variantKeyword = "variant"
)

// newImageBuilder keeps the action's real signal root in entrypoint tests.
// Tests replace it only inside their own subprocess, never the shared runner.
var newImageBuilder = func() imageBuilder { return interoplab.NewBuildToolDocker() }

var actions = leaction.New(area, leaction.Action{
	Verb: "run", Heavy: true, Writes: true,
	Summary: "build the patched VPP PPPoE lab image",
	Why:     "Build the pinned VPP source with retained PPPoE patches for the lab. The image includes af_packet, Linux CP and PPPoE, and is not an appliance or DPDK package.",
	Parameters: []leaction.Parameter{{Keyword: variantKeyword, Kind: leaction.KindEnum,
		Value: "name", Values: []string{baselineVariantName, metadataVariantName},
		ValueHelp: map[string]string{
			baselineVariantName: "build the normal patched lab image",
			metadataVariantName: "omit parsed metadata to test the runtime oracle",
		},
		Help: "select a proof image (default: baseline)", Requirement: leaction.Optional}},
	AnswerArgs: answerRun,
}, leaction.Action{
	Verb: "deployment", Does: "build", Heavy: true, Writes: true,
	Summary: "build the patched VPP image the deployment run boots",
	Why:     "Build VPP v26.06 from source with Ze's IPsec patch, which judges unprotected inbound traffic against the SPD. `./le test deployment vpp` boots this image from the image cache and never builds it.",
	Answer:  answerDeployment,
}, leaction.Action{
	Verb: "update", Heavy: true, Writes: true,
	Summary: "generate the patched VPP PPPoE Go bindings",
	Why:     "Generate the PPPoE binary API from the pinned patched VPP source without compiling VPP. Only the six canonical generated PPPoE, Feature and memclnt files are published.",
	Answer:  answerGenerate,
})

// imageOwner names this package as the owner of the images it registers.
const imageOwner = "build-vpp"

func init() {
	interoplab.RegisterImage(interoplab.RegisteredImage{
		Name: ImageName, Owner: imageOwner, Build: imageRecipe,
	})
	interoplab.RegisterImage(interoplab.RegisteredImage{
		Name: MetadataImageName, Owner: imageOwner, Build: metadataImageRecipe,
	})
	interoplab.RegisterImage(interoplab.RegisteredImage{
		Name: DeploymentImageName, Owner: imageOwner, Build: deploymentImageRecipe,
	})
	leroot.Register(area, leroot.GroupWorkflow, Answer, registry.Meta{
		ShortHelp:   "build the patched VPP lab images",
		Description: "Build the source-pinned VPP images used by the PPPoE mechanism proofs and the real-VPP deployment run.",
		Mode:        "offline", Section: registry.SectionTest, SubsFunc: actions.Subs,
	})
	leroot.RegisterActions(area, actions.Actions)
	leroot.RegisterShape(area, command.ShapeMap)
}

// Answer dispatches the declared build grammar. It is safe for concurrent use;
// the heavy action takes the checkout's shared job admission slot.
func Answer(args []string) (any, int) {
	return actions.Answer(args)
}

func answerRun(args leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	variant, err := parseProofVariant(args.One(variantKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	retryCommand := "./le build vpp run"
	if args.Has(variantKeyword) {
		retryCommand += " variant " + args.One(variantKeyword)
	}

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the VPP lab build cannot find the Ze checkout",
			"The source and patch recipe cannot be located. Change to the checkout containing go.mod and run the build there.",
			retryCommand))
		return nil, 1
	}
	report, err := buildImage(ctx, root, variant, newImageBuilder())
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "the VPP PPPoE lab image could not be prepared",
			"The mechanism proof cannot use an unbuilt image. Check the Docker build output and repair the reported source, patch or Docker failure before retrying.",
			retryCommand))
		return nil, 1
	}
	return report, 0
}
