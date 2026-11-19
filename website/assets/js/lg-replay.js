/* Ze looking glass replay.

   One file with two roles, told apart by the document it runs in.

   On a site page it finds each [data-lg-replay] mount, puts a sandboxed frame
   and the tour controls in it, and plays the tour one step at a time. It never
   starts by itself: the frame opens on the first recorded page and waits.

   Inside the frame document (written by the lg-replay producer,
   internal/le/site/lgreplay.go) it reads the recording that document embeds,
   puts the recorded page with the looking glass's own stylesheet and scripts
   in place, and answers every request they make from the recording. The frame
   is sandboxed without allow-same-origin and its policy is default-src 'none',
   so the looking glass's scripts reach neither the site page nor the network.
   Opened with no frame around it, or framed with no sandbox, the document
   plays nothing (frameRefusal).

   The recording's format and the request key are stated in
   internal/le/site/lgreplay/recording.go (Recording, ExchangeKey). */
(function () {
  'use strict';

  var FROM_HOST = 'lg-replay-host';
  var FROM_FRAME = 'lg-replay-frame';
  var SCHEMA = 2;
  /* The looking glass sends one stream message every five seconds
     (handleUIEvents), and the replay keeps that pace. */
  var STREAM_INTERVAL_MS = 5000;
  /* How long a step stays on screen before Play moves to the next one. */
  var STEP_DWELL_MS = 4000;
  /* How long the host waits for a step before it says the step did not end. */
  var STEP_WAIT_MS = 8000;
  /* How many errors of the frame a mount keeps, and how long each may be
     (keepError). */
  var ERRORS_MAX = 20;
  var ERROR_LENGTH_MAX = 300;

  /* ---- the request key: ExchangeKey in recording.go --------------------- */

  function canonicalFields(fields) {
    var pairs = [];
    fields.forEach(function (field) {
      if (field[1] !== '') pairs.push(field[0] + '=' + field[1]);
    });
    pairs.sort();
    return pairs.join('&');
  }

  function exchangeKey(method, path, query, form, htmx) {
    var key = method.toUpperCase() + ' ' + path;
    var fields = canonicalFields(query);
    if (fields) key += '?' + fields;
    fields = canonicalFields(form);
    if (fields) key += ' form ' + fields;
    if (htmx) key += ' htmx';
    return key;
  }

  function fieldsOf(collection) {
    var fields = [];
    collection.forEach(function (value, name) {
      if (typeof value === 'string') fields.push([name, value]);
    });
    return fields;
  }

  /* ---- the host page ---------------------------------------------------- */

  function newElement(name, className, text) {
    var node = document.createElement(name);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
  }

  function mountPlayer(mount) {
    var section = mount.closest('.lg-replay') || mount.parentNode;
    var steps = Array.prototype.slice.call(section.querySelectorAll('[data-lg-replay-step]'));
    /* step is the last step that finished, current the one in progress, and
       target the step the presses so far ask for: Next step moves target, and
       the tour plays one step after another until step reaches it. page is
       the key of the recorded page the frame loaded and nothing has asked a
       question on since, or '' once one was asked. recoveredStep is the step
       the tour already went back to the entry page for, so it does so once. */
    var state = {
      step: -1, current: -1, target: -1, playing: false, busy: false, ready: false, opening: false,
      page: '', progress: '', recoveredStep: -1, loads: 0,
      timer: 0, watchdog: 0, errors: 0, errorsDropped: 0
    };

    /* The frame can raise an error on every request for as long as the page
       stays open, so the mount keeps the first ERRORS_MAX of them in
       data-lg-replay-errors, each cut to ERROR_LENGTH_MAX characters. At the
       bound the list stops growing, and data-lg-replay-errors-dropped counts
       every error that came after it. The first errors are kept because they
       say what went wrong; the later ones are mostly their consequences. */
    function keepError(text) {
      if (state.errors === ERRORS_MAX) {
        state.errorsDropped += 1;
        mount.setAttribute('data-lg-replay-errors-dropped', String(state.errorsDropped));
        return;
      }
      state.errors += 1;
      var kept = mount.getAttribute('data-lg-replay-errors');
      var line = text.slice(0, ERROR_LENGTH_MAX);
      mount.setAttribute('data-lg-replay-errors', kept ? kept + ' | ' + line : line);
    }

    var controls = newElement('div', 'lg-replay__controls');
    controls.setAttribute('role', 'group');
    controls.setAttribute('aria-label', 'Tour of the recorded looking glass');
    var play = newElement('button', 'lg-replay__button', 'Play tour');
    var next = newElement('button', 'lg-replay__button', 'Next step');
    var restart = newElement('button', 'lg-replay__button', 'Restart');
    [play, next, restart].forEach(function (button) {
      button.type = 'button';
      controls.appendChild(button);
    });
    play.setAttribute('aria-pressed', 'false');

    var caption = newElement('p', 'lg-replay__caption',
      'The first recorded page is open. Click through it, or play the tour.');
    var status = newElement('p', 'lg-replay__status');
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');
    var answer = newElement('pre', 'lg-replay__answer');
    answer.hidden = true;
    answer.tabIndex = 0;
    answer.setAttribute('aria-label', 'Recorded text answer');

    var frameSource = mount.getAttribute('data-lg-replay-frame');
    var frame = null;

    /* The host owns which recorded page the frame shows. Every load is a new
       frame element, so the document is new whatever the old one was doing,
       and a message from the old one is no longer from the frame. An empty key
       opens the recording's entry page. The frame document says `ready` when
       the page is in place, and nothing plays before that.

       allow-forms lets the looking glass's search form raise its submit event,
       which htmx answers; a sandbox without it drops the submission before
       any script sees it. allow-same-origin is left out on purpose. */
    function loadFrame(key) {
      var fresh = document.createElement('iframe');
      fresh.className = 'lg-replay__frame';
      fresh.title = 'Recorded Ze looking glass';
      fresh.setAttribute('sandbox', 'allow-scripts allow-forms');
      fresh.src = frameSource + (key ? '#' + encodeURIComponent(key) : '');
      state.ready = false;
      state.page = '';
      mount.setAttribute('data-lg-replay-state', 'loading');
      mount.removeAttribute('data-lg-replay-page');
      if (frame) mount.replaceChild(fresh, frame);
      else mount.appendChild(fresh);
      frame = fresh;
    }

    while (mount.firstChild) mount.removeChild(mount.firstChild);
    mount.appendChild(controls);
    mount.appendChild(caption);
    mount.appendChild(status);
    loadFrame('');
    mount.appendChild(answer);

    function setPlaying(playing) {
      state.playing = playing;
      play.textContent = playing ? 'Pause tour' : 'Play tour';
      play.setAttribute('aria-pressed', playing ? 'true' : 'false');
      if (!playing) window.clearTimeout(state.timer);
    }

    function markStep(index) {
      steps.forEach(function (item, position) {
        if (position === index) item.setAttribute('aria-current', 'step');
        else item.removeAttribute('aria-current');
      });
    }

    var lastStep = steps.length - 1;
    var tourOver = 'The tour is over. Press Restart to play it again.';

    /* A press while a step is still loading is kept, never dropped: the
       status line says which step is loading and how many presses wait
       behind it, and Next step says it is busy to assistive technology. */
    function showProgress() {
      next.setAttribute('aria-busy', state.busy ? 'true' : 'false');
      if (!state.busy) {
        if (status.textContent === state.progress) status.textContent = '';
        state.progress = '';
        return;
      }
      var queued = state.target - state.current;
      state.progress = 'Step ' + (state.current + 1) + ' is loading' +
        (queued > 0 ? ', and ' + queued + ' more ' + (queued === 1 ? 'press waits' : 'presses wait') + ' behind it.' : '.');
      status.textContent = state.progress;
    }

    function playStep(index) {
      var text = steps[index].querySelector('[data-lg-replay-caption]');
      caption.textContent = 'Step ' + (index + 1) + ' of ' + steps.length + ': ' +
        (text ? text.textContent : '');
      answer.hidden = true;
      markStep(index);
      state.busy = true;
      state.current = index;
      mount.setAttribute('data-lg-replay-step-state', 'playing');
      showProgress();
      window.clearTimeout(state.watchdog);
      state.watchdog = window.setTimeout(function () {
        if (!state.busy) return;
        state.busy = false;
        state.target = state.step;
        setPlaying(false);
        showProgress();
        status.textContent = 'Step ' + (index + 1) + ' did not finish. Press Restart to play the tour from its first step.';
      }, STEP_WAIT_MS);
      frame.contentWindow.postMessage({ from: FROM_HOST, type: 'step', index: index }, '*');
    }

    /* advance plays the next step when one is asked for and nothing is in the
       way: no step in progress, and a frame that has said `ready`. Every
       event that removes an obstacle calls it, so a press made early plays
       as soon as it can. */
    function advance() {
      if (state.busy) {
        showProgress();
        return;
      }
      if (!state.ready) return;
      if (state.step >= state.target) return;
      playStep(state.step + 1);
    }

    function stepDone() {
      if (!state.busy) return;
      state.busy = false;
      window.clearTimeout(state.watchdog);
      state.step = state.current;
      if (state.recoveredStep === state.step) state.recoveredStep = -1;
      showProgress();
      mount.setAttribute('data-lg-replay-step-state', 'done');
      mount.setAttribute('data-lg-replay-step', String(state.step));
      if (state.step < state.target) {
        advance();
        return;
      }
      if (!state.playing) return;
      if (state.step === lastStep) {
        setPlaying(false);
        status.textContent = tourOver;
        return;
      }
      state.timer = window.setTimeout(function () {
        state.target = state.step + 1;
        advance();
      }, STEP_DWELL_MS);
    }

    /* Restart opens the entry page in a new frame document and asks for the
       first step, which plays once that document says `ready`. */
    function restartTour() {
      window.clearTimeout(state.timer);
      window.clearTimeout(state.watchdog);
      state.busy = false;
      state.opening = false;
      state.step = -1;
      state.current = -1;
      state.target = 0;
      state.recoveredStep = -1;
      showProgress();
      status.textContent = '';
      answer.hidden = true;
      mount.removeAttribute('data-lg-replay-step');
      loadFrame('');
    }

    play.addEventListener('click', function () {
      setPlaying(!state.playing);
      if (!state.playing) return;
      if (state.step >= lastStep) {
        restartTour();
        return;
      }
      if (state.target <= state.step) state.target = state.step + 1;
      advance();
    });
    next.addEventListener('click', function () {
      setPlaying(false);
      if (state.target >= lastStep) {
        if (state.busy) showProgress();
        else if (state.step >= lastStep) status.textContent = tourOver;
        return;
      }
      state.target += 1;
      advance();
    });
    restart.addEventListener('click', function () {
      setPlaying(false);
      restartTour();
    });

    window.addEventListener('message', function (event) {
      if (event.source !== frame.contentWindow) return;
      var message = event.data;
      if (!message || message.from !== FROM_FRAME) return;
      if (message.type === 'ready') {
        state.ready = true;
        state.page = message.key;
        state.loads += 1;
        mount.setAttribute('data-lg-replay-state', 'ready');
        mount.setAttribute('data-lg-replay-page', message.key);
        mount.setAttribute('data-lg-replay-loads', String(state.loads));
        if (state.opening) {
          state.opening = false;
          stepDone();
        } else {
          advance();
        }
      } else if (message.type === 'open') {
        /* The frame asks for a recorded page: a tour step that opens one, or
           a visitor's click on a plain link. A step whose page the frame
           already shows untouched is done; any other gets a new document. */
        if (message.step && state.page === message.key) {
          stepDone();
          return;
        }
        state.opening = Boolean(message.step);
        loadFrame(message.key);
      } else if (message.type === 'step-done') {
        stepDone();
      } else if (message.type === 'text') {
        answer.textContent = message.body;
        answer.hidden = false;
        stepDone();
      } else if (message.type === 'answered') {
        state.page = '';
        mount.setAttribute('data-lg-replay-answered', message.key);
      } else if (message.type === 'missing') {
        state.page = '';
        mount.setAttribute('data-lg-replay-missing', message.request);
        status.textContent = 'Not in this recording: ' + message.request +
          '. The replay holds the answers of the tour and of the rows its pages show.';
      } else if (message.type === 'unreachable') {
        /* The step presses a control of the page the steps before it leave,
           and a visitor who clicked around inside the frame left another
           page. The tour goes back once: it opens the entry page and plays
           the earlier steps again, up to this one. A step still unreachable
           after that is a step the recording cannot play, and it says so. */
        window.clearTimeout(state.watchdog);
        state.busy = false;
        if (state.recoveredStep !== state.current) {
          state.recoveredStep = state.current;
          state.step = -1;
          showProgress();
          status.textContent = 'Returning to the page step ' + (state.current + 1) + ' starts from.';
          state.progress = status.textContent;
          state.current = -1;
          loadFrame('');
          return;
        }
        state.recoveredStep = -1;
        state.target = state.step;
        setPlaying(false);
        showProgress();
        status.textContent = 'The control step ' + (state.current + 1) + ' presses is not on the page the earlier steps leave. ' +
          'Press Restart to play the tour from its first step.';
      } else if (message.type === 'error') {
        keepError(String(message.text));
      } else if (message.type === 'refused') {
        mount.setAttribute('data-lg-replay-state', 'refused');
        status.textContent = message.reason;
      }
    });
  }

  function hostMain() {
    Array.prototype.forEach.call(document.querySelectorAll('[data-lg-replay]'), mountPlayer);
  }

  /* ---- inside the frame ------------------------------------------------- */

  function frameMain(recordingElement) {
    function tellHost(message) {
      message.from = FROM_FRAME;
      window.parent.postMessage(message, '*');
    }

    /* The frame's origin is its own, so nothing outside can read its console.
       What goes wrong in it, a refused request included, is handed to the
       host, which keeps it on the mount for a reader of the page to find. */
    window.addEventListener('error', function (event) {
      tellHost({ type: 'error', text: 'error: ' + event.message + ' (line ' + event.lineno + ')' });
    });
    window.addEventListener('unhandledrejection', function (event) {
      tellHost({ type: 'error', text: 'unhandled rejection: ' + String(event.reason) });
    });
    document.addEventListener('securitypolicyviolation', function (event) {
      tellHost({ type: 'error', text: 'refused by the frame policy: ' + event.violatedDirective + ' ' + event.blockedURI });
    });

    var recording;
    try {
      recording = JSON.parse(recordingElement.textContent);
    } catch (error) {
      tellHost({ type: 'refused', reason: 'The recording in this page cannot be read: ' + error.message });
      return;
    }
    if (recording.schema !== SCHEMA) {
      tellHost({ type: 'refused', reason: 'The recording has schema ' + recording.schema +
        ', and this player reads schema ' + SCHEMA + '.' });
      return;
    }

    var exchanges = {};
    recording.exchanges.forEach(function (exchange) { exchanges[exchange.key] = exchange; });
    var assets = {};
    recording.assets.forEach(function (asset) { assets[asset.path] = asset; });
    /* The key of the answer the step in progress waits for. */
    var stepExchangeKey = '';

    function isMarkup(answer) {
      return answer['content-type'].indexOf('text/html') === 0;
    }

    function dataURI(asset) {
      return 'data:' + asset['content-type'].split(';')[0] + ';charset=utf-8,' + encodeURIComponent(asset.body);
    }

    /* An image of a swapped fragment would be fetched from the network, which
       the frame's policy refuses, so its recorded bytes go inline. */
    function inlineImages(markup) {
      return markup.replace(/\bsrc="(\/lg\/assets\/[^"]+)"/g, function (whole, path) {
        var asset = assets[path];
        if (!asset || asset['content-type'].indexOf('image/') !== 0) return whole;
        return 'src="' + dataURI(asset) + '"';
      });
    }

    function requestLabel(key) {
      return key.replace(/ htmx$/, '');
    }

    function reportMissing(key) {
      tellHost({ type: 'missing', request: requestLabel(key) });
    }

    /* The sandbox gives the frame an origin of its own with no storage, and
       the looking glass's theme script reads localStorage as it loads. */
    function memoryStorage() {
      var values = {};
      return {
        getItem: function (name) {
          return Object.prototype.hasOwnProperty.call(values, name) ? values[name] : null;
        },
        setItem: function (name, value) { values[name] = String(value); },
        removeItem: function (name) { delete values[name]; }
      };
    }
    ['localStorage', 'sessionStorage'].forEach(function (name) {
      Object.defineProperty(window, name, { configurable: true, value: memoryStorage() });
    });

    /* htmx pushes each page's URL. The frame document has one URL, and a
       sandboxed document may not take another, so the push is dropped. */
    window.history.pushState = function () {};
    window.history.replaceState = function () {};

    function requestOf(input, init) {
      var raw = typeof input === 'string' ? input : (input && input.url) ? input.url : String(input);
      var target = new URL(raw, window.location.href);
      var method = (init.method || (input && input.method) || 'GET').toUpperCase();
      var headers = new Headers(init.headers || (input && input.headers) || {});
      var form = [];
      if (typeof init.body === 'string') form = fieldsOf(new URLSearchParams(init.body));
      else if (init.body && typeof init.body.forEach === 'function') form = fieldsOf(init.body);
      return {
        method: method,
        path: target.pathname,
        htmx: headers.get('HX-Request') === 'true',
        key: exchangeKey(method, target.pathname, fieldsOf(target.searchParams), form,
          headers.get('HX-Request') === 'true')
      };
    }

    function streamResponse(signal) {
      var encoder = new TextEncoder();
      var timers = [];
      function stop() { timers.forEach(function (timer) { window.clearTimeout(timer); }); }
      var body = new ReadableStream({
        start: function (controller) {
          recording.stream.messages.forEach(function (message, index) {
            timers.push(window.setTimeout(function () {
              try {
                controller.enqueue(encoder.encode(inlineImages(message) + '\n\n'));
              } catch (error) {
                /* The page closed the stream: there is nobody to send to. */
              }
            }, STREAM_INTERVAL_MS * (index + 1)));
          });
          /* The stream stays open after its last recorded message, as a live
             one does between two changes. */
          if (signal) {
            signal.addEventListener('abort', function () {
              stop();
              try { controller.close(); } catch (error) { /* already closed */ }
            });
          }
        },
        cancel: stop
      });
      return new Response(body, {
        status: 200,
        headers: { 'Content-Type': recording.stream['content-type'] || 'text/event-stream' }
      });
    }

    function noticeResponse(request) {
      if (!request.htmx) {
        return new Response('Not in this recording: ' + requestLabel(request.key) + '\n',
          { status: 404, headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
      }
      var notice = document.createElement('div');
      notice.className = 'error-banner';
      notice.setAttribute('role', 'status');
      notice.textContent = 'Not in this recording: ' + requestLabel(request.key) +
        '. The replay holds the answers of the tour and of the rows its pages show.';
      return new Response(notice.outerHTML,
        { status: 200, headers: { 'Content-Type': 'text/html; charset=utf-8' } });
    }

    /* Every request the looking glass's scripts make arrives here: htmx and
       its stream extension both call fetch. Nothing is sent anywhere. */
    window.fetch = function (input, init) {
      var request = requestOf(input, init || {});
      if (recording.stream && request.method === 'GET' && request.path === recording.stream.path) {
        return Promise.resolve(streamResponse(init && init.signal));
      }
      var exchange = exchanges[request.key];
      if (!exchange) {
        reportMissing(request.key);
        return Promise.resolve(noticeResponse(request));
      }
      tellHost({ type: 'answered', key: exchange.key });
      if (stepExchangeKey === exchange.key) {
        stepExchangeKey = '';
        /* After the swap the answer triggers, so the host moves on from a
           page that shows it. */
        window.setTimeout(function () { tellHost({ type: 'step-done' }); }, 150);
      }
      var body = isMarkup(exchange) ? inlineImages(exchange.body) : exchange.body;
      return Promise.resolve(new Response(body,
        { status: exchange.status, headers: { 'Content-Type': exchange['content-type'] } }));
    };

    /* The host loads every document of the frame, in a new frame element, so
       a recorded page is opened by asking for it. step says a tour step asks,
       which the host then counts as played once the page is in place. The
       frame does not reload itself: a reload it asked for and did not get
       left the tour waiting on a page that never came. */
    function openDocument(key, step) {
      tellHost({ type: 'open', key: key, step: step });
    }

    /* A plain link or form would leave the frame document for a URL the site
       does not serve. A recorded page opens from the recording, and anything
       else gets the notice. */
    document.addEventListener('click', function (event) {
      var link = event.target && event.target.closest ? event.target.closest('a[href]') : null;
      if (!link) return;
      if (link.hasAttribute('hx-get')) return;
      if (link.hasAttribute('hx-post')) return;
      event.preventDefault();
      var target = new URL(link.getAttribute('href'), window.location.href);
      var key = exchangeKey('GET', target.pathname, fieldsOf(target.searchParams), [], false);
      if (exchanges[key] && isMarkup(exchanges[key])) {
        openDocument(key, false);
        return;
      }
      reportMissing(key);
    }, true);
    document.addEventListener('submit', function (event) {
      var form = event.target;
      if (form.hasAttribute('hx-post')) return;
      if (form.hasAttribute('hx-get')) return;
      event.preventDefault();
      reportMissing((form.getAttribute('method') || 'GET').toUpperCase() + ' ' + (form.getAttribute('action') || ''));
    }, true);

    function showDocument(key) {
      var exchange = exchanges[key];
      if (!exchange) {
        document.body.textContent = 'Not in this recording: ' + requestLabel(key);
        reportMissing(key);
        return;
      }
      var recordedDocument = new DOMParser().parseFromString(exchange.body, 'text/html');
      var scripts = [];
      var lang = recordedDocument.documentElement.getAttribute('lang');
      if (lang) document.documentElement.setAttribute('lang', lang);
      document.title = recordedDocument.title;
      /* The frame document keeps its own charset and policy, so a recorded
         meta element is left out. */
      Array.prototype.forEach.call(recordedDocument.head.children, function (node) {
        var name = node.nodeName.toLowerCase();
        if (name === 'link' && node.getAttribute('rel') === 'stylesheet') {
          var sheet = assets[node.getAttribute('href')];
          if (!sheet) {
            reportMissing('GET ' + node.getAttribute('href'));
            return;
          }
          document.head.appendChild(newElement('style', '', sheet.body));
        } else if (name === 'script' && node.hasAttribute('src')) {
          var script = assets[node.getAttribute('src')];
          if (!script) {
            reportMissing('GET ' + node.getAttribute('src'));
            return;
          }
          scripts.push(script.body);
        } else if (name === 'script') {
          scripts.push(node.textContent);
        } else if (name === 'style') {
          document.head.appendChild(document.importNode(node, true));
        }
      });
      Array.prototype.forEach.call(recordedDocument.querySelectorAll('img[src]'), function (image) {
        var asset = assets[image.getAttribute('src')];
        if (asset) image.setAttribute('src', dataURI(asset));
      });
      document.documentElement.replaceChild(document.importNode(recordedDocument.body, true), document.body);
      /* A script element made here runs as it is inserted, so the looking
         glass's scripts run in the order its page loads them. */
      scripts.forEach(function (text) {
        document.head.appendChild(newElement('script', '', text));
      });
      /* This runs while the frame document is still loading, so the scripts'
         DOMContentLoaded handlers fire when it ends. Were it ever to run
         later, they would wait for an event that has passed. */
      if (document.readyState !== 'loading') {
        document.dispatchEvent(new Event('DOMContentLoaded', { bubbles: true }));
      }
      tellHost({ type: 'ready', key: key });
    }

    function findControl(key) {
      var controls = document.querySelectorAll('[hx-get]');
      for (var index = 0; index < controls.length; index++) {
        var target = new URL(controls[index].getAttribute('hx-get'), window.location.href);
        if (exchangeKey('GET', target.pathname, fieldsOf(target.searchParams), [], true) === key) {
          return controls[index];
        }
      }
      return null;
    }

    function findForm(path) {
      var forms = document.querySelectorAll('form[hx-post]');
      for (var index = 0; index < forms.length; index++) {
        if (new URL(forms[index].getAttribute('hx-post'), window.location.href).pathname === path) {
          return forms[index];
        }
      }
      return null;
    }

    /* A step is played the way a visitor would ask it: the page's own control
       is pressed, or its own form filled and submitted. A step no control
       asks, the text form of a graph, is handed to the host to show. */
    function playStep(index) {
      var step = recording.tour[index];
      var exchange = step ? exchanges[step.exchange] : null;
      if (!exchange) {
        tellHost({ type: 'unreachable' });
        return;
      }
      if (!exchange.htmx) {
        if (isMarkup(exchange)) openDocument(exchange.key, true);
        else tellHost({ type: 'text', key: exchange.key, body: exchange.body });
        return;
      }
      if (exchange.method === 'GET') {
        var control = findControl(exchange.key);
        if (!control) {
          tellHost({ type: 'unreachable' });
          return;
        }
        stepExchangeKey = exchange.key;
        control.scrollIntoView({ block: 'nearest' });
        control.click();
        return;
      }
      var form = findForm(exchange.path);
      if (!form) {
        tellHost({ type: 'unreachable' });
        return;
      }
      var values = {};
      (exchange.form || []).forEach(function (field) { values[field.name] = field.value; });
      Array.prototype.forEach.call(form.elements, function (field) {
        if (!field.name) return;
        field.value = Object.prototype.hasOwnProperty.call(values, field.name) ? values[field.name] : '';
      });
      stepExchangeKey = exchange.key;
      form.requestSubmit();
    }

    window.addEventListener('message', function (event) {
      if (event.source !== window.parent) return;
      var message = event.data;
      if (!message || message.from !== FROM_HOST) return;
      if (message.type === 'step') playStep(message.index);
    });

    var openingKey = recording.entry;
    if (window.location.hash.length > 1) openingKey = decodeURIComponent(window.location.hash.slice(1));
    showDocument(openingKey);
  }

  /* ---- which role ------------------------------------------------------- */

  /* The frame document is a file of the site, so a browser can open it with no
     frame around it, and a page can frame it and leave the sandbox out. The
     recorded scripts would then run in the site's own origin. The player
     plays only where the sandbox is in force: framed, and with the opaque
     origin a sandbox without allow-same-origin gives. The answer is why it
     refuses, and empty when it plays. */
  function frameRefusal() {
    if (window.parent === window) return 'opened outside its page';
    if (window.origin !== 'null') return 'framed without a sandbox';
    return '';
  }

  var recordingElement = document.getElementById('lg-replay-recording');
  if (recordingElement) {
    var refusal = frameRefusal();
    if (refusal) {
      /* Decided before the recording is read: no recorded markup is parsed
         and no recorded script is put in place. The document keeps its one
         paragraph, which names the page that plays it. */
      document.documentElement.setAttribute('data-lg-replay-frame', 'refused: ' + refusal);
    } else {
      frameMain(recordingElement);
    }
  } else if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', hostMain);
  } else {
    hostMain();
  }
})();
