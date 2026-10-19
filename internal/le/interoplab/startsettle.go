// Design: docs/architecture/testing/interop.md -- Crash-safe teardown
// Related: docker.go -- RemoveContainer, which settles a failed start here,
// and forceRemoveContainer, which settles a removal already in progress here.
// Related: container.go -- RunDetached, one of the starts that marks a name.

package interoplab

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// createSettleWindow bounds how long a removal keeps looking for a container
// the daemon is still settling, and createSettleInterval is the wait between
// two looks. Two daemon actions outlive the client call that asked for them.
// Killing the docker client does not cancel a create the daemon already
// accepted: on 2026-10-19 the daemon created the container in the second a
// Ctrl-C arrived, after the removal had found nothing (D-e). And a --rm
// removal the daemon started refuses the `docker rm -f` that follows it as
// "already in progress" while it runs (D-f, 2026-10-20). A container that
// outlasts the window still carries its owner labels, so
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
		if err == nil {
			return nil
		}
		// A removal still in progress already names the container and the
		// cleanup command, so it is not wrapped into a second fix.
		if _, pending := errors.AsType[*pendingRemovalError](err); pending {
			return err
		}
		return &settlingRemovalError{name: name, err: err, appeared: true}
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

// removalInProgressAnswer is the refusal docker gives `docker rm -f ref`
// while the daemon is already removing that container. It is the only
// "already in progress" answer in the dockerd 29.9 binary (checked on
// 2026-10-20), and it names a container, so a network or image removal never
// gives it.
func removalInProgressAnswer(ref string) string {
	var tb textbuf.Buffer
	return tb.Str("removal of container ").Str(ref).Str(" is already in progress").String()
}

// awaitRemoval settles a `docker rm -f name` the daemon refused because it is
// already removing the container: a --rm one-shot whose bound ended (D-f).
// That removal runs in the daemon, so the refusal does not say the container
// stays. It looks for the container at once, then every createSettleInterval
// until createSettleWindow ends. It answers nil once the container is gone, a
// removalUnknownError when a look fails, and a pendingRemovalError when the
// container outlasts the window or the wait is stopped.
func (d *Docker) awaitRemoval(ctx context.Context, name string, removeErr error) error {
	for look := range createSettleWindow/createSettleInterval + 1 {
		if look > 0 {
			select {
			case <-ctx.Done():
				return &pendingRemovalError{name: name, removal: removeErr, stop: ctx.Err()}
			case <-time.After(createSettleInterval):
			}
		}
		present, err := d.resourcePresent(ctx, resourceContainer, name)
		if err != nil {
			return &removalUnknownError{removal: removeErr, check: err}
		}
		if !present {
			return nil
		}
	}
	return &pendingRemovalError{name: name, removal: removeErr, stop: nil}
}

// pendingRemovalError is a container the daemon was already removing and that
// was still present when this run stopped waiting: the window ended, or stop
// says why the wait was stopped.
type pendingRemovalError struct {
	name    string
	removal error
	stop    error
}

func (e *pendingRemovalError) Error() string {
	var tb textbuf.Buffer
	tb.Str("container ").Str(e.name).Str(" is still present: docker was already removing it (")
	tb.Err(e.removal).Str("), and ")
	if e.stop != nil {
		tb.Str("this run stopped waiting for it (").Err(e.stop).Str(")")
	} else {
		tb.Str("it was not gone after ").Str(createSettleWindow.String())
	}
	return tb.Str(". Fix: run `").Str(cleanupCommand).
		Str("`, which removes it once this process has exited").String()
}

func (e *pendingRemovalError) Unwrap() []error {
	if e.stop == nil {
		return []error{e.removal}
	}
	return []error{e.removal, e.stop}
}

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
