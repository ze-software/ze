/* Ze CLI bar -- Tab/? autocomplete, clickable completions.
   Completions replace the last partial token, not append. */
(function(){
  'use strict';

  // Register a default Trusted Types policy to silence Chromium console warnings
  // about innerHTML/outerHTML assignments in our code and HTMX. This is cosmetic --
  // Trusted Types are not enforced by our CSP, so the pass-through adds no security
  // but removes noise that could alarm users inspecting the console.
  if (window.trustedTypes && window.trustedTypes.createPolicy) {
    if (!window.trustedTypes.defaultPolicy) {
      try {
        window.trustedTypes.createPolicy('default', {
          createHTML: function(s) { return s; },
          createScript: function(s) { return s; },
          createScriptURL: function(s) { return s; }
        });
      } catch (e) {
        // Policy already created or blocked by CSP -- safe to ignore.
      }
    }
  }

  var cachedItems = null;  // Last fetched completions for live filtering.
  var cachedPrefix = '';   // Prefix (up to last space) when completions were fetched.
  var selectedIndex = -1;  // Currently highlighted completion item (-1 = none).

  function newSubmission() {
    if (window.crypto.randomUUID) return window.crypto.randomUUID();
    var words = new Uint32Array(4);
    window.crypto.getRandomValues(words);
    return Array.from(words, function(word) { return word.toString(16); }).join('-');
  }

  // Both inputs consume the same server projection. Only drafts and navigation
  // belong to the page; command text is never appended to a local recall list.
  function commandRecall(input, getMode, showWarning) {
    function scope() { return {entries: [], position: -1, draft: '', request: 0, controller: null}; }
    var scopes = {config: scope(), operational: scope()};
    var mode = getMode();
    var revision = 0;
    var pending = null;
    var readWarning = '';
    var writeWarning = '';

    function warn() {
      showWarning([readWarning, writeWarning].filter(function(value, index, values) {
        return value && values.indexOf(value) === index;
      }).join('\n'));
    }

    function refresh() {
      var requestedMode = mode;
      var state = scopes[requestedMode];
      var request = ++state.request;
      if (state.controller) state.controller.abort();
      state.controller = new AbortController();
      fetch('/cli?history=1&mode=' + encodeURIComponent(requestedMode), {
        credentials: 'same-origin', cache: 'no-store', signal: state.controller.signal
      }).then(function(response) {
        if (!response.ok) return response.text().then(function(message) { throw new Error(message); });
        return response.json();
      }).then(function(data) {
        if (request !== state.request || requestedMode !== mode) return;
        if (!data || !Array.isArray(data.entries) || !data.entries.every(function(entry) { return typeof entry === 'string'; })) {
          throw new Error('The server returned invalid command recall.');
        }
        state.entries = data.entries.slice();
        state.position = Math.min(state.position, state.entries.length - 1);
        readWarning = '';
        warn();
        if (!pending || pending.revision !== revision || pending.mode !== mode || input.value !== pending.draft) return;
        var browse = pending;
        pending = null;
        if (state.entries.length) {
          state.position = Math.min(browse.steps, state.entries.length) - 1;
          input.value = state.entries[state.entries.length - 1 - state.position];
          revision++;
        }
        if (browse.accept) browse.accept();
      }).catch(function(error) {
        if (error.name === 'AbortError' || request !== state.request || requestedMode !== mode) return;
        var accept = pending && pending.revision === revision && input.value === pending.draft ? pending.accept : null;
        pending = null;
        readWarning = 'Command recall is unavailable. Commands can still run.\n' + error.message + '\nReload /cli and try Up again';
        warn();
        if (accept) accept();
      });
    }

    function edited() {
      revision++;
      pending = null;
      scopes[mode].position = -1;
      scopes[mode].draft = input.value;
    }

    input.addEventListener('input', edited);
    refresh();
    return {
      up: function() {
        var state = scopes[mode];
        if (pending) { pending.steps++; return; }
        if (state.position < 0) {
          state.draft = input.value;
          pending = {mode: mode, revision: revision, draft: input.value, steps: 1};
          refresh();
          return;
        }
        if (state.position < state.entries.length - 1) state.position++;
        input.value = state.entries[state.entries.length - 1 - state.position];
        revision++;
      },
      down: function() {
        if (pending) {
          if (--pending.steps === 0) pending = null;
          return;
        }
        var state = scopes[mode];
        if (state.position < 0) return;
        state.position--;
        input.value = state.position < 0 ? state.draft : state.entries[state.entries.length - 1 - state.position];
        revision++;
      },
      submitted: function() {
        input.value = '';
        edited();
      },
      ready: function(accept) {
        // Enter waits for the first Up lookup. New input cancels that Enter.
        if (pending) pending.accept = accept;
        else accept();
      },
      edited: edited,
      modeChanged: function() {
        var next = getMode();
        if (next === mode) return;
        var previous = scopes[mode];
        if (previous.position < 0 && !pending) previous.draft = input.value;
        previous.position = -1;
        pending = null;
        mode = next;
        var state = scopes[mode];
        state.position = -1;
        if (!input.value) input.value = state.draft;
        else state.draft = input.value;
        revision++;
      },
      refresh: refresh,
      warning: function(message) { writeWarning = message || ''; warn(); }
    };
  }

  function showBox(box) {
    if (box) box.classList.remove('is-hidden');
  }

  function hideBox(box) {
    if (box) box.classList.add('is-hidden');
  }

  function isBoxVisible(box) {
    return !!box && !box.classList.contains('is-hidden');
  }

  // Read CLI context path from the hidden element (updated via OOB swaps).
  function getContextPath() {
    var el = document.getElementById('cli-context-path');
    return el ? el.textContent.trim() : '';
  }

  function init() {
    var input = document.getElementById('cli-input');
    var box = document.getElementById('cli-completions');
    if (!input || !box) return;
    var barQueue = Promise.resolve();
    var barHistoryWarning = '';
    var recall = commandRecall(input, function() { return 'config'; }, function(message) {
      var notification = document.getElementById('notification-bar');
      if (notification && (message || notification.textContent === barHistoryWarning)) notification.textContent = message;
      barHistoryWarning = message;
    });

    input.addEventListener('keydown', function(e) {
      var dropdownVisible = isBoxVisible(box);

      // Arrow keys: dropdown navigation when visible, history when hidden.
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (dropdownVisible) {
          var items = box.querySelectorAll('.cli-completion-item');
          if (items.length === 0) return;
          if (selectedIndex > 0) selectedIndex--;
          else selectedIndex = items.length - 1;
          highlightItem(box, selectedIndex);
          return;
        }
        recall.up();
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (dropdownVisible) {
          var items = box.querySelectorAll('.cli-completion-item');
          if (items.length === 0) return;
          if (selectedIndex < items.length - 1) selectedIndex++;
          else selectedIndex = 0;
          highlightItem(box, selectedIndex);
          return;
        }
        recall.down();
        return;
      }

      if (e.key === 'Enter') {
        e.preventDefault();
        // If dropdown is visible and an item is selected, accept it.
        if (dropdownVisible && selectedIndex >= 0) {
          var items = box.querySelectorAll('.cli-completion-item');
          if (selectedIndex < items.length) {
            items[selectedIndex].click();
            return;
          }
        }
        hideBox(box);
        cachedItems = null;
        selectedIndex = -1;
        recall.ready(function() {
          var cmd = input.value.trim();
          if (!cmd) return;
          var submission = newSubmission();
          recall.submitted();

          barQueue = barQueue.then(function() {
            if (!window.htmx) throw new Error('The command interface has not loaded; reload this page.');
            return htmx.ajax('POST', '/cli', {
              target: '#content-area', swap: 'outerHTML',
              values: {command: cmd, path: getContextPath(), submission: submission}
            });
          }).then(function() { recall.refresh(); refreshCommitBar(); }).catch(function(err) {
            var notification = document.getElementById('notification-bar');
            if (notification) notification.textContent = err.message;
          });
        });
        return;
      }

      if (e.key === 'Tab' || (e.key === '?' && !e.ctrlKey && !inputQuoteOpen(input.value.slice(0, input.selectionStart)))) {
        e.preventDefault();
        recall.edited();
        var val = input.value;
        if (e.key === '?') {
          val = val.replace(/\?+$/, '');
          if (!val.endsWith(' ')) val += ' ';
        }
        doComplete(input, box, val);
        return;
      }

      if (e.key === 'Escape') {
        recall.edited();
        hideBox(box);
        cachedItems = null;
        selectedIndex = -1;
        return;
      }

      // Live filter: after a small delay, filter cached completions as user types.
      if (cachedItems && e.key.length === 1) {
        setTimeout(function() { filterCached(input, box); }, 0);
      }
    });

    // Also filter on input event (covers paste, backspace via input event).
    input.addEventListener('input', function() {
      if (cachedItems) filterCached(input, box);
    });
    document.addEventListener('htmx:after:request', function(event) {
      var ctx = event.detail && event.detail.ctx;
      if (!ctx || !ctx.request || !ctx.response || !ctx.response.raw) return;
      if (new URL(ctx.request.action, window.location.href).pathname !== '/cli') return;
      recall.warning(ctx.response.raw.headers.get('X-Ze-History-Warning'));
    });
  }

  // Match the configuration codec's quote boundaries when replacing a token.
  function inputQuoteOpen(value) {
    var quote = '';
    for (var i = 0; i < value.length; i++) {
      var ch = value[i];
      if (quote === '"' && ch === '\\') { i++; continue; }
      if (quote) { if (ch === quote) quote = ''; }
      else if (ch === '"' || ch === "'") quote = ch;
    }
    return quote !== '';
  }

  function splitInput(val) {
    var lastSpace = -1;
    var quote = '';
    for (var i = 0; i < val.length; i++) {
      var ch = val[i];
      if (quote === '"' && ch === '\\') { i++; continue; }
      if (quote) { if (ch === quote) quote = ''; }
      else if (ch === '"' || ch === "'") quote = ch;
      else if (/\s/.test(ch)) lastSpace = i;
    }
    if (lastSpace < 0) return { prefix: '', partial: val };
    return { prefix: val.substring(0, lastSpace + 1), partial: val.substring(lastSpace + 1) };
  }

  function doComplete(input, box, val) {
    var completePath = getContextPath();
    fetch('/cli/complete?input=' + encodeURIComponent(val) + '&path=' + encodeURIComponent(completePath), {credentials: 'same-origin'})
      .then(completionResponse)
      .then(function(items){
        if (!items || items.length === 0) {
          hideBox(box);
          cachedItems = null;
          return;
        }

        var parts = splitInput(val);
        cachedItems = items;
        cachedPrefix = parts.prefix;

        if (items.length === 1) {
          input.value = cachedPrefix + items[0].text + ' ';
          hideBox(box);
          cachedItems = null;
          input.dispatchEvent(new Event('input'));
          return;
        }

        showBarCompletions(input, box, items, cachedPrefix);
      })
      .catch(function(err){
        hideBox(box);
        var notification = document.getElementById('notification-bar');
        if (notification) notification.textContent = 'Completion failed: ' + err.message;
      });
  }

  // Checked completion failures carry the daemon's correction as plain text.
  function completionResponse(response) {
    if (!response.ok) return response.text().then(function(message) {
      throw new Error(message || 'Completion is unavailable. Submit the command or try Tab again.');
    });
    return response.json();
  }

  function filterCached(input, box) {
    if (!cachedItems) return;
    var parts = splitInput(input.value);
    var partial = parts.partial.toLowerCase();

    // If prefix changed (user typed a space = new token), clear cache.
    if (parts.prefix !== cachedPrefix) {
      cachedItems = null;
      hideBox(box);
      return;
    }

    var filtered = cachedItems.filter(function(c) {
      return c.text.toLowerCase().indexOf(partial) === 0;
    });

    if (filtered.length === 0) {
      hideBox(box);
      return;
    }

    if (filtered.length === 1 && filtered[0].text.toLowerCase() === partial) {
      // Exact match, nothing more to show.
      hideBox(box);
      return;
    }

    showBarCompletions(input, box, filtered, cachedPrefix);
  }

  function highlightItem(box, index) {
    var items = box.querySelectorAll('.cli-completion-item');
    for (var i = 0; i < items.length; i++) {
      items[i].classList.toggle('selected', i === index);
    }
    if (index >= 0 && index < items.length) {
      items[index].scrollIntoView({ block: 'nearest' });
    }
  }

  function showBarCompletions(input, box, items, prefix) {
    selectedIndex = 0;
    showCompletions(input, box, items, prefix, function() {
      cachedItems = null;
      selectedIndex = -1;
    });
  }

  // Rendering is shared, but each input owns its cache and selected candidate.
  function showCompletions(input, box, items, prefix, onPick) {
    while (box.firstChild) box.removeChild(box.firstChild);
    items.forEach(function(c) {
      var div = document.createElement('div');
      div.className = 'cli-completion-item';
      div.dataset.explanation = c.explanation || '';
      var b = document.createElement('b');
      b.textContent = c.text;
      div.appendChild(b);
      if (c.description) {
        var sp = document.createElement('span');
        sp.textContent = ' ' + c.description;
        div.appendChild(sp);
      }
      div.addEventListener('click', function() {
        input.value = prefix + c.text + ' ';
        hideBox(box);
        onPick();
        input.dispatchEvent(new Event('input'));
        input.focus();
      });
      box.appendChild(div);
    });
    showBox(box);
    highlightItem(box, 0);
  }

  function initViewToggle() {
    // Retained as no-op; CLI is now a dedicated page at /cli.
  }

  // The error drawer opens itself when an error lands in it and closes itself
  // when the last one is dismissed. A handler answers a rejected edit with the
  // oob_error fragment, whose only swap appends to #error-list. No htmx swap
  // can set a class, so the collapsed state is held here.
  //
  // The toggle carries data-count while the list holds an item. style.css keys
  // on it to keep the toggle on screen as a tab when the operator closes a
  // panel that still holds errors, and to show the count; with no errors the
  // attribute is gone and the collapsed panel is fully hidden.
  function syncErrorPanel() {
    var panel = document.getElementById('error-panel');
    var list = document.getElementById('error-list');
    if (!panel || !list) return;
    var count = list.querySelectorAll('.error-item').length;
    panel.classList.toggle('collapsed', count === 0);
    var toggle = document.getElementById('error-toggle');
    if (!toggle) return;
    if (count > 0) {
      toggle.setAttribute('data-count', String(count));
    } else {
      toggle.removeAttribute('data-count');
    }
  }

  function initErrorPanel() {
    // dismiss-error dispatches this after removing an item.
    document.addEventListener('ze-error-update', syncErrorPanel);
    // htmx 4 dispatches htmx:after:settle once per SWAP TASK, on the element
    // that task swapped, and its detail carries the task rather than a target.
    // The error fragment appends out of band to #error-list, so the settle of
    // that append reports #error-list as the event's own target.
    //
    // htmx:after:swap is the wrong event here: htmx 4 fires it once per REQUEST
    // on the element that issued it, so a swap into #error-list is reported on
    // the field the operator was editing.
    document.addEventListener('htmx:after:settle', function(e) {
      if (e.target && e.target.id === 'error-list') syncErrorPanel();
    });
  }

  // Refresh the detail panel via fragment endpoint without full page reload.
  function refreshCommitBar() {
    fetch('/config/changes', { credentials: 'same-origin' })
      .then(function(r) { return r.text(); })
      .then(function(html) {
        var bar = document.getElementById('commit-bar');
        if (bar) {
          bar.outerHTML = html;
          var newBar = document.getElementById('commit-bar');
          if (newBar && window.htmx) htmx.process(newBar);
        }
      });
  }

  function refreshDetail(path) {
    if (!window.htmx) return;
    htmx.ajax('GET', '/fragment/detail?path=' + encodeURIComponent(path), {
      target: '#content-area',
      swap: 'innerHTML'
    });
  }

  // Block 'e/E/+/-' on number inputs (scientific notation).
  function initNumberInputs() {
    document.addEventListener('keydown', function(e) {
      if (e.target && e.target.type === 'number') {
        if (e.key === 'e' || e.key === 'E' || e.key === '+' || e.key === '-') {
          e.preventDefault();
        }
      }
    });
  }

  // Enter fires the ze-enter event on the control that asks for it.
  //
  // htmx compiles a bracketed trigger filter such as keyup[key=='Enter'] with
  // Function(). setSecurityHeaders (auth.go) sends script-src 'self' with no
  // unsafe-eval, so the compile throws, htmx reports htmx:syntax:error and
  // leaves the filter unset. A trigger with no filter ignores nothing, so the
  // bare key event fired on EVERY keystroke. An inline editor posted a partial
  // value and swapped itself away under the operator, and the terminal ran a
  // partial command.
  //
  // The markup names the event and this listener dispatches it. Delegation
  // survives every swap, and nothing here compiles source. A control whose own
  // handler already acted leaves defaultPrevented set, and keeps this one off.
  function initEnterSubmit() {
    document.addEventListener('keydown', function(e) {
      if (e.key !== 'Enter' || e.defaultPrevented) return;
      var el = e.target;
      if (!el || !el.getAttribute) return;
      var trigger = el.getAttribute('hx-trigger');
      if (!trigger || trigger.split(/[\s,]+/).indexOf('ze-enter') < 0) return;
      e.preventDefault();
      el.dispatchEvent(new CustomEvent('ze-enter'));
    });
  }

  // Delegated click handlers for data-action buttons (CSP-safe, no inline handlers).
  function initActions() {
    document.addEventListener('click', function(e) {
      var btn = e.target.closest('[data-action]');
      if (!btn) return;
      var action = btn.getAttribute('data-action');
      if (action === 'dismiss-banner') {
        var banner = btn.closest('.notification-banner');
        if (banner) banner.remove();
      } else if (action === 'dismiss-error') {
        var item = btn.parentNode;
        if (item) item.remove();
        document.dispatchEvent(new Event('ze-error-update'));
      } else if (action === 'toggle-error-panel') {
        var panel = document.getElementById('error-panel');
        if (panel) panel.classList.toggle('collapsed');
      } else if (action === 'dismiss-login') {
        var overlay = document.getElementById('login-overlay');
        if (overlay) overlay.remove();
      } else if (action === 'add-entry') {
        var baseURL = btn.getAttribute('data-base-url') || '/show/';
        showAddEntryOverlay(baseURL);
      } else if (action === 'close-add-overlay') {
        var addOverlay = document.getElementById('add-entry-overlay');
        if (addOverlay) addOverlay.remove();
      } else if (action === 'rename-entry') {
        var url = btn.getAttribute('data-url');
        var key = btn.getAttribute('data-key');
        showRenameOverlay(url, key);
      } else if (action === 'delete-entry') {
        var path = btn.getAttribute('data-path');
        var entryKey = btn.getAttribute('data-key');
        deleteEntry(path, entryKey);
      } else if (action === 'save-field-page') {
        if (document.activeElement && document.activeElement.blur) {
          document.activeElement.blur();
        }
        setTimeout(refreshCommitBar, 300);
      } else if (action === 'switch-ui') {
        var mode = btn.getAttribute('data-mode');
        if (mode) {
          document.cookie = 'ze-ui-mode=' + mode + ';path=/;max-age=31536000;samesite=lax';
          document.cookie = 'ze-ui=;path=/;max-age=0;samesite=lax';
        }
      } else if (action === 'toggle-theme') {
        var html = document.documentElement;
        var current = html.getAttribute('data-theme');
        var next = current === 'light' ? 'dark' : 'light';
        html.setAttribute('data-theme', next);
        try { localStorage.setItem('ze-theme', next); } catch(_) {}
      } else if (action === 'toggle-portal') {
        var dropdown = btn.parentNode.querySelector('.portal-dropdown');
        if (dropdown) {
          var visible = dropdown.classList.contains('portal-dropdown--open');
          dropdown.classList.toggle('portal-dropdown--open', !visible);
        }
        e.stopPropagation();
      }
    });

    document.addEventListener('click', function(e) {
      var openDropdown = document.querySelector('.portal-dropdown--open');
      if (openDropdown && !e.target.closest('.portal-menu')) {
        openDropdown.classList.remove('portal-dropdown--open');
      }
    });

    document.addEventListener('click', function(e) {
      if (e.target && e.target.classList && e.target.classList.contains('add-entry-overlay')) {
        e.target.remove();
      }
    });

    document.addEventListener('keydown', function(e) {
      if (e.key !== 'Escape') return;
      var overlay = document.getElementById('add-entry-overlay');
      if (overlay && e.target && e.target.closest('#add-entry-overlay')) {
        overlay.remove();
      }
    });

    document.addEventListener('change', function(e) {
      var control = e.target.closest('[data-action="submit-on-change"]');
      if (control && control.form) control.form.submit();
    });

    document.addEventListener('htmx:after:request', function(e) {
      if (e.target && e.target.matches('[data-close-on-after-request="true"]')) {
        var overlay = document.getElementById('add-entry-overlay');
        if (overlay) overlay.remove();
        setTimeout(refreshCommitBar, 300);
      }
      if (e.target && e.target.closest && e.target.closest('.wb-form-post')) {
        // htmx 4 carries the request context on every event and no longer
        // reports a boolean. A response the server answered at all is a
        // response with a status, and under 400 is what htmx 2 called
        // successful.
        var ctx = e.detail && e.detail.ctx;
        if (ctx && ctx.response && ctx.response.status < 400) {
          setTimeout(function() { window.location.reload(); }, 50);
        }
      }
    });
  }

  // Add-entry overlay: fetches the server-rendered form for the list at baseURL.
  // The server template handles keyed vs keyless lists, title, and field labels.
  // Keyless lists create the entry server-side and return a redirect (no form).
  function showAddEntryOverlay(baseURL) {
    var formURL = baseURL.replace(/^\/show\//, '/config/add-form/');
    if (document.getElementById('workbench-shell')) {
      formURL += (formURL.indexOf('?') === -1 ? '?' : '&') + 'ui=workbench';
    }
    fetch(formURL, { credentials: 'same-origin' })
      .then(function(r) {
        // Keyless lists: server creates entry and returns HX-Redirect.
        var redirect = r.headers.get('HX-Redirect');
        if (redirect) {
          window.location.href = redirect;
          return '';
        }
        return r.text();
      })
      .then(function(html) {
        if (!html) return;
        // Safe: HTML is server-rendered from our own templates, not user input.
        var wrapper = document.createElement('div');
        wrapper.innerHTML = html; // nosec: trusted same-origin server response
        var overlay = wrapper.firstElementChild;
        if (overlay) {
          document.body.appendChild(overlay);
          if (window.htmx) htmx.process(overlay);
          var input = overlay.querySelector('.add-entry-input');
          if (input) input.focus();
        }
      });
  }

  // Rename overlay: shows a modal to rename a list entry key.
  function showRenameOverlay(currentURL, currentKey) {
    var renameURL = (currentURL || '').replace(/^\/show\//, '/config/rename/');
    var overlay = document.createElement('div');
    overlay.className = 'add-entry-overlay';
    var card = document.createElement('div');
    card.className = 'add-entry-card';
    var heading = document.createElement('h3');
    heading.textContent = 'Rename ' + currentKey;
    var input = document.createElement('input');
    input.type = 'text';
    input.className = 'add-entry-input';
    input.value = currentKey;
    input.placeholder = 'new name';

    function submitRename() {
      var newName = input.value;
      if (!newName) return;
      fetch(renameURL, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          'HX-Request': 'true'
        },
        body: 'new-key=' + encodeURIComponent(newName)
      }).then(function(r) {
        var redirect = r.headers.get('HX-Redirect');
        if (!r.ok) {
          return r.text().then(function(text) {
            throw new Error(text || 'Rename failed');
          });
        }
        if (redirect) {
          window.location.href = redirect;
          return;
        }
        overlay.remove();
      }).catch(function(err) {
        window.alert(err.message || 'Rename failed');
        input.focus();
        input.select();
      });
    }

    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        submitRename();
      }
      if (e.key === 'Escape') overlay.remove();
    });
    card.appendChild(heading);
    card.appendChild(input);
    overlay.appendChild(card);
    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) overlay.remove();
    });
    document.body.appendChild(overlay);
    input.select();
  }

  // Delete a list entry and refresh the view.
  function deleteEntry(path, key) {
    if (!confirm('Delete ' + key + '?')) return;
    // Extract parent path (remove the key segment) for the delete command
    var parts = path.split('/');
    var entryKey = decodeURIComponent(parts.pop());
    if (entryKey === '~.') entryKey = '.';
    else if (entryKey.indexOf('~~') === 0) entryKey = entryKey.slice(1);
    var parentPath = parts.join('/');
    var deletePath = '/config/delete/' + parentPath + '/';
    fetch(deletePath, {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Content-Type': 'application/x-www-form-urlencoded'},
      body: 'leaf=' + encodeURIComponent(entryKey)
    }).then(function(r) {
      if (!r.ok) {
        // Surface the failure. Swallowing it is how a delete that never
        // reached the tree looked identical to one that succeeded.
        return r.text().then(function(text) {
          throw new Error(text || 'Delete failed');
        });
      }
      // Refresh the current view
      var curPath = window.location.pathname.replace(/^\/show\//, '').replace(/\/$/, '');
      refreshDetail(curPath);
    }).catch(function(err) {
      window.alert(err.message || 'Delete failed');
    });
  }

  // Sidebar flyout: position and show on hover over list groups.
  function initFlyout() {
    var activeFlyout = null;
    document.addEventListener('mouseenter', function(e) {
      var group = e.target.closest('.sidebar-list-group');
      if (!group) return;
      var flyout = group.querySelector('.sidebar-flyout');
      if (!flyout) return;
      if (activeFlyout && activeFlyout !== flyout) activeFlyout.classList.remove('visible');
      var rect = group.getBoundingClientRect();
      flyout.style.left = rect.right + 'px';
      flyout.style.top = rect.top + 'px';
      flyout.classList.add('visible');
      activeFlyout = flyout;
    }, true);

    document.addEventListener('mouseleave', function(e) {
      var group = e.target.closest('.sidebar-list-group');
      if (!group) return;
      // Delay hiding so mouse can move into the flyout.
      setTimeout(function() {
        var flyout = group.querySelector('.sidebar-flyout');
        if (flyout && !group.matches(':hover') && !flyout.matches(':hover')) {
          flyout.classList.remove('visible');
          if (activeFlyout === flyout) activeFlyout = null;
        }
      }, 150);
    }, true);
  }

  // Restore saved theme preference.
  function initTheme() {
    try {
      var saved = localStorage.getItem('ze-theme');
      if (saved) document.documentElement.setAttribute('data-theme', saved);
    } catch(_) {}
  }

  // Delegated handler for workbench tool-overlay close/cancel buttons.
  // CSP forbids inline event handlers and `unsafe-eval`, so HTMX `hx-on:click`
  // attributes do not work; this listener performs the same `node.remove()`
  // action via standard DOM events.
  function initToolOverlay() {
    document.addEventListener('click', function(e) {
      var t = e.target;
      if (!t || !t.classList) return;
      if (t.classList.contains('tool-overlay-close') ||
          t.classList.contains('tool-overlay-confirm-cancel')) {
        var overlay = t.closest('.tool-overlay');
        if (overlay) overlay.remove();
      }
    });
    document.addEventListener('htmx:after:request', function(e) {
      if (e.target && e.target.classList &&
          e.target.classList.contains('tool-overlay-confirm-form')) {
        var overlay = e.target.closest('.tool-overlay');
        if (overlay) overlay.remove();
      }
    });
  }

  // CLI page: SSH-style terminal with output viewport, message area, and input.
  // Output viewport replaces its content on each command (not a scrollback).
  // Message area shows feedback (line 1) and hints (line 2).
  function initTerminal() {
    var input = document.getElementById('terminal-input');
    if (!input) return;

    var outputEl = document.getElementById('cli-output');
    var feedbackEl = document.getElementById('cli-feedback');
    var hintEl = document.getElementById('cli-hint');
    var promptEl = document.getElementById('terminal-prompt');
    var contextEl = document.getElementById('cli-context-path');
    var modeEl = document.getElementById('cli-mode');
    var completionsBox = document.getElementById('terminal-completions');

    var termCachedItems = null;
    var termCachedPrefix = '';
    var termSelectedIndex = -1;
    var termQuerySequence = 0;
    var termExplanation = false;
    var termQueue = Promise.resolve();
    var loadPanel = document.getElementById('cli-load-panel');
    var activeRequest = null;
    var streaming = false;
    var requestGeneration = 0;
    var terminalClosed = false;
    var activeView = null;
    var viewControls = document.getElementById('cli-view-controls');
    var viewTemplate = document.getElementById('cli-view-control-template');
    var viewMediaType = 'application/x-ze-cli-view+ndjson';
    var outputMediaType = 'application/x-ze-cli-output+ndjson';
    var stopButton = document.getElementById('cli-stop');
    var downloadButton = document.querySelector('[data-cli-local-operator="save"]');
    var downloadAnchor = document.getElementById('cli-download-file');
    var downloadStatus = document.getElementById('cli-download-status');
    var outputBytesMax = 4 * 1024 * 1024;
    var downloadBytes = null;
    var downloadURL = '';
    var downloadType = 'text/plain;charset=utf-8';
    if (!loadPanel || !stopButton) return;
    if (downloadButton) downloadButton.addEventListener('click', function() {
      if (!downloadBytes || !downloadAnchor || terminalClosed) return;
      if (!downloadURL) downloadURL = URL.createObjectURL(new Blob([downloadBytes], {type: downloadType}));
      downloadAnchor.href = downloadURL;
      downloadAnchor.click();
    });
    stopButton.addEventListener('click', stopRequest);
    if (viewControls) viewControls.addEventListener('keydown', function(event) {
      if (event.ctrlKey && event.key.toLowerCase() === 'c' && activeRequest) {
        event.preventDefault();
        stopRequest();
        input.focus();
      }
    });
    window.addEventListener('pagehide', closeTerminal, {once: true});
    window.addEventListener('pageshow', function(event) {
      if (!event.persisted) return;
      terminalClosed = false;
      window.addEventListener('pagehide', closeTerminal, {once: true});
      document.addEventListener('htmx:before:cleanup', terminalCleanup);
    });
    document.addEventListener('htmx:before:cleanup', terminalCleanup);
    var historyWarning = '';
    var feedbackMessage = '';
    var feedbackError = false;
    var recall = commandRecall(input, getTermMode, function(message) {
      historyWarning = message;
      renderFeedback();
    });

    function getTermPath() {
      return contextEl ? contextEl.textContent.trim() : '';
    }

    function getTermMode() {
      return modeEl ? (modeEl.textContent.trim() || 'config') : 'config';
    }

    function setTermState(path, prompt, mode) {
      termQuerySequence++;
      termExplanation = false;
      setHint('');
      if (contextEl && path !== undefined && path !== null) contextEl.textContent = path;
      if (modeEl && mode) modeEl.textContent = mode;
      if (promptEl && prompt) promptEl.textContent = prompt;
      recall.modeChanged();
    }

    function setOutput(text) {
      if (!outputEl) return;
      if (text) {
        outputEl.textContent = text;
      } else {
        outputEl.textContent = '';
      }
    }

    function clearDownload() {
      if (downloadURL) URL.revokeObjectURL(downloadURL);
      downloadURL = '';
      downloadBytes = null;
      if (downloadAnchor) downloadAnchor.removeAttribute('href');
      if (downloadButton) downloadButton.disabled = true;
      if (downloadStatus) downloadStatus.textContent = '';
    }

    function offerFiniteDownload(data) {
      if (data['output-complete'] !== true || data.input || data.confirmation || data.navigate) return;
      var count = data['output-byte-count'];
      if (!Number.isSafeInteger(count) || count < 0) throw outputFailure('The finite result has an invalid byte count.');
      if (count > outputBytesMax) { refuseDownloadLimit(); return; }
      var carrier = data['output-bytes'];
      var bytes = finiteOutputBytes(carrier, count);
      var binary = carrier.blob.encoding === 'base64';
      offerDownloadBytes(bytes, 'Complete result', binary ? 'ze-command.bin' : 'ze-command.txt',
        binary ? 'application/octet-stream' : 'text/plain;charset=utf-8');
    }

    // Decode the command owner's Blob codec after the authoritative byte count
    // is bounded. Display text is never a substitute for a missing carrier.
    function finiteOutputBytes(carrier, count) {
      if (!carrier || typeof carrier !== 'object' || Array.isArray(carrier)) {
        throw outputFailure('The finite result has no valid byte carrier.');
      }
      var blob = carrier.blob;
      if (!blob || typeof blob !== 'object' || Array.isArray(blob) || typeof blob.content !== 'string') {
        throw outputFailure('The finite result has an invalid Blob document.');
      }
      if (blob.encoding === 'utf-8') {
        var textBytes = new Uint8Array(Math.min(count + 1, blob.content.length * 3));
        var encoded = new TextEncoder().encodeInto(blob.content, textBytes);
        if (encoded.read !== blob.content.length || encoded.written !== count) {
          throw outputFailure('The finite result does not match its declared byte count.');
        }
        return textBytes.subarray(0, encoded.written);
      }
      if (blob.encoding !== 'base64') throw outputFailure('The finite result uses an unknown Blob encoding.');
      if (blob.content.length !== Math.ceil(count / 3) * 4 ||
          !/^[A-Za-z0-9+/]*={0,2}$/.test(blob.content)) {
        throw outputFailure('The finite result contains invalid encoded bytes.');
      }
      var decoded;
      try { decoded = atob(blob.content); }
      catch (error) { throw outputFailure('The finite result contains invalid encoded bytes.'); }
      if (decoded.length !== count) throw outputFailure('The finite result does not match its declared byte count.');
      var bytes = new Uint8Array(count);
      // The checked count limits this copy to the same 4 MiB artifact budget.
      for (var index = 0; index < count; index++) bytes[index] = decoded.charCodeAt(index);
      return bytes;
    }

    function refuseDownloadLimit() {
      if (downloadStatus) downloadStatus.textContent = 'Download exceeds 4 MiB. No partial file was saved. Run a command that selects less output, then choose Download.';
    }

    function offerDownloadBytes(bytes, label, filename, mediaType) {
      clearDownload();
      if (!downloadButton || !downloadAnchor) return;
      downloadBytes = bytes;
      downloadType = mediaType;
      downloadAnchor.download = filename;
      downloadButton.disabled = false;
      if (downloadStatus) downloadStatus.textContent = label;
    }

    function offerTextDownload(text, label, filename) {
      if (!downloadButton || !downloadAnchor) return;
      // A bounded destination prevents a large finite answer from allocating
      // an unbounded second copy just to discover its UTF-8 byte size.
      var bytes = new Uint8Array(Math.min(outputBytesMax + 1, text.length * 3));
      var encoded = new TextEncoder().encodeInto(text, bytes);
      if (encoded.read !== text.length || encoded.written > outputBytesMax) {
        refuseDownloadLimit();
        return;
      }
      offerDownloadBytes(bytes.subarray(0, encoded.written), label, filename, 'text/plain;charset=utf-8');
    }

    function setFeedback(msg, isError) {
      feedbackMessage = msg || '';
      feedbackError = isError;
      renderFeedback();
    }

    function renderFeedback() {
      if (!feedbackEl) return;
      feedbackEl.textContent = [feedbackMessage, historyWarning].filter(Boolean).join('\n');
      feedbackEl.className = 'cli-feedback' + (feedbackError || historyWarning ? ' cli-feedback--error' : '');
    }

    function setHint(msg) {
      if (!hintEl) return;
      hintEl.textContent = msg || 'Tab/?: complete, Enter: execute';
    }

    function currentRequest(controller, generation) {
      return !terminalClosed && input.isConnected && activeRequest === controller &&
        requestGeneration === generation && !controller.signal.aborted;
    }

    function clearView() {
      activeView = null;
      if (viewControls) {
        viewControls.replaceChildren();
        viewControls.hidden = true;
      }
    }

    function stopRequest() {
      clearView();
      stopButton.hidden = true;
      if (activeRequest) activeRequest.abort();
    }

    function closeTerminal() {
      terminalClosed = true;
      requestGeneration++;
      clearDownload();
      stopRequest();
      document.removeEventListener('htmx:before:cleanup', terminalCleanup);
      window.removeEventListener('pagehide', closeTerminal);
    }

    function terminalCleanup(event) {
      if (event.target === input || (event.target.contains && event.target.contains(input))) closeTerminal();
    }

    function viewFailure(problem) {
      return new Error(problem + '\nLive updates have stopped.\nOpen /cli and run the monitor again');
    }

    function outputFailure(problem) {
      return new Error(problem + '\nNo complete download is available.\nOpen /cli and run the command again');
    }

    function renderViewControls(view) {
      if (activeView !== view || !currentRequest(view.controller, view.generation)) return;
      var focused = viewControls.contains(document.activeElement) ? document.activeElement.id : '';
      viewControls.replaceChildren();
      view.controls.forEach(function(control) {
        var button = viewTemplate.content.firstElementChild.cloneNode(true);
        button.id = 'cli-view-' + encodeURIComponent(control.id);
        button.textContent = control.label;
        button.disabled = view.stopping || (view.pending > 0 && control.id !== 'stop');
        button.addEventListener('click', function() { sendViewAction(view, control.id); });
        viewControls.appendChild(button);
        if (focused && focused === button.id && !button.disabled) button.focus();
      });
      viewControls.hidden = !view.controls.length;
    }

    function sendViewAction(view, action) {
      if (activeView !== view || !currentRequest(view.controller, view.generation) ||
          !view.controls.some(function(control) { return control.id === action; }) ||
          view.stopping || (view.pending && action !== 'stop')) return;
      view.pending++;
      if (action === 'stop') view.stopping = true;
      renderViewControls(view);
      return fetch('/cli/view/action', {
        method: 'POST', credentials: 'same-origin', signal: view.controller.signal,
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'handle=' + encodeURIComponent(view.handle) + '&action=' + encodeURIComponent(action)
      }).then(function(response) {
        if (!response.ok) return response.text().then(function(message) { throw new Error(message); });
      }).catch(function(error) {
        if (error.name !== 'AbortError' && activeView === view && currentRequest(view.controller, view.generation)) {
          if (action === 'stop') view.stopping = false;
          view.actionError = true;
          setFeedback(error.message, true);
        }
      }).finally(function() {
        view.pending--;
        renderViewControls(view);
      });
    }

    // Decode small byte slices so one network chunk cannot grow a retained text
    // or frame buffer without its bound being checked. TextDecoder retains an
    // incomplete UTF-8 character between slices and reads.
    function receiveStream(response, controller, generation, rich, framed) {
      var reader = response.body.getReader();
      var decoder = new TextDecoder('utf-8', {fatal: rich || framed});
      var dataDecoder = framed ? new TextDecoder('utf-8') : null;
      var text = '';
      var outputText = '';
      var truncated = false;
      var frameBytesMax = 64 * 1024;
      var charactersMax = 512 * 1024;
      var encoder = rich || framed ? new TextEncoder() : null;
      var rawBytes = framed ? new Uint8Array(outputBytesMax) : null;
      var rawOffset = 0;
      var rawDone = false;
      var rawFailed = false;
      var view = rich ? {controller: controller, generation: generation, handle: '', sequence: 0,
        controls: [], pending: 0, stopping: false, done: false, actionError: false} : null;
      if (rich) activeView = view;

      function frame(line) {
        if (line.length > frameBytesMax || encoder.encode(line).length + 1 > frameBytesMax) {
          throw viewFailure('The live monitor sent a frame larger than 64 KiB.');
        }
        var value;
        try { value = JSON.parse(line); }
        catch (error) { throw viewFailure('The live monitor sent an unreadable frame.'); }
        if (!value || typeof value.handle !== 'string' || !/^[a-f0-9]{64}$/.test(value.handle) ||
            !Number.isSafeInteger(value.sequence) || value.sequence < 1 || typeof value.text !== 'string' ||
            typeof value.done !== 'boolean' || (value.error !== undefined && typeof value.error !== 'string') ||
            (!Array.isArray(value.controls) && !(value.done && value.controls === null))) {
          throw viewFailure('The live monitor sent an invalid frame.');
        }
        var controls = value.controls || [];
        var ids = new Set();
        if (controls.length > 16 || !controls.every(function(control) {
          if (!control || typeof control.id !== 'string' || !control.id || control.id.length > 64 ||
              typeof control.label !== 'string' || control.label.length > 80 || ids.has(control.id)) return false;
          ids.add(control.id);
          return true;
        }) || (!value.done && !ids.has('stop'))) {
          throw viewFailure('The live monitor sent invalid controls.');
        }
        if (!currentRequest(controller, generation) || activeView !== view || (view.done && !value.done)) return;
        if (view.handle && value.handle !== view.handle) return;
        if (value.sequence <= view.sequence) return;
        view.handle = value.handle;
        view.sequence = value.sequence;
        view.done = value.done;
        view.controls = value.done ? [] : controls;
        if (value.text) {
          setOutput(value.text);
          if (!value.error) offerTextDownload(value.text, 'Current frame snapshot', 'ze-monitor-snapshot.txt');
        }
        renderViewControls(view);
        if (value.error) {
          clearDownload();
          view.actionError = true;
          setFeedback(value.error, true);
        } else if (value.done && !view.actionError) setFeedback('Stopped', false);
      }

      function rawFrame(line) {
        if (line.length + 1 > frameBytesMax || encoder.encode(line).length + 1 > frameBytesMax) {
          throw outputFailure('The live command sent a frame larger than 64 KiB.');
        }
        if (rawDone) throw outputFailure('The live command sent data after its final frame.');
        var value;
        try { value = JSON.parse(line); }
        catch (error) { throw outputFailure('The live command sent an unreadable frame.'); }
        if (!value || Array.isArray(value) || typeof value.done !== 'boolean' ||
            (value.error !== undefined && typeof value.error !== 'string')) {
          throw outputFailure('The live command sent an invalid frame.');
        }
        if (value.done) {
          if (value.data !== undefined) throw outputFailure('The live command mixed output with its final frame.');
          rawDone = true;
          rawFailed = !!value.error;
          if (rawFailed) {
            rawBytes = null;
            clearDownload();
            setFeedback(value.error, true);
          }
          return;
        }
        if (value.error !== undefined || typeof value.data !== 'string' ||
            !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value.data)) {
          throw outputFailure('The live command sent invalid encoded output.');
        }
        var binary;
        try { binary = atob(value.data); }
        catch (error) { throw outputFailure('The live command sent invalid encoded output.'); }
        if (binary.length > 32 * 1024) throw outputFailure('The live command sent an output chunk larger than 32 KiB.');
        if (rawOffset + binary.length > rawBytes.length) throw outputFailure('Live output reached its 4 MiB limit.');
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
        rawBytes.set(bytes, rawOffset);
        rawOffset += bytes.length;
        displayRaw(dataDecoder.decode(bytes, {stream: true}));
      }

      function displayRaw(value) {
        outputText += value;
        if (outputText.length > charactersMax) {
          outputText = outputText.slice(-charactersMax);
          // Do not retain the low half of a character cut by the tail bound.
          if (outputText.charCodeAt(0) >= 0xdc00 && outputText.charCodeAt(0) <= 0xdfff) outputText = outputText.slice(1);
          truncated = true;
        }
        setOutput((truncated ? '[Older output removed]\n' : '') + outputText);
      }

      function decoded(value) {
        if (!currentRequest(controller, generation)) return;
        if (rich || framed) {
          text += value;
          var newline;
          while ((newline = text.indexOf('\n')) >= 0) {
            if (framed) rawFrame(text.slice(0, newline));
            else frame(text.slice(0, newline));
            text = text.slice(newline + 1);
          }
          if (text.length > frameBytesMax || encoder.encode(text).length + 1 > frameBytesMax) {
            throw (framed ? outputFailure : viewFailure)('The live command sent a frame larger than 64 KiB.');
          }
          return;
        }
        displayRaw(value);
      }

      function decode(bytes, ongoing) {
        try { return decoder.decode(bytes, {stream: ongoing}); }
        catch (error) { throw (framed ? outputFailure : viewFailure)('The live command sent invalid UTF-8 text.'); }
      }

      async function receive() {
        // The request lifetime bounds the reader; await avoids retaining a
        // promise chain for every frame of an indefinitely running monitor.
        while (currentRequest(controller, generation)) {
          var part = await reader.read();
          if (!currentRequest(controller, generation)) return null;
          if (part.done) {
            decoded(decode(undefined, false));
            if (rich) {
              if (text || !view.done) throw viewFailure('The live monitor ended before its final frame.');
            } else if (framed) {
              if (text || !rawDone) throw outputFailure('The live command ended before its final frame.');
              displayRaw(dataDecoder.decode());
              if (!rawFailed) {
                offerDownloadBytes(rawBytes.subarray(0, rawOffset), 'Complete result', 'ze-stream-output.bin', 'application/octet-stream');
                setFeedback('Finished', false);
              }
            } else setFeedback('Finished', false);
            return null;
          }
          for (var offset = 0; offset < part.value.length; offset += 16 * 1024) {
            decoded(decode(part.value.subarray(offset, offset + 16 * 1024), true));
          }
        }
        return null;
      }
      return receive().finally(function() {
        rawBytes = null;
        // A malformed or stale stream also releases the server's request handle.
        return reader.cancel().catch(function() {}).finally(function() { reader.releaseLock(); });
      });
    }

    // A queued command reads context only after the preceding answer applied it.
    function submitTermCommand(cmd, content, submission, automatic) {
      clearDownload();
      submission = submission || newSubmission();
      termQueue = termQueue.then(function() {
        if (terminalClosed || !input.isConnected) return;
        var controller = new AbortController();
        var generation = ++requestGeneration;
        clearView();
        clearDownload();
        activeRequest = controller;
        function send(content, confirmed) {
        var body = 'command=' + encodeURIComponent(cmd) + '&path=' + encodeURIComponent(getTermPath()) + '&mode=' + encodeURIComponent(getTermMode());
        body += '&submission=' + encodeURIComponent(submission);
        if (automatic) body += '&history=0';
        if (confirmed) body += '&confirm=true';
        if (content !== undefined) body += '&content=' + encodeURIComponent(content);
        return fetch('/cli/stream', {
          method: 'POST', credentials: 'same-origin',
          headers: {'Content-Type': 'application/x-www-form-urlencoded', 'Accept': viewMediaType + ', ' + outputMediaType + ', application/json, text/plain'}, body: body,
          signal: controller.signal
        }).then(function(r) {
          if (!currentRequest(controller, generation)) return null;
          recall.warning(r.headers.get('X-Ze-History-Warning'));
          if (!r.ok) {
            recall.refresh();
            return r.text().then(function(text) { throw new Error(text); });
          }
          var mediaType = (r.headers.get('Content-Type') || '').split(';')[0].trim();
          if (mediaType === viewMediaType || mediaType === outputMediaType || mediaType === 'text/plain') {
            var rich = mediaType === viewMediaType;
            if (rich && (!viewControls || !viewTemplate)) throw viewFailure('The live monitor controls could not be opened.');
            streaming = true; stopButton.hidden = rich;
            setFeedback('Running — Stop or Ctrl+C ends the stream.', false);
            recall.refresh();
            return receiveStream(r, controller, generation, rich, mediaType === outputMediaType);
          }
          return r.json();
        }).then(function(data) {
          if (!data || !currentRequest(controller, generation)) return;
          setTermState(data.path, data.prompt, data.mode);
          recall.warning(data['history-warning']);
          if (data.confirmation) return collectConfirmation(data.confirmation, controller.signal).then(function(confirmed) {
            if (!currentRequest(controller, generation)) return;
            if (confirmed) return send(content, true);
            setFeedback('Canceled', false);
          });
          if (data.navigate) { window.location.assign(data.navigate); return; }
          setOutput(data.output || '');
          offerFiniteDownload(data);
          var isError = data.feedback && (data.feedback.indexOf('error') === 0 || data.feedback.indexOf('conflict') === 0);
          setFeedback(data.feedback ? '► ' + data.feedback : '', isError);
          recall.refresh();
          setHint('');
          if (data.input) return collectLoadContent(data.input).then(function(payload) {
            if (payload !== null && currentRequest(controller, generation)) return send(payload);
          });
        });
        }
        return send(content).finally(function() {
          if (activeRequest === controller) {
            clearView();
            activeRequest = null;
            streaming = false; stopButton.hidden = true;
          }
          controller.abort();
        });
      }).catch(function(err) {
        if (terminalClosed || !input.isConnected) return;
        clearDownload();
        if (err.name === 'AbortError') setFeedback('Stopped', false);
        else setFeedback('error: ' + err.message, true);
      });
      return termQueue;
    }

    function collectConfirmation(message, signal) {
      return new Promise(function(resolve) {
        var panel = document.getElementById('cli-confirm-panel');
        var prompt = document.getElementById('cli-confirm-prompt');
        var yes = document.getElementById('cli-confirm-yes');
        var cancel = document.getElementById('cli-confirm-cancel');
        var finished = false;
        input.disabled = true;
        prompt.textContent = message;
        panel.hidden = false;
        function close(confirmed) {
          if (finished) return;
          finished = true;
          panel.hidden = true;
          panel.onsubmit = null;
          panel.onkeydown = null;
          cancel.onclick = null;
          signal.removeEventListener('abort', canceled);
          input.disabled = false;
          input.focus();
          resolve(confirmed);
        }
        function canceled() { close(false); }
        panel.onsubmit = function(event) { event.preventDefault(); close(true); };
        panel.onkeydown = function(event) {
          if (event.key === 'Escape' || (event.ctrlKey && event.key.toLowerCase() === 'c')) {
            event.preventDefault();
            close(false);
          }
        };
        cancel.onclick = canceled;
        signal.addEventListener('abort', canceled, {once: true});
        if (signal.aborted) { canceled(); return; }
        yes.focus();
      });
    }

    function collectLoadContent(kind) {
      return new Promise(function(resolve) {
        var text = document.getElementById('cli-load-text');
        var file = document.getElementById('cli-load-file');
        var textLabel = document.getElementById('cli-load-text-label');
        var fileLabel = document.getElementById('cli-load-file-label');
        var cancel = document.getElementById('cli-load-cancel');
        var fileMode = kind === 'file';
        input.disabled = true;
        loadPanel.reset();
        text.hidden = text.disabled = textLabel.hidden = fileMode;
        file.hidden = file.disabled = fileLabel.hidden = !fileMode;
        loadPanel.hidden = false;
        function close(content) {
          loadPanel.hidden = true;
          loadPanel.onsubmit = null;
          cancel.onclick = null;
          input.disabled = false;
          input.focus();
          resolve(content);
        }
        loadPanel.onsubmit = function(event) {
          event.preventDefault();
          var reading;
          if (fileMode) {
            if (!file.files.length) { setFeedback('Choose a configuration file.', true); return; }
            if (file.files[0].size > 4 * 1024 * 1024) {
              setFeedback('Configuration exceeds 4 MiB; choose a smaller file or load a subtree.', true);
              return;
            }
            reading = file.files[0].text();
          } else {
            reading = Promise.resolve(text.value);
          }
          reading.then(close).catch(function(err) { setFeedback(err.message, true); });
        };
        cancel.onclick = function() { close(null); };
        (fileMode ? file : text).focus();
      });
    }

    input.addEventListener('keydown', function(e) {
      var dropdownVisible = isBoxVisible(completionsBox) && !termExplanation;

      if (e.ctrlKey && e.key.toLowerCase() === 'c' && activeRequest) {
        e.preventDefault(); stopRequest(); return;
      }

      if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (dropdownVisible) {
          var items = completionsBox.querySelectorAll('.cli-completion-item');
          if (items.length === 0) return;
          if (termSelectedIndex > 0) termSelectedIndex--;
          else termSelectedIndex = items.length - 1;
          highlightItem(completionsBox, termSelectedIndex);
          return;
        }
        termQuerySequence++;
        termExplanation = false;
        setHint('');
        hideBox(completionsBox);
        recall.up();
        return;
      }

      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (dropdownVisible) {
          var items = completionsBox.querySelectorAll('.cli-completion-item');
          if (items.length === 0) return;
          if (termSelectedIndex < items.length - 1) termSelectedIndex++;
          else termSelectedIndex = 0;
          highlightItem(completionsBox, termSelectedIndex);
          return;
        }
        termQuerySequence++;
        termExplanation = false;
        setHint('');
        hideBox(completionsBox);
        recall.down();
        return;
      }

      if (e.key === 'Enter') {
        e.preventDefault();
        if (dropdownVisible && termSelectedIndex >= 0) {
          var items = completionsBox.querySelectorAll('.cli-completion-item');
          if (termSelectedIndex < items.length) {
            items[termSelectedIndex].click();
            return;
          }
        }
        hideBox(completionsBox);
        termCachedItems = null;
        termSelectedIndex = -1;
        recall.ready(function() {
          var cmd = input.value.trim();
          if (!cmd) return;
          if (streaming && activeRequest) stopRequest();
          recall.submitted();
          submitTermCommand(cmd);
        });
        return;
      }

      if (e.key === 'Tab' || (e.key === '?' && !e.ctrlKey && !inputQuoteOpen(input.value.slice(0, input.selectionStart)))) {
        e.preventDefault();
        recall.edited();
        if (e.key === '?' && dropdownVisible) {
          var chosen = completionsBox.querySelectorAll('.cli-completion-item')[termSelectedIndex < 0 ? 0 : termSelectedIndex];
          if (chosen) {
            revealTermExplanation(termCachedPrefix + chosen.querySelector('b').textContent);
            return;
          }
        }
        if (e.key === 'Tab' && dropdownVisible) {
          var options = completionsBox.querySelectorAll('.cli-completion-item');
          termSelectedIndex = (termSelectedIndex + 1) % options.length;
          highlightItem(completionsBox, termSelectedIndex);
          return;
        }
        doTermComplete(input, completionsBox, input.value, e.key === '?');
        return;
      }

      if (e.key === 'Escape') {
        recall.edited();
        termQuerySequence++;
        if (termExplanation) {
          termExplanation = false;
          setHint('');
          return;
        }
        hideBox(completionsBox);
        termCachedItems = null;
        termSelectedIndex = -1;
        return;
      }

      if (termCachedItems && e.key.length === 1) {
        setTimeout(function() { filterTermCached(input, completionsBox); }, 0);
      }
    });

    input.addEventListener('input', function() {
      termQuerySequence++;
      termExplanation = false;
      setHint('');
      if (termCachedItems) filterTermCached(input, completionsBox);
    });

    function beginTermQuery() {
      return {sequence: ++termQuerySequence, value: input.value, path: getTermPath(), mode: getTermMode()};
    }

    function currentTermQuery(query) {
      return query.sequence === termQuerySequence && query.value === input.value && query.path === getTermPath() && query.mode === getTermMode();
    }

    function termCompletionURL(value, query) {
      return '/cli/complete?input=' + encodeURIComponent(value) + '&path=' + encodeURIComponent(query.path) + '&mode=' + encodeURIComponent(query.mode);
    }

    function revealTermExplanation(subject, missing) {
      var query = beginTermQuery();
      fetch(termCompletionURL(subject, query) + '&explain=true', {credentials: 'same-origin'})
        .then(completionResponse)
        .then(function(answer) {
          if (!currentTermQuery(query)) return;
          if (!answer.found && missing) { missing(); return; }
          termExplanation = true;
          setHint(answer.found ? answer.explanation : subject.trim() + ': no explanation is declared');
        })
        .catch(function(err) {
          if (currentTermQuery(query)) setFeedback('Completion failed: ' + err.message, true);
        });
    }

    function doTermComplete(inp, box, val, question) {
      var query = beginTermQuery();
      fetch(termCompletionURL(val, query), {credentials: 'same-origin'})
        .then(completionResponse)
        .then(function(items) {
          if (!currentTermQuery(query)) return;
          if (!items || items.length === 0) {
            hideBox(box);
            termCachedItems = null;
            revealTermExplanation(val);
            return;
          }
          var parts = splitInput(val);
          termCachedItems = items;
          termCachedPrefix = parts.prefix;

          if (items.length === 1) {
            if (question) {
              var completed = termCachedPrefix + items[0].text;
              revealTermExplanation(completed, function() {
                doTermComplete(inp, box, completed + ' ', false);
              });
              return;
            }
            inp.value = termCachedPrefix + items[0].text + ' ';
            hideBox(box);
            termCachedItems = null;
            inp.dispatchEvent(new Event('input'));
            return;
          }
          showTermCompletions(inp, box, items, termCachedPrefix);
        })
        .catch(function(err) {
          if (!currentTermQuery(query)) return;
          hideBox(box);
          termCachedItems = null;
          setFeedback('Completion failed: ' + err.message, true);
        });
    }

    function showTermCompletions(inp, box, items, prefix) {
      termSelectedIndex = 0;
      showCompletions(inp, box, items, prefix, function() {
        termCachedItems = null;
        termSelectedIndex = -1;
      });
    }

    function filterTermCached(inp, box) {
      if (!termCachedItems) return;
      var parts = splitInput(inp.value);
      var partial = parts.partial.toLowerCase();
      if (parts.prefix !== termCachedPrefix) {
        termCachedItems = null;
          hideBox(box);
        return;
      }
      var filtered = termCachedItems.filter(function(c) {
        return c.text.toLowerCase().indexOf(partial) === 0;
      });
      if (filtered.length === 0) {
        hideBox(box);
        return;
      }
      if (filtered.length === 1 && filtered[0].text.toLowerCase() === partial) {
        hideBox(box);
        return;
      }
      showTermCompletions(inp, box, filtered, termCachedPrefix);
    }

    // A load handoff opens the same content form without applying anything.
    var page = input.closest('.cli-page');
    var initialCommand = page ? page.getAttribute('data-initial-command') : '';
    var initialSubmission = new URLSearchParams(window.location.search).get('submission');
    submitTermCommand(initialCommand || 'show', undefined, initialSubmission, !initialCommand);

    input.focus();
  }

  document.addEventListener('DOMContentLoaded', function() {
    initTheme();
    init();
    initViewToggle();
    initNumberInputs();
    initEnterSubmit();
    initActions();
    initErrorPanel();
    initFlyout();
    initToolOverlay();
    refreshCommitBar();
    initTerminal();
  });
})();
