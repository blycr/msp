// 桌面端播放列表列的折叠/展开。移动端（<=1024px）由 mobile-nav 面板切换
// 全权管理，本模块在非桌面宽度自动失效（按钮隐藏、折叠类不生效）。
import { el, lsGet, lsSet, LS } from '../state.js';
import { icon } from '../icons.js';

const DESKTOP_QUERY = '(min-width: 1025px)';

let layout = null;
let desktopMQ = null;

function syncButtons(collapsed) {
  const desktop = desktopMQ?.matches ?? false;
  const hide = el('btnCollapsePl');
  if (hide) hide.hidden = !desktop || collapsed;
  const show = el('btnShowPl');
  if (show) show.hidden = !desktop || !collapsed;
}

export function applyPlCollapsed(collapsed) {
  if (!layout) return;
  layout.classList.toggle('layout--pl-collapsed', collapsed);
  syncButtons(collapsed);
}

function togglePlPanel() {
  const next = !layout.classList.contains('layout--pl-collapsed');
  lsSet(LS.plCollapsed, next ? '1' : '0');
  applyPlCollapsed(next);
}

export function bindPanelToggle() {
  layout = document.querySelector('.layout');
  if (!layout) return;

  const hide = el('btnCollapsePl');
  const show = el('btnShowPl');
  if (hide) hide.innerHTML = icon('chevronRight', 19);
  if (show) {
    const ic = el('iconShowPl');
    if (ic) ic.innerHTML = icon('chevronLeft', 16);
  }
  if (hide) hide.addEventListener('click', togglePlPanel);
  if (show) show.addEventListener('click', togglePlPanel);

  desktopMQ = window.matchMedia(DESKTOP_QUERY);
  const onViewport = () => {
    // 跨断点（移动 <-> 桌面）时恢复/清除折叠态；状态以 localStorage 为准。
    if (desktopMQ.matches) {
      applyPlCollapsed(lsGet(LS.plCollapsed) === '1');
    } else {
      applyPlCollapsed(false);
    }
  };
  if (desktopMQ.addEventListener) desktopMQ.addEventListener('change', onViewport);
  else desktopMQ.addListener(onViewport);
  onViewport();
}
