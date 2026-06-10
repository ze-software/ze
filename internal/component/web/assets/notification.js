/* Ze notification toasts. Kept in an external asset so CSP does not need unsafe-inline. */
(function() {
  'use strict';

  function armToast(toast) {
    if (!toast || toast.dataset.zeArmed === 'true') return;
    var countdown = toast.querySelector('.notification-countdown');
    var close = toast.querySelector('.notification-close');
    if (!countdown || !close) return;

    toast.dataset.zeArmed = 'true';
    var seconds = parseInt(countdown.textContent, 10) || 30;
    var timer = null;

    function tick() {
      seconds--;
      countdown.textContent = String(seconds);
      if (seconds <= 0) {
        toast.remove();
        return;
      }
      timer = setTimeout(tick, 1000);
    }

    timer = setTimeout(tick, 1000);
    countdown.addEventListener('click', function() {
      if (timer) clearTimeout(timer);
      timer = null;
      countdown.textContent = 'paused';
      countdown.title = 'paused';
    });
    close.addEventListener('click', function() {
      if (timer) clearTimeout(timer);
      toast.remove();
    });
  }

  function armExistingToasts() {
    document.querySelectorAll('.notification').forEach(armToast);
  }

  function showQueryError() {
    var params = new URLSearchParams(window.location.search);
    var err = params.get('error');
    if (!err) return;
    var area = document.getElementById('notification-area');
    if (!area) return;

    var toast = document.createElement('div');
    toast.className = 'notification notification-error';
    toast.id = 'notif-toast';

    var msg = document.createElement('span');
    msg.className = 'notification-message';
    msg.textContent = err;

    var countdown = document.createElement('span');
    countdown.className = 'notification-countdown';
    countdown.textContent = '30';

    var close = document.createElement('button');
    close.className = 'notification-close';
    close.textContent = 'x';

    toast.appendChild(msg);
    toast.appendChild(countdown);
    toast.appendChild(close);
    area.appendChild(toast);
    armToast(toast);
    history.replaceState(null, '', window.location.pathname);
  }

  // showErrorToast renders a persistent (no auto-dismiss) error toast. Used for
  // htmx responses that htmx would otherwise drop silently. textContent escapes
  // the message, which may contain server/validator text derived from user input.
  function showErrorToast(message) {
    var area = document.getElementById('notification-area');
    if (!area) return;

    var toast = document.createElement('div');
    toast.className = 'notification notification-error';

    var msg = document.createElement('span');
    msg.className = 'notification-message';
    msg.textContent = message;

    var close = document.createElement('button');
    close.className = 'notification-close';
    close.textContent = 'x';
    close.addEventListener('click', function() { toast.remove(); });

    toast.appendChild(msg);
    toast.appendChild(close);
    area.appendChild(toast);
  }

  // Global htmx error handler: htmx does not swap on non-2xx responses and fires
  // htmx:responseError instead. Without this, failed actions (commit validation
  // errors, tool 400s) vanish with no UI feedback. Surface the response body as
  // a retained error toast so no action fails silently.
  function handleResponseError(evt) {
    var xhr = evt.detail && evt.detail.xhr;
    if (!xhr) return;
    var body = (xhr.responseText || '').trim();
    var prefix = 'Error ' + xhr.status + ': ';
    showErrorToast(body ? prefix + body : prefix + 'request failed');
  }

  document.addEventListener('DOMContentLoaded', function() {
    showQueryError();
    armExistingToasts();
  });
  document.addEventListener('htmx:afterSwap', armExistingToasts);
  document.addEventListener('htmx:responseError', handleResponseError);
  document.addEventListener('htmx:sendError', handleResponseError);
})();
