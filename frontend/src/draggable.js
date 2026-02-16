/** @type {Map<HTMLElement, {element: HTMLElement, storageKey: string, isDragging: boolean}>} */
const registry = new Map();

/** @type {Map<HTMLElement, ResizeObserver>} */
const observers = new Map();

/**
 * Make an element draggable within its parent container.
 * Saves/restores position as percentage of parent size via localStorage.
 * Automatically clamps position on parent resize.
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

  const entry = { element, storageKey, isDragging: false };
  registry.set(element, entry);

  restorePosition(element, storageKey);
  observeParent(element);

  function onPointerDown(e) {
    if (exclude && e.target.closest(exclude)) return;
    entry.isDragging = true;
    const rect = element.getBoundingClientRect();
    entry._offsetX = e.clientX - rect.left;
    entry._offsetY = e.clientY - rect.top;
    element.classList.add('dragging');
    handle.setPointerCapture(e.pointerId);
    e.preventDefault();
  }

  function onPointerMove(e) {
    if (!entry.isDragging) return;
    const parent = element.parentElement;
    const parentRect = parent.getBoundingClientRect();

    let x = e.clientX - parentRect.left - entry._offsetX;
    let y = e.clientY - parentRect.top - entry._offsetY;

    // Constrain within parent bounds
    x = Math.max(0, Math.min(x, parentRect.width - element.offsetWidth));
    y = Math.max(0, Math.min(y, parentRect.height - element.offsetHeight));

    element.style.left = x + 'px';
    element.style.top = y + 'px';
    element.style.right = 'auto';
    element.style.bottom = 'auto';
  }

  function onPointerUp(e) {
    if (!entry.isDragging) return;
    entry.isDragging = false;
    element.classList.remove('dragging');
    handle.releasePointerCapture(e.pointerId);
    savePosition(element, storageKey);
  }

  handle.addEventListener('pointerdown', onPointerDown);
  handle.addEventListener('pointermove', onPointerMove);
  handle.addEventListener('pointerup', onPointerUp);
}

/**
 * Set up ResizeObserver for the parent container (once per parent).
 */
function observeParent(element) {
  const parent = element.parentElement;
  if (!parent || observers.has(parent)) return;

  const observer = new ResizeObserver(() => {
    clampAllInParent(parent);
  });
  observer.observe(parent);
  observers.set(parent, observer);
}

/**
 * Clamp all registered draggable elements within the given parent.
 */
function clampAllInParent(parent) {
  const parentRect = parent.getBoundingClientRect();
  if (parentRect.width === 0 || parentRect.height === 0) return;

  for (const entry of registry.values()) {
    if (entry.element.parentElement !== parent) continue;
    if (entry.isDragging) continue;

    // Skip elements that haven't been repositioned yet (still using CSS defaults)
    if (!entry.element.style.left || entry.element.style.left === 'auto') continue;

    const x = parseFloat(entry.element.style.left) || 0;
    const y = parseFloat(entry.element.style.top) || 0;
    const w = entry.element.offsetWidth;
    const h = entry.element.offsetHeight;

    const clampedX = Math.max(0, Math.min(x, parentRect.width - w));
    const clampedY = Math.max(0, Math.min(y, parentRect.height - h));

    if (clampedX !== x || clampedY !== y) {
      entry.element.classList.add('drag-transition');
      entry.element.style.left = clampedX + 'px';
      entry.element.style.top = clampedY + 'px';
      savePosition(entry.element, entry.storageKey);

      // Remove transition class after animation completes
      setTimeout(() => entry.element.classList.remove('drag-transition'), 200);
    }
  }
}

/**
 * Clamp a single registered draggable element within its parent bounds.
 * Call this after making a hidden element visible to ensure correct position.
 *
 * @param {HTMLElement} element - A previously registered draggable element
 */
export function clampElement(element) {
  const entry = registry.get(element);
  if (!entry) return;
  const parent = element.parentElement;
  if (!parent) return;

  // Skip elements that haven't been repositioned yet
  if (!element.style.left || element.style.left === 'auto') return;

  const parentRect = parent.getBoundingClientRect();
  if (parentRect.width === 0 || parentRect.height === 0) return;

  const x = parseFloat(element.style.left) || 0;
  const y = parseFloat(element.style.top) || 0;
  const w = element.offsetWidth;
  const h = element.offsetHeight;

  const clampedX = Math.max(0, Math.min(x, parentRect.width - w));
  const clampedY = Math.max(0, Math.min(y, parentRect.height - h));

  if (clampedX !== x || clampedY !== y) {
    element.style.left = clampedX + 'px';
    element.style.top = clampedY + 'px';
    savePosition(element, entry.storageKey);
  }
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
