// Copy text content of an element to clipboard
function copyToClipboard(elementId) {
  var el = document.getElementById(elementId);
  if (!el) return;
  navigator.clipboard.writeText(el.textContent).then(function() {
    showToast('Copied to clipboard', 'success');
  }).catch(function() {
    showToast('Failed to copy to clipboard', 'error');
  });
}

// Copy a hex color string to clipboard (palettes page)
function copyHex(hex) {
  navigator.clipboard.writeText(hex).then(function() {
    showToast('Copied ' + hex, 'success');
  }).catch(function() {
    showToast('Failed to copy', 'error');
  });
}

// ── Palette CRUD (palettes page) ──

function paletteSetColor(palette, color, hex) {
  htmx.ajax('POST', '/api/palette/color/set', {
    target: '#palette-cards',
    swap: 'innerHTML',
    values: { palette: palette, color: color, hex: hex }
  });
  showToast(color + ': ' + hex, 'success');
}

function paletteAddColor(palette) {
  var name = prompt('color name:');
  if (!name || !name.trim()) return;
  htmx.ajax('POST', '/api/palette/color/set', {
    target: '#palette-cards',
    swap: 'innerHTML',
    values: { palette: palette, color: name.trim(), hex: '#6366f1' }
  });
}

function paletteRenameColor(palette, oldName) {
  var newName = prompt('rename "' + oldName + '" to:', oldName);
  if (!newName || !newName.trim() || newName.trim() === oldName) return;
  htmx.ajax('POST', '/api/palette/color/rename', {
    target: '#palette-cards',
    swap: 'innerHTML',
    values: { palette: palette, color: oldName, new_name: newName.trim() }
  });
}

// Download text content of an element as a JSON file
function downloadJSON(filename, elementId) {
  var el = document.getElementById(elementId);
  if (!el) return;
  var blob = new Blob([el.textContent], { type: 'application/json' });
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = filename || 'dashboard.json';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

// Toggle between visual and JSON preview tabs
function togglePreviewTab(tab) {
  document.querySelectorAll('.preview-tab-btn').forEach(function(btn) {
    btn.classList.toggle('tab-active', btn.dataset.tab === tab);
  });
  document.querySelectorAll('.preview-tab-content').forEach(function(el) {
    el.style.display = (el.dataset.tab === tab) ? 'block' : 'none';
  });
}

// Show a toast notification (uses toast-msg class to avoid DaisyUI .toast conflict)
function showToast(message, type) {
  var toast = document.createElement('div');
  toast.className = 'toast-msg ' + (type || 'success');
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(function() {
    toast.classList.add('dismissing');
    setTimeout(function() { toast.remove(); }, 300);
  }, 4000);
}

// Tab key support for textareas + Ctrl+S save shortcut
document.addEventListener('keydown', function(e) {
  if (e.key === 'Tab' && e.target.tagName === 'TEXTAREA') {
    e.preventDefault();
    var start = e.target.selectionStart;
    var end = e.target.selectionEnd;
    e.target.value = e.target.value.substring(0, start) + '  ' + e.target.value.substring(end);
    e.target.selectionStart = e.target.selectionEnd = start + 2;
  }
  // Ctrl+S / Cmd+S — trigger save on editor page
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    var saveBtn = document.getElementById('save-btn');
    if (saveBtn) {
      e.preventDefault();
      saveBtn.click();
    }
  }
});

// Comparison mode toggle for metrics page
document.addEventListener('DOMContentLoaded', function() {
  var toggle = document.getElementById('compare-toggle');
  if (!toggle) return;
  toggle.addEventListener('change', function() {
    var form = document.getElementById('browse-form');
    var dsB = document.getElementById('ds-b-container');
    var jobTabs = document.getElementById('job-tabs');
    if (this.checked) {
      dsB.classList.remove('hidden');
      jobTabs.classList.add('hidden');
      form.setAttribute('hx-get', '/api/metrics/compare');
      htmx.process(form);
    } else {
      dsB.classList.add('hidden');
      jobTabs.classList.remove('hidden');
      form.setAttribute('hx-get', '/api/metrics/browse');
      htmx.process(form);
    }
  });
});

// Set active job tab
function setActiveJobTab(btn) {
  btn.parentElement.querySelectorAll('button').forEach(function(b) {
    b.classList.remove('tab-active');
  });
  btn.classList.add('tab-active');
}

// Switch between comparison tabs (shared/only-a/only-b)
function showCompareTab(tabName, btn) {
  document.querySelectorAll('.compare-tab-content').forEach(function(el) {
    el.style.display = 'none';
  });
  document.getElementById('compare-tab-' + tabName).style.display = 'block';
  btn.parentElement.querySelectorAll('button').forEach(function(b) {
    b.classList.remove('tab-active');
  });
  btn.classList.add('tab-active');
}

// Switch between compare-all tabs (shared/exclusive per DS)
function showCompareAllTab(tabName, btn) {
  document.querySelectorAll('.compare-all-tab-content').forEach(function(el) {
    el.style.display = 'none';
  });
  document.getElementById('compare-all-tab-' + tabName).style.display = 'block';
  btn.parentElement.querySelectorAll('button').forEach(function(b) {
    b.classList.remove('tab-active');
  });
  btn.classList.add('tab-active');
}

// ── Panel detail drawer (preview page, client-side rendering) ──
// Panel data is embedded as JSON in the page by the server (window._panelData).
// All values are HTML-escaped via escHtml() before DOM insertion.

function openPanelDetail(panelId) {
  var panels = window._panelData || [];
  var panel = null;
  for (var i = 0; i < panels.length; i++) {
    if (panels[i].ID === panelId) { panel = panels[i]; break; }
  }
  if (!panel) return;

  // Highlight the selected panel in the grid
  document.querySelectorAll('.preview-panel.selected').forEach(function(p) {
    p.classList.remove('selected');
  });
  var el = document.querySelector('.preview-panel[data-panel-id="' + panelId + '"]');
  if (el) el.classList.add('selected');

  // Render detail using safe DOM construction
  var container = document.getElementById('panel-detail-content');
  container.textContent = ''; // clear previous content
  var root = document.createElement('div');
  root.className = 'p-5';
  root.appendChild(buildDetailHeader(panel));
  root.appendChild(buildDetailBody(panel));
  container.appendChild(root);
  document.getElementById('panel-detail-drawer').style.display = 'block';
}

function buildDetailHeader(panel) {
  var header = document.createElement('div');
  header.className = 'flex justify-between items-start mb-4';
  var left = document.createElement('div');
  var badge = document.createElement('span');
  badge.className = 'type-badge-' + panel.Type;
  badge.textContent = panel.Type;
  left.appendChild(badge);
  var title = document.createElement('h3');
  title.className = 'text-base font-semibold mt-1';
  title.textContent = panel.Title;
  left.appendChild(title);
  if (panel.Description) {
    var desc = document.createElement('p');
    desc.className = 'text-xs text-base-content/50 mt-1';
    desc.textContent = panel.Description;
    left.appendChild(desc);
  }
  header.appendChild(left);
  var closeBtn = document.createElement('button');
  closeBtn.className = 'btn btn-ghost btn-xs';
  closeBtn.setAttribute('onclick', 'closePanelDetail()');
  closeBtn.textContent = '\u2715';
  header.appendChild(closeBtn);
  return header;
}

function buildDetailBody(panel) {
  var body = document.createElement('div');
  body.className = 'space-y-4';

  // Layout section
  body.appendChild(buildDetailSection('layout', function(content) {
    var badges = document.createElement('div');
    badges.className = 'flex flex-wrap gap-2 text-xs';
    addBadge(badges, panel.W + ' x ' + panel.H, 'badge-outline');
    addBadge(badges, 'pos (' + panel.X + ', ' + panel.Y + ')', 'badge-ghost');
    if (panel.Section) addBadge(badges, panel.Section, 'badge-ghost');
    content.appendChild(badges);
  }));

  // Datasource
  if (panel.Datasource) {
    body.appendChild(buildDetailSection('datasource', function(content) {
      addBadge(content, panel.Datasource, 'badge-info');
    }));
  }

  // Unit
  if (panel.Unit) {
    body.appendChild(buildDetailSection('unit', function(content) {
      var span = document.createElement('span');
      span.className = 'text-xs font-mono';
      span.textContent = panel.Unit;
      content.appendChild(span);
    }));
  }

  // Queries
  if (panel.Queries && panel.Queries.length > 0) {
    body.appendChild(buildDetailSection('queries', function(content) {
      var list = document.createElement('div');
      list.className = 'space-y-2';
      panel.Queries.forEach(function(q) {
        var card = document.createElement('div');
        card.className = 'bg-base-200 border border-base-content/10 rounded-md p-3';
        var meta = document.createElement('div');
        meta.className = 'flex items-center gap-2 mb-1';
        addBadge(meta, q.RefID, 'badge-primary badge-xs');
        if (q.Legend) {
          var leg = document.createElement('span');
          leg.className = 'text-[0.65rem] text-base-content/40';
          leg.textContent = 'legend: ' + q.Legend;
          meta.appendChild(leg);
        }
        if (q.Datasource) addBadge(meta, q.Datasource, 'badge-ghost badge-xs');
        card.appendChild(meta);
        var pre = document.createElement('pre');
        pre.className = 'text-xs font-mono text-base-content/80 whitespace-pre-wrap break-all leading-relaxed';
        pre.textContent = q.Expr;
        card.appendChild(pre);
        list.appendChild(card);
      });
      content.appendChild(list);
    }));
  }

  // Thresholds
  if (panel.Thresholds && panel.Thresholds.length > 0) {
    body.appendChild(buildDetailSection('thresholds', function(content) {
      var bar = document.createElement('div');
      bar.className = 'flex gap-0.5 h-5 rounded overflow-hidden mb-1';
      panel.Thresholds.forEach(function(s) {
        var seg = document.createElement('div');
        seg.className = 'flex-1';
        seg.style.background = s.Color;
        seg.title = s.Color + ' @ ' + s.Value;
        bar.appendChild(seg);
      });
      content.appendChild(bar);
      var labels = document.createElement('div');
      labels.className = 'flex flex-wrap gap-2';
      panel.Thresholds.forEach(function(s) {
        var span = document.createElement('span');
        span.className = 'text-[0.6rem] font-mono text-base-content/50';
        var dot = document.createElement('span');
        dot.className = 'inline-block w-2 h-2 rounded-full mr-0.5';
        dot.style.background = s.Color;
        span.appendChild(dot);
        span.appendChild(document.createTextNode(s.Value));
        labels.appendChild(span);
      });
      content.appendChild(labels);
    }));
  }

  return body;
}

function buildDetailSection(title, buildContent) {
  var section = document.createElement('div');
  var label = document.createElement('div');
  label.className = 'text-[0.65rem] uppercase tracking-wider text-base-content/50 font-semibold mb-1';
  label.textContent = title;
  section.appendChild(label);
  buildContent(section);
  return section;
}

function addBadge(parent, text, extraClass) {
  var badge = document.createElement('span');
  badge.className = 'badge badge-sm ' + (extraClass || '');
  badge.textContent = text;
  parent.appendChild(badge);
}

function closePanelDetail() {
  var drawer = document.getElementById('panel-detail-drawer');
  if (drawer) drawer.style.display = 'none';
  document.querySelectorAll('.preview-panel.selected').forEach(function(p) {
    p.classList.remove('selected');
  });
}

// Close panel detail on Escape key
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') closePanelDetail();
});

// ── Preview zoom controls ──

function setPreviewZoom(value) {
  var grid = document.getElementById('preview-grid');
  if (grid) grid.style.setProperty('--preview-scale', value);
  var label = document.getElementById('zoom-label');
  if (label) label.textContent = parseFloat(value).toFixed(1) + 'x';
  var slider = document.getElementById('zoom-slider');
  if (slider) slider.value = value;
  localStorage.setItem('preview-zoom', value);
}

// ── Row collapse/expand ──

function toggleRowCollapse(rowEl) {
  var sectionY = rowEl.dataset.sectionY;
  var isCollapsed = rowEl.classList.toggle('collapsed');
  document.querySelectorAll('.preview-panel[data-section-y="' + sectionY + '"]').forEach(function(el) {
    el.style.display = isCollapsed ? 'none' : '';
  });
}

// ── Preview search and filter (combined to avoid conflicts) ──

var _activeTypeFilters = {};

function toggleTypeFilter(type, btn) {
  if (_activeTypeFilters[type]) {
    delete _activeTypeFilters[type];
    btn.classList.remove('active');
  } else {
    _activeTypeFilters[type] = true;
    btn.classList.add('active');
  }

  var hasFilters = Object.keys(_activeTypeFilters).length > 0;
  document.querySelectorAll('.type-filter-btn').forEach(function(b) {
    if (hasFilters && !_activeTypeFilters[b.dataset.type]) {
      b.classList.add('dimmed');
    } else {
      b.classList.remove('dimmed');
    }
  });

  applyPreviewFilters();
}

// Single filter function that evaluates both search query and type filters together
function applyPreviewFilters() {
  var searchEl = document.getElementById('panel-search');
  var q = searchEl ? searchEl.value.toLowerCase().trim() : '';
  var hasTypeFilters = Object.keys(_activeTypeFilters).length > 0;

  document.querySelectorAll('.preview-panel[data-panel-type]').forEach(function(el) {
    var matchesType = !hasTypeFilters || _activeTypeFilters[el.dataset.panelType];
    var matchesSearch = true;
    if (q !== '') {
      var title = (el.dataset.panelTitle || '').toLowerCase();
      var hint = el.querySelector('.panel-query-hint');
      var queryText = hint ? hint.textContent.toLowerCase() : '';
      matchesSearch = title.indexOf(q) !== -1 || queryText.indexOf(q) !== -1;
    }
    if (matchesType && matchesSearch) {
      el.classList.remove('panel-hidden');
    } else {
      el.classList.add('panel-hidden');
    }
  });
}

// Syntax highlighting — highlight code blocks with hljs-auto class
function highlightCodeBlocks(root) {
  (root || document).querySelectorAll('code.hljs-auto:not(.hljs)').forEach(function(block) {
    hljs.highlightElement(block);
  });
}

// Run on page load
document.addEventListener('DOMContentLoaded', function() { highlightCodeBlocks(); });

// Auto-dismiss toasts and highlight code blocks after HTMX swaps
document.addEventListener('htmx:afterSwap', function(evt) {
  document.querySelectorAll('.toast-msg:not([data-auto])').forEach(function(toast) {
    toast.setAttribute('data-auto', '1');
    setTimeout(function() {
      toast.classList.add('dismissing');
      setTimeout(function() { toast.remove(); }, 300);
    }, 4000);
  });
  highlightCodeBlocks(evt.detail.target);
});
