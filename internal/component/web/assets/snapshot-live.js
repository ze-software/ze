/* Ze snapshot live view -- replaces the contents of each [data-snapshot-stream]
   element with the payload of the named SSE event. The page names its stream
   and its event in data attributes, so this file carries no per-page state and
   the page carries no inline script, which script-src 'self' refuses. */
(function(){
  'use strict';
  var panes = document.querySelectorAll('[data-snapshot-stream]');
  if (!panes.length) return;

  var sources = [];

  Array.prototype.forEach.call(panes, function(pane) {
    var path = pane.getAttribute('data-snapshot-stream');
    var name = pane.getAttribute('data-snapshot-event');
    if (!path || !name) return;

    var es = new EventSource(path);
    es.addEventListener(name, function(e) {
      pane.textContent = e.data;
    });
    sources.push(es);
  });

  window.addEventListener('beforeunload', function() {
    sources.forEach(function(es) { es.close(); });
  });
})();
