/* Ze CLI bar -- Tab/? autocomplete, clickable completions.
   Completions replace the last partial token, not append. */
(function(){
  'use strict';

  var cachedItems = null;  // Last fetched completions for live filtering.
  var cachedPrefix = '';   // Prefix (up to last space) when completions were fetched.

  function init() {
    var input = document.getElementById('cli-input');
    var box = document.getElementById('cli-completions');
    if (!input || !box) return;

    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        box.style.display = 'none';
        cachedItems = null;
        var cmd = input.value.trim();
        if (!cmd) return;
        input.value = '';

        var tokens = cmd.split(/\s+/);
        var verb = tokens[0];

        // Navigation commands: go to the path.
        if (verb === 'show' || verb === 'edit') {
          var navPath = tokens.slice(1).join('/');
          window.location.href = '/show/' + navPath + '/';
          return;
        }

        // In terminal mode, send all commands to terminal endpoint and show output.
        if (window.zeTerminalMode) {
          fetch('/cli/terminal', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: 'command=' + encodeURIComponent(cmd)
          }).then(function(r) { return r.text(); })
            .then(function(text) {
              var out = document.getElementById('terminal-output');
              if (out) {
                out.textContent += '> ' + cmd + '\n' + text + '\n';
                out.scrollTop = out.scrollHeight;
              }
              // Commit bar updated via OOB from server.
            });
          return;
        }

        // GUI mode: POST then refresh view.
        fetch('/cli', {
          method: 'POST',
          credentials: 'same-origin',
          redirect: 'manual',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'},
          body: 'command=' + encodeURIComponent(cmd)
        }).then(function() {
          var curPath = window.location.pathname.replace(/^\/show\//, '').replace(/\/$/, '');
          refreshDetail(curPath);
          if (verb === 'set' || verb === 'delete') {
            if (window.zeCommitBump) window.zeCommitBump();
          }
        });
        return;
      }

      if (e.key === 'Tab' || (e.key === '?' && !e.ctrlKey)) {
        e.preventDefault();
        var val = input.value;
        if (e.key === '?') {
          val = val.replace(/\?+$/, '');
          if (!val.endsWith(' ')) val += ' ';
        }
        doComplete(input, box, val);
        return;
      }

      if (e.key === 'Escape') {
        box.style.display = 'none';
        cachedItems = null;
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
  }

  // Split input into prefix (everything before last token) and partial (token being typed).
  function splitInput(val) {
    var lastSpace = val.lastIndexOf(' ');
    if (lastSpace < 0) return { prefix: '', partial: val };
    return { prefix: val.substring(0, lastSpace + 1), partial: val.substring(lastSpace + 1) };
  }

  function doComplete(input, box, val) {
    fetch('/cli/complete?input=' + encodeURIComponent(val), {credentials: 'same-origin'})
      .then(function(r){ return r.json(); })
      .then(function(items){
        if (!items || items.length === 0) {
          box.style.display = 'none';
          cachedItems = null;
          return;
        }

        var parts = splitInput(val);
        cachedItems = items;
        cachedPrefix = parts.prefix;

        if (items.length === 1) {
          input.value = cachedPrefix + items[0].text + ' ';
          box.style.display = 'none';
          cachedItems = null;
          return;
        }

        showCompletions(input, box, items, cachedPrefix);
      })
      .catch(function(){ box.style.display='none'; });
  }

  function filterCached(input, box) {
    if (!cachedItems) return;
    var parts = splitInput(input.value);
    var partial = parts.partial.toLowerCase();

    // If prefix changed (user typed a space = new token), clear cache.
    if (parts.prefix !== cachedPrefix) {
      cachedItems = null;
      box.style.display = 'none';
      return;
    }

    var filtered = cachedItems.filter(function(c) {
      return c.text.toLowerCase().indexOf(partial) === 0;
    });

    if (filtered.length === 0) {
      box.style.display = 'none';
      return;
    }

    if (filtered.length === 1 && filtered[0].text.toLowerCase() === partial) {
      // Exact match, nothing more to show.
      box.style.display = 'none';
      return;
    }

    showCompletions(input, box, filtered, cachedPrefix);
  }

  function showCompletions(input, box, items, prefix) {
    while (box.firstChild) box.removeChild(box.firstChild);
    items.forEach(function(c) {
      var div = document.createElement('div');
      div.className = 'cli-completion-item';
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
        box.style.display = 'none';
        cachedItems = null;
        input.focus();
      });
      box.appendChild(div);
    });
    box.style.display = 'block';
  }

  // View toggle: GUI <-> CLI text mode.
  var cliMode = false;
  var savedGUI = null;

  function initViewToggle() {
    var btn = document.getElementById('view-toggle');
    if (!btn) return;

    btn.addEventListener('click', function() {
      var content = document.querySelector('.content-area');
      if (!content) return;

      if (!cliMode) {
        // Switch to CLI text view: save current GUI, show terminal.
        savedGUI = content.cloneNode(true);
        var terminal = document.createElement('div');
        terminal.id = 'terminal-view';
        terminal.className = 'terminal-view';
        var output = document.createElement('pre');
        output.id = 'terminal-output';
        output.className = 'terminal-output';
        output.textContent = 'Ze CLI -- type commands below, output appears here.\n\n';
        terminal.appendChild(output);
        while (content.firstChild) content.removeChild(content.firstChild);
        content.appendChild(terminal);
        cliMode = true;
        btn.textContent = 'GUI';
        btn.title = 'Switch to GUI view';

        // Redirect CLI Enter to terminal output.
        window.zeTerminalMode = true;
      } else {
        // Switch back to GUI: restore saved content.
        if (savedGUI) {
          while (content.firstChild) content.removeChild(content.firstChild);
          while (savedGUI.firstChild) content.appendChild(savedGUI.firstChild);
          savedGUI = null;
          // Re-init fields.
          document.dispatchEvent(new Event('htmx:afterSwap'));
        } else {
          // No saved GUI, reload page.
          window.location.reload();
        }
        cliMode = false;
        btn.textContent = 'CLI';
        btn.title = 'Switch to text/CLI view';
        window.zeTerminalMode = false;
      }
    });
  }

  // Refresh the detail panel via fragment endpoint without full page reload.
  function refreshDetail(path) {
    var detail = document.getElementById('detail');
    if (!detail || !window.htmx) return;
    htmx.ajax('GET', '/fragment/detail?path=' + encodeURIComponent(path), {
      target: '#detail',
      swap: 'innerHTML'
    });
  }

  // SSE: listen for config-change events from /events endpoint.
  function initSSE() {
    if (typeof EventSource === 'undefined') return;
    var src = new EventSource('/events');
    src.addEventListener('config-change', function(e) {
      var bar = document.getElementById('notification-bar');
      if (!bar || !e.data) return;
      bar.outerHTML = e.data;
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
      } else if (action === 'dismiss-login') {
        var overlay = document.getElementById('login-overlay');
        if (overlay) overlay.remove();
      }
    });

    // Key-form: append input value to base URL before submitting.
    document.addEventListener('submit', function(e) {
      var form = e.target.closest('[data-action="key-form"]');
      if (!form) return;
      var base = form.getAttribute('data-base-url') || '';
      var input = form.querySelector('input[type="text"]');
      if (input && input.value) {
        form.action = base + input.value + '/';
      }
    });
  }

  // After a tristate toggle POST completes, reload the detail panel to show new state.
  function initTristate() {
    document.addEventListener('htmx:afterRequest', function(e) {
      if (e.detail && e.detail.elt && e.detail.elt.hasAttribute('data-tristate')) {
        var curPath = window.location.pathname.replace(/^\/show\//, '').replace(/\/$/, '');
        var detail = document.getElementById('detail');
        if (detail && window.htmx) {
          htmx.ajax('GET', '/fragment/detail?path=' + encodeURIComponent(curPath), {
            target: '#detail',
            swap: 'innerHTML'
          });
        }
      }
    });
  }

  document.addEventListener('DOMContentLoaded', function() {
    init();
    initViewToggle();
    initSSE();
    initNumberInputs();
    initActions();
    initTristate();
  });
})();
