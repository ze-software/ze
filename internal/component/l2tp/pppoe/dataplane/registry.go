// Design: docs/architecture/l2tp/bng-5-pppoe.md -- registered session data planes
// Related: session.go -- validated resource ownership returned by a provider

// Package dataplane selects a registered PPPoE session provider. It contains no
// backend imports: each provider registers its own access preparer from init.
package dataplane

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/naming"
	"github.com/ze-software/ze/pkg/ze"
)

// Request is the raw admission request passed unchanged to the selected
// provider. The provider MUST validate its transport-specific requirements.
type Request struct {
	AccessName string
	// Device is the resolved session-side device (kernel OS name or VPP DP name).
	Device string
	// ControlDevice is the resolved Linux Ethernet device used for control frames.
	ControlDevice string
	// Namespace is the Linux control device's namespace; empty means the caller's.
	Namespace       string
	SessionID       uint16
	PeerMAC         [6]byte
	CaptureProxyLCP bool
}

// OpenFunc creates one session. Ownership of any nonnil result passes to the
// caller even on error; Access.Open closes that result before refusing it.
// A successful result MUST come from NewOpenedSession.
type OpenFunc func(context.Context, Request) (OpenedSession, error)

// PrepareFunc resolves an access and installs any provider-specific control
// mapping before discovery can admit sessions. Ownership of a nonnil result
// passes to the caller even on error; Backend.Prepare closes it on refusal.
type PrepareFunc func(context.Context, string) (Access, error)

// Backend is an exact registry selection. Only a successful lookup can create
// one; its nil interface is invalid, and callers cannot construct a zero value.
// Prepare is safe for concurrent use if its registered preparer is also safe.
type Backend interface {
	Name() string
	// Prepare requires a nonnil context. A successful caller MUST Close the
	// access after stopping its sessions and discovery dispatch.
	Prepare(context.Context, string) (Access, error)
	// WatchLifecycle returns explicit absence for a stable provider. A changing
	// provider's caller MUST subscribe before preparation and Close before
	// joining its sole lifecycle worker. No notification proves native release.
	WatchLifecycle(ze.EventBus) (*LifecycleWatch, bool, error)

	sessionDataplane()
}

var sessionDataplanes sessionRegistry

// RegisterSessionDataplane registers a provider's preparer, normally from its
// register.go init. Names use the shared node-name syntax, bounded to 63 bytes.
// An optional third argument declares one changing provider's lifecycle
// source; omission explicitly declares a stable provider. Explicit nil or
// multiple lifecycle factories are refused, as are invalid names, nil
// preparers and duplicate names. Safe for concurrent use.
func RegisterSessionDataplane(name string, prepare PrepareFunc, lifecycle ...SubscribeLifecycleFunc) error {
	return sessionDataplanes.register(name, prepare, lifecycle...)
}

// LookupSessionDataplane selects exactly name. An unavailable provider returns
// an error; no default or alternative provider is tried. Safe for concurrent use.
func LookupSessionDataplane(name string) (Backend, error) {
	return sessionDataplanes.lookup(name)
}

// SessionDataplaneNames returns an independent, sorted snapshot of registered
// provider names. Safe for concurrent use; a caller may change its copy.
func SessionDataplaneNames() []string {
	return sessionDataplanes.names()
}

// sessionRegistry's zero value is ready for registration and lookup. Its mutex
// protects only registration state; a factory runs without holding that mutex.
type sessionRegistry struct {
	mu        sync.RWMutex
	providers map[string]sessionProvider
}

type sessionProvider struct {
	prepare   PrepareFunc
	subscribe SubscribeLifecycleFunc
}

func (r *sessionRegistry) register(name string, prepare PrepareFunc, lifecycle ...SubscribeLifecycleFunc) error {
	const nameMax = 63
	if err := naming.ValidateNodeName("PPPoE session backend", name, nameMax); err != nil {
		return err
	}
	if prepare == nil {
		return fmt.Errorf("PPPoE session backend %q has no access preparer", name)
	}
	if len(lifecycle) > 1 {
		return fmt.Errorf("PPPoE session backend %q declares multiple lifecycle factories", name)
	}
	var subscribe SubscribeLifecycleFunc
	if len(lifecycle) == 1 {
		if lifecycle[0] == nil {
			return fmt.Errorf("PPPoE session backend %q has no lifecycle factory", name)
		}
		subscribe = lifecycle[0]
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("PPPoE session backend %q is already registered", name)
	}
	if r.providers == nil {
		r.providers = make(map[string]sessionProvider)
	}
	r.providers[name] = sessionProvider{prepare: prepare, subscribe: subscribe}
	return nil
}

func (r *sessionRegistry) lookup(name string) (Backend, error) {
	r.mu.RLock()
	provider, exists := r.providers[name]
	r.mu.RUnlock()
	if !exists {
		names := r.names()
		if len(names) == 0 {
			return nil, fmt.Errorf("PPPoE session backend %q is unavailable: no session backends are registered", name)
		}
		return nil, fmt.Errorf("PPPoE session backend %q is unavailable (registered: %s)",
			name, strings.Join(names, ", "))
	}
	return &sessionBackend{name: name, prepare: provider.prepare, subscribe: provider.subscribe}, nil
}

func (r *sessionRegistry) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// sessionBackend is private so the registry alone can create a validated
// selection. Its fields remain immutable after lookup.
type sessionBackend struct {
	name      string
	prepare   PrepareFunc
	subscribe SubscribeLifecycleFunc
}

func (b *sessionBackend) Name() string { return b.name }

func (b *sessionBackend) Prepare(ctx context.Context, name string) (Access, error) {
	if nilResource(ctx) {
		return nil, fmt.Errorf("PPPoE session backend %q requires a context", b.name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	access, err := b.prepare(ctx, name)
	if !nilResource(access) {
		if _, constructed := access.(*preparedAccess); !constructed {
			err = errors.Join(err, errors.New("access result was not created by NewAccess"))
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		if !nilResource(access) {
			err = refusePreparedAccess(err, access)
		}
		return nil, fmt.Errorf("PPPoE session backend %q: %w", b.name, err)
	}
	if nilResource(access) {
		return nil, fmt.Errorf("PPPoE session backend %q returned no access", b.name)
	}
	return access, nil
}

func closeSessionResult(opened OpenedSession) error {
	return errors.Join(closeSessionDataplane(opened), closeSessionTransport(opened))
}

// A refused factory result may inherit a nil accessor through interface
// embedding. Contain each owner's cleanup separately so one broken owner
// cannot prevent the other ownership role from being released.
func closeSessionDataplane(opened OpenedSession) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("PPPoE session data-plane cleanup panicked: %v", failure)
		}
	}()
	if handle := opened.Dataplane(); !nilResource(handle) {
		return handle.Close()
	}
	return nil
}

func closeSessionTransport(opened OpenedSession) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("PPPoE session transport cleanup panicked: %v", failure)
		}
	}()
	return opened.CloseTransport()
}

func (*sessionBackend) sessionDataplane() {}
