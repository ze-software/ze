/* Ze Live Log -- streams daemon events into #log-stream via the shared SSE
   connection (window.zeSSE). Pause/resume via button. Cleans up listener on
   HTMX page swap so events stop flowing when the user navigates away. */
(function(){
  'use strict';
  var stream = document.querySelector('[data-log-live]');
  if (!stream || !window.zeSSE) return;

  var status = document.getElementById('log-status');
  var paused = false;
  var maxEntries = 500;

  window.zeSSE.on('log-entry', function(e) {
    if (paused || !e.data) return;
    if (status) { status.remove(); status = null; }
    var div = document.createElement('div');
    div.className = 'wb-log-entry';
    div.textContent = e.data;
    stream.appendChild(div);
    while (stream.children.length > maxEntries) stream.removeChild(stream.firstChild);
    stream.scrollTop = stream.scrollHeight;
  });

  window.zeSSE.onOpen(function() {
    if (status) status.textContent = 'Connected. Waiting for events...';
  });

  window.zeSSE.onError(function() {
    if (status) status.textContent = 'Connection lost. Reconnecting...';
  });

  var btn = document.getElementById('log-pause');
  if (btn) btn.addEventListener('click', function() {
    paused = !paused;
    btn.textContent = paused ? 'Resume' : 'Pause';
  });

  document.addEventListener('htmx:before:swap', function cleanup() {
    document.removeEventListener('htmx:before:swap', cleanup);
    window.zeSSE.off('log-entry');
  });
})();
