// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact provider-request lifetimes
// Related: auth.go, ncp.go -- request minting and retirement at each wait
// Related: manager.go -- admitted-worker check before delivering any answer

package ppp

import (
	"slices"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// Each wait owns a fresh buffered-one channel. A retained provider capability
// never acquires another wait's channel, even when the PPP worker is unchanged.
// The actual emitter alone constructs these private immutable bindings.
type authRequest struct {
	control      SessionControl
	subscriberID string
	// Immutable link to the actual initial L2TP auth round, not its answer wait.
	admission           *authAdmissionBinding
	framedRoutesAllowed bool
	answers             chan authResponseMsg
	mu                  sync.Mutex
	waiting             bool
	answered            bool
	framedRoutes        []l2tpevents.FramedRoute
}

type ipRequest struct {
	control      SessionControl
	subscriberID string
	family       AddressFamily
	answers      chan ipResponseMsg
	mu           sync.Mutex
	waiting      bool
	answered     bool
}

func newAuthRequest(control SessionControl, subscriberID string) *authRequest {
	return &authRequest{control: control, subscriberID: subscriberID,
		answers: make(chan authResponseMsg, 1), waiting: true}
}

func newIPRequest(control SessionControl, subscriberID string, family AddressFamily) *ipRequest {
	return &ipRequest{control: control, subscriberID: subscriberID, family: family,
		answers: make(chan ipResponseMsg, 1), waiting: true}
}

// Retirement runs on every wait exit, including LCP interruption, timeout,
// cancellation and a consumed decision. Channels stay open: a retained provider
// receives an explicit expired result instead of racing a close or future wait.
func (pending *authRequest) retire() {
	pending.mu.Lock()
	pending.waiting = false
	pending.framedRoutes = nil
	pending.mu.Unlock()
}

func (pending *ipRequest) retire() {
	pending.mu.Lock()
	pending.waiting = false
	pending.mu.Unlock()
}

// validLocked checks the immutable binding and its public event snapshot.
// Caller MUST hold pending.mu and MUST distinguish an already answered wait.
func (request EventAuthRequest) validLocked() bool {
	if request.Control != request.pending.control {
		return false
	}
	if request.SubscriberID != request.pending.subscriberID {
		return false
	}
	return request.pending.waiting
}

func (request EventIPRequest) validLocked() bool {
	if request.Control != request.pending.control {
		return false
	}
	if request.SubscriberID != request.pending.subscriberID {
		return false
	}
	if request.Family != request.pending.family {
		return false
	}
	return request.pending.waiting
}

// Active reports whether this exact emitted request can still accept its first
// answer on a live admitted worker. It grants no authority by itself: delivery
// rechecks under the pending mutex. Synthetic and retired requests are inactive.
func (request EventAuthRequest) Active() bool {
	if request.pending == nil {
		return false
	}
	request.pending.mu.Lock()
	defer request.pending.mu.Unlock()
	if !request.validLocked() {
		return false
	}
	if request.pending.answered {
		return false
	}
	return request.Control.Active()
}

// StageFramedRoutes copies the bounded, validated profile for the original
// credentialed PPPoE authentication round. The provider MUST call this before
// replying Accept. Other kinds of authentication do not stage a route profile.
// Safe for concurrent use. A zero or expired eligible request cannot stage.
func (request EventAuthRequest) StageFramedRoutes(framedRoutes []l2tpevents.FramedRoute) error {
	pending := request.pending
	if pending == nil {
		return ErrRequestExpired
	}
	if !pending.framedRoutesAllowed {
		return nil
	}
	pending.mu.Lock()
	defer pending.mu.Unlock()
	if !request.validLocked() {
		return ErrRequestExpired
	}
	if pending.answered {
		return ErrAuthResponsePending
	}
	if !request.Control.Active() {
		return ErrRequestExpired
	}
	pending.framedRoutes = slices.Clone(framedRoutes)
	return nil
}

func (request EventIPRequest) Active() bool {
	if request.pending == nil {
		return false
	}
	request.pending.mu.Lock()
	defer request.pending.mu.Unlock()
	if !request.validLocked() {
		return false
	}
	if request.pending.answered {
		return false
	}
	return request.Control.Active()
}

// answer enqueues only for the actual emitted request. Driver first validates
// admitted-worker authority; no request constructor or current-key fallback is
// exported. The private mutex serializes answers with the wait's retirement.
func (request EventAuthRequest) answer(decision authResponseMsg) error {
	if request.pending == nil {
		return ErrRequestExpired
	}
	request.pending.mu.Lock()
	defer request.pending.mu.Unlock()
	if !request.validLocked() {
		return ErrRequestExpired
	}
	if request.pending.answered {
		return ErrAuthResponsePending
	}
	if decision.accept {
		decision.framedRoutes = request.pending.framedRoutes
	}
	select {
	case request.pending.answers <- decision:
		request.pending.answered = true
		request.pending.framedRoutes = nil
		return nil
	default:
		return ErrAuthResponsePending
	}
}

func (request EventIPRequest) answer(decision ipResponseMsg) error {
	if request.pending == nil {
		return ErrRequestExpired
	}
	request.pending.mu.Lock()
	defer request.pending.mu.Unlock()
	if !request.validLocked() {
		return ErrRequestExpired
	}
	if request.pending.answered {
		return ErrIPResponsePending
	}
	if decision.family != request.pending.family {
		return ErrIPResponseFamilyMismatch
	}
	select {
	case request.pending.answers <- decision:
		request.pending.answered = true
		return nil
	default:
		return ErrIPResponsePending
	}
}

// lockAuthRequest fences a request-derived relay action against retirement.
// Success leaves pending.mu held. The caller MUST unlock it after the action,
// before joining a worker whose wait's deferred retirement takes this mutex.
func (d *Driver) lockAuthRequest(request EventAuthRequest) (*pppSession, error) {
	if request.pending == nil {
		return nil, ErrRequestExpired
	}
	request.pending.mu.Lock()
	if !request.validLocked() {
		request.pending.mu.Unlock()
		return nil, ErrRequestExpired
	}
	if request.pending.answered {
		request.pending.mu.Unlock()
		return nil, ErrRequestExpired
	}
	session, err := d.liveSession(request.Control)
	if err != nil {
		request.pending.mu.Unlock()
		return nil, err
	}
	return session, nil
}

// retireProviderRequests runs on the PPP worker before LCP-down cleanup or final
// down publication can block. Retirement only at the await's return would let
// a provider enqueue after a timeout was published or teardown had begun.
// Callers outside the worker cancel via SessionControl.
func (session *pppSession) retireProviderRequests() {
	if session.authRequest != nil {
		session.authRequest.retire()
	}
	if session.ipRequest != nil {
		session.ipRequest.retire()
	}
}
