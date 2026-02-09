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
    btn.classList.toggle('active', btn.dataset.tab === tab);
  });
  document.querySelectorAll('.preview-tab-content').forEach(function(el) {
    el.classList.toggle('active', el.dataset.tab === tab);
  });
}

// Show a toast notification
function showToast(message, type) {
  var toast = document.createElement('div');
  toast.className = 'toast ' + (type || 'success');
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(function() {
    toast.classList.add('dismissing');
    setTimeout(function() { toast.remove(); }, 300);
  }, 4000);
}

// Tab key support for textareas
document.addEventListener('keydown', function(e) {
  if (e.key === 'Tab' && e.target.tagName === 'TEXTAREA') {
    e.preventDefault();
    var start = e.target.selectionStart;
    var end = e.target.selectionEnd;
    e.target.value = e.target.value.substring(0, start) + '  ' + e.target.value.substring(end);
    e.target.selectionStart = e.target.selectionEnd = start + 2;
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
      dsB.style.display = 'block';
      jobTabs.style.display = 'none';
      form.setAttribute('hx-get', '/api/metrics/compare');
      htmx.process(form);
    } else {
      dsB.style.display = 'none';
      jobTabs.style.display = '';
      form.setAttribute('hx-get', '/api/metrics/browse');
      htmx.process(form);
    }
  });
});

// Set active job tab
function setActiveJobTab(btn) {
  btn.parentElement.querySelectorAll('button').forEach(function(b) {
    b.classList.remove('active');
  });
  btn.classList.add('active');
}

// Switch between comparison tabs (shared/only-a/only-b)
function showCompareTab(tabName, btn) {
  document.querySelectorAll('.compare-tab-content').forEach(function(el) {
    el.style.display = 'none';
  });
  document.getElementById('compare-tab-' + tabName).style.display = 'block';
  btn.parentElement.querySelectorAll('button').forEach(function(b) {
    b.classList.remove('active');
  });
  btn.classList.add('active');
}

// Auto-dismiss toasts rendered by server (HTMX responses)
document.addEventListener('htmx:afterSwap', function() {
  document.querySelectorAll('.toast:not([data-auto])').forEach(function(toast) {
    toast.setAttribute('data-auto', '1');
    setTimeout(function() {
      toast.classList.add('dismissing');
      setTimeout(function() { toast.remove(); }, 300);
    }, 4000);
  });
});
