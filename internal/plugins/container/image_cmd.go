// Design: docs/architecture/container/image-store.md -- the image commands: their paths, proxies and handlers
// Related: storeseam_operations.go -- the operations each handler calls
// Related: container.go -- handleCommand, which looks the handlers up
// Related: register.go -- where the proxies are registered

package container

import (
	"context"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The image commands: the plugin command each declares, and the wire method
// its YANG node binds (yang/ze-container-cmd.yang). As for the restart, a
// plugin command stops before the keyword of its first leaf, and the YANG
// command node is that keyword's node. The two strings of each pair are the
// contract between the schema, the proxy registration and handleCommand.
const (
	cmdShowImageList        = "show container image list"
	cmdShowStore            = "show container store"
	cmdShowVolumeList       = "show container volume list"
	cmdCreateImage          = "create container image"
	cmdUpdateImage          = "update container image"
	cmdUpdateImageRollback  = "update container image rollback"
	cmdDeleteImage          = "delete container image"
	cmdDeleteImageAll       = "delete container image all"
	cmdDeleteVolume         = "delete container volume"
	cmdClearLayer           = "clear container layer"
	wireShowImageList       = "ze-container:show-image-list"
	wireShowStore           = "ze-container:show-store"
	wireShowVolumeList      = "ze-container:show-volume-list"
	wireCreateImage         = "ze-container:create-image"
	wireUpdateImage         = "ze-container:update-image"
	wireUpdateImageRollback = "ze-container:update-image-rollback"
	wireDeleteImage         = "ze-container:delete-image"
	wireDeleteImageAll      = "ze-container:delete-image-all"
	wireDeleteVolume        = "ze-container:delete-volume"
	wireClearLayer          = "ze-container:clear-layer"
	referenceKeyword        = "reference"
	digestKeyword           = "digest"
	instanceKeyword         = "instance"
	shapeDocument           = "doc"
	shapeTable              = "tab"
	keyImages               = "images"
	keyVolumes              = "volumes"
	fixCreateImage          = "ze create container image reference REFERENCE"
	fixUpdateImage          = "ze update container image name NAME"
	fixUpdateImageRollback  = "ze update container image rollback name NAME"
	fixDeleteImage          = "ze delete container image digest DIGEST"
	fixDeleteVolume         = "ze delete container volume name VOLUME instance NAME"
	fixClearLayer           = "ze clear container layer name NAME"
	fixShowImageList        = "ze show container image list"
)

// imageCommandDecls are the image commands this plugin answers (SDK Stage
// 1), in the order of the operator surface's command table. commandDecls
// declares them after the restart, so the registration and the engine entry
// declare the same list.
//
// The fetch and the update run for minutes, and a plugin command is bounded
// at 30 seconds unless it declares its own bound. The command declaration has
// no field for a bound yet: adding one changes the plugin protocol and waits
// for the owner, so TestFetchCommandDeclaresItsBound stays red until it lands.
func imageCommandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      cmdShowImageList,
			ShortHelp: "Show the images in the store, the fetches in flight and the fetches that failed.",
			// The rows are under "images" (handleShowImageList).
			Shape:   shapeTable,
			Columns: []string{"reference", "digest", "size", "fetched", keyState, "received", "used-by", "error", "code"},
		},
		{
			Name:      cmdShowStore,
			ShortHelp: "Show the image store's place, its space and its writable layers.",
			// One document; its layers are a row set inside it.
			Shape: shapeDocument,
		},
		{
			Name:      cmdShowVolumeList,
			ShortHelp: "Show the stored volumes, their size and their use.",
			// The rows are under "volumes" (handleShowVolumeList).
			Shape:   shapeTable,
			Columns: []string{"instance", keyName, "size", "used", keyState},
		},
		{
			Name:      cmdCreateImage,
			ShortHelp: "Fetch an image into the store and start the containers that wait for it.",
			Args:      []string{referenceKeyword, "<image>"},
			Shape:     shapeDocument,
		},
		{
			Name:      cmdUpdateImage,
			ShortHelp: "Fetch the named container's image again and move it to a new digest.",
			Args:      []string{selectorKeyword, containerPlaceholder},
			Shape:     shapeDocument,
		},
		{
			Name:      cmdUpdateImageRollback,
			ShortHelp: "Return the named container to its previous image and its writes.",
			Args:      []string{selectorKeyword, containerPlaceholder},
			Shape:     shapeDocument,
		},
		{
			Name:      cmdDeleteImage,
			ShortHelp: "Delete the stored image a digest names, unless a container holds it.",
			Args:      []string{digestKeyword, "<digest>"},
			Shape:     shapeDocument,
		},
		{
			Name:      cmdDeleteImageAll,
			ShortHelp: "Delete every stored image no configured container holds.",
			Shape:     shapeDocument,
		},
		{
			Name:      cmdDeleteVolume,
			ShortHelp: "Delete a stored volume the configuration no longer declares.",
			Args:      []string{selectorKeyword, "<volume>", instanceKeyword, containerPlaceholder},
			Shape:     shapeDocument,
		},
		{
			Name:      cmdClearLayer,
			ShortHelp: "Empty the named container's writable layer.",
			Args:      []string{selectorKeyword, containerPlaceholder},
			Shape:     shapeDocument,
		},
	}
}

// imageCommandRPCs are the proxies the YANG command nodes bind: each forwards
// its command to this plugin's process, internal or forked.
func imageCommandRPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: wireShowImageList, Handler: forwardToPlugin(cmdShowImageList), PluginCommand: cmdShowImageList},
		{WireMethod: wireShowStore, Handler: forwardToPlugin(cmdShowStore), PluginCommand: cmdShowStore},
		{WireMethod: wireShowVolumeList, Handler: forwardToPlugin(cmdShowVolumeList), PluginCommand: cmdShowVolumeList},
		{WireMethod: wireCreateImage, Handler: forwardToPlugin(cmdCreateImage), PluginCommand: cmdCreateImage},
		{WireMethod: wireUpdateImage, Handler: forwardToPlugin(cmdUpdateImage), PluginCommand: cmdUpdateImage},
		{WireMethod: wireUpdateImageRollback, Handler: forwardToPlugin(cmdUpdateImageRollback), PluginCommand: cmdUpdateImageRollback},
		{WireMethod: wireDeleteImage, Handler: forwardToPlugin(cmdDeleteImage), PluginCommand: cmdDeleteImage},
		{WireMethod: wireDeleteImageAll, Handler: forwardToPlugin(cmdDeleteImageAll), PluginCommand: cmdDeleteImageAll},
		{WireMethod: wireDeleteVolume, Handler: forwardToPlugin(cmdDeleteVolume), PluginCommand: cmdDeleteVolume},
		{WireMethod: wireClearLayer, Handler: forwardToPlugin(cmdClearLayer), PluginCommand: cmdClearLayer},
	}
}

// forwardToPlugin answers the proxy of pluginCommand.
//
// It MUST forward, never dispatch: dispatching the same command string would
// route it straight back to this handler and recurse.
func forwardToPlugin(pluginCommand string) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
		dispatcher := ctx.Dispatcher()
		if dispatcher == nil {
			return plugin.OperatorErrorResponse(opererr.New(
				pluginCommand+" was not run: the daemon's command dispatcher is not available",
				"the command is answered by the container plugin's process, and nothing can forward it there while the daemon starts or stops",
				"ze show doctor",
			)), nil
		}
		return dispatcher.ForwardToPlugin(ctx, pluginCommand, validated.Tokens(), ctx.PeerSelector())
	}
}

// imageCommandHandler answers one image command in the plugin's process.
type imageCommandHandler func(ctx context.Context, operations ImageOperations, args []string) (any, error)

// imageCommandHandlers maps each image command to its handler. handleCommand
// looks a command up here, so a declared command with no entry is refused as
// unknown, never answered.
var imageCommandHandlers = map[string]imageCommandHandler{
	cmdShowImageList:       handleShowImageList,
	cmdShowStore:           handleShowStore,
	cmdShowVolumeList:      handleShowVolumeList,
	cmdCreateImage:         handleCreateImage,
	cmdUpdateImage:         handleUpdateImage,
	cmdUpdateImageRollback: handleUpdateImageRollback,
	cmdDeleteImage:         handleDeleteImage,
	cmdDeleteImageAll:      handleDeleteImageAll,
	cmdDeleteVolume:        handleDeleteVolume,
	cmdClearLayer:          handleClearLayer,
}

// runImageCommand answers one image command: it finds the store's
// operations, refusing when the build has none, and runs the handler.
func runImageCommand(ctx context.Context, sup *supervisor, name string, handler imageCommandHandler, args []string) (string, any, error) {
	if sup.store == nil {
		return rpc.StatusError, nil, opererr.New(
			name+" was not run: this build has no image store",
			"no image store package registered its store, so no image is fetched, listed or removed",
			exampleShowDoctor,
		)
	}
	// A functional-test daemon that names the fixture store runs a store with
	// no image command (storeseam_zetest.go).
	operations, ok := sup.store.(ImageOperations)
	if !ok {
		return rpc.StatusError, nil, opererr.New(
			name+" was not run: the image store of this build answers no image command",
			"the store that runs the containers holds a fixed image and fetches nothing",
			exampleShowDoctor,
		)
	}
	answer, err := handler(ctx, operations, args)
	if err != nil {
		return rpc.StatusError, nil, err
	}
	return rpc.StatusDone, answer, nil
}

func handleShowImageList(ctx context.Context, operations ImageOperations, _ []string) (any, error) {
	rows, err := operations.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{keyImages: rows}, nil
}

func handleShowStore(ctx context.Context, operations ImageOperations, _ []string) (any, error) {
	return operations.StoreTotals(ctx)
}

func handleShowVolumeList(ctx context.Context, operations ImageOperations, _ []string) (any, error) {
	rows, err := operations.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{keyVolumes: rows}, nil
}

func handleCreateImage(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	reference := singleValue(args, referenceKeyword)
	if reference == "" {
		return nil, opererr.New(
			cmdCreateImage+" reference needs an image reference",
			"the command fetches one image, and none was named",
			fixCreateImage,
		)
	}
	return operations.FetchImage(ctx, reference)
}

func handleUpdateImage(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	instance := selectorValue(args)
	if instance == "" {
		return nil, opererr.New(
			cmdUpdateImage+" name needs the name of a container",
			"the command fetches the image of one container, and none was named",
			fixUpdateImage,
		)
	}
	return operations.UpdateImage(ctx, instance)
}

func handleUpdateImageRollback(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	instance := selectorValue(args)
	if instance == "" {
		return nil, opererr.New(
			cmdUpdateImageRollback+" name needs the name of a container",
			"the command returns one container to its previous image, and none was named",
			fixUpdateImageRollback,
		)
	}
	return operations.RollbackImage(ctx, instance)
}

func handleDeleteImage(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	digest := singleValue(args, digestKeyword)
	if digest == "" {
		return nil, opererr.New(
			cmdDeleteImage+" digest needs a digest",
			"the command deletes the one image a digest names, and none was given; the image listing shows each digest",
			fixShowImageList,
		)
	}
	return operations.DeleteImage(ctx, digest)
}

func handleDeleteImageAll(ctx context.Context, operations ImageOperations, _ []string) (any, error) {
	return operations.DeleteAllImages(ctx)
}

// handleDeleteVolume reads its two leaves by their keywords only: with two
// values, a bare word could be either.
func handleDeleteVolume(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	volume := keywordValue(args, selectorKeyword)
	if volume == "" {
		return nil, opererr.New(
			cmdDeleteVolume+" name needs the name of a volume",
			"the command deletes one volume of one container, and no volume was named",
			fixDeleteVolume,
		)
	}
	instance := keywordValue(args, instanceKeyword)
	if instance == "" {
		return nil, opererr.New(
			cmdDeleteVolume+" needs the container the volume belongs to, after the instance keyword",
			"two containers may each hold a volume of the same name",
			fixDeleteVolume,
		)
	}
	return operations.DeleteVolume(ctx, instance, volume)
}

func handleClearLayer(ctx context.Context, operations ImageOperations, args []string) (any, error) {
	instance := selectorValue(args)
	if instance == "" {
		return nil, opererr.New(
			cmdClearLayer+" name needs the name of a container",
			"the command empties the writable layer of one container, and none was named",
			fixClearLayer,
		)
	}
	return operations.ClearLayer(ctx, instance)
}

// keywordValue answers the value after keyword in args. It answers empty for
// a keyword that is absent or has nothing after it: that is an incomplete
// command, never a value spelled like the keyword.
func keywordValue(args []string, keyword string) string {
	for i, arg := range args {
		if arg != keyword {
			continue
		}
		if i+1 < len(args) {
			return args[i+1]
		}
		return ""
	}
	return ""
}

// singleValue answers the value of a command with one leaf: the word after
// keyword, or the one bare word a programmatic sender passes after it
// stripped the keyword. A keyword with nothing after it answers empty.
func singleValue(args []string, keyword string) string {
	if slices.Contains(args, keyword) {
		return keywordValue(args, keyword)
	}
	if len(args) != 1 {
		return ""
	}
	if strings.HasPrefix(args[0], "-") {
		return ""
	}
	return args[0]
}
