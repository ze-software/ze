// Design: docs/architecture/l2tp/bng-5-pppoe.md -- transactional access preparation
// Related: subsystem_reload.go -- publication and removal of prepared accesses

package pppoe

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
)

// accessConfiguration is a fully prepared set: every entry has an owned
// mapping lease, distinct Linux index, and the same discovery namespace.
// It is private, immutable after construction and used under lifecycleMu.
// The zero/nil value is an idle subsystem with no preparation ownership.
type accessConfiguration struct {
	accesses  map[string]dataplane.Access
	namespace string
}

// prepareAccessConfiguration MUST precede opening discovery or publishing a
// reload. On failure it releases only additions, leaving all kept leases live.
// A successful caller MUST close unpublished additions if subsequent startup
// fails, or retain the resulting accesses until discovery and sessions stop.
func prepareAccessConfiguration(ctx context.Context, backend dataplane.Backend, interfaces []InterfaceConfig,
	kept map[string]dataplane.Access) (configuration *accessConfiguration, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	accesses := make(map[string]dataplane.Access, len(interfaces))
	defer func() {
		if err != nil {
			err = errors.Join(err, closeAccessConfiguration(accesses, kept))
		}
	}()
	indexes := make(map[int]string, len(interfaces))
	namespace := ""
	for i := range interfaces {
		name := interfaces[i].Name
		if _, exists := accesses[name]; exists {
			return nil, fmt.Errorf("duplicate PPPoE access %q", name)
		}
		access := kept[name]
		if access == nil {
			access, err = backend.Prepare(ctx, name)
			if err != nil {
				return nil, opererr.Wrap(err, fmt.Sprintf("PPPoE access interface %q could not be prepared", name),
					"Discovery cannot start until the selected interface backend resolves this access and its control device.",
					"delete pppoe interface "+name)
			}
		}
		accesses[name] = access
		if len(accesses) == 1 {
			namespace = access.Namespace()
		}
		if access.Namespace() != namespace {
			return nil, opererr.New(fmt.Sprintf("PPPoE access interface %q uses a different control namespace", name),
				"All accesses on the selected backend must use one discovery socket namespace.", "delete pppoe interface "+name)
		}
		index := access.DiscoveryIfIndex()
		// The shared PPP driver's tunnel key is uint16. Refuse before a Linux
		// index can truncate into another access's session-key space.
		const discoveryIndexMax = 1<<16 - 1
		if index > discoveryIndexMax {
			return nil, opererr.New(fmt.Sprintf("PPPoE access %q has Linux interface index %d above %d", name, index, discoveryIndexMax),
				"The shared PPP session key cannot represent this access without truncation.", "ze doctor")
		}
		if existing, duplicate := indexes[index]; duplicate {
			return nil, opererr.New(fmt.Sprintf("PPPoE accesses %q and %q name the same Linux control interface", existing, name),
				"Discovery cannot distinguish these configured accesses.", "delete pppoe interface "+name)
		}
		indexes[index] = name
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &accessConfiguration{accesses: accesses, namespace: namespace}, nil
}

// closeAccessConfiguration MUST run after removed accesses' sessions stop and
// discovery no longer dispatches to them. keep protects retained leases during
// a failed preparation or a successful live reload. Every release is attempted.
func closeAccessConfiguration(accesses, keep map[string]dataplane.Access) error {
	// A provider can accidentally return an existing lease under another name.
	// Refusing its duplicate Linux index MUST NOT release the running owner.
	protected := make(map[dataplane.Access]struct{}, len(keep))
	for _, access := range keep {
		protected[access] = struct{}{}
	}
	var err error
	for _, access := range accesses {
		if _, retained := protected[access]; retained {
			continue
		}
		err = errors.Join(err, access.Close())
	}
	return err
}

func accessReleaseError(err error) error {
	if err == nil {
		return nil
	}
	return opererr.Wrap(err, "A PPPoE access mapping could not be released",
		"Discovery and subscriber sessions have stopped using the removed access, but its backend reported incomplete cleanup.", "ze doctor")
}
