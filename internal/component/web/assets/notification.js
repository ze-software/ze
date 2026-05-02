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

  document.addEventListener('DOMContentLoaded', function() {
    showQueryError();
    armExistingToasts();
  });
  document.addEventListener('htmx:afterSwap', armExistingToasts);
})();
