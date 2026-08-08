import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { currentList, filterFiles, sortFiles } from '../playlist.js';
import { formatName, formatBytes, formatTime } from '../utils.js';
import { bus } from '../eventbus.js';
import { getFolderContents } from '../folder.js';
import { icon } from '../icons.js';
import { createPager, updatePager } from './pager.js';
import { diffList, clearList } from './diff.js';

// Auto-fit file list page size so a page always fills the box and the pager
// stays flush at the bottom (same idea as the playlist auto-fit).
const listAutoFit = {
  raf: 0,
  inUpdate: false,
  last: { boxW: 0, itemH: 0, pagerH: 0 },
  ro: null,
};

function scheduleAutoFitListPageSize() {
  if (listAutoFit.raf) return;
  listAutoFit.raf = requestAnimationFrame(() => {
    listAutoFit.raf = 0;
    autoFitListPageSize();
  });
}

function measureListHeights(box) {
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
  row.className = "item";
  const main = document.createElement("div");
  main.className = "item__main";
  const name = document.createElement("div");
  name.className = "item__name";
  name.textContent = "Sample File Item";
  const sub = document.createElement("div");
  sub.className = "item__sub";
  sub.textContent = "Share · MP4";
  main.appendChild(name);
  main.appendChild(sub);
  row.appendChild(main);
  wrap.appendChild(row);

  const pager = createPager({ page: 1, totalPages: 99, onPrev: () => { }, onNext: () => { } });
  wrap.appendChild(pager);

  const itemH = Math.ceil(row.getBoundingClientRect().height + (parseFloat(getComputedStyle(row).marginBottom) || 0));
  const pagerH = Math.ceil(pager.getBoundingClientRect().height + (parseFloat(getComputedStyle(pager).marginTop) || 0));
  wrap.remove();

  return {
    itemH: itemH > 0 ? itemH : 67,
    pagerH: pagerH > 0 ? pagerH : 50,
  };
}

function autoFitListPageSize() {
  if (listAutoFit.inUpdate) return;

  const box = el("list");
  if (!box) return;
  if (state.browseMode === 'folder' && state.tab !== 'favorites') return;

  const total = filterFiles(currentList()).length;
  if (!total) return;

  const boxH = box.clientHeight || 0;
  const boxW = box.clientWidth || 0;
  if (boxH <= 0 || boxW <= 0) return;

  if (!listAutoFit.last.itemH || !listAutoFit.last.pagerH || listAutoFit.last.boxW !== boxW) {
    const m = measureListHeights(box);
    listAutoFit.last.itemH = m.itemH;
    listAutoFit.last.pagerH = m.pagerH;
  }
  listAutoFit.last.boxW = boxW;

  const itemH = listAutoFit.last.itemH || 1;
  const pagerH = listAutoFit.last.pagerH || 0;

  const currentPageSize = state.listPageSize || 10;
  const totalPagesNow = Math.max(1, Math.ceil(total / currentPageSize));
  const willHavePager = totalPagesNow > 1;
  const usable = Math.max(0, boxH - (willHavePager ? pagerH : 0) - 8);

  let target = Math.floor(usable / itemH);
  if (!Number.isFinite(target)) target = currentPageSize;
  target = Math.max(5, Math.min(200, target));

  if (target === currentPageSize) return;

  listAutoFit.inUpdate = true;
  try {
    const firstIdx = ((state.listPage || 1) - 1) * currentPageSize;
    state.listPageSize = target;
    state.listPage = Math.floor(firstIdx / target) + 1;
    renderList();
  } finally {
    listAutoFit.inUpdate = false;
  }
}

export function setMeta(text) {
  const meta = el("meta");
  if (!meta) return;
  // Auto-convert URLs to clickable links while keeping plain text
  const html = String(text || "").replace(
    /(https?:\/\/[^\s]+)/g,
    '<a href="$1" target="_blank" rel="noreferrer" class="meta-link">$1</a>'
  );
  meta.innerHTML = html;
}

// 设置对话框内联提示（替代原生 alert，符合设计语言）。自动 4 秒后消失。
let dlgMsgTimer = 0;
export function setDlgMsg(text, isError) {
  const msg = el("dlgMsg");
  if (!msg) return;
  msg.textContent = text || "";
  msg.hidden = !text;
  msg.classList.toggle("dialog__msg--error", !!isError);
  clearTimeout(dlgMsgTimer);
  if (text) dlgMsgTimer = setTimeout(() => { msg.hidden = true; }, 4000);
}

export function showDlg(show) {
  if (show && state.accessLevel !== 'local') {
    return;
  }
  // 类驱动的可见性（见 other.css .dialog--open）：[hidden] 会被 display:none
  // 杀死过渡，类切换让声明的淡入/scale 动画真正生效。
  el('dlgBackdrop').classList.toggle('dialog--open', !!show);
  el('dlg').classList.toggle('dialog--open', !!show);
}

export function updateUIForLang() {
  const btn = el("langBtn");
  if (btn) btn.textContent = state.lang === "en" ? "CN" : "EN";

  document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";
  document.title = t("title");

  document.querySelectorAll("[data-i18n]").forEach(el => {
    const k = el.getAttribute("data-i18n");
    if (k === "preview_none" && state.current) return;
    if (k === "mode_folder" || k === "mode_flat") return;
    if (k) el.textContent = t(k);
  });
  document.querySelectorAll("[data-i18n-ph]").forEach(el => {
    const k = el.getAttribute("data-i18n-ph");
    if (k) el.placeholder = t(k);
  });
  document.querySelectorAll("[data-i18n-title]").forEach(el => {
    const k = el.getAttribute("data-i18n-title");
    if (k) el.title = t(k);
  });
  document.querySelectorAll("[data-i18n-aria-label]").forEach(el => {
    const k = el.getAttribute("data-i18n-aria-label");
    if (k) el.setAttribute("aria-label", t(k));
  });

  const sharePathEl = el("sharePath");
  if (sharePathEl) {
    const plat = (navigator.platform || navigator.userAgent || "").toLowerCase();
    let ph = t("path_ph");
    if (plat.includes("win")) {
      ph = state.lang === "zh" ? "例如：D:\\\\Media 或 D:/Media（自动兼容斜杠）" : "e.g. D:\\\\Media or D:/Media";
    } else if (plat.includes("mac") || plat.includes("darwin")) {
      ph = state.lang === "zh" ? "例如：/Users/你的用户名/Media" : "e.g. /Users/yourname/Media";
    } else {
      ph = state.lang === "zh" ? "例如：/home/你的用户名/Media 或 ~/Media" : "e.g. /home/username/Media or ~/Media";
    }
    sharePathEl.placeholder = ph;
  }

  const blHint = el("blHint");
  if (blHint) blHint.innerHTML = t("bl_hint");

  // Refresh meta text on language switch
  if (state.configUrls !== undefined && state.accessLevel !== 'remote') {
    const urls = (state.configUrls || []).slice(0, 3).join("  ");
    bus.emit('meta:update', urls ? t("meta_urls", urls) : t("meta_noip"));
  }
}

// —— 列表渲染的模块级状态 ——
// listMode：当前 #list 的内容结构。结构切换（空态/文件夹/平铺互转）允许整体
// 清空重建（低频）；同为 flat 模式时翻页/排序/搜索走 keyed diff 复用行。
let listMode = '';

// 可见行 id → item，事件委托点击时按 dataset.id 查找（替代逐行闭包）。
let rowItemsById = new Map();

export function getRowItem(id) {
  return rowItemsById.get(id) || null;
}

// DB 降级提示：追加在 hint 末尾，朴素中文、不打扰。
const DB_NOTE = '本地数据库不可用，收藏与播放进度已停用';

function setHint(hint, text) {
  if (!hint) return;
  hint.textContent = state.dbAvailable === false ? `${text} · ${DB_NOTE}` : text;
}

// 整体清空：清理缩略图重试定时器（按 data-id 覆盖所有行，含未参与 diff 的
// 文件夹模式行），同时让稳定 pager 引用失效（元素已移除）。
function clearBox(box) {
  for (const row of box.querySelectorAll('[data-id]')) cleanupThumb(row.dataset.id);
  clearList(box, onRemoveRow);
  listPager.el = null;
}

// —— 稳定 pager：元素只创建一次，prev/next 为模块级稳定函数（点击时读当前
// state.listPage），每次渲染仅 updatePager 刷新文案与 disabled，不再新建闭包。——
const listPager = { el: null, totalPages: 1 };

function onListPrev() {
  state.listPage = Math.max(1, (state.listPage || 1) - 1);
  renderList();
}

function onListNext() {
  state.listPage = Math.min(listPager.totalPages, (state.listPage || 1) + 1);
  renderList();
}

function syncListPager(box, totalPages) {
  listPager.totalPages = totalPages;
  if (totalPages <= 1) {
    if (listPager.el) {
      listPager.el.remove();
      listPager.el = null;
    }
    return;
  }
  if (!listPager.el) {
    listPager.el = createPager({ page: 1, totalPages, onPrev: onListPrev, onNext: onListNext });
  }
  // appendChild 对已存在节点是移动操作：保证 pager 始终是最后一个子节点
  // （.pager 靠 margin-top:auto 贴底，必须保持为 flex 容器的直接子节点）。
  box.appendChild(listPager.el);
  updatePager(listPager.el, { page: state.listPage, totalPages });
}

// —— 播放中 active 行切换：只翻转新旧两行的 class，不做全量渲染。——
let lastActiveId = null;

bus.on('player:current', (item) => {
  const box = el('list');
  if (!box) return;
  if (lastActiveId && lastActiveId !== item?.id) {
    box.querySelector(`[data-id="${lastActiveId}"]`)?.classList.remove('item--active');
  }
  lastActiveId = item?.id || null;
  if (lastActiveId) {
    box.querySelector(`[data-id="${lastActiveId}"]`)?.classList.add('item--active');
  }
});

// DB 不可用：重新渲染一次（diff 成本极低），fav 按钮经 updateFileRow 置灰、
// hint 经 setHint 追加降级提示。
bus.on('db:unavailable', () => {
  renderList();
});

export function renderList() {
  // 始终让 .tab--active 高亮与 state.tab 一致——renderList 是左侧列表内容的
  // 单一渲染入口；若后台（resumeLast / loadMedia 304 分支）改写了 state.tab，
  // 这里负责把视觉同步回来，避免"video 标签页高亮却显示音频列表"的脱节。
  for (const x of document.querySelectorAll(".tab")) {
    x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
  }

  const box = el("list");
  const hint = el("hint");

  // Re-fit page size when the box resizes (e.g. F11 fullscreen, window drag).
  // 防御：SPA 无卸载钩子，若 box 已不在文档中则释放 observer。
  if (!listAutoFit.ro && typeof ResizeObserver !== "undefined") {
    listAutoFit.ro = new ResizeObserver(() => {
      if (!box.isConnected) {
        listAutoFit.ro.disconnect();
        listAutoFit.ro = null;
        return;
      }
      scheduleAutoFitListPageSize();
    });
    listAutoFit.ro.observe(box);
  }

  const hasItems = state.media && (
    (state.media.videos || []).length > 0 ||
    (state.media.audios || []).length > 0 ||
    (state.media.images || []).length > 0 ||
    (state.media.others || []).length > 0
  );
  if (!hasItems) {
    clearBox(box);
    listMode = 'empty:lib';
    setHint(hint, t(state.accessLevel === 'local' ? 'hint_noshare_local' : 'hint_noshare_remote'));
    const empty = document.createElement('div');
    empty.className = 'list-empty';

    const iconWrap = document.createElement('div');
    iconWrap.className = 'list-empty__icon';
    iconWrap.setAttribute('aria-hidden', 'true');
    iconWrap.innerHTML = icon('folder', 32);

    const title = document.createElement('div');
    title.className = 'list-empty__title';
    title.textContent = t('empty_library_title');
    empty.appendChild(iconWrap);
    empty.appendChild(title);

    if (state.accessLevel === 'local') {
      const action = document.createElement('button');
      action.type = 'button';
      action.className = 'btn btn--ghost list-empty__action';
      action.textContent = t('open_settings');
      empty.appendChild(action);
    }

    box.appendChild(empty);
    return;
  }

  if (state.browseMode === 'folder' && state.tab !== 'favorites') {
    // 文件夹模式与平铺模式结构不同：进出文件夹/模式切换都走整体重建（低频），
    // 同模式内的翻页/排序/搜索才走 keyed diff。
    clearBox(box);
    listMode = 'folder';
    renderFolderView(box, hint);
    return;
  }

  const raw = currentList();
  let list = filterFiles(raw);
  list = sortFiles(list);

  if (!list.length) {
    clearBox(box);
    listMode = 'empty:search';
    const empty = document.createElement('div');
    empty.className = 'list-empty';
    const title = document.createElement('div');
    title.className = 'list-empty__title';
    title.textContent = t('empty_search_title');
    const detail = document.createElement('div');
    detail.className = 'list-empty__detail';
    detail.textContent = t('empty_search_hint');
    empty.append(title, detail);
    box.appendChild(empty);
    setHint(hint, t('item_count', 0, 0));
    return;
  }

  const kindName = t("kind_" + state.tab) || state.tab;
  let totalForHint = list.length;
  if (!String(state.q || "").trim() && state.media?.limited) {
    const totals = {
      video: state.media.videosTotal,
      audio: state.media.audiosTotal,
      image: state.media.imagesTotal,
      other: state.media.othersTotal,
    };
    const v = totals[state.tab];
    if (Number.isFinite(v) && v > 0) totalForHint = v;
  }
  setHint(hint, t("hint_stats", kindName, totalForHint));

  const pageSize = state.listPageSize || 10;
  const total = list.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  state.listPage = Math.max(1, Math.min(state.listPage || 1, totalPages));
  const start = (state.listPage - 1) * pageSize;
  const pageItems = list.slice(start, start + pageSize);

  if (listMode !== 'flat') clearBox(box);
  listMode = 'flat';

  // 可见行 id → item，供事件委托点击时查找（无需逐行闭包）。
  rowItemsById = new Map(pageItems.map(x => [x.id, x]));

  diffList(box, pageItems, x => 'i:' + x.id, renderFileRow, updateFileRow, onRemoveRow);
  syncListPager(box, totalPages);
  lastActiveId = state.current?.id || null;
  scheduleAutoFitListPageSize();
}

function renderFolderView(box, hint) {
  const allItems = [
    ...(state.media?.videos || []),
    ...(state.media?.audios || []),
    ...(state.media?.images || []),
    ...(state.media?.others || []),
  ];
  const { folders, files } = getFolderContents(allItems, state.currentFolder);
  rowItemsById = new Map(files.map(x => [x.id, x]));

  if (state.currentFolder) {
    const breadcrumb = document.createElement('div');
    breadcrumb.className = 'folder-breadcrumb';

    const backBtn = document.createElement('button');
    backBtn.className = 'btn btn--ghost folder-back';
    backBtn.innerHTML = icon('chevronLeft', 16);
    const backLabel = document.createElement('span');
    backLabel.textContent = t('folder_back');
    backBtn.appendChild(backLabel);
    breadcrumb.appendChild(backBtn);

    const pathSpan = document.createElement('span');
    pathSpan.className = 'folder-breadcrumb__path';
    pathSpan.textContent = state.currentFolder;
    breadcrumb.appendChild(pathSpan);
    box.appendChild(breadcrumb);
  }

  for (const folder of folders) {
    const row = document.createElement('div');
    row.className = 'item item--folder';
    row.setAttribute('role', 'button');
    row.tabIndex = 0;
    row.setAttribute('aria-label', `${folder.name}, ${folder.count} ${t('folder_items')}`);
    row.dataset.path = folder.path;
    const iconWrap = document.createElement('span');
    iconWrap.className = 'folder-icon';
    iconWrap.innerHTML = icon('folder', 20);
    const name = document.createElement('div');
    name.className = 'item__name';
    name.textContent = folder.name;
    const count = document.createElement('div');
    count.className = 'item__sub';
    count.textContent = folder.count > 0 ? `${folder.count} ${t('folder_items')}` : '';
    const main = document.createElement('div');
    main.className = 'item__main';
    main.appendChild(name);
    main.appendChild(count);
    row.appendChild(iconWrap);
    row.appendChild(main);
    box.appendChild(row);
  }

  for (const item of files) {
    const row = renderFileRow(item);
    box.appendChild(row);
  }

  if (hint) {
    setHint(hint, `${folders.length} ${t('folder_folders')} · ${files.length} ${t('folder_files')}`);
  }
}

function renderFileRow(item) {
  const row = document.createElement("div");
  row.className = "item" + (state.current?.id === item.id ? " item--active" : "");
  row.setAttribute('role', 'button');
  row.tabIndex = 0;
  row.setAttribute('aria-label', formatName(item));
  row.dataset.id = item.id;

  if (item.kind === "video") {
    const thumb = document.createElement("img");
    thumb.className = "file-thumb";
    thumb.loading = "lazy";
    thumb.alt = "";
    // 初始不设 src，避免立刻触发大量并发首请求；由 IO 观察器在可见时按需加载。
    // 失败时带退避重试，覆盖 429/5xx 临时拥塞与短视频生成延迟。
    setupThumbRetry(thumb, `/api/thumbnail?id=${encodeURIComponent(item.id)}`, item.id);
    row.appendChild(thumb);
  }

  const main = document.createElement("div");
  main.className = "item__main";

  const name = document.createElement("div");
  name.className = "item__name";
  name.textContent = formatName(item);

  const sub = document.createElement("div");
  sub.className = "item__sub";
  sub.textContent = `${item.shareLabel || ""}  \u00B7  ${formatBytes(item.size)}  \u00B7  ${formatTime(item.modTime)}`;

  main.appendChild(name);
  main.appendChild(sub);

  const badge = document.createElement("div");
  badge.className = "badge";
  badge.textContent = (item.ext || "").replace(".", "").toUpperCase();

  row.appendChild(main);
  row.appendChild(badge);

  // Favorite button（点击由容器级事件委托处理，见 ui/delegate.js）
  const favBtn = document.createElement('button');
  favBtn.type = 'button';
  favBtn.className = 'fav-btn';
  setFavBtnState(favBtn, state.favoriteIds?.has(item.id));
  row.appendChild(favBtn);

  return row;
}

// 收藏按钮状态：star 图标、active class、ARIA；DB 不可用（503 降级）时置灰禁用。
export function setFavBtnState(btn, isFav) {
  const fav = !!isFav;
  const disabled = state.dbAvailable === false;
  btn.classList.toggle('fav-btn--active', fav);
  btn.classList.toggle('fav-btn--disabled', disabled);
  btn.disabled = disabled;
  btn.innerHTML = icon(fav ? 'starFilled' : 'star');
  btn.setAttribute('aria-pressed', String(fav));
  const label = t(fav ? 'favorite_remove' : 'favorite_add');
  btn.setAttribute('aria-label', label);
  btn.title = label;
}

// keyed diff 的就地更新：只刷新 name/sub/badge 文本、收藏按钮状态与
// item--active class；复用现有 <img>（不重建、不动 src），缩略图加载不被打断。
function updateFileRow(row, item) {
  row.dataset.id = item.id;
  row.setAttribute('aria-label', formatName(item));
  row.classList.toggle('item--active', state.current?.id === item.id);

  const name = row.querySelector('.item__name');
  if (name) name.textContent = formatName(item);
  const sub = row.querySelector('.item__sub');
  if (sub) sub.textContent = `${item.shareLabel || ""}  ·  ${formatBytes(item.size)}  ·  ${formatTime(item.modTime)}`;
  const badge = row.querySelector('.badge');
  if (badge) badge.textContent = (item.ext || "").replace(".", "").toUpperCase();

  const favBtn = row.querySelector('.fav-btn');
  if (favBtn) setFavBtnState(favBtn, state.favoriteIds?.has(item.id));
}

// diff 移除行时的清理：取消该行的缩略图重试定时器。
function onRemoveRow(row) {
  if (row.dataset?.id) cleanupThumb(row.dataset.id);
}

bus.on('transcode:status', (status) => {
  let indicator = document.getElementById('transcodeStatus');
  if (!indicator) {
    indicator = document.createElement('div');
    indicator.id = 'transcodeStatus';
    indicator.className = 'transcode-status';
    const playerBox = el('playerBox');
    if (playerBox) playerBox.prepend(indicator);
  }

  if (!status) {
    indicator.hidden = true;
    return;
  }

  indicator.hidden = false;
  if (status === 'checking') {
    indicator.textContent = t('transcode_checking');
  } else if (status === 'transcoding') {
    indicator.innerHTML = `<span class="transcode-status__spinner"></span>${t('transcode_preparing')}`;
  }
});

// —— 缩略图加载与重试 ——
// 后端对并发生成做了排队，但首次批量加载仍可能遇到临时 5xx（排队超时）
// 或短视频生成失败（已回退首帧，仍失败时返回 404）。这里做有限次退避重试，
// 并在彻底失败时隐藏 <img>，避免显示碎图图标。
// 重试状态存模块级 Map（按 item id），diff 复用行时 <img> 不动、状态延续；
// 行被 diff 移除时经 cleanupThumb 取消挂起的定时器。
const THUMB_MAX_RETRIES = 3;
const THUMB_BASE_DELAY = 400; // ms

const thumbStates = new Map(); // id -> { timer, attempts }

function cleanupThumb(id) {
  const st = thumbStates.get(id);
  if (!st) return;
  if (st.timer) clearTimeout(st.timer);
  thumbStates.delete(id);
}

function setupThumbRetry(img, url, id) {
  const st = { timer: null, attempts: 0 };
  thumbStates.set(id, st);

  const load = () => {
    if (!img.isConnected) return; // 行已被移除：不再发起请求
    img.src = url;
  };

  img.addEventListener('error', () => {
    if (!img.isConnected) return;
    if (st.timer) return; // 已在等待重试
    if (st.attempts >= THUMB_MAX_RETRIES) {
      // 彻底失败：隐藏占位，避免碎图图标
      img.classList.add('file-thumb--failed');
      img.removeAttribute('src');
      return;
    }
    st.attempts++;
    const delay = THUMB_BASE_DELAY * Math.pow(2, st.attempts - 1);
    st.timer = setTimeout(() => {
      st.timer = null;
      load();
    }, delay);
  });

  // 成功时清理状态
  img.addEventListener('load', () => {
    if (!img.isConnected) return;
    if (st.timer) { clearTimeout(st.timer); st.timer = null; }
    img.classList.remove('file-thumb--failed');
  });

  // 首次加载：行尚未插入 DOM（isConnected=false），直接设置 src。
  img.src = url;
}
