/* Ze looking glass graph mode -- marks the graph-mode button the operator
   pressed. The buttons carry data-mode and the HTMX attributes that fetch the
   drawing, and this file carries the class change.

   It is external because the looking glass answers every page with
   `default-src 'self'` and no script-src beside it (setSecurityHeaders,
   server.go). A browser refuses an inline handler under that header, so the
   onclick this replaced ran in no browser and the pressed button never looked
   pressed.

   The listener sits on the document, so it survives the HTMX swap that
   replaces #results with a new result panel and its buttons. */
(function(){
  'use strict';
  document.addEventListener('click', function(e) {
    var btn = e.target.closest ? e.target.closest('.graph-mode-btn') : null;
    if (!btn) return;

    var group = btn.closest('.graph-controls');
    if (!group) return;

    Array.prototype.forEach.call(group.querySelectorAll('.graph-mode-btn'), function(b) {
      b.classList.remove('active');
    });
    btn.classList.add('active');
  });
})();
