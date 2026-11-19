// Design: docs/architecture/fleet-interface.md -- fleet shell script: the stored theme and panel pin
//
// Applies the two stored choices that change the first paint. The theme is
// the browser setting `ze-theme`, the one the other web modes write
// (internal/component/web/assets/cli.js), so it carries across modes. The side
// panel pin is `ze-fleet-panel-pinned`, which fleet.js writes: the server
// renders the unpinned form, the default, and the class set here makes the
// stylesheet draw the pinned one before anything is painted. The page loads
// this file from its head, with no defer, for that reason, and it does
// nothing else: everything that can wait is in fleet.js.
(function () {
  'use strict';

  var saved = null;
  var panelPin = null;
  try {
    saved = window.localStorage.getItem('ze-theme');
    panelPin = window.localStorage.getItem('ze-fleet-panel-pinned');
  } catch (_) {
    // The browser refuses storage access: keep the default theme and the
    // unpinned panel.
  }

  // Only the two themes the stylesheet declares are applied.
  if (saved === 'light' || saved === 'dark') {
    document.documentElement.setAttribute('data-theme', saved);
  }

  // Only the value fleet.js writes for a pinned panel pins it.
  if (panelPin === '1') {
    document.documentElement.classList.add('fleet-panel-pinned');
  }
})();
