/* Ze SSE client -- single persistent EventSource with exponential backoff.
   Survives HTMX page swaps. Cleans up on page unload. */
(function(){
  'use strict';
  var es = null;
  var retryMs = 1000;
  var maxRetryMs = 30000;
  var retryTimer = null;

  function connect() {
    if (es) return;
    es = new EventSource('/events');

    es.addEventListener('config-change', function(e) {
      var bar = document.getElementById('notification-bar');
      if (bar && e.data) {
        // Parse server HTML safely via DOMParser (no script execution).
        var doc = new DOMParser().parseFromString(e.data, 'text/html');
        bar.textContent = '';
        while (doc.body.firstChild) {
          bar.appendChild(doc.body.firstChild);
        }
      }
      retryMs = 1000;
    });

    es.onopen = function() {
      retryMs = 1000;
    };

    es.onerror = function() {
      cleanup();
      retryTimer = setTimeout(function() {
        retryMs = Math.min(retryMs * 2, maxRetryMs);
        connect();
      }, retryMs);
    };
  }

  function cleanup() {
    if (es) {
      es.close();
      es = null;
    }
  }

  window.addEventListener('beforeunload', function() {
    clearTimeout(retryTimer);
    cleanup();
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', connect);
  } else {
    connect();
  }
})();
