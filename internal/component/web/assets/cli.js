/* Ze CLI bar -- Tab/? autocomplete, clickable completions.
   Completions replace the last partial token, not append. */
(function(){
  'use strict';

  var cachedItems = null;  // Last fetched completions for live filtering.
  var cachedPrefix = '';   // Prefix (up to last space) when completions were fetched.
  var history = [];        // Command history.
  var historyPos = -1;     // Current position in history (-1 = not browsing).
  var historyDraft = '';   // Saved input before browsing history.

  function init() {
    var input = document.getElementById('cli-input');
    var box = document.getElementById('cli-completions');
    if (!input || !box) return;

    input.addEventListener('keydown', function(e) {
      // History navigation.
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (history.length === 0) return;
        if (historyPos < 0) historyDraft = input.value;
        if (historyPos < history.length - 1) {
          historyPos++;
          input.value = history[history.length - 1 - historyPos];
        }
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (historyPos < 0) return;
        historyPos--;
        if (historyPos < 0) {
          input.value = historyDraft;
        } else {
          input.value = history[history.length - 1 - historyPos];
        }
        return;
      }

      if (e.key === 'Enter') {
        e.preventDefault();
        box.style.display = 'none';
        cachedItems = null;
        var cmd = input.value.trim();
        if (!cmd) return;
        history.push(cmd);
        historyPos = -1;
        input.value = '';

        var tokens = cmd.split(/\s+/);
        var verb = tokens[0];

        // Navigation commands: use HTMX so OOB swaps update breadcrumb and prompt.
        if (verb === 'show' || verb === 'edit') {
          if (window.htmx) {
            htmx.ajax('POST', '/cli', {
              target: '#content-area',
              swap: 'outerHTML',
              values: {command: cmd}
            });
          }
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
            .then(function(html) {
              var out = document.getElementById('terminal-scrollback');
              if (out) {
                // Server returns HTML fragments (terminal-entry divs).
                out.insertAdjacentHTML('beforeend', html);
                out.scrollTop = out.scrollHeight;
              }
            });
          return;
        }

        // GUI mode: POST then refresh view.
        var curPath = window.location.pathname.replace(/^\/show\//, '').replace(/^\/config\/edit\//, '').replace(/\/$/, '');
        fetch('/cli', {
          method: 'POST',
          credentials: 'same-origin',
          redirect: 'manual',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'},
          body: 'command=' + encodeURIComponent(cmd) + '&path=' + encodeURIComponent(curPath)
        }).then(function() {
          refreshDetail(curPath);
          if (verb === 'set' || verb === 'delete' || verb === 'commit' || verb === 'discard') {
            refreshCommitBar();
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
    var completePath = window.location.pathname.replace(/^\/show\//, '').replace(/^\/config\/edit\//, '').replace(/\/$/, '');
    fetch('/cli/complete?input=' + encodeURIComponent(val) + '&path=' + encodeURIComponent(completePath), {credentials: 'same-origin'})
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

  // Track terminal mode state for CLI Enter key routing.
  // Updated after HTMX mode toggle completes.
  function initViewToggle() {
    document.addEventListener('htmx:afterSwap', function(e) {
      // Check if the content area was swapped (mode toggle or navigation).
      var content = document.querySelector('.content-area');
      if (!content) return;
      var hasTerminal = content.querySelector('#terminal-scrollback');
      window.zeTerminalMode = !!hasTerminal;
      var btn = document.getElementById('view-toggle');
      if (!btn) return;
      if (hasTerminal) {
        btn.textContent = 'GUI';
        btn.title = 'Switch to GUI view';
        btn.setAttribute('hx-vals', '{"mode":"integrated"}');
      } else {
        btn.textContent = 'CLI';
        btn.title = 'Switch to text/CLI view';
        btn.setAttribute('hx-vals', '{"mode":"terminal"}');
      }
      if (window.htmx) htmx.process(btn);
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
      } else if (action === 'add-entry') {
        var baseURL = btn.getAttribute('data-base-url') || '/show/';
        showAddEntryOverlay(baseURL);
      } else if (action === 'rename-entry') {
        var url = btn.getAttribute('data-url');
        var key = btn.getAttribute('data-key');
        showRenameOverlay(url, key);
      } else if (action === 'delete-entry') {
        var path = btn.getAttribute('data-path');
        var entryKey = btn.getAttribute('data-key');
        deleteEntry(path, entryKey);
      } else if (action === 'toggle-theme') {
        var html = document.documentElement;
        var current = html.getAttribute('data-theme');
        var next = current === 'light' ? 'dark' : 'light';
        html.setAttribute('data-theme', next);
        try { localStorage.setItem('ze-theme', next); } catch(_) {}
      }
    });

  }

  // Add-entry overlay: fetches the server-rendered form for the list at baseURL.
  // The server template handles keyed vs keyless lists, title, and field labels.
  // Keyless lists create the entry server-side and return a redirect (no form).
  function showAddEntryOverlay(baseURL) {
    var formURL = baseURL.replace(/^\/show\//, '/config/add-form/');
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
    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        var newName = input.value.trim();
        if (newName && newName !== currentKey) {
          // TODO: implement rename via API when backend supports it
          overlay.remove();
        }
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
    var entryKey = parts.pop();
    var parentPath = parts.join('/');
    var deletePath = '/config/delete/' + parentPath + '/';
    fetch(deletePath, {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Content-Type': 'application/x-www-form-urlencoded'},
      body: 'leaf=' + encodeURIComponent(entryKey)
    }).then(function(r) {
      if (r.ok) {
        // Refresh the current view
        var curPath = window.location.pathname.replace(/^\/show\//, '').replace(/\/$/, '');
        refreshDetail(curPath);
      }
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

  document.addEventListener('DOMContentLoaded', function() {
    initTheme();
    init();
    initViewToggle();
    initNumberInputs();
    initActions();
    initFlyout();
  });
})();
