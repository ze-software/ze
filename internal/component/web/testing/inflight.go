// Design: docs/architecture/testing/runner-architecture.md -- browser request observation
// Related: runner.go -- waitLoad polls the observer installed here

package webtesting

import (
	"context"
	"fmt"
	"strings"
)

// describePending reads the requests the observer still holds open, after the
// case deadline ended the wait. The case context is done by then, so the read
// runs under its own short bound detached from that cancellation, the same way
// Close bounds cleanup. It answers an error in every case: the pending list, or
// why the list could not be read.
func (b *Browser) describePending() error {
	parent := b.ctx
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), pendingReadDeadline)
	b.ctx = ctx
	defer func() {
		cancel()
		b.ctx = parent
	}()
	out, err := b.runAgentOutput("eval", inflightPendingExpr)
	if err != nil {
		return fmt.Errorf("pending browser requests could not be read: %w", err)
	}
	return fmt.Errorf("pending browser requests: %s", strings.TrimSpace(out))
}

// inflightInitJS instruments fetch, XHR and finite HTMX request lifetimes before
// navigation through the native daemon's addinitscript action. EventSource is
// deliberately left alone. Generic fetch settles when Response headers arrive;
// HTMX retains its body processing and awaited swap until finally:request, so
// the counter pairs htmx:before:request with htmx:finally:request. HTMX awaits
// the hx-sse extension's promise before finally:request: by default hx-sse
// releases it at once for an element with hx-sse:connect, so that stream
// releases at once, and holds other text/event-stream responses until they
// end, so those count as finite work until they end. Every counted request is
// also recorded in window.__zePending by kind, method and URL until it ends, so
// a wait that reaches the case deadline can name what it waited for
// (inflightPendingExpr).
const inflightInitJS = `(function () {
  if (window.__zeWaitInstalled) return;
  window.__zeWaitInstalled = true;
  window.__zeInflight = 0;
  window.__zeLastChange = performance.now();
  window.__zePending = {};
  var pendingSeq = 0;
  function mark() { window.__zeLastChange = performance.now(); }
  function begin(kind, method, url) {
    var id = ++pendingSeq;
    window.__zePending[id] = {kind: kind, method: String(method || 'GET').toUpperCase(), url: String(url), at: performance.now()};
    window.__zeInflight++; mark();
    return id;
  }
  function end(id) { delete window.__zePending[id]; window.__zeInflight--; mark(); }
  // Capture sees cancellation in a later application listener and completion
  // rehomed to document when the source element has been disconnected.
  var htmxRequests = new WeakMap();
  document.addEventListener('htmx:before:request', function (event) {
    var ctx = event.detail && event.detail.ctx;
    if (!ctx || typeof ctx !== 'object' || htmxRequests.has(ctx)) return;
    var request = ctx.request || {};
    htmxRequests.set(ctx, begin('htmx', request.method, request.action));
  }, true);
  document.addEventListener('htmx:finally:request', function (event) {
    var ctx = event.detail && event.detail.ctx;
    if (!ctx || typeof ctx !== 'object' || !htmxRequests.has(ctx)) return;
    var id = htmxRequests.get(ctx);
    htmxRequests.delete(ctx);
    end(id);
  }, true);
  var origFetch = window.fetch;
  if (origFetch) {
    window.fetch = function (input, init) {
      var url = typeof input === 'string' ? input : (input && input.url) || String(input);
      var method = (init && init.method) || (input && typeof input === 'object' && input.method);
      var id = begin('fetch', method, url);
      return origFetch.apply(this, arguments).finally(function () { end(id); });
    };
  }
  var origOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url) {
    this.__zeMethod = method; this.__zeURL = url;
    return origOpen.apply(this, arguments);
  };
  var origSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function () {
    var id = begin('xhr', this.__zeMethod, this.__zeURL);
    this.addEventListener('loadend', function () { end(id); });
    return origSend.apply(this, arguments);
  };
})();
`

// inflightPendingExpr lists the requests inflightInitJS still holds open, one
// "kind METHOD url pending Nms" entry each, for the error a wait that reached
// the case deadline returns.
const inflightPendingExpr = `window.__zeWaitInstalled===true ? JSON.stringify(Object.keys(window.__zePending||{}).map(function(k){var p=window.__zePending[k];return p.kind+' '+p.method+' '+p.url+' pending '+Math.round(performance.now()-p.at)+'ms';})) : 'request observation is not installed'`

// inflightIdleExpr is the predicate waitLoad polls: no finite fetch/XHR/HTMX work
// and quiet for a short debounce. The debounce bridges chained requests --
// cli.js issues fetch(/cli) -> htmx refresh -> fetch(/config/changes) in
// sequence, so the counter dips to zero for a microtask between them.
const inflightIdleExpr = `window.__zeWaitInstalled===true && Number.isFinite(window.__zeInflight) && Number.isFinite(window.__zeLastChange) && window.__zeInflight===0 && (performance.now()-window.__zeLastChange)>=120`
