// Design: docs/architecture/api/commands.md — local-data commands, the route a handler answers on
// Related: local_data.go — the routes that set it
// Related: argbind.go — the value that carries it to the handler
//
// route.go names who a local-data handler answers for. One handler serves two
// routes: the operator's own `ze` process, and the daemon answering a caller
// in another process. Most handlers answer both alike. A handler that reads
// what only the operator's process holds (its standard input, a program it
// names by path) asks the route, and refuses on the daemon.

package command

import "context"

// Route says which process a local-data handler is answering for. The route
// that calls the handler sets it on the ValidatedArgs it passes, and only
// this package can set it.
type Route uint8

const (
	// RouteUnspecified is the zero value: a ValidatedArgs no local-data route
	// marked. A handler reads it as RouteRemote, so a route that forgot to
	// mark its call refuses the caller-only forms rather than running them.
	RouteUnspecified Route = iota
	// RouteLocal is the operator's own `ze` process: `ze <verb>`, and
	// `ze cli -c` when the client answers the command itself.
	RouteLocal
	// RouteRemote is the daemon answering a caller in another process,
	// through the dispatcher every surface shares.
	RouteRemote
)

// CallerLocal reports whether the handler runs in the operator's own
// process, so the operator's standard input, files and programs are its to
// read and run. Only RouteLocal answers true: RouteRemote and the unmarked
// RouteUnspecified both refuse the caller-only forms.
func (r Route) CallerLocal() bool {
	return r == RouteLocal
}

// Route answers the route that called the handler holding this value.
func (v ValidatedArgs) Route() Route {
	return v.route
}

// onRoute answers a copy of v marked as called on route. The token slice and
// the bindings are shared, which is safe: a ValidatedArgs is never written
// after ValidateArgs returns it.
func (v ValidatedArgs) onRoute(route Route) ValidatedArgs {
	v.route = route
	return v
}

// AnswerRemote runs a local-data handler for a caller in another process: the
// daemon route. The handler sees RouteRemote, and its answer comes back as a
// value for the route to hand to that caller; nothing is printed.
func AnswerRemote(handler LocalDataHandler, validated ValidatedArgs) LocalAnswer {
	return handler(validated.onRoute(RouteRemote))
}

// WithContext answers a copy of v that carries the context of the caller's
// request. The daemon dispatcher sets it before it answers a local-data
// command for a caller in another process, so a handler that collects for
// long (the support archive) stops when that caller goes. A nil ctx leaves
// the copy without one, which Context answers as the process lifetime.
func (v ValidatedArgs) WithContext(ctx context.Context) ValidatedArgs {
	v.request = ctx
	return v
}

// Context answers the context of the request the handler answers. On the
// daemon route it is the caller's request context (WithContext), so
// a caller that disconnects or times out stops the work. Every other route
// answers context.Background, deliberately: there the work runs in the
// operator's own process, and it lives exactly as long as that process.
func (v ValidatedArgs) Context() context.Context {
	if v.request != nil {
		return v.request
	}
	return context.Background()
}

// AnswerLocal runs a local-data handler for the operator's own process and
// answers its value without printing it: the handler sees RouteLocal, as it
// does under `ze <verb>`. A caller that renders the answer itself, a test
// among them, uses it to state the route rather than leave it unmarked, which
// a handler reads as the daemon route.
func AnswerLocal(handler LocalDataHandler, validated ValidatedArgs) LocalAnswer {
	return handler(validated.onRoute(RouteLocal))
}
