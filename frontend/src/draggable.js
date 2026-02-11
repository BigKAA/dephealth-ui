/**
 * Make an element draggable within its parent container.
 * Saves/restores position as percentage of parent size via localStorage.
 *
 * @param {HTMLElement} element - The element to make draggable
 * @param {string} storageKey - localStorage key for persisting position
 * @param {object} [options]
 * @param {string} [options.dragHandle] - CSS selector for drag handle (defaults to whole element)
 * @param {string} [options.exclude] - CSS selector to exclude from triggering drag (e.g. 'button')
 */
export function makeDraggable(element, storageKey, options = {}) {
  const { dragHandle, exclude } = options;
  const handle = dragHandle ? element.querySelector(dragHandle) : element;
  if (!handle) return;

  restorePosition(element, storageKey);

  let isDragging = false;
  let offsetX = 0;
  let offsetY = 0;

  function onPointerDown(e) {
    if (exclude && e.target.closest(exclude)) return;
    isDragging = true;
    const rect = element.getBoundingClientRect();
    offsetX = e.clientX - rect.left;
    offsetY = e.clientY - rect.top;
    element.classList.add('dragging');
    handle.setPointerCapture(e.pointerId);
    e.preventDefault();
  }

  function onPointerMove(e) {
    if (!isDragging) return;
    const parent = element.parentElement;
    const parentRect = parent.getBoundingClientRect();

    let x = e.clientX - parentRect.left - offsetX;
    let y = e.clientY - parentRect.top - offsetY;

    // Constrain within parent bounds
    x = Math.max(0, Math.min(x, parentRect.width - element.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - element.offsetHeight));

    element.style.left = x + 'px';
    element.style.top = y + 'px';
    element.style.right = 'auto';
    element.style.bottom = 'auto';
  }

  function onPointerUp(e) {
    if (!isDragging) return;
    isDragging = false;
    element.classList.remove('dragging');
    handle.releasePointerCapture(e.pointerId);
    savePosition(element, storageKey);
  }

  handle.addEventListener('pointerdown', onPointerDown);
  handle.addEventListener('pointermove', onPointerMove);
  handle.addEventListener('pointerup', onPointerUp);
}

function savePosition(element, storageKey) {
  const parent = element.parentElement;
  if (!parent) return;
  const parentRect = parent.getBoundingClientRect();
  const rect = element.getBoundingClientRect();
  const pos = {
    xPct: (rect.left - parentRect.left) / parentRect.width,
    yPct: (rect.top - parentRect.top) / parentRect.height,
  };
  localStorage.setItem(storageKey, JSON.stringify(pos));
}

function restorePosition(element, storageKey) {
  const raw = localStorage.getItem(storageKey);
  if (!raw) return;
  try {
    const pos = JSON.parse(raw);
    const parent = element.parentElement;
    if (!parent) return;
    const parentRect = parent.getBoundingClientRect();

    let x = pos.xPct * parentRect.width;
    let y = pos.yPct * parentRect.height;

    // Constrain within bounds
    x = Math.max(0, Math.min(x, parentRect.width - element.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - element.offsetHeight));

    element.style.left = x + 'px';
    element.style.top = y + 'px';
    element.style.right = 'auto';
    element.style.bottom = 'auto';
  } catch {
    // Ignore invalid stored position
  }
}
