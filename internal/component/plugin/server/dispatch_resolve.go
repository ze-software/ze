// Design: docs/architecture/api/process-protocol.md -- plugin RPC dispatch
// Related: dispatch_registry.go -- the engineOp entry that routes resolve-dns here
//
// The handler reaches the daemon's DNS resolver through the handler slot in
// pkg/plugin/rpc rather than holding a *resolve.Resolvers of its own. That is
// not indirection for its own sake: the Component Boundaries table in
// docs/architecture/core-design.md admits aaa and audit into this package and
// nothing else, so a resolver field here would be the boundary violation the
// indirection exists to avoid. The hub owns the wiring, because the hub is
// where the single resolver is built.

package server

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// opResolveDNS answers a plugin's name lookup from the engine's single DNS
// resolver, so a plugin never builds a second one.
//
// An unregistered resolver is refused rather than answered with an empty
// record list. The two are different facts and a plugin cannot tell them
// apart: a firewall plugin reading an empty answer as "this name holds no
// address" would empty a live set because the hub was started without a
// resolver (ai/rules/principles.md).
func (s *Server) opResolveDNS(_ *process.Process, params json.RawMessage) (any, error) {
	var input rpc.ResolveDNSInput
	if err := json.Unmarshal(params, &input); err != nil {
		var tb textbuf.Buffer
		return nil, &rpc.RPCCallError{Message: tb.Str("invalid resolve-dns params: ").Err(err).String()}
	}
	if input.Name == "" {
		return nil, &rpc.RPCCallError{Message: "resolve-dns: no name given"}
	}
	fn := rpc.GetDNSResolver()
	if fn == nil {
		return nil, &rpc.RPCCallError{Message: "resolve-dns: no DNS resolver registered"}
	}
	records, ttl, status, err := fn(input.Name, input.Type)
	if err != nil {
		return nil, err
	}
	return &rpc.ResolveDNSOutput{Records: records, TTL: ttl, Status: status}, nil
}
