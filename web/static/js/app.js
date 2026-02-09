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

// Update swatch color from color picker and copy new hex
function updateSwatch(input, colorName) {
  var hex = input.value;
  var swatch = input.previousElementSibling;
  swatch.style.background = hex;
  var hexLabel = input.parentElement.nextElementSibling.nextElementSibling;
  if (hexLabel) hexLabel.textContent = hex;
  navigator.clipboard.writeText(hex).then(function() {
    showToast(colorName + ': ' + hex + ' copied', 'success');
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

// Auto-dismiss toasts rendered by server (HTMX responses)
document.addEventListener('htmx:afterSwap', function() {
  document.querySelectorAll('.toast-msg:not([data-auto])').forEach(function(toast) {
    toast.setAttribute('data-auto', '1');
    setTimeout(function() {
      toast.classList.add('dismissing');
      setTimeout(function() { toast.remove(); }, 300);
    }, 4000);
  });
});
