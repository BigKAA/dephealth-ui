import { t } from './i18n.js';

const MAX_TOASTS = 5;
const DEFAULT_DURATION = 5000;

let container = null;

function getContainer() {
  if (!container) {
    container = document.getElementById('toast-container');
  }
  return container;
}

/**
 * Show a toast notification.
 * @param {string} message
 * @param {'error'|'warning'|'info'|'success'} [type='info']
 * @param {number} [duration=5000] - auto-dismiss in ms, 0 to disable
 */
export function showToast(message, type = 'info', duration = DEFAULT_DURATION) {
  const c = getContainer();
  if (!c) return;

  // Enforce max visible toasts
  while (c.children.length >= MAX_TOASTS) {
    c.removeChild(c.firstChild);
  }

  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.setAttribute('role', 'alert');

  const text = document.createElement('span');
  text.className = 'toast-message';
  text.textContent = message;

  const close = document.createElement('button');
  close.className = 'toast-close';
  close.textContent = '\u00d7';
  close.setAttribute('aria-label', t('toast.close'));
  close.addEventListener('click', () => dismissToast(toast));

  toast.appendChild(text);
  toast.appendChild(close);
  c.appendChild(toast);

  // Trigger slide-in animation
  requestAnimationFrame(() => {
    toast.classList.add('toast-visible');
  });

  if (duration > 0) {
    setTimeout(() => dismissToast(toast), duration);
  }
}

function dismissToast(toast) {
  if (!toast.parentNode) return;
  toast.classList.add('toast-exit');
  toast.addEventListener('animationend', () => {
    if (toast.parentNode) {
      toast.parentNode.removeChild(toast);
    }
  });
}
