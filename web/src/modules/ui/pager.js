import { t } from '../i18n.js';
import { icon } from '../icons.js';

// Shared pager: ‹ page/total › — one spec for both file list and playlist.
// Icon buttons keep the bar compact; labels stay i18n via title/aria-label.
//
// The pager element is created once per list and reused across renders:
// callers pass stable module-level onPrev/onNext (read state at click time)
// and refresh page/disabled state via updatePager instead of recreating.
export function createPager({ page, totalPages, onPrev, onNext }) {
  const pager = document.createElement('div');
  pager.className = 'pager';

  const prevBtn = document.createElement('button');
  prevBtn.type = 'button';
  prevBtn.className = 'btn btn--ghost pager__btn';
  prevBtn.innerHTML = icon('chevronLeft', 16);
  prevBtn.disabled = page <= 1;
  prevBtn.title = t('prev');
  prevBtn.setAttribute('aria-label', t('prev'));
  prevBtn.addEventListener('click', onPrev);

  const left = document.createElement('div');
  left.className = 'pager__side';
  left.appendChild(prevBtn);

  const info = document.createElement('div');
  info.className = 'small pager__center';
  info.textContent = `${page}/${totalPages}`;

  const nextBtn = document.createElement('button');
  nextBtn.type = 'button';
  nextBtn.className = 'btn btn--ghost pager__btn';
  nextBtn.innerHTML = icon('chevronRight', 16);
  nextBtn.disabled = page >= totalPages;
  nextBtn.title = t('next');
  nextBtn.setAttribute('aria-label', t('next'));
  nextBtn.addEventListener('click', onNext);

  const right = document.createElement('div');
  right.className = 'pager__side';
  right.appendChild(nextBtn);

  pager.appendChild(left);
  pager.appendChild(info);
  pager.appendChild(right);

  // Stash refs for cheap in-place updates (no per-render re-creation).
  pager._prevBtn = prevBtn;
  pager._nextBtn = nextBtn;
  pager._info = info;
  return pager;
}

// Refresh an existing pager in place: page text + prev/next disabled state.
export function updatePager(pager, { page, totalPages }) {
  if (!pager) return;
  pager._info.textContent = `${page}/${totalPages}`;
  pager._prevBtn.disabled = page <= 1;
  pager._nextBtn.disabled = page >= totalPages;
}
