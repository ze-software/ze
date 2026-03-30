// Design: docs/architecture/web-interface.md -- LG static assets
// Overview: server.go -- LG server and route registration

package lg

// lgStyleCSS is the embedded CSS for the looking glass UI.
// Supports light and dark modes via prefers-color-scheme.
const lgStyleCSS = `
:root {
  --bg: #f5f7fa;
  --text: #333;
  --heading: #2a3f54;
  --header-bg: #2a3f54;
  --header-text: #fff;
  --table-bg: #fff;
  --table-head: #f0f4f8;
  --table-border: #ddd;
  --table-row-border: #eee;
  --table-hover: #f8fafc;
  --th-text: #555;
  --input-border: #ccc;
  --input-bg: #fff;
  --input-text: #333;
  --btn-bg: #4a90d9;
  --btn-hover: #3a7bc8;
  --detail-bg: #f8fafc;
  --detail-border: #4a90d9;
  --detail-dt: #555;
  --detail-dd: #333;
  --peer-info-bg: #fff;
  --shadow: rgba(0,0,0,0.1);
  --state-up: #22863a;
  --state-down: #cb2431;
  --state-unknown: #999;
  --error-bg: #fff3cd;
  --error-border: #ffc107;
  --error-text: #856404;
}

@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1a1b1e;
    --text: #d4d4d8;
    --heading: #93b3d4;
    --header-bg: #1e2a38;
    --header-text: #e0e0e0;
    --table-bg: #25272b;
    --table-head: #2a2d32;
    --table-border: #3a3d42;
    --table-row-border: #333639;
    --table-hover: #2e3136;
    --th-text: #9ca3af;
    --input-border: #4a4d52;
    --input-bg: #2a2d32;
    --input-text: #d4d4d8;
    --btn-bg: #4a90d9;
    --btn-hover: #5a9ee6;
    --detail-bg: #2a2d32;
    --detail-border: #4a90d9;
    --detail-dt: #9ca3af;
    --detail-dd: #d4d4d8;
    --peer-info-bg: #25272b;
    --shadow: rgba(0,0,0,0.3);
    --state-up: #3fb950;
    --state-down: #f85149;
    --state-unknown: #6e7681;
    --error-bg: #3b2e00;
    --error-border: #9e6a00;
    --error-text: #f0c000;
  }
}

* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: var(--text); background: var(--bg); }
header { background: var(--header-bg); padding: 0.5rem 1rem; }
nav { display: flex; align-items: center; gap: 1.5rem; max-width: 1200px; margin: 0 auto; }
nav a { color: var(--header-text); text-decoration: none; font-size: 0.9rem; opacity: 0.85; }
nav a:hover { opacity: 1; }
nav .nav-brand { font-weight: bold; font-size: 1.1rem; opacity: 1; margin-right: 1rem; }
main { max-width: 1200px; margin: 1.5rem auto; padding: 0 1rem; }
h1 { margin-bottom: 1rem; color: var(--heading); }

.search-form { display: flex; gap: 0.5rem; align-items: end; margin-bottom: 1rem; flex-wrap: wrap; }
.search-form label { font-weight: 500; font-size: 0.9rem; }
.search-form input { padding: 0.4rem 0.6rem; border: 1px solid var(--input-border); border-radius: 4px; font-size: 0.9rem; min-width: 250px; background: var(--input-bg); color: var(--input-text); }
.search-form button { padding: 0.4rem 1rem; background: var(--btn-bg); color: #fff; border: none; border-radius: 4px; cursor: pointer; font-size: 0.9rem; }
.search-form button:hover { background: var(--btn-hover); }

table { width: 100%; border-collapse: collapse; background: var(--table-bg); border-radius: 6px; overflow: hidden; box-shadow: 0 1px 3px var(--shadow); }
thead { background: var(--table-head); }
th { padding: 0.6rem 0.8rem; text-align: left; font-size: 0.85rem; font-weight: 600; color: var(--th-text); border-bottom: 2px solid var(--table-border); }
td { padding: 0.5rem 0.8rem; font-size: 0.85rem; border-bottom: 1px solid var(--table-row-border); }
tbody tr:hover { background: var(--table-hover); cursor: pointer; }

.state-up .state { color: var(--state-up); font-weight: 600; }
.state-down .state { color: var(--state-down); font-weight: 600; }
.state-unknown .state { color: var(--state-unknown); }

.peer-info { display: flex; gap: 1rem; align-items: center; margin-bottom: 1rem; padding: 0.8rem; background: var(--peer-info-bg); border-radius: 6px; box-shadow: 0 1px 3px var(--shadow); }

.detail-panel { padding: 1rem; background: var(--detail-bg); border-left: 3px solid var(--detail-border); }
.detail-panel h3 { margin-bottom: 0.5rem; }
.detail-panel dl { display: grid; grid-template-columns: 160px 1fr; gap: 0.3rem; }
.detail-panel dt { font-weight: 600; color: var(--detail-dt); }
.detail-panel dd { color: var(--detail-dd); }

.route-detail { background: var(--detail-bg); }

#graph-container { margin: 1rem 0; }
#graph-container svg { max-width: 100%; height: auto; }

.error-banner { padding: 0.8rem 1rem; background: var(--error-bg); border: 1px solid var(--error-border); border-radius: 6px; color: var(--error-text); margin-bottom: 1rem; font-weight: 500; }
.empty-state { padding: 2rem; text-align: center; color: var(--state-unknown); font-style: italic; }

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
