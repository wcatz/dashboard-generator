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
