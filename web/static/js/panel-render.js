// panel-render.js — Grafana-like visual panel renderers
// Renders preview panels to look like actual Grafana panels (stat, gauge, timeseries, etc.)
// All rendering is client-side using the PanelInfo JSON embedded in the page.
// Security: All text values pass through escHtml() before DOM insertion.
// Panel data originates from server-side Go JSON marshaling (trusted source).

(function() {
  'use strict';

  // Grafana's classic palette colors
  var PALETTE = [
    '#7EB26D', '#EAB839', '#6ED0E0', '#EF843C', '#E24D42',
    '#1F78C4', '#BA43A9', '#705DA0', '#508642', '#CCA300',
    '#447EBC', '#C15C17', '#890F02', '#0A437C', '#6D1F62',
    '#584477', '#B7DBAB', '#F4D598', '#70DBED', '#F9BA8F'
  ];

  // Resolve Grafana named colors to hex
  function resolveColor(c) {
    if (!c) return '#888';
    if (c.charAt(0) === '#') return c;
    var named = {
      green: '#73BF69', red: '#F2495C', orange: '#FF9830',
      yellow: '#FADE2A', blue: '#5794F2', purple: '#B877D9',
      'super-light-green': '#96D98D', 'light-green': '#73BF69',
      'semi-dark-green': '#56A64B', 'dark-green': '#37872D',
      'super-light-red': '#FF7383', 'light-red': '#F2495C',
      'semi-dark-red': '#E02F44', 'dark-red': '#C4162A',
      'super-light-blue': '#8AB8FF', 'light-blue': '#5794F2',
      'semi-dark-blue': '#3274D9', 'dark-blue': '#1F60C4',
      'super-light-yellow': '#FADE2A', 'light-yellow': '#F2CC0C',
      'semi-dark-yellow': '#E0B400', 'dark-yellow': '#CC9D00',
      'super-light-orange': '#FFB357', 'light-orange': '#FF9830',
      'semi-dark-orange': '#E0752D', 'dark-orange': '#C4612A',
      'super-light-purple': '#DEB6F2', 'light-purple': '#B877D9',
      'semi-dark-purple': '#8F3BB8', 'dark-purple': '#6C2EA2',
      transparent: 'transparent', white: '#fff', 'text': '#ccc'
    };
    return named[c] || '#888';
  }

  // Get threshold color for a value
  function thresholdColor(thresholds, val) {
    if (!thresholds || thresholds.length === 0) return '#73BF69';
    var color = resolveColor(thresholds[0].Color);
    for (var i = 1; i < thresholds.length; i++) {
      var tv = parseFloat(thresholds[i].Value);
      if (!isNaN(tv) && val >= tv) {
        color = resolveColor(thresholds[i].Color);
      }
    }
    return color;
  }

  // Format a value with unit
  function formatValue(val, unit) {
    if (val === null || val === undefined || isNaN(val)) return '\u2014';
    if (unit === 'percent' || unit === 'percentunit') {
      var pv = unit === 'percentunit' ? val * 100 : val;
      return pv.toFixed(1) + '%';
    }
    if (unit === 'bytes' || unit === 'decbytes') return formatBytes(val);
    if (unit === 'Bps' || unit === 'binBps') return formatBytes(val) + '/s';
    if (unit === 's' || unit === 'seconds') return formatDuration(val);
    if (unit === 'ms') return formatDuration(val / 1000);
    if (unit === 'short' || unit === 'none' || !unit) {
      if (Math.abs(val) >= 1e9) return (val / 1e9).toFixed(1) + 'B';
      if (Math.abs(val) >= 1e6) return (val / 1e6).toFixed(1) + 'M';
      if (Math.abs(val) >= 1e3) return (val / 1e3).toFixed(1) + 'K';
      return val % 1 === 0 ? val.toString() : val.toFixed(2);
    }
    return val.toFixed(2) + ' ' + unit;
  }

  function formatBytes(b) {
    if (Math.abs(b) < 1024) return b.toFixed(0) + ' B';
    if (Math.abs(b) < 1048576) return (b / 1024).toFixed(1) + ' KiB';
    if (Math.abs(b) < 1073741824) return (b / 1048576).toFixed(1) + ' MiB';
    return (b / 1073741824).toFixed(2) + ' GiB';
  }

  function formatDuration(s) {
    if (Math.abs(s) < 0.001) return (s * 1e6).toFixed(0) + ' \u00b5s';
    if (Math.abs(s) < 1) return (s * 1000).toFixed(1) + ' ms';
    if (Math.abs(s) < 60) return s.toFixed(1) + ' s';
    if (Math.abs(s) < 3600) return (s / 60).toFixed(1) + ' min';
    return (s / 3600).toFixed(1) + ' h';
  }

  // Generate a fake display value based on panel ID as seed
  function fakeValue(panel) {
    var seed = panel.ID || 1;
    var r = ((seed * 9301 + 49297) % 233280) / 233280;

    if (panel.Unit === 'percent' || panel.Unit === 'percentunit') {
      return panel.Unit === 'percentunit' ? r * 0.95 + 0.02 : r * 95 + 2;
    }
    if (panel.Unit === 'bytes' || panel.Unit === 'decbytes') {
      return r * 8e9 + 1e6;
    }
    if (panel.Unit === 'Bps' || panel.Unit === 'binBps') {
      return r * 1e8;
    }
    if (panel.Unit === 's' || panel.Unit === 'seconds' || panel.Unit === 'ms') {
      return r * 30;
    }
    return r * 1000;
  }

  // ── HTML escape — all text must pass through this ──
  function escHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function truncate(s, max) {
    if (!s || s.length <= max) return s;
    return s.substring(0, max) + '\u2026';
  }

  // ── DOM builder helpers (safe, no innerHTML) ──
  function el(tag, cls, children) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    if (typeof children === 'string') node.textContent = children;
    else if (Array.isArray(children)) children.forEach(function(c) { if (c) node.appendChild(c); });
    return node;
  }

  function svgEl(tag, attrs) {
    var node = document.createElementNS('http://www.w3.org/2000/svg', tag);
    if (attrs) Object.keys(attrs).forEach(function(k) { node.setAttribute(k, attrs[k]); });
    return node;
  }

  function styled(node, styles) {
    Object.keys(styles).forEach(function(k) { node.style[k] = styles[k]; });
    return node;
  }

  // ── STAT PANEL RENDERER ──
  function renderStat(container, panel) {
    var val = fakeValue(panel);
    var color = thresholdColor(panel.Thresholds, val);
    var isBg = panel.ColorMode === 'background' || !panel.ColorMode;
    var formatted = formatValue(val, panel.Unit);

    var root = el('div', 'gf-stat' + (isBg ? ' gf-stat-bg' : ''));
    if (isBg) {
      root.style.background = color + '20';
      root.style.borderColor = color + '40';
    }
    root.appendChild(el('div', 'gf-stat-title', panel.Title));
    var valEl = el('div', 'gf-stat-value', formatted);
    valEl.style.color = color;
    root.appendChild(valEl);

    if (panel.GraphMode !== 'none') {
      root.appendChild(buildSparklineSVG(panel, color));
    }

    container.textContent = '';
    container.appendChild(root);
  }

  // ── GAUGE PANEL RENDERER ──
  function renderGauge(container, panel) {
    var val = fakeValue(panel);
    var min = panel.GaugeMin || 0;
    var max = panel.GaugeMax || 100;
    if (panel.Unit === 'percentunit') { min = 0; max = 1; }
    var pct = Math.max(0, Math.min(1, (val - min) / (max - min)));
    var color = thresholdColor(panel.Thresholds, val);
    var formatted = formatValue(val, panel.Unit);

    var r = 38, cx = 50, cy = 58;
    var startAngle = -210 * Math.PI / 180;
    var endAngle = 30 * Math.PI / 180;
    var totalAngle = endAngle - startAngle;
    var valAngle = startAngle + totalAngle * pct;

    function arcPoint(angle) {
      return [cx + r * Math.cos(angle), cy + r * Math.sin(angle)];
    }
    function arcPath(from, to) {
      var p1 = arcPoint(from), p2 = arcPoint(to);
      var large = (to - from) > Math.PI ? 1 : 0;
      return 'M ' + p1[0] + ' ' + p1[1] + ' A ' + r + ' ' + r + ' 0 ' + large + ' 1 ' + p2[0] + ' ' + p2[1];
    }

    var root = el('div', 'gf-gauge');
    root.appendChild(el('div', 'gf-gauge-title', panel.Title));

    var svg = svgEl('svg', { viewBox: '0 0 100 75', class: 'gf-gauge-svg' });

    // Threshold segments
    if (panel.Thresholds && panel.Thresholds.length > 0) {
      for (var i = 0; i < panel.Thresholds.length; i++) {
        var tVal = i === 0 ? min : parseFloat(panel.Thresholds[i].Value);
        var tNext = (i + 1 < panel.Thresholds.length) ? parseFloat(panel.Thresholds[i+1].Value) : max;
        if (isNaN(tVal)) tVal = min;
        if (isNaN(tNext)) tNext = max;
        var tFrom = startAngle + totalAngle * ((tVal - min) / (max - min));
        var tTo = startAngle + totalAngle * ((tNext - min) / (max - min));
        tFrom = Math.max(tFrom, startAngle);
        tTo = Math.min(tTo, endAngle);
        if (tTo > tFrom) {
          svg.appendChild(svgEl('path', { d: arcPath(tFrom, tTo), stroke: resolveColor(panel.Thresholds[i].Color), 'stroke-width': '6', fill: 'none', opacity: '0.3' }));
        }
      }
    } else {
      svg.appendChild(svgEl('path', { d: arcPath(startAngle, endAngle), stroke: '#555', 'stroke-width': '6', fill: 'none', opacity: '0.2' }));
    }
    // Value arc
    if (pct > 0.005) {
      svg.appendChild(svgEl('path', { d: arcPath(startAngle, valAngle), stroke: color, 'stroke-width': '6', fill: 'none', 'stroke-linecap': 'round' }));
    }
    root.appendChild(svg);

    var valEl = el('div', 'gf-gauge-value', formatted);
    valEl.style.color = color;
    root.appendChild(valEl);

    container.textContent = '';
    container.appendChild(root);
  }

  // ── TIMESERIES PANEL RENDERER ──
  function renderTimeseries(container, panel) {
    var queryCount = (panel.Queries && panel.Queries.length) || 1;
    var lines = Math.min(queryCount, 5);
    var isBar = panel.DrawStyle === 'bars';
    var isStacked = panel.StackMode === 'normal';

    var root = el('div', 'gf-timeseries');
    root.appendChild(el('div', 'gf-ts-title', panel.Title));

    var svg = svgEl('svg', { viewBox: '0 0 200 60', preserveAspectRatio: 'none', class: 'gf-ts-svg' });
    for (var l = 0; l < lines; l++) {
      var color = PALETTE[l % PALETTE.length];
      var points = generateWave(panel.ID + l * 37, 25);
      if (isBar) {
        appendBarsSVG(svg, points, color, l, lines, isStacked, panel.FillOpacity || 80);
      } else {
        appendLineSVG(svg, points, color, panel.FillOpacity || 10);
      }
    }
    root.appendChild(svg);

    // Legend
    if (panel.Queries && panel.Queries.length > 0) {
      var legend = el('div', 'gf-ts-legend');
      for (var q = 0; q < Math.min(panel.Queries.length, 5); q++) {
        var c = PALETTE[q % PALETTE.length];
        var label = panel.Queries[q].Legend || panel.Queries[q].RefID || String.fromCharCode(65 + q);
        if (label.indexOf('{{') >= 0) label = label.replace(/\{\{(\w+)\}\}/g, '$1');
        var item = el('span', 'gf-ts-legend-item');
        var dot = el('span', 'gf-legend-dot');
        dot.style.background = c;
        item.appendChild(dot);
        item.appendChild(document.createTextNode(truncate(label, 20)));
        legend.appendChild(item);
      }
      root.appendChild(legend);
    }

    container.textContent = '';
    container.appendChild(root);
  }

  // ── BARGAUGE PANEL RENDERER ──
  function renderBargauge(container, panel) {
    var count = Math.max((panel.Queries && panel.Queries.length) || 3, 3);
    count = Math.min(count, 6);
    var max = panel.GaugeMax || 100;

    var root = el('div', 'gf-bargauge');
    root.appendChild(el('div', 'gf-bg-title', panel.Title));
    var bars = el('div', 'gf-bg-bars');
    for (var i = 0; i < count; i++) {
      var seed = panel.ID + i * 53;
      var val = ((seed * 9301 + 49297) % 233280) / 233280 * max;
      var pct = Math.min(100, (val / max) * 100);
      var color = thresholdColor(panel.Thresholds, val);
      var row = el('div', 'gf-bg-row');
      row.appendChild(el('span', 'gf-bg-label', 'series ' + String.fromCharCode(65 + i)));
      var track = el('div', 'gf-bg-track');
      var fill = el('div', 'gf-bg-fill');
      fill.style.width = pct.toFixed(1) + '%';
      fill.style.background = color;
      track.appendChild(fill);
      row.appendChild(track);
      row.appendChild(el('span', 'gf-bg-val', formatValue(val, panel.Unit)));
      bars.appendChild(row);
    }
    root.appendChild(bars);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── TABLE PANEL RENDERER ──
  function renderTable(container, panel) {
    var root = el('div', 'gf-table');
    root.appendChild(el('div', 'gf-table-title', panel.Title));
    var table = document.createElement('table');
    table.className = 'gf-table-grid';
    var thead = document.createElement('thead');
    var headRow = document.createElement('tr');
    ['instance', 'job', 'value'].forEach(function(c) {
      var th = document.createElement('th');
      th.textContent = c;
      headRow.appendChild(th);
    });
    thead.appendChild(headRow);
    table.appendChild(thead);

    var tbody = document.createElement('tbody');
    for (var i = 0; i < 4; i++) {
      var tr = document.createElement('tr');
      var td1 = document.createElement('td');
      td1.className = 'gf-table-mono';
      td1.textContent = 'instance-' + (i+1) + ':9090';
      tr.appendChild(td1);
      var td2 = document.createElement('td');
      td2.textContent = 'node-exporter';
      tr.appendChild(td2);
      var td3 = document.createElement('td');
      var v = ((panel.ID + i * 71) * 9301 + 49297) % 233280 / 233280 * 100;
      td3.textContent = formatValue(v, panel.Unit);
      tr.appendChild(td3);
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    root.appendChild(table);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── HEATMAP PANEL RENDERER ──
  function renderHeatmap(container, panel) {
    var root = el('div', 'gf-heatmap');
    root.appendChild(el('div', 'gf-hm-title', panel.Title));
    var grid = el('div', 'gf-hm-grid');
    var rows = 6, cols = 16;
    for (var y = 0; y < rows; y++) {
      for (var x = 0; x < cols; x++) {
        var seed = (panel.ID + y * 17 + x * 31);
        var intensity = ((seed * 9301 + 49297) % 233280) / 233280;
        var cell = el('div', 'gf-hm-cell');
        cell.style.opacity = (intensity * 0.8 + 0.05).toFixed(2);
        grid.appendChild(cell);
      }
    }
    root.appendChild(grid);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── PIECHART PANEL RENDERER ──
  function renderPiechart(container, panel) {
    var slices = Math.min((panel.Queries && panel.Queries.length) || 4, 6);
    if (slices < 2) slices = 4;
    var total = 0;
    var vals = [];
    for (var i = 0; i < slices; i++) {
      var v = ((panel.ID + i * 67) * 9301 + 49297) % 233280 / 233280 * 100 + 10;
      vals.push(v);
      total += v;
    }

    var isDonut = panel.PieType === 'donut' || !panel.PieType;
    var cx = 50, cy = 50, r = 38, ir = isDonut ? 22 : 0;

    var root = el('div', 'gf-piechart');
    root.appendChild(el('div', 'gf-pie-title', panel.Title));
    var svg = svgEl('svg', { viewBox: '0 0 100 100', class: 'gf-pie-svg' });

    var angle = -Math.PI / 2;
    for (var i = 0; i < vals.length; i++) {
      var sweep = (vals[i] / total) * 2 * Math.PI;
      var x1 = cx + r * Math.cos(angle);
      var y1 = cy + r * Math.sin(angle);
      var x2 = cx + r * Math.cos(angle + sweep);
      var y2 = cy + r * Math.sin(angle + sweep);
      var large = sweep > Math.PI ? 1 : 0;
      var path = 'M ' + cx + ' ' + cy + ' L ' + x1 + ' ' + y1 + ' A ' + r + ' ' + r + ' 0 ' + large + ' 1 ' + x2 + ' ' + y2 + ' Z';
      svg.appendChild(svgEl('path', { d: path, fill: PALETTE[i % PALETTE.length], stroke: '#1e1e2e', 'stroke-width': '1' }));
      angle += sweep;
    }
    if (isDonut) {
      svg.appendChild(svgEl('circle', { cx: cx, cy: cy, r: ir, fill: '#1e1e2e' }));
    }
    root.appendChild(svg);

    var legend = el('div', 'gf-pie-legend');
    for (var i = 0; i < vals.length; i++) {
      var label = (panel.Queries && panel.Queries[i]) ? (panel.Queries[i].Legend || panel.Queries[i].RefID || String.fromCharCode(65 + i)) : String.fromCharCode(65 + i);
      if (label.indexOf('{{') >= 0) label = label.replace(/\{\{(\w+)\}\}/g, '$1');
      var item = el('span', 'gf-pie-legend-item');
      var dot = el('span', 'gf-legend-dot');
      dot.style.background = PALETTE[i % PALETTE.length];
      item.appendChild(dot);
      item.appendChild(document.createTextNode(truncate(label, 15)));
      legend.appendChild(item);
    }
    root.appendChild(legend);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── TEXT PANEL RENDERER ──
  function renderText(container, panel) {
    var root = el('div', 'gf-text');
    root.appendChild(el('div', 'gf-text-content', panel.TextContent || panel.Description || panel.Title));
    container.textContent = '';
    container.appendChild(root);
  }

  // ── STATE-TIMELINE / STATUS-HISTORY RENDERER ──
  function renderStateTimeline(container, panel) {
    var rows = Math.min((panel.Queries && panel.Queries.length) || 3, 5);
    if (rows < 2) rows = 3;
    var cols = 20;

    var root = el('div', 'gf-state-timeline');
    root.appendChild(el('div', 'gf-st-title', panel.Title));
    var grid = el('div', 'gf-st-grid');
    for (var y = 0; y < rows; y++) {
      var row = el('div', 'gf-st-row');
      row.appendChild(el('span', 'gf-st-label', 'series ' + String.fromCharCode(65 + y)));
      var cells = el('div', 'gf-st-cells');
      for (var x = 0; x < cols; x++) {
        var seed = (panel.ID + y * 13 + x * 29);
        var state = ((seed * 9301 + 49297) % 233280) / 233280;
        var color = state > 0.8 ? '#F2495C' : state > 0.3 ? '#FF9830' : '#73BF69';
        var cell = el('div', 'gf-st-cell');
        cell.style.background = color;
        cells.appendChild(cell);
      }
      row.appendChild(cells);
      grid.appendChild(row);
    }
    root.appendChild(grid);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── HISTOGRAM RENDERER ──
  function renderHistogram(container, panel) {
    var bins = 12;
    var vals = [];
    var maxV = 0;
    for (var i = 0; i < bins; i++) {
      var seed = (panel.ID + i * 43);
      var x = (i - bins/2) / (bins/4);
      var v = Math.exp(-x*x/2) * 100 + ((seed * 9301 + 49297) % 233280) / 233280 * 20;
      vals.push(v);
      if (v > maxV) maxV = v;
    }

    var root = el('div', 'gf-histogram');
    root.appendChild(el('div', 'gf-hist-title', panel.Title));
    var barsEl = el('div', 'gf-hist-bars');
    for (var i = 0; i < bins; i++) {
      var bar = el('div', 'gf-hist-bar');
      bar.style.height = (vals[i] / maxV * 100).toFixed(1) + '%';
      bar.style.background = PALETTE[0];
      barsEl.appendChild(bar);
    }
    root.appendChild(barsEl);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── LOGS RENDERER ──
  function renderLogs(container, panel) {
    var root = el('div', 'gf-logs');
    root.appendChild(el('div', 'gf-logs-title', panel.Title));
    var logLines = [
      { ts: '12:34:56', level: 'info', msg: 'request completed status=200 duration=12ms' },
      { ts: '12:34:55', level: 'warn', msg: 'slow query detected duration=850ms' },
      { ts: '12:34:54', level: 'info', msg: 'connection established peer=10.0.1.5' },
      { ts: '12:34:53', level: 'error', msg: 'timeout waiting for response upstream=api-gateway' },
      { ts: '12:34:52', level: 'info', msg: 'health check passed checks=3/3' }
    ];
    var linesEl = el('div', 'gf-logs-lines');
    logLines.forEach(function(l) {
      var line = el('div', 'gf-log-line');
      line.appendChild(el('span', 'gf-log-ts', l.ts));
      line.appendChild(el('span', 'gf-log-level gf-log-' + l.level, l.level));
      line.appendChild(el('span', 'gf-log-msg', l.msg));
      linesEl.appendChild(line);
    });
    root.appendChild(linesEl);
    container.textContent = '';
    container.appendChild(root);
  }

  // ── HELPER: Generate pseudo-random wave data ──
  function generateWave(seed, points) {
    var data = [];
    var phase = ((seed * 7) % 100) / 100 * Math.PI * 2;
    var freq = 0.3 + ((seed * 13) % 50) / 100;
    var amp = 0.3 + ((seed * 31) % 40) / 100;
    var base = 0.5;
    for (var i = 0; i < points; i++) {
      var t = i / (points - 1);
      var v = base + amp * Math.sin(t * Math.PI * 2 * freq + phase) +
              amp * 0.3 * Math.sin(t * Math.PI * 4 * freq + phase * 2);
      v += ((seed + i * 97) * 9301 + 49297) % 233280 / 233280 * 0.1 - 0.05;
      data.push(Math.max(0.02, Math.min(0.98, v)));
    }
    return data;
  }

  // ── HELPER: Build sparkline SVG element for stat panels ──
  function buildSparklineSVG(panel, color) {
    var points = generateWave(panel.ID, 20);
    var w = 200, h = 30;
    var pathD = 'M 0 ' + ((1 - points[0]) * h).toFixed(1);
    for (var i = 1; i < points.length; i++) {
      var x = (i / (points.length - 1)) * w;
      var y = (1 - points[i]) * h;
      pathD += ' L ' + x.toFixed(1) + ' ' + y.toFixed(1);
    }
    var areaD = pathD + ' L ' + w + ' ' + h + ' L 0 ' + h + ' Z';

    var svg = svgEl('svg', { viewBox: '0 0 ' + w + ' ' + h, preserveAspectRatio: 'none', class: 'gf-stat-sparkline' });
    svg.appendChild(svgEl('path', { d: areaD, fill: color, opacity: '0.15' }));
    svg.appendChild(svgEl('path', { d: pathD, stroke: color, 'stroke-width': '1.5', fill: 'none' }));
    return svg;
  }

  // ── HELPER: Append line SVG paths to a timeseries SVG ──
  function appendLineSVG(svg, points, color, fillOpacity) {
    var w = 200, h = 60;
    var pathD = 'M 0 ' + ((1 - points[0]) * h).toFixed(1);
    for (var i = 1; i < points.length; i++) {
      var x = (i / (points.length - 1)) * w;
      var y = (1 - points[i]) * h;
      var prevX = ((i - 1) / (points.length - 1)) * w;
      var prevY = (1 - points[i - 1]) * h;
      var cpx = (prevX + x) / 2;
      pathD += ' C ' + cpx.toFixed(1) + ' ' + prevY.toFixed(1) + ' ' + cpx.toFixed(1) + ' ' + y.toFixed(1) + ' ' + x.toFixed(1) + ' ' + y.toFixed(1);
    }
    var opacity = Math.max(5, Math.min(100, fillOpacity || 10)) / 100;
    var areaD = pathD + ' L ' + w + ' ' + h + ' L 0 ' + h + ' Z';
    svg.appendChild(svgEl('path', { d: areaD, fill: color, opacity: opacity.toFixed(2) }));
    svg.appendChild(svgEl('path', { d: pathD, stroke: color, 'stroke-width': '1.5', fill: 'none' }));
  }

  // ── HELPER: Append bar SVG rects to a timeseries SVG ──
  function appendBarsSVG(svg, points, color, index, total, stacked, fillOpacity) {
    var w = 200, h = 60;
    var barW = w / points.length * 0.7 / (stacked ? 1 : total);
    for (var i = 0; i < points.length; i++) {
      var barH = points[i] * h * 0.9;
      var x = (i / points.length) * w + (stacked ? 0 : index * barW);
      var y = h - barH;
      svg.appendChild(svgEl('rect', { x: x.toFixed(1), y: y.toFixed(1), width: barW.toFixed(1), height: barH.toFixed(1), fill: color, opacity: ((fillOpacity || 80) / 100).toFixed(2) }));
    }
  }

  // ── MAIN: Render all panels ──
  var renderers = {
    stat: renderStat,
    gauge: renderGauge,
    timeseries: renderTimeseries,
    bargauge: renderBargauge,
    table: renderTable,
    heatmap: renderHeatmap,
    histogram: renderHistogram,
    piechart: renderPiechart,
    'state-timeline': renderStateTimeline,
    'status-history': renderStateTimeline,
    text: renderText,
    logs: renderLogs
  };

  window.renderGrafanaPanels = function() {
    var panelData = window._panelData || [];
    var dataMap = {};
    panelData.forEach(function(p) { dataMap[p.ID] = p; });

    document.querySelectorAll('.preview-panel[data-panel-id]').forEach(function(panelEl) {
      var id = parseInt(panelEl.dataset.panelId);
      var panel = dataMap[id];
      if (!panel) return;

      var renderer = renderers[panel.Type];
      if (renderer) {
        panelEl.classList.add('gf-rendered');
        renderer(panelEl, panel);
      }
    });
  };
})();
