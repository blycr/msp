// Generic keyed diff for small lists (≤200 rows per page). Reuses existing
// rows by key: rows whose key persists are updated in place (updateRow), new
// keys are created via renderRow, reordered rows are moved (insertBefore),
// and leftover old rows are removed (onRemove for cleanup hooks).
//
// Children without a data-key attribute (e.g. the stable pager element kept
// as the container's last child) are never touched by the diff, so the pager
// can live in the same container without being recreated each render.

/**
 * @param {HTMLElement} container
 * @param {Array} items new list of items to display
 * @param {(item: any) => string} keyFn stable key for an item
 * @param {(item: any, index: number) => HTMLElement} renderRow create a new row
 * @param {(row: HTMLElement, item: any, index: number) => void} updateRow patch a reused row
 * @param {(row: HTMLElement, key: string) => void} [onRemove] cleanup before a row is dropped
 */
export function diffList(container, items, keyFn, renderRow, updateRow, onRemove) {
  const oldRows = new Map();
  for (const child of container.children) {
    const k = child.dataset?.key;
    if (k != null && !oldRows.has(k)) oldRows.set(k, child);
  }

  const kept = new Set();
  // ref tracks the node currently expected at the diff position; rows that
  // are out of place are inserted before it. Non-keyed nodes (pager) simply
  // become the final ref and stay put.
  let ref = container.firstChild;

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    const key = String(keyFn(item));
    let row = oldRows.get(key) || null;
    if (row) {
      kept.add(key);
      updateRow?.(row, item, i);
    } else {
      row = renderRow(item, i);
      row.dataset.key = key;
    }
    if (row === ref) {
      ref = ref.nextSibling;
    } else {
      container.insertBefore(row, ref);
    }
  }

  for (const [key, row] of oldRows) {
    if (kept.has(key)) continue;
    onRemove?.(row, key);
    row.remove();
  }
}

/**
 * Remove all children of a container (mode switches, empty states). Keyed
 * rows go through onRemove so per-row resources (e.g. thumb retry timers)
 * are cleaned up; non-keyed children are removed silently.
 */
export function clearList(container, onRemove) {
  for (const child of [...container.children]) {
    const k = child.dataset?.key;
    if (k != null) onRemove?.(child, k);
    child.remove();
  }
}
