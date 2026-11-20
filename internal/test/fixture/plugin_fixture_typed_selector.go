// Design: docs/architecture/api/commands.md -- one declared command across real API boundaries.
// Related: plugin_fixture_typed_selector_observer.go -- authenticated REST observer.
// Related: ../externalyang/typed_selector.go -- the exact Stage-1 module owner.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/test/externalyang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const typedSelectorRecord = "fixture-original-7"
const typedSelectorID uint16 = 7
const typedSelectorRequestsMax = 32
const (
	typedSelectorLeafID       = "id"
	typedSelectorLeafListener = "listener"
	typedSelectorLeafEnabled  = "enabled"
	typedSelectorLeafForce    = "force"
)

func init() {
	Register("plugin/typed-selector-provider", typedSelectorProvider)
}

// typedSelectorProvider is a real forked SDK process. Declare mode starts no
// transport. It owns only process-local counters and the connection; exit zero
// confirms cleanup only after Run and Close both succeed. A new process never
// claims cleanup for an earlier process.
func typedSelectorProvider(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("typed-selector-provider takes no arguments")
	}
	registration := &sdk.Registration{ExitConfirmsCleanup: true}
	for _, module := range externalyang.TypedSelectorModules() {
		registration.Schemas = append(registration.Schemas, sdk.SchemaDecl{
			Module: module.Name, Namespace: module.Namespace, YANGText: module.Text,
		})
	}
	for _, path := range []string{externalyang.TypedSelectorCommand, externalyang.TypedSelectorState} {
		registration.Commands = append(registration.Commands, sdk.CommandDecl{Name: path, ShortHelp: "Exercise typed selection.", Shape: "doc"})
	}
	var serveErr error
	code := sdk.RunOrDeclare(registration, func() int {
		serveErr = typedSelectorServe(ctx, registration)
		if serveErr != nil {
			return 1
		}
		return 0
	})
	if serveErr != nil {
		return serveErr
	}
	if code != 0 {
		return fmt.Errorf("typed selector declaration returned %d", code)
	}
	return nil
}

func typedSelectorServe(ctx context.Context, registration *sdk.Registration) error {
	loader := yangloader.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return err
	}
	if err := loader.LoadRegistered(); err != nil {
		return err
	}
	for _, module := range externalyang.TypedSelectorModules() {
		if err := loader.AddModuleFromText(module.Name, module.Text); err != nil {
			return err
		}
	}
	schema, err := loader.Resolve()
	if err != nil {
		return err
	}
	definitions := yangloader.PathToArgDefs(schema)[externalyang.TypedSelectorCommand]
	if len(definitions) == 0 {
		return errors.New("typed selector fixture has no declared arguments")
	}
	provider, err := newObserver(externalyang.TypedSelectorProvider)
	if err != nil {
		return err
	}
	state := typedSelectorState{definitions: definitions}
	provider.OnExecuteCommand(state.execute)
	runErr := provider.Run(ctx, registration)
	return errors.Join(runErr, provider.Close())
}

type typedSelectorCounts struct {
	Received  uint64 `json:"received"`
	Completed uint64 `json:"completed"`
	Forced    uint64 `json:"forced"`
}

type typedSelectorState struct {
	// The immutable definitions come from the same YANG the provider declares.
	// mu protects all counters together; snapshots never mix request episodes.
	mu          sync.Mutex
	definitions []command.ArgDef
	counts      typedSelectorCounts
}

type typedSelectorStateReply struct {
	Record string              `json:"record"`
	Counts typedSelectorCounts `json:"counts"`
}

type typedSelectorReply struct {
	Record    string              `json:"record"`
	ID        uint16              `json:"id"`
	Listener  string              `json:"listener"`
	Count     uint8               `json:"count"`
	Enabled   bool                `json:"enabled"`
	InlineID  bool                `json:"inline-id"`
	Counts    typedSelectorCounts `json:"counts"`
	Arguments []string            `json:"arguments"`
}

// execute counts every target call before judging it: a daemon that forwards
// a refused request cannot hide behind a second provider-side refusal. The
// 32-request process budget bounds counters; the scenario uses seven. Validation
// uses the original YANG, never a second parser or reconstructed argument types.
// Safe for concurrent SDK callbacks; this lock contains only bounded local work.
func (state *typedSelectorState) execute(_, path string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if path == externalyang.TypedSelectorState {
		if len(args) != 0 {
			return typedSelectorRefused(errors.New("state takes no arguments"))
		}
		return rpc.StatusDone, typedSelectorStateReply{Record: typedSelectorRecord, Counts: state.counts}, nil
	}
	if path != externalyang.TypedSelectorCommand {
		return typedSelectorRefused(fmt.Errorf("unknown fixture command %q", path))
	}
	if state.counts.Received == typedSelectorRequestsMax {
		return typedSelectorRefused(errors.New("fixture request budget is exhausted"))
	}
	state.counts.Received++
	arguments, err := command.ValidateArgs(args, state.definitions, nil)
	if err != nil {
		return typedSelectorRefused(err)
	}
	idValue, err := typedSelectorValue(arguments, typedSelectorLeafID)
	if err != nil {
		return typedSelectorRefused(err)
	}
	id, err := strconv.ParseUint(idValue, 10, 16)
	if err != nil {
		return typedSelectorRefused(err)
	}
	if id != uint64(typedSelectorID) {
		return typedSelectorRefused(fmt.Errorf("fixture original %d does not exist", id))
	}
	listener, err := typedSelectorValue(arguments, typedSelectorLeafListener)
	if err != nil {
		return typedSelectorRefused(err)
	}
	countValue, err := typedSelectorValue(arguments, fieldCount)
	if err != nil {
		return typedSelectorRefused(err)
	}
	count, err := strconv.ParseUint(countValue, 10, 8)
	if err != nil {
		return typedSelectorRefused(err)
	}
	enabledValue, err := typedSelectorValue(arguments, typedSelectorLeafEnabled)
	if err != nil {
		return typedSelectorRefused(err)
	}
	enabled, err := strconv.ParseBool(enabledValue)
	if err != nil {
		return typedSelectorRefused(err)
	}
	state.counts.Completed++
	if arguments.Flag(typedSelectorLeafForce) {
		state.counts.Forced++
	}
	inline := false
	if len(args) > 0 {
		inline = args[0] == idValue
	}
	return rpc.StatusDone, typedSelectorReply{
		Record: typedSelectorRecord, ID: uint16(id), Listener: listener,
		Count: uint8(count), Enabled: enabled, InlineID: inline, Counts: state.counts,
		Arguments: slices.Clone(args),
	}, nil
}

func typedSelectorValue(arguments command.ValidatedArgs, name string) (string, error) {
	value, found := arguments.Value(name)
	if !found {
		return "", fmt.Errorf("declared fixture value %q is absent", name)
	}
	return value, nil
}

func typedSelectorRefused(cause error) (string, any, error) {
	return rpc.StatusError, nil, opererr.Wrap(cause, "Cannot run the typed selector fixture",
		"The fixture only owns original 7 and its declared controls", "ze help command request typed-selector id")
}
