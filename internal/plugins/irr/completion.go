// Design: docs/architecture/irr-owner.md -- the AS-SET names the editor and the commands complete
// Related: register_completion.go -- the config-leaf suggestion that offers these names
// Related: register_commands.go -- the command argument completers that offer them
//
// Every AS-SET name the irr plugin knows is in the daemon's store: a consumer
// names it in its interest key, or the owner stored a pointer for it. The
// completion reads those keys and nothing else, so it answers the same whether
// the owner runs in the daemon or forked, and while it is stopped: the store
// belongs to the daemon, and the editor and the command dispatcher that ask
// run there too. It never reads the worker's memory, which a forked owner
// holds in another process.

package irr

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The help each offered AS-SET carries: whether `update irr as-set` already
// stored its dataset, or only a consumer names it so far.
const (
	asSetHelpStored     = "AS-SET with a stored IRR dataset"
	asSetHelpReferenced = "AS-SET a consumer references, nothing stored yet"
)

// errASSetListingTooLarge refuses a listing over the bound the SDK's own
// StateList answer carries, so the completion reads no more than a forked
// owner could.
var errASSetListingTooLarge = errors.New("irr: too many stored keys to complete from")

// asSetSource is the half of the state RPCs the completion reads. The daemon
// answers it from its own store (daemonStore); a test answers it from a fake.
type asSetSource interface {
	StateList(ctx context.Context, prefix string) ([]string, error)
	StateGet(ctx context.Context, key string) ([]byte, bool, error)
}

// knownASSet is one AS-SET name the store holds, and whether a pointer, and so
// a stored dataset, exists for it.
type knownASSet struct {
	name   string
	stored bool
}

// help is the one line a completion prints beside the name.
func (k knownASSet) help() string {
	if k.stored {
		return asSetHelpStored
	}
	return asSetHelpReferenced
}

// knownASSets answers every AS-SET name the consumers' interest keys and the
// owner's pointers hold, once each, in upper case and sorted.
//
// A listing that fails answers nil and the error: nothing was read. An
// interest value that cannot be read or decoded is left out, and its error is
// joined to the answer, so the caller still offers every other name and can
// say which key it could not read.
func knownASSets(ctx context.Context, source asSetSource) ([]knownASSet, error) {
	stored, err := pointerNames(ctx, source)
	if err != nil {
		return nil, err
	}
	referenced, interestErr := interestNames(ctx, source)
	if referenced == nil && interestErr != nil {
		return nil, interestErr
	}
	known := make([]knownASSet, 0, len(stored)+len(referenced))
	for name := range stored {
		known = append(known, knownASSet{name: name, stored: true})
	}
	for name := range referenced {
		if _, isStored := stored[name]; isStored {
			continue
		}
		known = append(known, knownASSet{name: name})
	}
	slices.SortFunc(known, func(a, b knownASSet) int { return strings.Compare(a.name, b.name) })
	return known, interestErr
}

// pointerNames answers the AS-SET each pointer key names. The key is the
// fact: a pointer whose value cannot be decoded still names a set the owner
// stored, and `show irr` and the doctor report the bad value.
func pointerNames(ctx context.Context, source asSetSource) (map[string]struct{}, error) {
	keys, err := listBounded(ctx, source, store.PointerPrefix)
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		name, err := store.ASSetName(strings.TrimPrefix(key, store.PointerPrefix))
		if err != nil {
			continue // the owner writes only ASSetName keys; another is no AS-SET to offer
		}
		names[name] = struct{}{}
	}
	return names, nil
}

// interestNames answers every AS-SET a consumer's interest key names. The
// bare AS numbers an interest also holds are not AS-SET names, so they are
// not offered. A nil map with an error means the listing failed.
func interestNames(ctx context.Context, source asSetSource) (map[string]struct{}, error) {
	keys, err := listBounded(ctx, source, store.InterestPrefix)
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{})
	var unreadable error
	for _, key := range keys {
		value, found, err := source.StateGet(ctx, key)
		if err != nil {
			unreadable = errors.Join(unreadable, fmt.Errorf("%s: %w", key, err))
			continue
		}
		if !found {
			continue // removed between the listing and the read: it names nothing now
		}
		in, err := store.DecodeInterest(value)
		if err != nil {
			unreadable = errors.Join(unreadable, fmt.Errorf("%s: %w", key, err))
			continue
		}
		for _, name := range in.ASSets {
			names[name] = struct{}{}
		}
	}
	return names, unreadable
}

// listBounded lists the keys under prefix, refusing more than one SDK answer
// holds.
func listBounded(ctx context.Context, source asSetSource, prefix string) ([]string, error) {
	keys, err := source.StateList(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("irr: list %s: %w", prefix, err)
	}
	if len(keys) > rpc.StateListMax {
		return nil, fmt.Errorf("%w: %d keys under %s, over %d", errASSetListingTooLarge, len(keys), prefix, rpc.StateListMax)
	}
	return keys, nil
}

// daemonStore reads the daemon's own store under the irr owner's grants: the
// same keys, and only those, the owner's StateList and StateGet reach. It is
// read in the daemon, where the editor and the dispatcher run. Safe for
// concurrent use, as statestore is.
type daemonStore struct{}

func (daemonStore) StateList(_ context.Context, prefix string) ([]string, error) {
	return statestore.PluginList(pluginName, prefix)
}

func (daemonStore) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	if !statestore.PluginKeyReadable(pluginName, key) {
		return nil, false, fmt.Errorf("irr: plugin %s may not read %s", pluginName, key)
	}
	return statestore.Read(key)
}

// daemonKnownASSets answers knownASSets over the daemon's store for a
// completion, which has no error to return: what could not be read is logged
// and every name that could be read is offered.
func daemonKnownASSets() []knownASSet {
	known, err := knownASSets(context.Background(), daemonStore{})
	if err == nil {
		return known
	}
	// refuse-by-default: guards asSetValues -- a process with no store
	// (an offline tool, a test without one) holds no stored AS-SET, so it
	// offers none and the editor prints the leaf's type hint. It is not a
	// fault, so it is not logged on every keystroke.
	if errors.Is(err, statestore.ErrUnavailable) {
		return nil
	}
	logger().Warn("irr: the stored AS-SET names cannot all be read, completion offers the readable ones",
		"error", err, "code", codeIRRDatasetUnreadable)
	return known
}
