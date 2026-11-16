// Design: docs/architecture/container/image-store.md -- the image commands

package container

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFetchCommandDeclaresItsBound proves the fetch and the update declare a
// bound of one hour at Stage 1: a fetch takes minutes, and a plugin command
// that declares no bound is cut at 30 seconds.
//
// Method: marshal the plugin's declarations as the SDK sends them, and read
// the bound key, "timeout" in whole seconds, of the two commands.
//
// Red until the plugin protocol carries a command's own bound: the Stage 1
// command declaration has no such field, and adding one changes the public
// plugin protocol, which waits for the owner's agreement
// (spec-container-2-image-store, "Deviations from Plan").
func TestFetchCommandDeclaresItsBound(t *testing.T) {
	encoded, err := json.Marshal(commandDecls())
	if err != nil {
		t.Fatalf("marshal the declarations: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal the declarations: %v", err)
	}
	for _, name := range []string{cmdCreateImage, cmdUpdateImage} {
		index := slices.IndexFunc(decoded, func(decl map[string]any) bool { return decl["name"] == name })
		if index < 0 {
			t.Errorf("%s is not declared", name)
			continue
		}
		bound, ok := decoded[index]["timeout"].(float64)
		if !ok {
			t.Errorf("%s declares no bound: its declaration holds no timeout key", name)
			continue
		}
		if bound != 3600 {
			t.Errorf("%s declares a bound of %v seconds, want 3600", name, bound)
		}
	}
}

// TestEveryImageCommandIsDeclaredAndHandled proves the declarations, the
// proxies and the handlers name the same ten commands, so a declared command
// is never refused as unknown and a handled one is never undeclared.
//
// Method: compare the names of imageCommandDecls, the plugin commands of
// imageCommandRPCs and the keys of imageCommandHandlers.
//
// MUTATION: drop an entry from imageCommandHandlers. The command is then
// declared and unhandled.
func TestEveryImageCommandIsDeclaredAndHandled(t *testing.T) {
	declared := make([]string, 0, len(imageCommandDecls()))
	for _, decl := range imageCommandDecls() {
		declared = append(declared, decl.Name)
	}
	proxied := make([]string, 0, len(imageCommandRPCs()))
	for _, registration := range imageCommandRPCs() {
		proxied = append(proxied, registration.PluginCommand)
		if !strings.HasPrefix(registration.WireMethod, "ze-container:") {
			t.Errorf("wire method %s is outside the plugin's module", registration.WireMethod)
		}
	}
	handled := make([]string, 0, len(imageCommandHandlers))
	for name := range imageCommandHandlers {
		handled = append(handled, name)
	}
	slices.Sort(declared)
	slices.Sort(proxied)
	slices.Sort(handled)
	if len(declared) != 10 {
		t.Errorf("%d image commands are declared, want the 10 of the command table", len(declared))
	}
	if !slices.Equal(declared, proxied) {
		t.Errorf("declared %v, proxied %v", declared, proxied)
	}
	if !slices.Equal(declared, handled) {
		t.Errorf("declared %v, handled %v", declared, handled)
	}
}

// fakeImageStore is a store that also answers the image commands, and
// records each call. Safe for use by one test goroutine.
type fakeImageStore struct {
	fakeStore
	calls []string
}

func (f *fakeImageStore) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeImageStore) FetchImage(_ context.Context, reference string) (FetchAnswer, error) {
	f.record("fetch " + reference)
	return FetchAnswer{Reference: reference, Digest: "sha256:aa", Created: true}, nil
}

func (f *fakeImageStore) ListImages(context.Context) ([]ImageRow, error) {
	f.record("list images")
	return []ImageRow{{Reference: "docker.io/library/redis:latest", State: ImageStatePresent}}, nil
}

func (f *fakeImageStore) StoreTotals(context.Context) (StoreTotals, error) {
	f.record("store")
	return StoreTotals{Path: "/var/lib/ze/container"}, nil
}

func (f *fakeImageStore) ListVolumes(context.Context) ([]VolumeRow, error) {
	f.record("list volumes")
	return nil, nil
}

func (f *fakeImageStore) UpdateImage(_ context.Context, instance string) (UpdateAnswer, error) {
	f.record("update " + instance)
	return UpdateAnswer{Name: instance}, nil
}

func (f *fakeImageStore) RollbackImage(_ context.Context, instance string) (RollbackAnswer, error) {
	f.record("rollback " + instance)
	return RollbackAnswer{Name: instance}, nil
}

func (f *fakeImageStore) DeleteImage(_ context.Context, digest string) (DeleteAnswer, error) {
	f.record("delete " + digest)
	return DeleteAnswer{}, nil
}

func (f *fakeImageStore) DeleteAllImages(context.Context) (DeleteAnswer, error) {
	f.record("delete all")
	return DeleteAnswer{}, nil
}

func (f *fakeImageStore) DeleteVolume(_ context.Context, instance, volume string) (VolumeDeleteAnswer, error) {
	f.record("delete volume " + volume + " of " + instance)
	return VolumeDeleteAnswer{Instance: instance, Name: volume}, nil
}

func (f *fakeImageStore) ClearLayer(_ context.Context, instance string) (ClearAnswer, error) {
	f.record("clear " + instance)
	return ClearAnswer{Name: instance, Cleared: true}, nil
}

// TestImageCommandsReachTheStore proves each image command reaches the
// store's operation with the values its leaves carry, by keyword and, for a
// one-leaf command, as the bare word a programmatic sender passes.
//
// Method: handleCommand over a supervisor whose store answers the image
// commands, then the calls the store recorded.
func TestImageCommandsReachTheStore(t *testing.T) {
	store := &fakeImageStore{}
	sup := newSupervisor(slogutil.DiscardLogger(), nil, storeFactoryOf(store), nil)
	commands := []struct {
		command string
		args    []string
	}{
		{cmdCreateImage, []string{referenceKeyword, "redis"}},
		{cmdCreateImage, []string{"nginx:1.27"}},
		{cmdShowImageList, nil},
		{cmdShowStore, nil},
		{cmdShowVolumeList, nil},
		{cmdUpdateImage, []string{selectorKeyword, "web"}},
		{cmdUpdateImageRollback, []string{"web"}},
		{cmdDeleteImage, []string{digestKeyword, "0123456789ab"}},
		{cmdDeleteImageAll, nil},
		{cmdDeleteVolume, []string{selectorKeyword, "data", instanceKeyword, "web"}},
		{cmdClearLayer, []string{selectorKeyword, "web"}},
	}
	for _, c := range commands {
		status, _, err := handleCommand(context.Background(), sup, c.command, c.args)
		if err != nil {
			t.Fatalf("%s %v: %v", c.command, c.args, err)
		}
		if status != rpc.StatusDone {
			t.Errorf("%s %v answered status %s, want %s", c.command, c.args, status, rpc.StatusDone)
		}
	}
	want := []string{
		"fetch redis", "fetch nginx:1.27", "list images", "store", "list volumes",
		"update web", "rollback web", "delete 0123456789ab", "delete all",
		"delete volume data of web", "clear web",
	}
	if !slices.Equal(store.calls, want) {
		t.Errorf("store calls %v, want %v", store.calls, want)
	}
}

// TestImageCommandRefusesWhatItCannotRun proves an image command is refused
// with an operator error, never answered, when its leaves are missing or the
// store of this build answers no image command.
//
// Method: a volume delete with no instance keyword over a store that answers
// the image commands, then a listing over the runtime's fake store, which is
// a Store and no ImageOperations.
func TestImageCommandRefusesWhatItCannotRun(t *testing.T) {
	imageStore := &fakeImageStore{}
	sup := newSupervisor(slogutil.DiscardLogger(), nil, storeFactoryOf(imageStore), nil)
	status, _, err := handleCommand(context.Background(), sup, cmdDeleteVolume, []string{selectorKeyword, "data"})
	if status != rpc.StatusError {
		t.Errorf("a volume delete with no instance answered status %s, want %s", status, rpc.StatusError)
	}
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Errorf("a volume delete with no instance answered %v, want an operator error", err)
	}
	if len(imageStore.calls) != 0 {
		t.Errorf("the refused command reached the store: %v", imageStore.calls)
	}

	plain := newSupervisor(slogutil.DiscardLogger(), nil, storeFactoryOf(&fakeStore{}), nil)
	status, _, err = handleCommand(context.Background(), plain, cmdShowImageList, nil)
	if status != rpc.StatusError {
		t.Errorf("a listing over a store with no image command answered status %s, want %s", status, rpc.StatusError)
	}
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Errorf("a listing over a store with no image command answered %v, want an operator error", err)
	}
}
