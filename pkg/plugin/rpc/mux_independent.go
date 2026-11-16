// Design: docs/architecture/api/process-protocol.md -- independent current lookup ingress.
// Related: mux.go -- routing occurs before the ordinary callback queue.
package rpc

import "errors"

// IndependentRequestHandler admits a request without blocking or doing I/O.
// The registered owner MUST use a bounded queue and join its worker at stop.
// An admission error closes the connection: the sole reader cannot wait for a
// refusal write, nor silently discard a validation request. This is transport
// registration, not a plugin-name or feature dispatch table.
type IndependentRequestHandler func(*Request) error

// RegisterIndependentRequestHandler registers one method before runtime starts.
// The returned removal MUST run before the owner joins its worker. Removal
// fences concurrent admission; the handler must never call removal itself.
func (m *MuxConn) RegisterIndependentRequestHandler(method string, handler IndependentRequestHandler) (func(), error) {
	if method == "" {
		return nil, errors.New("independent callback method is empty; register an explicit method")
	}
	if handler == nil {
		return nil, errors.New("independent callback handler is nil; register a bounded admission handler")
	}
	m.independentMu.Lock()
	defer m.independentMu.Unlock()
	if m.independent == nil {
		m.independent = make(map[string]IndependentRequestHandler)
	}
	if _, exists := m.independent[method]; exists {
		return nil, errors.New("independent callback already registered; register each method once per connection")
	}
	m.independent[method] = handler
	return func() {
		m.independentMu.Lock()
		delete(m.independent, method)
		m.independentMu.Unlock()
	}, nil
}

func (m *MuxConn) routeIndependent(request *Request) (bool, error) {
	m.independentMu.RLock()
	defer m.independentMu.RUnlock()
	handler, exists := m.independent[request.Method]
	if !exists {
		return false, nil
	}
	return true, handler(request)
}
