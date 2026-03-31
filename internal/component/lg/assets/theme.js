(function(){
  var t = localStorage.getItem('lg-theme');
  if (t) document.documentElement.setAttribute('data-theme', t);
})();

document.addEventListener('DOMContentLoaded', function() {
  // Tab active state -- update on HTMX navigation
  document.querySelectorAll('.tab-bar a').forEach(function(tab) {
    tab.addEventListener('click', function() {
      document.querySelectorAll('.tab-bar a').forEach(function(t) { t.classList.remove('tab-active'); });
      this.classList.add('tab-active');
    });
  });

  var btn = document.getElementById('theme-toggle');
  if (btn) btn.addEventListener('click', function() {
    var h = document.documentElement;
    var c = h.getAttribute('data-theme');
    var n = c === 'dark' ? 'light' : c === 'light' ? 'dark'
      : window.matchMedia('(prefers-color-scheme:dark)').matches ? 'light' : 'dark';
    h.setAttribute('data-theme', n);
    localStorage.setItem('lg-theme', n);
  });
});
