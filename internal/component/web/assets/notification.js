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

  // errorMessageFromBody reads the sentence out of a refusal. A handler answers
  // a failed htmx action with an error fragment (component_error_fragment.templ),
  // so the body is markup and the toast must not repeat it verbatim.
  //
  // DOMParser runs no script and loads no subresource, and only textContent is
  // read, so the markup never becomes live nodes. A body that is not markup is
  // returned unchanged, which is what every non-htmx client and any handler
  // writing plain text still sends.
  function errorMessageFromBody(body) {
    if (body.charAt(0) !== '<') return body;

    var parsed = new DOMParser().parseFromString(body, 'text/html');
    var message = parsed.querySelector('.error-fragment-message');
    if (message) return message.textContent.trim();

    return (parsed.body.textContent || '').trim();
  }

  // Global htmx error handler: htmx 4 fires htmx:response:error for a response
  // whose status is 400 or more. Without this, a failed action (a commit
  // validation error, a tool 400) reaches the operator only on a page that
  // swaps the error fragment into view. Surface the response body as a
  // retained error toast so no action fails silently.
  //
  // htmx 4 requests with fetch, so no event carries an xhr. Every event carries
  // detail.ctx instead: ctx.text is the body, and ctx.response holds the status.
  // A failure with no response at all leaves ctx.response absent, and this
  // reports nothing rather than reporting a status it does not have.
  function handleResponseError(evt) {
    var ctx = evt.detail && evt.detail.ctx;
    if (!ctx || !ctx.response) return;
    var body = errorMessageFromBody((ctx.text || '').trim());
    var prefix = 'Error ' + ctx.response.status + ': ';
    showErrorToast(body ? prefix + body : prefix + 'request failed');
  }

  // errorText is the sentence an error carries. htmx passes the thrown value
  // through untouched, and a failed fetch throws a TypeError whose message is
  // the only readable part of it.
  function errorText(err) {
    if (!err) return 'no reason given';

    return err.message || String(err);
  }

  // htmx 4 has ONE error event, and htmx 2's sendError is gone. htmx:error fires
  // when the request never left (a target that resolves to nothing, an hx-on
  // body that threw), when the fetch failed, when the timeout aborted it, and
  // when the swap threw. htmx 2 named only the network case, so the causes are
  // told apart here rather than reported as one.
  //
  // The ctx says which one happened. A request that got no answer leaves
  // ctx.response absent: it never reached the daemon, or nothing came back. A
  // ctx.response of 400 or more was already reported by handleResponseError
  // through htmx:response:error, and a second toast for one refusal tells the
  // operator the action failed twice.
  function handleRequestError(evt) {
    var detail = evt.detail || {};
    var ctx = detail.ctx;

    if (ctx && ctx.response) {
      if (ctx.response.status < 400) {
        showErrorToast('The page could not be updated: ' + errorText(detail.error));
      }

      return;
    }

    showErrorToast('The request did not reach the server: ' + errorText(detail.error));
  }

  document.addEventListener('DOMContentLoaded', function() {
    showQueryError();
    armExistingToasts();
  });
  document.addEventListener('htmx:after:swap', armExistingToasts);
  document.addEventListener('htmx:response:error', handleResponseError);
  document.addEventListener('htmx:error', handleRequestError);
})();
