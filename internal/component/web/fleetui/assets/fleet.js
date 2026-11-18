// Design: docs/architecture/fleet-interface.md -- fleet shell script: the stored theme
//
// Applies the theme the operator chose, before the first paint. The choice is
// the browser setting `ze-theme`, the one the other web modes write
// (internal/component/web/assets/cli.js), so it carries across modes. The page
// loads this file from its head, with no defer, for that reason.
(function () {
  'use strict';

  var saved = null;
  try {
    saved = window.localStorage.getItem('ze-theme');
  } catch (_) {
    // The browser refuses storage access: keep the default theme.
  }

  // Only the two themes the stylesheet declares are applied.
  if (saved === 'light' || saved === 'dark') {
    document.documentElement.setAttribute('data-theme', saved);
  }
})();
