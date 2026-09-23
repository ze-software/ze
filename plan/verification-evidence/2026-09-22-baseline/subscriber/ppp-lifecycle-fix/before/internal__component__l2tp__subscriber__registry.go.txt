// Design: docs/architecture/l2tp/subscriber-session-model.md -- session registry

package subscriber

import "sync"

// Registry is a thread-safe in-memory store of active subscriber sessions.
// Transports call Add on session-up and Remove on session-down. Show
// handlers and telemetry read from this registry.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
	}
}

func (r *Registry) Add(s *Session) {
	cp := *s
	r.mu.Lock()
	r.sessions[s.ID] = &cp
	r.mu.Unlock()
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	delete(r.sessions, id)
	r.mu.Unlock()
}

func (r *Registry) Get(id string) (Session, bool) {
	r.mu.RLock()
	s, ok := r.sessions[id]
	r.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	return *s, true
}

func (r *Registry) All() []Session {
	r.mu.RLock()
	out := make([]Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, *s)
	}
	r.mu.RUnlock()
	return out
}

type SessionCounts struct {
	Total int
	PPPoE int
	L2TP  int
}

func (r *Registry) Count() SessionCounts {
	r.mu.RLock()
	var c SessionCounts
	c.Total = len(r.sessions)
	for _, s := range r.sessions {
		switch s.AccessType {
		case AccessPPPoE:
			c.PPPoE++
		case AccessL2TP:
			c.L2TP++
		}
	}
	r.mu.RUnlock()
	return c
}

func (r *Registry) ByAccessType(t AccessType) []Session {
	r.mu.RLock()
	var out []Session
	for _, s := range r.sessions {
		if s.AccessType == t {
			out = append(out, *s)
		}
	}
	r.mu.RUnlock()
	return out
}

func (r *Registry) LookupByAcctSessionID(acctID string) (Session, bool) {
	if acctID == "" {
		return Session{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		if s.AcctSessionID == acctID {
			return *s, true
		}
	}
	return Session{}, false
}
