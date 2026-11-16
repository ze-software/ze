// Design: docs/architecture/web-interface.md -- HTTP admission ends before borrowed store release.
// Related: server.go -- Shutdown seals, cancels and joins these request owners.
package web

import (
	"context"
	"net/http"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
)

const webRequestsMax = 1024

// requestOwners owns cancellation for admitted HTTP handlers. It has no worker:
// the existing HTTP handler lifecycle calls finish. The server MUST seal before
// canceling and MUST wait before closing resources borrowed by these handlers.
// Safe for concurrent use; MUST NOT be copied after use.
type requestOwners struct {
	mu      sync.Mutex
	active  map[*httpRequestOwner]struct{}
	sealed  bool
	drained chan struct{}
}

type httpRequestOwner struct {
	cancel context.CancelFunc
}

func (s *WebServer) serveRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		owner := &httpRequestOwner{cancel: cancel}
		if !s.requests.admit(owner) {
			cancel()
			http.Error(w, opererr.New("The web service cannot admit another request.",
				"The service is stopping or its active request limit is in use; this request was not run.",
				"Reconnect to /cli after the web service is available").Error(), http.StatusServiceUnavailable)
			return
		}
		defer s.requests.finish(owner)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (o *requestOwners) admit(owner *httpRequestOwner) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.sealed {
		return false
	}
	if len(o.active) >= webRequestsMax {
		return false
	}
	if o.active == nil {
		o.active = make(map[*httpRequestOwner]struct{})
	}
	o.active[owner] = struct{}{}
	return true
}

// finish MUST follow a successful admit, after the actual handler returns.
func (o *requestOwners) finish(owner *httpRequestOwner) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.active, owner)
	if o.sealed {
		if len(o.active) == 0 {
			close(o.drained)
		}
	}
}

// seal MUST run once and MUST be followed by cancel and wait before release.
func (o *requestOwners) seal() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sealed = true
	o.drained = make(chan struct{})
	if len(o.active) == 0 {
		close(o.drained)
	}
}

func (o *requestOwners) cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for owner := range o.active {
		owner.cancel()
	}
}

// wait MUST follow seal/cancel. The service lifecycle owner bounds its wait for
// this join; returning from a network shutdown alone is never proof of it.
func (o *requestOwners) wait() {
	o.mu.Lock()
	drained := o.drained
	o.mu.Unlock()
	<-drained
}
