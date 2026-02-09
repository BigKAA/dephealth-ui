const STORAGE_KEY = 'dephealth-toolbar-pos';

/**
 * Initialize drag & drop for the floating graph toolbar.
 * Saves position to localStorage and restores it on init.
 */
export function initToolbar() {
  const toolbar = document.getElementById('graph-toolbar');
  if (!toolbar) return;

  restorePosition(toolbar);

  let isDragging = false;
  let offsetX = 0;
  let offsetY = 0;

  function onPointerDown(e) {
    // Only drag from toolbar background, not from buttons
    if (e.target.closest('button')) return;
    isDragging = true;
    const rect = toolbar.getBoundingClientRect();
    offsetX = e.clientX - rect.left;
    offsetY = e.clientY - rect.top;
    toolbar.classList.add('dragging');
    toolbar.setPointerCapture(e.pointerId);
    e.preventDefault();
  }

  function onPointerMove(e) {
    if (!isDragging) return;
    const parent = toolbar.parentElement;
    const parentRect = parent.getBoundingClientRect();

    let x = e.clientX - parentRect.left - offsetX;
    let y = e.clientY - parentRect.top - offsetY;

    // Constrain within parent bounds
    x = Math.max(0, Math.min(x, parentRect.width - toolbar.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - toolbar.offsetHeight));

    toolbar.style.left = x + 'px';
    toolbar.style.top = y + 'px';
    toolbar.style.right = 'auto';
  }

  function onPointerUp(e) {
    if (!isDragging) return;
    isDragging = false;
    toolbar.classList.remove('dragging');
    toolbar.releasePointerCapture(e.pointerId);
    savePosition(toolbar);
  }

  toolbar.addEventListener('pointerdown', onPointerDown);
  toolbar.addEventListener('pointermove', onPointerMove);
  toolbar.addEventListener('pointerup', onPointerUp);
}

function savePosition(toolbar) {
  const parent = toolbar.parentElement;
  if (!parent) return;
  const parentRect = parent.getBoundingClientRect();
  const rect = toolbar.getBoundingClientRect();
  // Store as percentage of parent size for responsive adaptation
  const pos = {
    xPct: (rect.left - parentRect.left) / parentRect.width,
    yPct: (rect.top - parentRect.top) / parentRect.height,
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(pos));
}

function restorePosition(toolbar) {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return;
  try {
    const pos = JSON.parse(raw);
    const parent = toolbar.parentElement;
    if (!parent) return;
    const parentRect = parent.getBoundingClientRect();

    let x = pos.xPct * parentRect.width;
    let y = pos.yPct * parentRect.height;

    // Constrain within bounds
    x = Math.max(0, Math.min(x, parentRect.width - toolbar.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - toolbar.offsetHeight));

    toolbar.style.left = x + 'px';
    toolbar.style.top = y + 'px';
    toolbar.style.right = 'auto';
  } catch {
    // Ignore invalid stored position
  }
}
