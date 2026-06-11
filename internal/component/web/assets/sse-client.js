/* Ze SSE client -- single persistent EventSource with exponential backoff.
   Survives HTMX page swaps. Cleans up on page unload.
   Other scripts register listeners via window.zeSSE.on(type, fn). */
(function(){
  'use strict';
  var es = null;
  var retryMs = 1000;
  var maxRetryMs = 30000;
  var retryTimer = null;
  var listeners = {};
  var openCallbacks = [];
  var errorCallbacks = [];

  function connect() {
    if (es) return;
    es = new EventSource('/events');

    es.addEventListener('config-change', function(e) {
      var bar = document.getElementById('notification-bar');
      if (bar && e.data) {
        var doc = new DOMParser().parseFromString(e.data, 'text/html');
        bar.textContent = '';
        while (doc.body.firstChild) {
          bar.appendChild(doc.body.firstChild);
        }
      }
      retryMs = 1000;
    });

    Object.keys(listeners).forEach(function(type) {
      es.addEventListener(type, listeners[type]);
    });

    es.onopen = function() {
      retryMs = 1000;
      openCallbacks.forEach(function(fn) { fn(); });
    };

    es.onerror = function() {
      errorCallbacks.forEach(function(fn) { fn(); });
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

  window.zeSSE = {
    on: function(type, fn) {
      listeners[type] = fn;
      if (es) es.addEventListener(type, fn);
    },
    off: function(type) {
      if (es && listeners[type]) es.removeEventListener(type, listeners[type]);
      delete listeners[type];
    },
    onOpen: function(fn) { openCallbacks.push(fn); },
    onError: function(fn) { errorCallbacks.push(fn); }
  };

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
