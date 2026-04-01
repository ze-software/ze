(function(){
  var t = localStorage.getItem('lg-theme');
  if (t) document.documentElement.setAttribute('data-theme', t);
})();

document.addEventListener('DOMContentLoaded', function() {
  // Tab active state -- update on HTMX navigation
  document.querySelectorAll('.tab-bar a').forEach(function(tab) {
    tab.addEventListener('click', function() {
      document.querySelectorAll('.tab-bar a').forEach(function(t) { t.classList.remove('tab-active'); });
      this.classList.add('tab-active');
    });
  });

  // Graph mode persistence -- restore from localStorage after HTMX swaps in new content.
  function applyGraphMode() {
    var mode = localStorage.getItem('lg-graph-mode') || 'aspath';
    var btns = document.querySelectorAll('.graph-mode-btn');
    if (btns.length === 0) return;
    btns.forEach(function(b) {
      b.classList.toggle('active', b.getAttribute('data-mode') === mode);
    });
    var container = document.getElementById('graph-container');
    if (container && container.getAttribute('data-loaded') !== 'true') {
      var prefix = btns[0].getAttribute('data-prefix');
      container.setAttribute('data-loaded', 'true');
      htmx.ajax('GET', '/lg/graph?prefix=' + encodeURIComponent(prefix) + '&mode=' + mode, {target: container, swap: 'innerHTML'});
    }
  }

  // Save mode on click.
  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.graph-mode-btn');
    if (!btn) return;
    localStorage.setItem('lg-graph-mode', btn.getAttribute('data-mode'));
  });

  // Apply after HTMX swaps (search results, navigation).
  document.addEventListener('htmx:afterSettle', applyGraphMode);
  applyGraphMode();

  var btn = document.getElementById('theme-toggle');
  if (btn) btn.addEventListener('click', function() {
    var h = document.documentElement;
    var c = h.getAttribute('data-theme');
    var n = c === 'dark' ? 'light' : c === 'light' ? 'dark'
      : window.matchMedia('(prefers-color-scheme:dark)').matches ? 'light' : 'dark';
    h.setAttribute('data-theme', n);
    localStorage.setItem('lg-theme', n);
  });
});
