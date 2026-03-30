// Design: docs/architecture/web-interface.md -- LG static assets
// Overview: server.go -- LG server and route registration

package lg

// lgStyleCSS is the embedded CSS for the looking glass UI.
const lgStyleCSS = `
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: #333; background: #f5f7fa; }
header { background: #2a3f54; padding: 0.5rem 1rem; }
nav { display: flex; align-items: center; gap: 1.5rem; max-width: 1200px; margin: 0 auto; }
nav a { color: #fff; text-decoration: none; font-size: 0.9rem; opacity: 0.85; }
nav a:hover { opacity: 1; }
nav .nav-brand { font-weight: bold; font-size: 1.1rem; opacity: 1; margin-right: 1rem; }
main { max-width: 1200px; margin: 1.5rem auto; padding: 0 1rem; }
h1 { margin-bottom: 1rem; color: #2a3f54; }

.search-form { display: flex; gap: 0.5rem; align-items: end; margin-bottom: 1rem; flex-wrap: wrap; }
.search-form label { font-weight: 500; font-size: 0.9rem; }
.search-form input { padding: 0.4rem 0.6rem; border: 1px solid #ccc; border-radius: 4px; font-size: 0.9rem; min-width: 250px; }
.search-form button { padding: 0.4rem 1rem; background: #4a90d9; color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 0.9rem; }
.search-form button:hover { background: #3a7bc8; }

table { width: 100%; border-collapse: collapse; background: #fff; border-radius: 6px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
thead { background: #f0f4f8; }
th { padding: 0.6rem 0.8rem; text-align: left; font-size: 0.85rem; font-weight: 600; color: #555; border-bottom: 2px solid #ddd; }
td { padding: 0.5rem 0.8rem; font-size: 0.85rem; border-bottom: 1px solid #eee; }
tbody tr:hover { background: #f8fafc; cursor: pointer; }

.state-up .state { color: #22863a; font-weight: 600; }
.state-down .state { color: #cb2431; font-weight: 600; }
.state-unknown .state { color: #999; }

.peer-info { display: flex; gap: 1rem; align-items: center; margin-bottom: 1rem; padding: 0.8rem; background: #fff; border-radius: 6px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }

.detail-panel { padding: 1rem; background: #f8fafc; border-left: 3px solid #4a90d9; }
.detail-panel h3 { margin-bottom: 0.5rem; }
.detail-panel dl { display: grid; grid-template-columns: 160px 1fr; gap: 0.3rem; }
.detail-panel dt { font-weight: 600; color: #555; }
.detail-panel dd { color: #333; }

.route-detail { background: #f8fafc; }

#graph-container { margin: 1rem 0; }
#graph-container svg { max-width: 100%; height: auto; }

.error-banner { padding: 0.8rem 1rem; background: #fff3cd; border: 1px solid #ffc107; border-radius: 6px; color: #856404; margin-bottom: 1rem; font-weight: 500; }

@media (max-width: 768px) {
  .search-form { flex-direction: column; align-items: stretch; }
  .search-form input { min-width: auto; }
  table { font-size: 0.8rem; }
  th, td { padding: 0.4rem; }
}
`

// htmxMinJS is a minimal HTMX shim for the looking glass.
// It handles hx-get, hx-post, hx-target, and hx-swap for basic functionality.
// All HTML content is server-rendered via Go html/template which auto-escapes,
// so the DOM insertion here processes only trusted server output.
const htmxMinJS = `
(function(){
  function process(elt) {
    var triggers = elt.querySelectorAll('[hx-get],[hx-post]');
    triggers.forEach(function(el) {
      if (el._htmxBound) return;
      el._htmxBound = true;
      var event = el.tagName === 'FORM' ? 'submit' : 'click';
      el.addEventListener(event, function(e) {
        e.preventDefault();
        var method = el.getAttribute('hx-post') ? 'POST' : 'GET';
        var url = el.getAttribute('hx-post') || el.getAttribute('hx-get');
        var target = el.getAttribute('hx-target');
        var swap = el.getAttribute('hx-swap') || 'outerHTML';
        var push = el.getAttribute('hx-push-url');
        var targetEl = target ? document.querySelector(target) : el;
        var opts = {method: method, headers: {'HX-Request': 'true'}};
        if (method === 'POST' && el.tagName === 'FORM') {
          opts.body = new FormData(el);
        }
        fetch(url, opts)
          .then(function(r) { return r.text(); })
          .then(function(html) {
            if (targetEl) {
              // Server-rendered content from Go html/template (auto-escaped).
              if (swap === 'afterend') {
                targetEl.insertAdjacentHTML('afterend', html);
              } else {
                targetEl.outerHTML = html;
              }
            }
            if (push === 'true') { history.pushState({}, '', url); }
            process(document.body);
          });
      });
    });
  }
  document.addEventListener('DOMContentLoaded', function() { process(document.body); });
})();
`
