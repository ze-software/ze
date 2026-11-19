// Design: docs/architecture/fleet-interface.md -- fleet shell script
//
// The behaviour of the fleet shell, with no framework: the theme switch, the
// sidebar (collapse, resize, pin, its two tabs), the estate tree (expansion,
// lazy branches, the filter box, the keyboard), the side panel pin, the
// header search and the live event stream.
//
// The markup carries no script and no event attribute, because the content
// security policy refuses both. Everything here is found through an id or a
// data-fleet-* attribute that shell.templ and tree.templ write, and every
// listener is delegated from the document, so markup htmx swaps in later
// needs no wiring of its own.
//
// Menus, the task panel toggle, toasts and the confirm dialog belong to
// fleet-components.js. The theme is applied before the first paint by
// fleet-theme.js.
(function () {
  'use strict';

  var root = document.documentElement;
  var body = document.body;

  // --- Bounds ---------------------------------------------------------------

  var SIDEBAR_MIN = 12;        // rem
  var SIDEBAR_MAX = 28;        // rem
  var SIDEBAR_DEFAULT = 17;    // rem
  var SIDEBAR_STEP = 1;        // rem per arrow key on the handle
  var FILTER_DELAY = 250;      // ms between the last key and the tree request
  var SEARCH_DELAY = 200;      // ms between the last key and the search request
  var BACKOFF_FIRST = 1000;    // ms before the first reconnection
  var BACKOFF_MAX = 30000;     // ms, the longest wait between reconnections
  var RECONNECT_MAX = 8;       // attempts before the banner waits for the operator
  var TREE_STATE_MAX = 500;    // remembered rows per tree view
  var NARROW = '(max-width: 60rem)';

  // --- Browser settings -----------------------------------------------------

  var KEY_THEME = 'ze-theme';
  var KEY_WIDTH = 'ze-fleet-sidebar-width';
  var KEY_PINNED = 'ze-fleet-sidebar-pinned';
  var KEY_COLLAPSED = 'ze-fleet-sidebar-collapsed';
  var KEY_FALLBACK = 'ze-fleet-sidebar-fallback';
  var KEY_PANEL_PINNED = 'ze-fleet-panel-pinned';
  var KEY_TREE_VIEW = 'ze-fleet-tree-view';
  var KEY_TREE_STATE = 'ze-fleet-tree:';
  var TREE_VIEW_DEFAULT = 'site-role';

  function read(storage, key) {
    try {
      return storage.getItem(key);
    } catch (_) {
      // The browser refuses storage access: the setting is not remembered.
      return null;
    }
  }

  function write(storage, key, value) {
    try {
      storage.setItem(key, value);
    } catch (_) {
      // The browser refuses storage access: the setting lasts for this page.
    }
  }

  function local(key) { return read(window.localStorage, key); }
  function setLocal(key, value) { write(window.localStorage, key, value); }

  function byId(id) { return document.getElementById(id); }

  function each(selector, fn, scope) {
    Array.prototype.forEach.call((scope || document).querySelectorAll(selector), fn);
  }

  // --- Addresses ------------------------------------------------------------

  // pagePath answers the path of the page in hand after the fleet prefix,
  // which is the tail of the address of each of its fragments. The estate
  // dashboard is the prefix itself, and its fragments spell it out.
  function pagePath() {
    var path = window.location.pathname;
    var rest = path.indexOf('/fleet/') === 0 ? path.slice('/fleet/'.length) : '';
    return rest === '' ? 'estate/dashboard/' : rest;
  }

  // fragmentURL answers the address of one fragment of the page in hand,
  // with the page's view parameters and the item, when there is one.
  function fragmentURL(name, item) {
    var params = new URLSearchParams(window.location.search);
    params.delete('item');
    if (item) params.set('item', item);
    var query = params.toString();
    return '/fleet/fragment/' + encodeURIComponent(name) + '/' + pagePath() + (query ? '?' + query : '');
  }

  // load asks htmx to fetch url and swap the answer into target.
  function load(url, target, swap) {
    if (!window.htmx || !target) return;
    window.htmx.ajax('GET', url, { target: target, swap: swap || 'innerHTML' });
  }

  // --- Theme ----------------------------------------------------------------

  function toggleTheme() {
    var next = root.getAttribute('data-theme') === 'light' ? 'dark' : 'light';
    root.setAttribute('data-theme', next);
    setLocal(KEY_THEME, next);
    // Charts read their colours at draw time, so they redraw on this event.
    document.dispatchEvent(new CustomEvent('fleet:theme', { detail: { theme: next } }));
  }

  // --- Sidebar --------------------------------------------------------------

  // The sidebar's width is the custom property --fleet-sidebar-width, set on
  // the root element through the object model. Where the browser refuses
  // that, the width is one of three, chosen by a class on the body.
  var widthFallback = local(KEY_FALLBACK) === '1';
  var WIDTH_CLASSES = ['fleet-shell--sidebar-narrow', 'fleet-shell--sidebar-default', 'fleet-shell--sidebar-wide'];
  var WIDTH_STEPS = [SIDEBAR_MIN, SIDEBAR_DEFAULT, SIDEBAR_MAX];

  function clampWidth(rem) {
    if (!(rem >= SIDEBAR_MIN)) return SIDEBAR_MIN;
    if (rem > SIDEBAR_MAX) return SIDEBAR_MAX;
    return rem;
  }

  function nearestStep(rem) {
    var best = 0;
    for (var index = 1; index < WIDTH_STEPS.length; index++) {
      if (Math.abs(WIDTH_STEPS[index] - rem) < Math.abs(WIDTH_STEPS[best] - rem)) best = index;
    }
    return best;
  }

  // applyWidth sets the sidebar's width and answers the width it took.
  function applyWidth(rem) {
    rem = clampWidth(rem);

    if (!widthFallback) {
      root.style.setProperty('--fleet-sidebar-width', rem + 'rem');
      if (root.style.getPropertyValue('--fleet-sidebar-width') === '') widthFallback = true;
    }

    if (widthFallback) {
      var step = nearestStep(rem);
      WIDTH_CLASSES.forEach(function (name) { body.classList.remove(name); });
      body.classList.add(WIDTH_CLASSES[step]);
      rem = WIDTH_STEPS[step];
    }

    var handle = byId('fleet-sidebar-handle');
    if (handle) handle.setAttribute('aria-valuenow', String(Math.round(rem)));
    return rem;
  }

  function storedWidth() {
    var saved = parseFloat(local(KEY_WIDTH));
    return isNaN(saved) ? SIDEBAR_DEFAULT : saved;
  }

  function rootFontSize() {
    return parseFloat(window.getComputedStyle(root).fontSize) || 14;
  }

  // narrow reports whether the viewport is below the one breakpoint of the
  // layout, where the sidebar overlays whatever its pin says.
  function narrow() {
    return window.matchMedia(NARROW).matches;
  }

  function pinned() {
    return local(KEY_PINNED) !== '0';
  }

  // overlaying reports whether the sidebar lies over the content, which it
  // does unpinned and on a narrow viewport. It then closes after a selection.
  function overlaying() {
    return !pinned() || narrow();
  }

  function setCollapsed(collapsed) {
    body.classList.toggle('fleet-shell--collapsed', collapsed);
    each('[data-fleet-action="sidebar-toggle"]', function (button) {
      button.setAttribute('aria-expanded', collapsed ? 'false' : 'true');
    });
  }

  function applyPin() {
    var isPinned = pinned();
    body.classList.toggle('fleet-shell--unpinned', !isPinned);
    each('[data-fleet-action="sidebar-pin"]', function (button) {
      var label = isPinned ? 'Let the sidebar close after a selection' : 'Keep the sidebar open';
      button.setAttribute('aria-pressed', isPinned ? 'true' : 'false');
      button.setAttribute('aria-label', label);
      button.setAttribute('title', label);
    });
  }

  function toggleSidebar() {
    var collapsed = !body.classList.contains('fleet-shell--collapsed');
    setCollapsed(collapsed);
    // An overlaying sidebar opens closed on every page, so only the pinned
    // one remembers.
    if (!overlaying()) setLocal(KEY_COLLAPSED, collapsed ? '1' : '0');
  }

  function togglePin() {
    setLocal(KEY_PINNED, pinned() ? '0' : '1');
    applyPin();
  }

  function initSidebar() {
    applyWidth(storedWidth());
    applyPin();
    setCollapsed(overlaying() || local(KEY_COLLAPSED) === '1');
  }

  function initSidebarHandle() {
    var handle = byId('fleet-sidebar-handle');
    var sidebar = byId('fleet-sidebar');
    if (!handle || !sidebar) return;

    var dragging = false;

    handle.addEventListener('pointerdown', function (event) {
      dragging = true;
      handle.setPointerCapture(event.pointerId);
      body.classList.add('fleet-shell--resizing');
      event.preventDefault();
    });

    handle.addEventListener('pointermove', function (event) {
      if (!dragging) return;
      var left = sidebar.getBoundingClientRect().left;
      applyWidth((event.clientX - left) / rootFontSize());
    });

    function release(event) {
      if (!dragging) return;
      dragging = false;
      handle.releasePointerCapture(event.pointerId);
      body.classList.remove('fleet-shell--resizing');
      var left = sidebar.getBoundingClientRect().left;
      setLocal(KEY_WIDTH, String(applyWidth((event.clientX - left) / rootFontSize())));
    }

    handle.addEventListener('pointerup', release);
    handle.addEventListener('pointercancel', release);

    // The width is adjustable from the keyboard on the handle, so dragging
    // is never the only way.
    handle.addEventListener('keydown', function (event) {
      var current = parseFloat(handle.getAttribute('aria-valuenow')) || SIDEBAR_DEFAULT;
      var next = null;
      if (event.key === 'ArrowLeft') next = widthFallback ? WIDTH_STEPS[Math.max(0, nearestStep(current) - 1)] : current - SIDEBAR_STEP;
      if (event.key === 'ArrowRight') next = widthFallback ? WIDTH_STEPS[Math.min(WIDTH_STEPS.length - 1, nearestStep(current) + 1)] : current + SIDEBAR_STEP;
      if (event.key === 'Home') next = SIDEBAR_MIN;
      if (event.key === 'End') next = SIDEBAR_MAX;
      if (next === null) return;
      event.preventDefault();
      setLocal(KEY_WIDTH, String(applyWidth(next)));
    });
  }

  // selectSidebarTab opens the pane of one of the two sidebar tabs.
  function selectSidebarTab(tab) {
    each('[data-fleet-action="sidebar-tab"]', function (other) {
      var active = other === tab;
      other.classList.toggle('fleet-sidebar-tab--active', active);
      other.setAttribute('aria-selected', active ? 'true' : 'false');
      var pane = byId(other.getAttribute('aria-controls'));
      if (pane) pane.hidden = !active;
    });
  }

  // --- Tree -----------------------------------------------------------------

  function treeElement() { return byId('fleet-tree'); }

  function treeViewName() {
    var list = document.querySelector('#fleet-tree .fleet-tree-list');
    return (list && list.getAttribute('data-fleet-view')) || TREE_VIEW_DEFAULT;
  }

  // The rows the operator expanded or collapsed are kept per tree view, for
  // the browser session: true for expanded, false for collapsed.
  function treeState() {
    var raw = read(window.sessionStorage, KEY_TREE_STATE + treeViewName());
    if (!raw) return {};
    try {
      var state = JSON.parse(raw);
      return state && typeof state === 'object' ? state : {};
    } catch (_) {
      return {};
    }
  }

  function rememberRow(key, expanded) {
    var state = treeState();
    if (Object.keys(state).length >= TREE_STATE_MAX) state = {};
    state[key] = expanded;
    write(window.sessionStorage, KEY_TREE_STATE + treeViewName(), JSON.stringify(state));
  }

  function childList(item) {
    for (var node = item.firstElementChild; node; node = node.nextElementSibling) {
      if (node.classList.contains('fleet-tree-children')) return node;
    }
    return null;
  }

  // setExpanded opens or closes one row. A row whose children were left out
  // of the tree loads them the first time it opens, with skeleton rows in
  // their place meanwhile.
  function setExpanded(item, expanded) {
    if (!item.hasAttribute('aria-expanded')) return;
    item.setAttribute('aria-expanded', expanded ? 'true' : 'false');

    var children = childList(item);
    if (!children) return;
    children.hidden = !expanded;

    var lazy = item.getAttribute('data-fleet-lazy');
    if (expanded && lazy) {
      item.removeAttribute('data-fleet-lazy');
      load(lazy, children, 'innerHTML');
    }
  }

  // applyTreeState re-applies the remembered rows to the tree on the page. A
  // row that leads to the selected row stays open, whatever was remembered:
  // the selected row is always in view after a load.
  function applyTreeState() {
    var tree = treeElement();
    if (!tree) return;
    var state = treeState();

    each('.fleet-tree-item[aria-expanded]', function (item) {
      var key = item.getAttribute('data-fleet-key');
      if (!Object.prototype.hasOwnProperty.call(state, key)) return;
      var want = state[key] === true;
      var open = item.getAttribute('aria-expanded') === 'true';
      if (want === open) return;
      if (!want && item.querySelector('.fleet-tree-row--selected')) return;
      setExpanded(item, want);
    }, tree);
  }

  function scrollToSelected() {
    var row = document.querySelector('#fleet-tree .fleet-tree-row--selected');
    if (row && row.scrollIntoView) row.scrollIntoView({ block: 'nearest' });
  }

  function toggleRow(item) {
    var expanded = item.getAttribute('aria-expanded') !== 'true';
    setExpanded(item, expanded);
    rememberRow(item.getAttribute('data-fleet-key'), expanded);
  }

  function visibleTreeLinks() {
    var links = [];
    each('#fleet-tree .fleet-tree-link', function (link) {
      if (link.offsetParent !== null) links.push(link);
    });
    return links;
  }

  function treeItemOf(element) {
    return element.closest('.fleet-tree-item');
  }

  // Up and down move, right expands, left collapses, enter opens: the link
  // of the row is what holds the focus, so enter is the link's own.
  function onTreeKey(event) {
    var link = event.target.closest('.fleet-tree-link');
    if (!link) return;
    var item = treeItemOf(link);
    var links = visibleTreeLinks();
    var at = links.indexOf(link);
    var target = null;

    switch (event.key) {
      case 'ArrowDown':
        target = links[at + 1];
        break;
      case 'ArrowUp':
        target = links[at - 1];
        break;
      case 'Home':
        target = links[0];
        break;
      case 'End':
        target = links[links.length - 1];
        break;
      case 'ArrowRight':
        if (!item.hasAttribute('aria-expanded')) return;
        if (item.getAttribute('aria-expanded') === 'true') {
          target = links[at + 1];
        } else {
          toggleRow(item);
        }
        break;
      case 'ArrowLeft':
        if (item.getAttribute('aria-expanded') === 'true') {
          toggleRow(item);
        } else {
          var parent = item.parentElement ? treeItemOf(item.parentElement) : null;
          target = parent ? parent.querySelector('.fleet-tree-link') : null;
        }
        break;
      default:
        return;
    }

    event.preventDefault();
    if (target) target.focus();
  }

  var filterTimer = null;

  function refreshTree() {
    var tree = treeElement();
    if (tree) refreshElement(tree);
  }

  function onTreeFilterInput() {
    window.clearTimeout(filterTimer);
    filterTimer = window.setTimeout(refreshTree, FILTER_DELAY);
  }

  function clearTreeFilter() {
    var field = byId('fleet-tree-filter');
    if (!field) return;
    field.value = '';
    window.clearTimeout(filterTimer);
    refreshTree();
    field.focus();
  }

  function viewOf(address) {
    try {
      return new URL(address, window.location.origin).searchParams.get('view') || TREE_VIEW_DEFAULT;
    } catch (_) {
      return TREE_VIEW_DEFAULT;
    }
  }

  // The tree view is in the address and remembered per browser: an address
  // that names none opens under the view the operator chose last.
  function initTreeView() {
    var select = byId('fleet-tree-view');
    if (!select) return;
    if (new URLSearchParams(window.location.search).has('view')) return;

    var saved = local(KEY_TREE_VIEW);
    if (!saved || saved === TREE_VIEW_DEFAULT) return;

    for (var index = 0; index < select.options.length; index++) {
      if (viewOf(select.options[index].value) === saved) {
        window.location.replace(select.options[index].value);
        return;
      }
    }
  }

  function onTreeViewChange(select) {
    setLocal(KEY_TREE_VIEW, viewOf(select.value));
    window.location.assign(select.value);
  }

  // --- Side panel -----------------------------------------------------------

  function panelPinned() {
    return local(KEY_PANEL_PINNED) !== '0';
  }

  // applyPanel shows the side panel region as its state and its pin ask:
  // pinned, it stays beside the content and says how to fill it; unpinned, it
  // opens over the content only while it shows a row.
  function applyPanel() {
    var panel = byId('fleet-side-panel');
    if (!panel) return;

    var state = panel.getAttribute('data-fleet-panel');
    var isPinned = panelPinned();
    var open = state === 'open';

    panel.hidden = !(open || (state === 'empty' && isPinned));
    panel.classList.toggle('fleet-side-panel--overlay', open && !isPinned);
    body.classList.toggle('fleet-shell--panel', !panel.hidden && isPinned);

    each('[data-fleet-panel-pin]', function (button) {
      var label = isPinned ? 'Unpin the side panel' : 'Pin the side panel';
      button.setAttribute('aria-pressed', isPinned ? 'true' : 'false');
      button.setAttribute('aria-label', label);
      button.setAttribute('title', label);
    }, panel);
  }

  function togglePanelPin() {
    setLocal(KEY_PANEL_PINNED, panelPinned() ? '0' : '1');
    applyPanel();
  }

  // --- Regions --------------------------------------------------------------

  // refreshElement fetches again the fragment one element of a region names,
  // into that element. An element that names a field sends its text as the
  // item, which is how the tree keeps its filter across a refresh.
  function refreshElement(element) {
    var name = element.getAttribute('data-fleet-name');
    if (!name) return;

    var item = '';
    var from = element.getAttribute('data-fleet-item-from');
    if (from) {
      var field = byId(from);
      if (field) item = field.value;
    }

    load(fragmentURL(name, item), element, 'innerHTML');
  }

  // refreshTasks fetches the task panel again. It answers itself whole, on
  // the tab it shows, and closed when it is closed.
  function refreshTasks() {
    var panel = byId('fleet-task-panel');
    if (!panel) return;

    var url = fragmentURL('tasks', '');
    if (!panel.hidden) {
      var active = panel.querySelector('.fleet-task-tab--active');
      url = (active && active.getAttribute('hx-get')) || fragmentURL('tasks', 'running');
    }

    load(url, panel, 'outerHTML');
  }

  function refreshRegion(region) {
    if (region === 'tasks') {
      refreshTasks();
      return;
    }
    each('[data-fleet-region="' + region + '"]', refreshElement);
  }

  var REGIONS = ['tree', 'content', 'tasks', 'header'];

  function refreshEverything() {
    REGIONS.forEach(refreshRegion);
  }

  // --- Live events ----------------------------------------------------------

  var stream = null;
  var streamPath = null;
  var streamTimer = null;
  var streamAttempts = 0;
  var streamLost = false;

  function eventsURL() {
    var content = byId('fleet-content-body');
    return content ? content.getAttribute('data-fleet-events') : null;
  }

  function showStale() {
    var banner = byId('fleet-stale');
    if (!banner || !banner.hidden) return;
    var time = byId('fleet-stale-time');
    if (time) {
      var now = new Date();
      time.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      time.setAttribute('datetime', now.toISOString());
    }
    banner.hidden = false;
  }

  function hideStale() {
    var banner = byId('fleet-stale');
    if (banner) banner.hidden = true;
  }

  function closeStream() {
    window.clearTimeout(streamTimer);
    streamTimer = null;
    if (stream) {
      stream.close();
      stream = null;
    }
  }

  // streamFailed shows the banner and tries again, each time after twice the
  // wait, up to BACKOFF_MAX, and RECONNECT_MAX times. After that the banner
  // stays until the operator asks.
  function streamFailed() {
    closeStream();
    streamLost = true;
    showStale();

    if (streamAttempts >= RECONNECT_MAX) return;
    var wait = Math.min(BACKOFF_MAX, BACKOFF_FIRST * Math.pow(2, streamAttempts));
    streamAttempts++;
    streamTimer = window.setTimeout(openStream, wait);
  }

  // openStream follows the page in hand. A page with no stream address, the
  // not-found state, follows none.
  function openStream() {
    closeStream();
    streamPath = window.location.pathname;

    var base = eventsURL();
    if (!base || !window.EventSource) return;

    stream = new window.EventSource(base + '?scope=' + encodeURIComponent(streamPath));

    stream.onopen = function () {
      streamAttempts = 0;
      if (!streamLost) return;
      // The stream is back. What changed meanwhile is not known, so every
      // region is fetched again, once.
      streamLost = false;
      hideStale();
      refreshEverything();
    };

    stream.addEventListener('stale', function (event) {
      String(event.data).split(',').forEach(function (region) {
        if (REGIONS.indexOf(region) >= 0) refreshRegion(region);
      });
    });

    stream.onerror = streamFailed;
  }

  function reconnect() {
    streamAttempts = 0;
    streamLost = true;
    openStream();
  }

  // --- Header search --------------------------------------------------------

  var searchTimer = null;
  var searchOpensFirst = false;

  function searchField() { return byId('fleet-search'); }
  function searchBox() { return byId('fleet-search-results'); }

  function setSearchOpen(open) {
    var box = searchBox();
    var field = searchField();
    if (box) box.hidden = !open;
    if (field) field.setAttribute('aria-expanded', open ? 'true' : 'false');
  }

  function runSearch() {
    var field = searchField();
    var box = searchBox();
    if (!field || !box) return;

    window.clearTimeout(searchTimer);
    searchTimer = null;

    var text = field.value.trim();
    if (text === '') {
      box.textContent = '';
      setSearchOpen(false);
      return;
    }

    var base = field.getAttribute('data-fleet-fragment') || '';
    var join = base.indexOf('?') >= 0 ? '&' : '?';
    load(base + join + 'item=' + encodeURIComponent(text), box, 'innerHTML');
    setSearchOpen(true);
  }

  function closeSearch(clear) {
    var field = searchField();
    var box = searchBox();
    window.clearTimeout(searchTimer);
    searchTimer = null;
    searchOpensFirst = false;
    if (clear && field) field.value = '';
    if (clear && box) box.textContent = '';
    setSearchOpen(false);
  }

  function searchLinks() {
    var links = [];
    each('#fleet-search-results .fleet-search-link', function (link) { links.push(link); });
    return links;
  }

  function onSearchKey(event) {
    var field = searchField();
    var links = searchLinks();
    var inField = event.target === field;
    var at = links.indexOf(event.target);
    if (!inField && at < 0) return;

    if (event.key === 'Escape') {
      closeSearch(true);
      if (field) field.focus();
      event.preventDefault();
      return;
    }

    if (event.key === 'ArrowDown') {
      if (links[at + 1]) links[at + 1].focus();
      event.preventDefault();
      return;
    }

    if (event.key === 'ArrowUp') {
      if (at > 0) links[at - 1].focus();
      if (at === 0 && field) field.focus();
      event.preventDefault();
      return;
    }

    // Enter in the field opens the first result. When the text changed since
    // the last answer, the search runs first and the answer is opened.
    if (event.key === 'Enter' && inField) {
      event.preventDefault();
      if (searchTimer !== null) {
        searchOpensFirst = true;
        runSearch();
        return;
      }
      if (links[0]) links[0].click();
    }
  }

  // --- After a swap ---------------------------------------------------------

  // afterSwap runs after htmx swapped anything: a navigation inside the
  // shell, a refreshed region, a branch of the tree.
  function afterSwap() {
    var content = byId('fleet-content-body');
    var title = content ? content.getAttribute('data-fleet-title') : null;
    if (title) document.title = title;

    applyTreeState();
    applyPanel();

    if (searchOpensFirst) {
      var links = searchLinks();
      if (links[0]) {
        searchOpensFirst = false;
        links[0].click();
      }
    }

    // The page changed: the stream follows the new one, and the selected
    // row is brought into view.
    if (streamPath !== window.location.pathname) {
      streamAttempts = 0;
      openStream();
      scrollToSelected();
    }
  }

  // --- Listeners ------------------------------------------------------------

  function typing(target) {
    if (!target || !target.tagName) return false;
    var tag = target.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
  }

  function onClick(event) {
    var target = event.target;
    if (!target || !target.closest) return;

    if (target.closest('[data-fleet-panel-pin]')) {
      togglePanelPin();
      return;
    }

    // A link of the search closes it, and a link of an overlaying sidebar
    // closes the sidebar: both got the operator where they wanted to go.
    if (target.closest('.fleet-search-link')) closeSearch(true);
    if (target.closest('.fleet-tree-link, .fleet-admin-link') && overlaying()) setCollapsed(true);
    if (!target.closest('.fleet-search')) closeSearch(false);

    var control = target.closest('[data-fleet-action]');
    if (!control) return;

    switch (control.getAttribute('data-fleet-action')) {
      case 'theme-toggle':
        toggleTheme();
        break;
      case 'sidebar-toggle':
        toggleSidebar();
        break;
      case 'sidebar-pin':
        togglePin();
        break;
      case 'sidebar-tab':
        selectSidebarTab(control);
        break;
      case 'tree-toggle':
        toggleRow(treeItemOf(control));
        break;
      case 'tree-filter-clear':
        clearTreeFilter();
        break;
      case 'events-reconnect':
        reconnect();
        break;
      default:
        break;
    }
  }

  function onKey(event) {
    if (event.target && event.target.closest) {
      if (event.target.closest('#fleet-tree')) onTreeKey(event);
      if (event.target.closest('.fleet-search')) onSearchKey(event);
    }

    if (event.target && event.target.id === 'fleet-tree-filter' && event.key === 'Escape') {
      clearTreeFilter();
      event.preventDefault();
      return;
    }

    // The slash focuses the tree filter, unless the operator is typing.
    if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !typing(event.target)) {
      var field = byId('fleet-tree-filter');
      if (!field) return;
      var estate = byId('fleet-tab-estate');
      if (estate) selectSidebarTab(estate);
      setCollapsed(false);
      field.focus();
      event.preventDefault();
    }
  }

  function onInput(event) {
    if (event.target.id === 'fleet-tree-filter') onTreeFilterInput();
    if (event.target.id === 'fleet-search') {
      window.clearTimeout(searchTimer);
      searchTimer = window.setTimeout(runSearch, SEARCH_DELAY);
    }
  }

  function onChange(event) {
    if (event.target.getAttribute && event.target.getAttribute('data-fleet-action') === 'tree-view') {
      onTreeViewChange(event.target);
    }
  }

  function start() {
    initTreeView();
    initSidebar();
    initSidebarHandle();
    applyTreeState();
    applyPanel();
    scrollToSelected();
    openStream();

    document.addEventListener('click', onClick);
    document.addEventListener('keydown', onKey);
    document.addEventListener('input', onInput);
    document.addEventListener('change', onChange);
    document.addEventListener('htmx:after:swap', afterSwap);
    window.addEventListener('popstate', function () { window.setTimeout(afterSwap, 0); });
    window.addEventListener('pagehide', closeStream);
  }

  // The script is deferred, so the document is parsed when it runs.
  start();
})();
