// L2TP CQM chart: uPlot init from JSON, SSE append, CSS color bands.
function initCQMChart(elemId, login) {
  var el = document.getElementById(elemId);
  if (!el) return;

  var cs = getComputedStyle(document.documentElement);
  var colorEstablished = cs.getPropertyValue("--color-l2tp-established").trim() || "#22c55e";
  var colorNegotiating = cs.getPropertyValue("--color-l2tp-negotiating").trim() || "#f59e0b";
  var colorDown = cs.getPropertyValue("--color-l2tp-down").trim() || "#a855f7";

  var chart = null;
  var chartData = null;

  function stateColor(state) {
    if (state === "established") return colorEstablished;
    if (state === "negotiating") return colorNegotiating;
    if (state === "down") return colorDown;
    return "#888";
  }

  function buildOpts(width) {
    return {
      width: width,
      height: 300,
      series: [
        {},
        { label: "Min RTT", stroke: colorEstablished, width: 1 },
        { label: "Avg RTT", stroke: colorNegotiating, width: 2 },
        { label: "Max RTT", stroke: colorDown, width: 1 }
      ],
      axes: [
        {},
        { label: "RTT (us)" }
      ]
    };
  }

  function render(data) {
    var uData = [
      data.timestamps.map(function(t) { return t; }),
      data.minRTT,
      data.avgRTT,
      data.maxRTT
    ];
    chartData = data;
    if (chart) chart.destroy();
    chart = new uPlot(buildOpts(el.clientWidth), uData, el);
  }

  fetch("/l2tp/" + encodeURIComponent(login) + "/samples")
    .then(function(r) { return r.json(); })
    .then(function(data) {
      if (data.timestamps && data.timestamps.length > 0) {
        render(data);
      } else {
        el.textContent = "No CQM data available.";
      }
    })
    .catch(function() {
      el.textContent = "Failed to load CQM data.";
    });

  var evtSrc = new EventSource("/l2tp/" + encodeURIComponent(login) + "/samples/stream");
  evtSrc.addEventListener("bucket", function(e) {
    try {
      var newData = JSON.parse(e.data);
      if (!chartData || !newData.timestamps || newData.timestamps.length === 0) return;
      chartData.timestamps = chartData.timestamps.concat(newData.timestamps);
      chartData.minRTT = chartData.minRTT.concat(newData.minRTT);
      chartData.avgRTT = chartData.avgRTT.concat(newData.avgRTT);
      chartData.maxRTT = chartData.maxRTT.concat(newData.maxRTT);
      chartData.states = chartData.states.concat(newData.states);
      render(chartData);
    } catch (ex) {
      // ignore parse errors
    }
  });
}

document.addEventListener("DOMContentLoaded", function() {
  var chart = document.getElementById("cqm-chart");
  if (chart) {
    var login = chart.getAttribute("data-login");
    if (login) initCQMChart("cqm-chart", login);
  }

  document.addEventListener("submit", function(e) {
    var form = e.target.closest("[data-confirm]");
    if (!form) return;
    var message = form.getAttribute("data-confirm");
    if (message && !window.confirm(message)) {
      e.preventDefault();
    }
  });
});
