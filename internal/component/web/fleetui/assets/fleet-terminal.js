/* Design: docs/architecture/fleet-interface.md -- the console terminal of a device
 *
 * The terminal (Terminal in component_terminal.templ) posts one command at a
 * time to the address in data-terminal-url, form encoded as "command". The
 * answer is one TerminalEntry fragment, which is appended to the scrollback.
 * An entry marked data-terminal-refused="true" is the device's refusal: the
 * input is then closed, because every next command would be refused the same
 * way. The answer is parsed apart from the page and only its entries are
 * taken, so nothing else it holds becomes part of the page.
 *
 * It depends on nothing of the web package's command line script.
 */
(function() {
  'use strict';

  var HISTORY_MAX = 100;

  // histories holds the commands of each terminal and the place the arrow
  // keys are at, keyed by the terminal element.
  var histories = new WeakMap();

  function historyOf(terminal) {
    var history = histories.get(terminal);
    if (!history) {
      history = { commands: [], index: 0, draft: '' };
      histories.set(terminal, history);
    }
    return history;
  }

  function remember(terminal, command) {
    var history = historyOf(terminal);
    if (history.commands[history.commands.length - 1] !== command) history.commands.push(command);
    if (history.commands.length > HISTORY_MAX) history.commands.shift();
    history.index = history.commands.length;
    history.draft = '';
  }

  function recall(terminal, field, step) {
    var history = historyOf(terminal);
    if (history.index === history.commands.length) history.draft = field.value;

    var next = Math.min(Math.max(history.index + step, 0), history.commands.length);
    if (next === history.index) return;
    history.index = next;
    field.value = next === history.commands.length ? history.draft : history.commands[next];
  }

  function scrollback(terminal) {
    return terminal.querySelector('[data-terminal-scrollback]');
  }

  function setClosed(terminal, closed) {
    terminal.querySelectorAll('[data-terminal-form] input, [data-terminal-form] button').forEach(function(control) {
      control.disabled = closed;
    });
  }

  // appendFailure writes a failure the terminal itself met, a request that
  // got no answer or an answer with no entry, in the shape of an entry.
  function appendFailure(terminal, command, text) {
    var entry = document.createElement('div');
    entry.className = 'fleet-terminal-entry fleet-terminal-entry--failed';

    var line = document.createElement('p');
    line.className = 'fleet-terminal-command';
    line.textContent = command;

    var output = document.createElement('pre');
    output.className = 'fleet-terminal-output';
    output.textContent = text;

    entry.appendChild(line);
    entry.appendChild(output);
    scrollback(terminal).appendChild(entry);
  }

  // appendAnswer takes the entries of an answer into the scrollback and says
  // whether one of them is a refusal. It answers null when the answer holds
  // no entry.
  function appendAnswer(terminal, body) {
    var parsed = new DOMParser().parseFromString(body, 'text/html');
    var entries = parsed.querySelectorAll('.fleet-terminal-entry');
    if (entries.length === 0) return null;

    var refused = false;
    entries.forEach(function(entry) {
      if (entry.getAttribute('data-terminal-refused') === 'true') refused = true;
      scrollback(terminal).appendChild(document.importNode(entry, true));
    });
    return refused;
  }

  function run(terminal, form, field) {
    var command = field.value.trim();
    if (command === '') return;

    var url = terminal.getAttribute('data-terminal-url');
    remember(terminal, command);
    field.value = '';
    setClosed(terminal, true);

    fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: 'command=' + encodeURIComponent(command)
    })
      .then(function(response) {
        return response.text().then(function(body) {
          return { status: response.status, body: body };
        });
      })
      .then(function(answer) {
        var refused = appendAnswer(terminal, answer.body);
        if (refused === null) {
          appendFailure(terminal, command, 'The hub answered ' + answer.status + ' with no output. Run the command again, or open the device\'s Tasks tab to see whether the device is reachable.');
          refused = false;
        }
        setClosed(terminal, refused);
        if (!refused) field.focus();
      })
      .catch(function(error) {
        appendFailure(terminal, command, 'The command did not reach the hub (' + (error && error.message ? error.message : 'no reason given') + '). Check the connection to the hub and run it again.');
        setClosed(terminal, false);
        field.focus();
      })
      .then(function() {
        var region = scrollback(terminal);
        region.scrollTop = region.scrollHeight;
      });
  }

  document.addEventListener('submit', function(event) {
    var form = event.target;
    if (!form.matches || !form.matches('[data-terminal-form]')) return;

    var terminal = form.closest('[data-terminal-url]');
    var field = form.querySelector('[data-terminal-command]');
    if (!terminal || !field) return;

    event.preventDefault();
    run(terminal, form, field);
  });

  document.addEventListener('keydown', function(event) {
    var field = event.target;
    if (!field.matches || !field.matches('[data-terminal-command]')) return;
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;

    var terminal = field.closest('[data-terminal-url]');
    if (!terminal) return;

    event.preventDefault();
    recall(terminal, field, event.key === 'ArrowUp' ? -1 : 1);
  });

  // A suggestion writes its command into the command line it controls, for
  // the operator to read and send.
  document.addEventListener('click', function(event) {
    var button = event.target.closest ? event.target.closest('[data-terminal-fill]') : null;
    if (!button) return;

    var field = document.getElementById(button.getAttribute('aria-controls') || '');
    if (!field) return;

    field.value = button.getAttribute('data-terminal-fill') || '';
    field.focus();
  });
})();
