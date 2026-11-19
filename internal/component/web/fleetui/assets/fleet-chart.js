/* Design: docs/architecture/fleet-interface.md -- draws the line chart of a fleet chart card with uPlot
 *
 * A chart card (ChartCard in component_card.templ) holds a .fleet-chart
 * element naming the address of its data in data-chart-url. The address
 * answers this JSON, already reduced by the hub to the width of a chart:
 *
 *   {
 *     "unit": "%",                        "%", "bit/s", "B" or a count's word: figures are
 *                                         written in its short form (Mb/s, GiB, 42%)
 *     "timestamps": [1763500000, ...],    Unix seconds, ascending, one per period
 *     "series": [
 *       {
 *         "name": "CPU",                  equal to a legend item's data-chart-series
 *         "average": [12.5, null, 14.1],  one value per timestamp
 *         "maximum": [30.0, null, 31.2]   optional, one value per timestamp
 *       }
 *     ],
 *     "empty": false,                     true when the history holds no sample
 *     "message": "No samples have been reported yet",
 *     "expected": "The first is expected at 14:05",
 *     "refresh": 60                       seconds until the data is asked again; 0 or absent: never
 *   }
 *
 * A null value is a period with no sample. It is drawn as a gap: the line is
 * never joined across it. A series the legend does not name is not drawn, and
 * at most four are.
 *
 * Colours are read from the chart variables when a chart is drawn, so the
 * shell script redraws every chart by dispatching the document event
 * "fleet:theme" after it changes the theme.
 */
(function() {
  'use strict';

  var REFRESH_SECONDS_MIN = 10;
  var PLOT_HEIGHT_MIN = 160;

  // charts holds what is drawn, keyed by the .fleet-chart element.
  var charts = new Map();

  function cssValue(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }

  function note(root, text) {
    var element = root.querySelector('[data-chart-note]');
    if (!element) return;
    element.textContent = text;
    element.hidden = text === '';
  }

  function legendItems(root) {
    return Array.prototype.slice.call(root.querySelectorAll('[data-chart-series]'));
  }

  // SCALED holds the units whose figures are written with a prefix: bits per
  // second in steps of 1000 (kb/s, Mb/s, Gb/s), bytes in steps of 1024 (KiB,
  // MiB, GiB). A percentage carries its sign; a count is written plain, its
  // unit named once, on the vertical axis.
  var SCALED = {
    'bit/s': { base: 1000, steps: ['b/s', 'kb/s', 'Mb/s', 'Gb/s', 'Tb/s'] },
    'B': { base: 1024, steps: ['B', 'KiB', 'MiB', 'GiB', 'TiB'] }
  };

  // AXIS_CHAR_WIDTH and AXIS_PADDING size the vertical axis to its widest
  // label, in pixels, so no label is clipped.
  var AXIS_CHAR_WIDTH = 7;
  var AXIS_PADDING = 18;

  function shortNumber(value) {
    var size = Math.abs(value);
    if (size >= 100) return String(Math.round(value));
    if (size >= 10) return String(Math.round(value * 10) / 10);
    return String(Math.round(value * 100) / 100);
  }

  // formatValue writes a figure in the short form of its unit: "2.4 Gb/s",
  // "512 MiB", "42%", or a plain count.
  function formatValue(value, unit) {
    if (value === null || value === undefined) return '-';
    var scale = SCALED[unit];
    if (scale) {
      var step = 0;
      while (Math.abs(value) >= scale.base && step < scale.steps.length - 1) {
        value = value / scale.base;
        step++;
      }
      return shortNumber(value) + ' ' + scale.steps[step];
    }
    if (unit === '%') return shortNumber(value) + '%';
    return shortNumber(value);
  }

  // axisLabel answers the caption of the vertical axis: the unit of a count,
  // which its figures do not carry. A scaled unit and a percentage are written
  // on every figure, so the axis needs no caption.
  function axisLabel(unit) {
    if (!unit || SCALED[unit] || unit === '%') return '';
    return unit;
  }

  // axisSize answers the width of the vertical axis for its labels.
  function axisSize(plot, values) {
    var longest = 0;
    (values || []).forEach(function(text) {
      longest = Math.max(longest, String(text).length);
    });
    return longest * AXIS_CHAR_WIDTH + AXIS_PADDING;
  }

  function latest(values) {
    for (var index = values.length - 1; index >= 0; index--) {
      if (values[index] !== null && values[index] !== undefined) return values[index];
    }
    return null;
  }

  // lines pairs each legend item with its series of the answer, in legend
  // order. An item whose series the answer does not hold is left out.
  function lines(root, data) {
    var byName = {};
    (data.series || []).forEach(function(series) { byName[series.name] = series; });

    var result = [];
    legendItems(root).forEach(function(item) {
      var series = byName[item.getAttribute('data-chart-series')];
      if (!series || !Array.isArray(series.average)) return;
      result.push({
        item: item,
        name: series.name,
        color: cssValue('--chart-' + item.getAttribute('data-chart-color')),
        average: series.average,
        maximum: item.getAttribute('data-chart-maximum') === 'true' && Array.isArray(series.maximum) ? series.maximum : null
      });
    });
    return result;
  }

  function showLatest(root, drawn, unit) {
    drawn.forEach(function(line) {
      var element = line.item.querySelector('[data-chart-latest]');
      if (element) element.textContent = formatValue(latest(line.average), unit);
    });
    var cursor = root.querySelector('[data-chart-cursor]');
    if (cursor) cursor.textContent = '';
  }

  function showCursor(root, plot, drawn, unit, timestamps) {
    var index = plot.cursor.idx;
    if (index === null || index === undefined) {
      showLatest(root, drawn, unit);
      return;
    }

    drawn.forEach(function(line) {
      var element = line.item.querySelector('[data-chart-latest]');
      if (element) element.textContent = formatValue(line.average[index], unit);
    });
    var cursor = root.querySelector('[data-chart-cursor]');
    if (cursor) cursor.textContent = new Date(timestamps[index] * 1000).toLocaleString();
  }

  function destroy(root) {
    var state = charts.get(root);
    if (!state) return;
    if (state.plot) state.plot.destroy();
    state.plot = null;
  }

  function draw(root) {
    var state = charts.get(root);
    if (!state || !state.data) return;
    destroy(root);

    var data = state.data;
    var timestamps = data.timestamps || [];
    var drawn = lines(root, data);
    var holder = root.querySelector('[data-chart-plot]');
    if (!holder) return;

    if (data.empty === true || timestamps.length === 0 || drawn.length === 0) {
      holder.hidden = true;
      var message = data.message || 'No samples have been reported yet';
      note(root, data.expected ? message + '. ' + data.expected : message);
      showLatest(root, drawn, data.unit);
      return;
    }

    holder.hidden = false;
    note(root, '');

    var axis = cssValue('--text-secondary');
    var grid = cssValue('--border');
    var series = [{}];
    var columns = [timestamps];
    drawn.forEach(function(line) {
      series.push({ label: line.name, stroke: line.color, width: 2, spanGaps: false, points: { show: false } });
      columns.push(line.average);
      if (line.maximum) {
        series.push({ label: line.name + ' maximum', stroke: line.color, width: 1, dash: [4, 4], spanGaps: false, points: { show: false } });
        columns.push(line.maximum);
      }
    });

    var options = {
      width: Math.max(holder.clientWidth, 1),
      height: Math.max(holder.clientHeight, PLOT_HEIGHT_MIN),
      legend: { show: false },
      cursor: { points: { show: false } },
      series: series,
      axes: [
        { stroke: axis, grid: { stroke: grid, width: 1 }, ticks: { stroke: grid, width: 1 } },
        {
          stroke: axis,
          grid: { stroke: grid, width: 1 },
          ticks: { stroke: grid, width: 1 },
          values: function(plot, ticks) {
            return ticks.map(function(tick) { return formatValue(tick, data.unit); });
          },
          size: axisSize,
          label: axisLabel(data.unit),
          labelSize: axisLabel(data.unit) ? 20 : 0
        }
      ],
      hooks: {
        setCursor: [function(plot) { showCursor(root, plot, drawn, data.unit, timestamps); }]
      }
    };

    state.plot = new uPlot(options, columns, holder);
    showLatest(root, drawn, data.unit);
  }

  function scheduleRefresh(root, seconds) {
    var state = charts.get(root);
    if (!state) return;
    if (state.timer) clearTimeout(state.timer);
    state.timer = null;
    if (!seconds || seconds <= 0) return;

    state.timer = setTimeout(function() {
      if (root.isConnected) load(root);
    }, Math.max(seconds, REFRESH_SECONDS_MIN) * 1000);
  }

  function load(root) {
    var url = root.getAttribute('data-chart-url');
    if (!url || typeof uPlot === 'undefined') {
      note(root, 'The chart cannot be drawn: its script did not load. Reload the page.');
      return;
    }

    fetch(url, { headers: { Accept: 'application/json' }, credentials: 'same-origin' })
      .then(function(response) {
        if (!response.ok) throw new Error('status ' + response.status);
        return response.json();
      })
      .then(function(data) {
        var state = charts.get(root);
        if (!state || !root.isConnected) return;
        state.data = data;
        draw(root);
        scheduleRefresh(root, data.refresh);
      })
      .catch(function(error) {
        destroy(root);
        note(root, 'The samples could not be loaded (' + (error && error.message ? error.message : 'no reason given') + '). Reload the page to ask again.');
      });
  }

  // scan forgets the charts a swap removed and starts the ones it brought. A
  // new chart range is a new address, so the swap brings new chart elements.
  function scan() {
    charts.forEach(function(state, root) {
      if (root.isConnected) return;
      if (state.timer) clearTimeout(state.timer);
      destroy(root);
      charts.delete(root);
    });

    document.querySelectorAll('.fleet-chart[data-chart-url]').forEach(function(root) {
      if (charts.has(root)) return;
      charts.set(root, { data: null, plot: null, timer: null });
      load(root);
    });
  }

  function redraw() {
    charts.forEach(function(state, root) {
      if (root.isConnected) draw(root);
    });
  }

  var resizing = false;
  function onResize() {
    if (resizing) return;
    resizing = true;
    requestAnimationFrame(function() {
      resizing = false;
      charts.forEach(function(state, root) {
        var holder = root.querySelector('[data-chart-plot]');
        if (state.plot && holder) {
          state.plot.setSize({ width: Math.max(holder.clientWidth, 1), height: state.plot.height });
        }
      });
    });
  }

  document.addEventListener('fleet:theme', redraw);
  document.addEventListener('htmx:after:swap', scan);
  window.addEventListener('resize', onResize);

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', scan);
  } else {
    scan();
  }
})();
