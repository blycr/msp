import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { currentList, filterFiles, sortFiles } from '../playlist.js';
import { formatName, formatBytes, formatTime } from '../utils.js';
import { bus } from '../eventbus.js';
import { getFolderContents } from '../folder.js';
import { addFavorite, removeFavorite } from '../api.js';

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

export function showDlg(show) {
  if (show && state.accessLevel !== 'local') {
    return;
  }
  el("dlgBackdrop").hidden = !show;
  el("dlg").hidden = !show;
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

export function renderList() {
  // 始终让 .tab--active 高亮与 state.tab 一致——renderList 是左侧列表内容的
  // 单一渲染入口；若后台（resumeLast / loadMedia 304 分支）改写了 state.tab，
  // 这里负责把视觉同步回来，避免"video 标签页高亮却显示音频列表"的脱节。
  for (const x of document.querySelectorAll(".tab")) {
    x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
  }

  const box = el("list");
  const hint = el("hint");
  box.innerHTML = "";

  const hasItems = state.media && (
    (state.media.videos || []).length > 0 ||
    (state.media.audios || []).length > 0 ||
    (state.media.images || []).length > 0 ||
    (state.media.others || []).length > 0
  );
  if (!hasItems) {
    const hintKey = state.accessLevel === 'local' ? 'hint_noshare_local' : 'hint_noshare_remote';
    hint.textContent = t(hintKey);
    return;
  }

  if (state.browseMode === 'folder' && state.tab !== 'favorites') {
    renderFolderView(box, hint);
    return;
  }

  const raw = currentList();
  let list = filterFiles(raw);
  list = sortFiles(list);

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
  hint.textContent = t("hint_stats", kindName, totalForHint);

  const pageSize = state.listPageSize || 10;
  const total = list.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  state.listPage = Math.max(1, Math.min(state.listPage || 1, totalPages));
  const start = (state.listPage - 1) * pageSize;
  const pageItems = list.slice(start, start + pageSize);

  for (const item of pageItems) {
    const row = renderFileRow(item);
    box.appendChild(row);
  }

  if (totalPages > 1) {
    const pager = document.createElement("div");
    pager.className = "pager";

    const prevBtn = document.createElement("button");
    prevBtn.className = "btn btn--ghost";
    prevBtn.textContent = t("prev");
    prevBtn.disabled = state.listPage <= 1;
    prevBtn.addEventListener("click", () => { state.listPage = Math.max(1, state.listPage - 1); renderList(); });

    const left = document.createElement("div");
    left.className = "pager__side";
    left.appendChild(prevBtn);

    const info = document.createElement("div");
    info.className = "small pager__center";
    info.textContent = `${state.listPage}/${totalPages}`;

    const nextBtn = document.createElement("button");
    nextBtn.className = "btn btn--ghost";
    nextBtn.textContent = t("next");
    nextBtn.disabled = state.listPage >= totalPages;
    nextBtn.addEventListener("click", () => { state.listPage = Math.min(totalPages, state.listPage + 1); renderList(); });

    const right = document.createElement("div");
    right.className = "pager__side";
    right.appendChild(nextBtn);

    pager.appendChild(left);
    pager.appendChild(info);
    pager.appendChild(right);
    box.appendChild(pager);
  }
}

function renderFolderView(box, hint) {
  const allItems = [
    ...(state.media?.videos || []),
    ...(state.media?.audios || []),
    ...(state.media?.images || []),
    ...(state.media?.others || []),
  ];
  const { folders, files } = getFolderContents(allItems, state.currentFolder);

  if (state.currentFolder) {
    const breadcrumb = document.createElement('div');
    breadcrumb.className = 'folder-breadcrumb';
    const parts = state.currentFolder.split('/');

    const backBtn = document.createElement('button');
    backBtn.className = 'btn btn--ghost folder-back';
    backBtn.textContent = '\u2190 ' + t('folder_back');
    backBtn.addEventListener('click', () => {
      if (parts.length <= 1) {
        state.currentFolder = null;
      } else {
        state.currentFolder = parts.slice(0, -1).join('/');
      }
      renderList();
    });
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
    row.addEventListener('click', () => {
      state.currentFolder = folder.path;
      renderList();
    });
    const icon = document.createElement('span');
    icon.className = 'folder-icon';
    icon.textContent = '\uD83D\uDCC1';
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
    row.appendChild(icon);
    row.appendChild(main);
    box.appendChild(row);
  }

  for (const item of files) {
    const row = renderFileRow(item);
    box.appendChild(row);
  }

  if (hint) {
    hint.textContent = `${folders.length} ${t('folder_folders')} \u00B7 ${files.length} ${t('folder_files')}`;
  }
}

function renderFileRow(item) {
  const row = document.createElement("div");
  row.className = "item";
  row.addEventListener("click", () => bus.emit('play:request', item, { user: true, autoplay: true }));

  if (item.kind === "video") {
    const thumb = document.createElement("img");
    thumb.className = "file-thumb";
    thumb.loading = "lazy";
    thumb.alt = "";
    // 初始不设 src，避免立刻触发大量并发首请求；由 IO 观察器在可见时按需加载。
    // 失败时带退避重试，覆盖 429/5xx 临时拥塞与短视频生成延迟。
    setupThumbRetry(thumb, `/api/thumbnail?id=${encodeURIComponent(item.id)}`);
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

  // Favorite button
  const favBtn = document.createElement('button');
  favBtn.className = 'fav-btn' + (state.favoriteIds?.has(item.id) ? ' fav-btn--active' : '');
  favBtn.textContent = state.favoriteIds?.has(item.id) ? '\u2605' : '\u2606';
  favBtn.addEventListener('click', async (e) => {
    e.stopPropagation();
    if (state.favoriteIds?.has(item.id)) {
      await removeFavorite(item.id);
      state.favoriteIds.delete(item.id);
    } else {
      await addFavorite(item.id);
      if (!state.favoriteIds) state.favoriteIds = new Set();
      state.favoriteIds.add(item.id);
    }
    renderList();
  });
  row.appendChild(favBtn);

  return row;
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
const THUMB_MAX_RETRIES = 3;
const THUMB_BASE_DELAY = 400; // ms

function setupThumbRetry(img, url) {
  let attempt = 0;
  let timer = null;

  const load = () => {
    img.src = url;
  };

  img.addEventListener('error', () => {
    if (timer) return; // 已在等待重试
    if (attempt >= THUMB_MAX_RETRIES) {
      // 彻底失败：隐藏占位，避免碎图图标
      img.classList.add('file-thumb--failed');
      img.removeAttribute('src');
      return;
    }
    attempt++;
    const delay = THUMB_BASE_DELAY * Math.pow(2, attempt - 1);
    timer = setTimeout(() => {
      timer = null;
      load();
    }, delay);
  });

  // 成功时清理状态
  img.addEventListener('load', () => {
    if (timer) { clearTimeout(timer); timer = null; }
    img.classList.remove('file-thumb--failed');
  });

  load();
}
