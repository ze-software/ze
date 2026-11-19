/* Design: docs/architecture/fleet-interface.md -- behaviour of the fleet components
 *
 * Everything is attached through data attributes and delegated from the
 * document, because htmx replaces the content region and a listener bound to
 * an element would be lost with it. No markup is built here: a toast, a menu
 * and the confirm dialog are drawn by their templates. The one exception is
 * the error toast of a confirmed action whose control is gone, which no
 * server answer exists to carry (confirmLost).
 *
 * A server answer adds a toast by carrying the ToastOOB component: htmx
 * appends it to #fleet-toasts, and the observer below arms it on arrival.
 */
(function() {
  'use strict';

  var TOASTS_SHOWN_MAX = 4;
  var COPIED_SHOWN_MS = 1500;

  function closest(node, selector) {
    return node && node.closest ? node.closest(selector) : null;
  }

  // --- Row selection and the bulk bar ---------------------------------------

  var lastChecked = new WeakMap();

  function tableOf(node) {
    return closest(node, '[data-fleet-table]');
  }

  function rowBoxes(table) {
    return Array.prototype.slice.call(table.querySelectorAll('[data-fleet-select-row]'));
  }

  function matchesField(formId) {
    var form = document.getElementById(formId);
    return form ? form.querySelector('[data-fleet-select-matches-field]') : null;
  }

  function refreshTable(table) {
    var formId = table.getAttribute('data-fleet-table');
    if (!formId) return;

    var boxes = rowBoxes(table);
    var checked = 0;
    boxes.forEach(function(box) {
      if (box.checked) checked++;
      var row = closest(box, 'tr');
      if (row) row.classList.toggle('fleet-table-row--selected', box.checked);
    });

    var page = table.querySelector('[data-fleet-select-page]');
    if (page) {
      page.checked = boxes.length > 0 && checked === boxes.length;
      page.indeterminate = checked > 0 && checked < boxes.length;
    }

    var field = matchesField(formId);
    var everyMatch = field !== null && field.value === '1';

    document.querySelectorAll('[data-fleet-selected-count]').forEach(function(count) {
      var selection = closest(count, '[data-fleet-selection]');
      if (!selection || selection.getAttribute('data-fleet-selection') !== formId) return;
      count.textContent = everyMatch ? count.getAttribute('data-fleet-total') : String(checked);
    });

    document.querySelectorAll('[data-fleet-bulk-action]').forEach(function(action) {
      if (action.form && action.form.id === formId) action.disabled = checked === 0;
    });
  }

  function refreshTables() {
    document.querySelectorAll('[data-fleet-table]').forEach(refreshTable);
  }

  function setEveryMatch(formId, on) {
    var field = matchesField(formId);
    if (field) field.value = on ? '1' : '';
  }

  function setAll(table, checked) {
    rowBoxes(table).forEach(function(box) { box.checked = checked; });
  }

  function onRowBox(box, shift) {
    var table = tableOf(box);
    if (!table) return;

    var boxes = rowBoxes(table);
    var last = lastChecked.get(table);
    if (shift && last && boxes.indexOf(last) >= 0) {
      var from = boxes.indexOf(last);
      var to = boxes.indexOf(box);
      for (var index = Math.min(from, to); index <= Math.max(from, to); index++) {
        boxes[index].checked = box.checked;
      }
    }

    lastChecked.set(table, box);
    // A selection changed by hand is no longer "every match".
    setEveryMatch(table.getAttribute('data-fleet-table'), false);
    refreshTable(table);
  }

  function onSelectButton(button) {
    var selection = closest(button, '[data-fleet-selection]');
    if (!selection) return;

    var formId = selection.getAttribute('data-fleet-selection');
    var how = button.getAttribute('data-fleet-select');
    document.querySelectorAll('[data-fleet-table]').forEach(function(table) {
      if (table.getAttribute('data-fleet-table') !== formId) return;
      setAll(table, how !== 'none');
      setEveryMatch(formId, how === 'matches');
      refreshTable(table);
    });
  }

  // SELECTION_DONE is the response header of an action carried out on the
  // rows its form posted (handler_action.go actionSelectionDoneHeader).
  var SELECTION_DONE = 'X-Fleet-Selection-Done';

  // clearDoneSelection unticks the rows of a bulk form whose action was
  // carried out: they did what they were ticked for, and a ticked row holds
  // the live refresh of its region (fleet.js holdsState), which fleet.js
  // releases after this request. A refusal leaves the rows ticked, for the
  // operator to try again.
  function clearDoneSelection(event) {
    // htmx names the form it posted, or the submit button that posted it.
    var form = event.target && event.target.form ? event.target.form : event.target;
    if (!form || !form.matches || !form.matches('[data-fleet-bulk-form]')) return;
    var ctx = event.detail && event.detail.ctx;
    if (!ctx || !ctx.response || !ctx.response.headers) return;
    if (ctx.response.headers.get(SELECTION_DONE) !== 'true') return;

    setEveryMatch(form.id, false);
    document.querySelectorAll('[data-fleet-table]').forEach(function(table) {
      if (table.getAttribute('data-fleet-table') !== form.id) return;
      setAll(table, false);
      refreshTable(table);
    });
  }

  // --- Menus and overlays ----------------------------------------------------

  function closeMenu(menu) {
    var list = menu.querySelector('.fleet-menu-list');
    var button = menu.querySelector('[data-fleet-menu-button]');
    if (list) list.hidden = true;
    if (button) button.setAttribute('aria-expanded', 'false');
  }

  function openMenus() {
    return Array.prototype.slice.call(document.querySelectorAll('[data-fleet-menu]')).filter(function(menu) {
      var list = menu.querySelector('.fleet-menu-list');
      return list && !list.hidden;
    });
  }

  function toggleMenu(menu) {
    var list = menu.querySelector('.fleet-menu-list');
    var button = menu.querySelector('[data-fleet-menu-button]');
    if (!list || !button) return;

    var opening = list.hidden;
    openMenus().forEach(closeMenu);
    list.hidden = !opening;
    button.setAttribute('aria-expanded', opening ? 'true' : 'false');
  }

  function toggleOverlay(id) {
    var overlay = document.getElementById(id);
    if (overlay) overlay.hidden = !overlay.hidden;
  }

  // --- Copy controls ---------------------------------------------------------

  function showCopied(button) {
    var done = button.querySelector('.fleet-copy-done');
    if (!done) return;
    done.hidden = false;
    setTimeout(function() { done.hidden = true; }, COPIED_SHOWN_MS);
  }

  function copy(button) {
    var text = button.getAttribute('data-fleet-copy') || '';
    if (!navigator.clipboard || !navigator.clipboard.writeText) {
      // The clipboard is offered on a secure origin only. Say so on the
      // control instead of pretending the text was copied.
      button.title = 'Copying needs HTTPS or localhost. Select the value and copy it by hand.';
      return;
    }

    navigator.clipboard.writeText(text).then(function() {
      showCopied(button);
    }, function() {
      button.title = 'The browser refused the copy. Select the value and copy it by hand.';
    });
  }

  // --- Toasts ----------------------------------------------------------------

  function toastArea() {
    return document.getElementById('fleet-toasts');
  }

  function startLifetime(toast) {
    if (toast.dataset.fleetArmed === 'true') return;
    toast.dataset.fleetArmed = 'true';

    var seconds = parseInt(toast.getAttribute('data-toast-lifetime'), 10) || 0;
    if (seconds > 0) setTimeout(function() { toast.remove(); }, seconds * 1000);
  }

  // The newest toasts show, at most four. The older ones wait, counted in the
  // line above, and a toast's lifetime starts when it shows.
  function arrangeToasts() {
    var area = toastArea();
    if (!area) return;

    var toasts = Array.prototype.slice.call(area.querySelectorAll('[data-fleet-toast]'));
    var waiting = Math.max(toasts.length - TOASTS_SHOWN_MAX, 0);
    toasts.forEach(function(toast, index) {
      toast.hidden = index < waiting;
      if (!toast.hidden) startLifetime(toast);
    });

    var more = document.querySelector('[data-fleet-toast-more]');
    if (!more) return;
    more.hidden = waiting === 0;
    more.textContent = waiting === 1 ? '1 more notification' : waiting + ' more notifications';
  }

  function watchToasts() {
    var area = toastArea();
    if (!area || area.dataset.fleetWatched === 'true') return;
    area.dataset.fleetWatched = 'true';
    new MutationObserver(arrangeToasts).observe(area, { childList: true });
    arrangeToasts();
  }

  // --- Confirm dialog --------------------------------------------------------

  var confirming = null;

  function slot(dialog, name) {
    return dialog.querySelector('[data-confirm-slot="' + name + '"]');
  }

  // The text a destructive confirmation asks for may hold {count}, the number
  // of selected rows of the control's form: what its selection row shows,
  // which is the total while every match is selected.
  function expectedText(control) {
    var expect = control.getAttribute('data-confirm-expect') || '';
    if (expect.indexOf('{count}') < 0) return expect;

    var formId = control.form ? control.form.id : control.getAttribute('form');
    var count = '0';
    document.querySelectorAll('[data-fleet-selection]').forEach(function(selection) {
      if (selection.getAttribute('data-fleet-selection') !== formId) return;
      var shown = selection.querySelector('[data-fleet-selected-count]');
      if (shown) count = shown.textContent;
    });

    return expect.split('{count}').join(count);
  }

  function openConfirm(control) {
    var dialog = document.getElementById('fleet-confirm');
    if (!dialog || typeof dialog.showModal !== 'function') return false;

    var expect = expectedText(control);
    var destructive = control.getAttribute('data-confirm-kind') === 'destructive';

    slot(dialog, 'action').textContent = control.getAttribute('data-confirm-action') || '';
    slot(dialog, 'targets').textContent = control.getAttribute('data-confirm-targets') || '';
    slot(dialog, 'detail').textContent = control.getAttribute('data-confirm-detail') || '';
    slot(dialog, 'expect').textContent = expect;
    slot(dialog, 'expect-block').hidden = expect === '';
    slot(dialog, 'input').value = '';
    slot(dialog, 'confirm').disabled = expect !== '';
    dialog.classList.toggle('fleet-confirm--destructive', destructive);
    dialog.returnValue = '';

    confirming = control;
    dialog.showModal();
    if (expect !== '') slot(dialog, 'input').focus();

    return true;
  }

  function onConfirmInput(input) {
    var dialog = closest(input, '#fleet-confirm');
    if (!dialog) return;

    var expect = slot(dialog, 'expect').textContent;
    slot(dialog, 'confirm').disabled = input.value.trim() !== expect;
  }

  // CONFIRM_IDENTITY names the attributes that tell one confirmable control
  // from another: the same action, on the same targets, of the same form,
  // with the same field.
  var CONFIRM_IDENTITY = ['id', 'form', 'name', 'value', 'data-confirm-action', 'data-confirm-targets'];

  function sameControl(a, b) {
    return CONFIRM_IDENTITY.every(function(name) {
      return a.getAttribute(name) === b.getAttribute(name);
    });
  }

  // controlInPage answers the control the operator confirmed, as the page
  // holds it now. A refresh that landed while the dialog was open replaced
  // the control the dialog was opened from, and a detached button submits
  // nothing when clicked: the one drawn in its place is found again.
  function controlInPage(saved) {
    if (saved.isConnected) return saved;

    var found = null;
    document.querySelectorAll('[data-confirm-action]').forEach(function(control) {
      if (!found && sameControl(control, saved)) found = control;
    });
    return found;
  }

  // confirmLost says, in an error toast, that the confirmed action could not
  // be sent, so a confirmed action never vanishes in silence.
  function confirmLost(action) {
    var area = toastArea();
    if (!area) return;

    var toast = document.createElement('div');
    toast.className = 'fleet-toast fleet-toast--error';
    toast.setAttribute('role', 'alert');
    toast.setAttribute('data-fleet-toast', 'true');
    toast.setAttribute('data-fleet-confirm-lost', 'true');
    toast.setAttribute('data-toast-lifetime', '0');

    var body = document.createElement('div');
    body.className = 'fleet-toast-body';
    var message = document.createElement('p');
    message.className = 'fleet-toast-message';
    var word = document.createElement('span');
    word.className = 'fleet-toast-word';
    word.textContent = 'Error';
    message.appendChild(word);
    message.appendChild(document.createTextNode(' "' + action + '" was not sent: the page changed while it was being confirmed.'));
    var detail = document.createElement('p');
    detail.className = 'fleet-toast-detail';
    detail.textContent = 'The control it was confirmed from is no longer on the page, or is disabled now. Check the page as it is, then start the action again.';
    body.appendChild(message);
    body.appendChild(detail);

    var close = document.createElement('button');
    close.type = 'button';
    close.className = 'fleet-icon-button';
    close.setAttribute('data-fleet-toast-close', 'true');
    close.setAttribute('aria-label', 'Dismiss this notification');
    close.title = 'Dismiss';
    close.textContent = 'x';

    toast.appendChild(body);
    toast.appendChild(close);
    area.appendChild(toast);
  }

  function onConfirmClose(dialog) {
    var saved = confirming;
    confirming = null;
    if (!saved || dialog.returnValue !== 'confirm') return;

    // The typed text is judged again here: the confirm button is the only
    // way to answer "confirm", and it is disabled until the text matches.
    var expect = slot(dialog, 'expect').textContent;
    if (expect !== '' && slot(dialog, 'input').value.trim() !== expect) return;

    var control = controlInPage(saved);
    if (!control || control.disabled) {
      confirmLost(saved.getAttribute('data-confirm-action') || 'The action');
      return;
    }

    control.dataset.fleetConfirmed = 'true';
    control.click();
  }

  // --- Wiring ----------------------------------------------------------------

  function onClick(event) {
    var target = event.target;

    var confirmable = closest(target, '[data-confirm-action]');
    if (confirmable) {
      if (confirmable.dataset.fleetConfirmed === 'true') {
        delete confirmable.dataset.fleetConfirmed;
      } else if (openConfirm(confirmable)) {
        event.preventDefault();
        event.stopPropagation();
        return;
      }
    }

    var menuButton = closest(target, '[data-fleet-menu-button]');
    if (menuButton) {
      toggleMenu(closest(menuButton, '[data-fleet-menu]'));
      return;
    }

    // A click outside a menu closes it, and so does choosing one of its items.
    var inside = closest(target, '[data-fleet-menu]');
    openMenus().forEach(function(menu) {
      if (menu !== inside || closest(target, '.fleet-menu-item')) closeMenu(menu);
    });

    var copier = closest(target, '[data-fleet-copy]');
    if (copier) {
      copy(copier);
      return;
    }

    var selector = closest(target, '[data-fleet-select]');
    if (selector) {
      onSelectButton(selector);
      return;
    }

    var toggler = closest(target, '[data-fleet-toggle]');
    if (toggler) {
      toggleOverlay(toggler.getAttribute('data-fleet-toggle'));
      return;
    }

    var toastClose = closest(target, '[data-fleet-toast-close]');
    if (toastClose) {
      var toast = closest(toastClose, '[data-fleet-toast]');
      if (toast) toast.remove();
      return;
    }

    if (target.matches && target.matches('[data-fleet-select-row]')) {
      onRowBox(target, event.shiftKey);
    }
  }

  function onChange(event) {
    var target = event.target;
    if (!target.matches) return;

    if (target.matches('[data-fleet-select-page]')) {
      var table = tableOf(target);
      if (!table) return;
      setAll(table, target.checked);
      setEveryMatch(table.getAttribute('data-fleet-table'), false);
      refreshTable(table);
      return;
    }

    if (target.matches('[data-fleet-submit]') && target.form) {
      target.form.requestSubmit();
    }
  }

  function onInput(event) {
    var target = event.target;
    if (target.matches && target.matches('[data-confirm-slot="input"]')) onConfirmInput(target);
  }

  function onKeydown(event) {
    if (event.key !== 'Escape') return;

    var menus = openMenus();
    if (menus.length > 0) {
      menus.forEach(function(menu) {
        closeMenu(menu);
        var button = menu.querySelector('[data-fleet-menu-button]');
        if (button) button.focus();
      });
      return;
    }

    document.querySelectorAll('[data-fleet-overlay]').forEach(function(overlay) {
      overlay.hidden = true;
    });
  }

  // refresh settles what a page load or a swap brought: the selection counts,
  // the fallback buttons the script replaces, the toast area and the dialog.
  function refresh() {
    refreshTables();
    document.querySelectorAll('[data-fleet-submit-fallback]').forEach(function(button) {
      button.hidden = true;
    });
    watchToasts();

    var dialog = document.getElementById('fleet-confirm');
    if (dialog && dialog.dataset.fleetWatched !== 'true') {
      dialog.dataset.fleetWatched = 'true';
      dialog.addEventListener('close', function() { onConfirmClose(dialog); });
    }
  }

  document.addEventListener('click', onClick, true);
  document.addEventListener('change', onChange);
  document.addEventListener('input', onInput);
  document.addEventListener('keydown', onKeydown);
  document.addEventListener('htmx:after:swap', refresh);
  document.addEventListener('htmx:after:request', clearDoneSelection);

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', refresh);
  } else {
    refresh();
  }
})();
