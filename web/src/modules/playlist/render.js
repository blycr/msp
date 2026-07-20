import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { formatName } from '../utils.js';
import { createPager, updatePager } from '../ui/pager.js';
import { diffList, clearList } from '../ui/diff.js';

const plAutoFit = {
  raf: 0,
  inUpdate: false,
  last: { boxH: 0, boxW: 0, itemH: 0, pagerH: 0 },
  ro: null,
};

export function scheduleAutoFitPlaylistPageSize() {
  if (plAutoFit.raf) return;
  plAutoFit.raf = requestAnimationFrame(() => {
    plAutoFit.raf = 0;
    autoFitPlaylistPageSize();
  });
}

export function getAutoFitState() {
  return plAutoFit;
}

function measurePlaylistHeights(box) {
  const w = Math.max(280, box?.clientWidth || 0);
  const wrap = document.createElement("div");
  wrap.style.position = "absolute";
  wrap.style.visibility = "hidden";
  wrap.style.pointerEvents = "none";
  wrap.style.left = "-10000px";
  wrap.style.top = "0";
  wrap.style.width = `${w}px`;
  document.body.appendChild(wrap);

  const row = document.createElement("div");
  row.className = "plitem";

  const idx = document.createElement("div");
  idx.className = "plitem__idx";
  idx.textContent = "99";

  const main = document.createElement("div");
  main.className = "plitem__main";

  const name = document.createElement("div");
  name.className = "plitem__name";
  name.textContent = "Sample Playlist Item";

  const sub = document.createElement("div");
  sub.className = "plitem__sub";
  sub.textContent = "Share · MP4";

  main.appendChild(name);
  main.appendChild(sub);
  row.appendChild(idx);
  row.appendChild(main);
  wrap.appendChild(row);

  const pager = createPager({ page: 1, totalPages: 99, onPrev: () => { }, onNext: () => { } });
  wrap.appendChild(pager);

  const itemH = Math.ceil(row.getBoundingClientRect().height || 0);
  const pagerH = Math.ceil(pager.getBoundingClientRect().height || 0);
  wrap.remove();

  return {
    itemH: itemH > 0 ? itemH : 44,
    pagerH: pagerH > 0 ? pagerH : 36,
  };
}

export function autoFitPlaylistPageSize() {
  if (plAutoFit.inUpdate) return;

  const box = el("plList");
  if (!box) return;
  const items = state.playlist.items || [];
  if (!items.length) return;

  const boxH = box.clientHeight || 0;
  const boxW = box.clientWidth || 0;
  if (boxH <= 0 || boxW <= 0) return;

  const needRemeasure = !plAutoFit.last.itemH || !plAutoFit.last.pagerH || plAutoFit.last.boxW !== boxW;
  if (needRemeasure) {
    const m = measurePlaylistHeights(box);
    plAutoFit.last.itemH = m.itemH;
    plAutoFit.last.pagerH = m.pagerH;
  }

  plAutoFit.last.boxH = boxH;
  plAutoFit.last.boxW = boxW;

  const itemH = plAutoFit.last.itemH || 1;
  const pagerH = plAutoFit.last.pagerH || 0;

  const currentPageSize = state.plPageSize || 10;
  const totalPagesNow = Math.max(1, Math.ceil(items.length / currentPageSize));
  const willHavePager = totalPagesNow > 1;
  const usable = Math.max(0, boxH - (willHavePager ? pagerH : 0));

  let target = Math.floor(usable / itemH);
  if (!Number.isFinite(target)) target = currentPageSize;
  target = Math.max(5, Math.min(200, target));

  if (target === currentPageSize) return;

  plAutoFit.inUpdate = true;
  try {
    state.plPageSize = target;
    const idx = state.playlist.index;
    if (idx >= 0) state.plPage = Math.floor(idx / target) + 1;
    else state.plPage = 1;
    renderPlaylist();
  } finally {
    plAutoFit.inUpdate = false;
  }
}

// plMode：#plList 当前内容结构（'' / 'list' / 'empty'）。结构切换整体重建，
// 同结构翻页/重排/active 切换走 keyed diff。
let plMode = '';

// 稳定 pager：元素只创建一次，prev/next 为模块级稳定函数（点击时读当前
// state.plPage），每次渲染仅 updatePager 刷新文案与 disabled。
const plPager = { el: null, totalPages: 1 };

function onPlPrev() {
  state.plPage = Math.max(1, (state.plPage || 1) - 1);
  renderPlaylist();
}

function onPlNext() {
  state.plPage = Math.min(plPager.totalPages, (state.plPage || 1) + 1);
  renderPlaylist();
}

function syncPlPager(box, totalPages) {
  plPager.totalPages = totalPages;
  if (totalPages <= 1) {
    if (plPager.el) {
      plPager.el.remove();
      plPager.el = null;
    }
    return;
  }
  if (!plPager.el) {
    plPager.el = createPager({ page: 1, totalPages, onPrev: onPlPrev, onNext: onPlNext });
  }
  box.appendChild(plPager.el); // 移动到最后，保持 flex 直接子节点（margin-top:auto）
  updatePager(plPager.el, { page: state.plPage, totalPages });
}

function clearPlBox(box) {
  clearList(box);
  plPager.el = null;
}

export function renderPlaylist() {
  const box = el("plList");
  const meta = el("plMeta");

  // Re-fit page size when the box resizes (e.g. F11 fullscreen, window drag).
  // 防御：SPA 无卸载钩子，若 box 已不在文档中则释放 observer。
  if (!plAutoFit.ro && typeof ResizeObserver !== "undefined") {
    plAutoFit.ro = new ResizeObserver(() => {
      if (!box.isConnected) {
        plAutoFit.ro.disconnect();
        plAutoFit.ro = null;
        return;
      }
      scheduleAutoFitPlaylistPageSize();
    });
    plAutoFit.ro.observe(box);
  }

  const items = state.playlist.items || [];
  if (!items.length) {
    clearPlBox(box);
    plMode = 'empty';
    meta.textContent = t("not_loaded");
    const empty = document.createElement('div');
    empty.className = 'panel-empty';
    empty.textContent = t('playlist_empty');
    box.appendChild(empty);
    return;
  }

  const kind = state.playlist.kind || "";
  meta.textContent = `${t("kind_" + kind) || kind} · ${t("item_count", "", items.length).replace(" · ", "")}`;

  const psize = state.plPageSize || 10;
  const total = items.length;
  const totalPages = Math.max(1, Math.ceil(total / psize));
  state.plPage = Math.max(1, Math.min(state.plPage || 1, totalPages));
  const start = (state.plPage - 1) * psize;

  if (plMode !== 'list') clearPlBox(box);
  plMode = 'list';

  // 每行携带其在 items 中的下标 i：active 判定与点击播放都依赖下标而非 id。
  const pageEntries = [];
  for (let i = start; i < Math.min(total, start + psize); i++) {
    pageEntries.push({ it: items[i], i });
  }

  diffList(box, pageEntries, e => 'p:' + e.it.id, renderPlRow, updatePlItem);
  syncPlPager(box, totalPages);
  scheduleAutoFitPlaylistPageSize();
}

function renderPlRow({ it, i }) {
  const row = document.createElement("div");
  row.dataset.id = it.id;
  row.appendChild(createPlIdx());
  row.appendChild(createPlMain());
  updatePlItem(row, { it, i });
  return row;
}

function createPlIdx() {
  const idx = document.createElement("div");
  idx.className = "plitem__idx";
  return idx;
}

function createPlMain() {
  const main = document.createElement("div");
  main.className = "plitem__main";
  const name = document.createElement("div");
  name.className = "plitem__name";
  const sub = document.createElement("div");
  sub.className = "plitem__sub";
  main.appendChild(name);
  main.appendChild(sub);
  return main;
}

// 就地更新：下标、name/sub、active 状态；点击播放由容器级委托按
// dataset.plIndex 处理（见 ui/delegate.js）。
function updatePlItem(row, { it, i }) {
  const isActive = state.playlist.playOrder[state.playlist.playIndex] === i;
  row.className = "plitem" + (isActive ? " plitem--active" : "");
  row.setAttribute('role', 'button');
  row.tabIndex = 0;
  row.setAttribute('aria-current', String(isActive));
  row.dataset.id = it.id;
  row.dataset.plIndex = String(i);

  const idx = row.querySelector('.plitem__idx');
  if (idx) idx.textContent = String(i + 1);
  const name = row.querySelector('.plitem__name');
  if (name) name.textContent = formatName(it);
  const sub = row.querySelector('.plitem__sub');
  if (sub) sub.textContent = `${it.shareLabel || ""} · ${(it.ext || "").toUpperCase()}`;
}
