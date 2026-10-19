// Design: docs/architecture/testing/interop.md -- Crash-safe teardown
// Related: docker.go -- RemoveContainer, which settles a failed start here.
// Related: container.go -- RunDetached, one of the starts that marks a name.

package interoplab

import (
	"context"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// createSettleWindow bounds how long a removal after a failed or canceled
// start keeps looking for the container, and createSettleInterval is the wait
// between two looks. Killing the docker client does not cancel a create the
// daemon already accepted: on 2026-10-19 the daemon created the container in
// the second a Ctrl-C arrived, after the removal had found nothing (D-e). A
// create that lands after the window still carries its owner labels, so
// `./le test integration cleanup` removes it once this process has exited.
const (
	createSettleWindow   = 5 * time.Second
	createSettleInterval = 250 * time.Millisecond
)

// removeSettlingContainer removes a container whose start failed or was
// canceled, once a first `docker rm -f` found none: it looks again every
// createSettleInterval until createSettleWindow ends, and removes the
// container if the daemon creates it meanwhile. It answers nil when the
// container was removed or never appeared within the window, and an error
// naming the container and the cleanup command otherwise.
func (d *Docker) removeSettlingContainer(ctx context.Context, name string) error {
	for range createSettleWindow / createSettleInterval {
		select {
		case <-ctx.Done():
			return &settlingRemovalError{name: name, err: ctx.Err(), appeared: false}
		case <-time.After(createSettleInterval):
		}
		absent, err := d.forceRemoveContainer(ctx, name)
		if absent {
			continue
		}
		if err != nil {
			return &settlingRemovalError{name: name, err: err, appeared: true}
		}
		return nil
	}
	return nil
}

// settlingRemovalError is a container whose start failed or was canceled, and
// that this run could not confirm removed: it appeared and its removal failed,
// or the wait for it ended before the window did.
type settlingRemovalError struct {
	name     string
	err      error
	appeared bool
}

func (e *settlingRemovalError) Error() string {
	var tb textbuf.Buffer
	tb.Str("container ").Str(e.name)
	if e.appeared {
		tb.Str(" was created after its start failed or was canceled, and this run could not remove it (")
	} else {
		tb.Str(" may still be created after its start failed or was canceled, and this run stopped looking for it (")
	}
	return tb.Err(e.err).Str("). Fix: run `").Str(cleanupCommand).
		Str("`, which removes it once this process has exited").String()
}

func (e *settlingRemovalError) Unwrap() error { return e.err }

// markUnsettled records name for removeSettlingContainer when err is an issued
// run that may have left a container (ContainerMayExist). A name another
// container holds is not this run's, and is not recorded.
func (d *Docker) markUnsettled(name string, err error) {
	if ContainerMayExist(err) {
		d.unsettled.mark(name)
	}
}

// unsettledNames is the set of container names whose start failed or was
// canceled and whose removal is still owed. Safe for concurrent use; the zero
// value is an empty set.
type unsettledNames struct {
	mu    sync.Mutex
	names map[string]struct{}
}

// mark records name. RemoveContainer of the name MUST follow: it takes the
// mark, so the set holds only removals still owed.
func (u *unsettledNames) mark(name string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.names == nil {
		u.names = make(map[string]struct{})
	}
	u.names[name] = struct{}{}
}

// take reports whether name was marked, and forgets it. It MUST be called
// only by RemoveContainer, after mark.
func (u *unsettledNames) take(name string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	_, marked := u.names[name]
	delete(u.names, name)
	return marked
}
