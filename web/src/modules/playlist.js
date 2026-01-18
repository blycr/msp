import { state, el } from './state.js';
import { t } from './i18n.js';
import { getCfg, formatName, dirOfAbsPath, absPathOfItem } from './utils.js';
import { logRemote } from './api.js';
import { playItem } from './player.js';

export function currentList() {
  if (!state.media) return [];
  switch (state.tab) {
    case "video": return state.media.videos || [];
    case "audio": return state.media.audios || [];
    case "image": return state.media.images || [];
    default: return state.media.others || [];
  }
}

export function navLabelsForKind(kind) {
  if (kind === "video") return { prev: t("prev_video"), next: t("next_video") };
  if (kind === "image") return { prev: t("prev_image"), next: t("next_image") };
  if (kind === "audio") return { prev: t("prev_audio"), next: t("next_audio") };
  return { prev: t("prev_item"), next: t("next_item") };
}

export function updateNavLabels() {
  const kind = state.current?.kind || state.playlist.kind || "";
  const { prev, next } = navLabelsForKind(kind);
  const prevBtn = el("btnPrev");
  const nextBtn = el("btnNext");
  if (prevBtn) prevBtn.textContent = prev;
  if (nextBtn) nextBtn.textContent = next;
}

export function getSortVal(item, field) {
  if (field === "size") return item.size || 0;
  if (field === "date") return item.modTime || 0;
  return String(item.name || "").toLowerCase();
}

export function sortFiles(list) {
  const field = state.sort?.field || "name";
  const order = state.sort?.order || 1;
  // Use a copy to avoid mutating the source list if it's the global one
  return [...list].sort((a, b) => {
    const va = getSortVal(a, field);
    const vb = getSortVal(b, field);
    if (field === "name") {
      // Natural sort with Chinese support
      return String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" }) * order;
    }
    if (va < vb) return -1 * order;
    if (va > vb) return 1 * order;
    // Fallback to name if other values are equal
    return String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" });
  });
}

export function filterFiles(list) {
  const q = (state.q || "").trim();
  if (!q) return list;

  // Regex search
  if (q.startsWith("/") && q.length > 2) {
    // Check if it ends with / or has flags
    const match = q.match(/^\/(.+)\/([a-z]*)$/);
    if (match) {
      try {
        const re = new RegExp(match[1], match[2] || "i");
        return list.filter(x => re.test(x.name));
      } catch { }
    }
  }

  // Pinyin/Fuzzy search
  const { pinyinPro } = window;
  if (pinyinPro) {
    return list.filter(x => {
      const name = x.name || "";
      // Check pinyin match
      const m = pinyinPro.match(name, q);
      if (m) return true;
      // Fallback to standard include
      return name.toLowerCase().includes(q.toLowerCase()) || (x.shareLabel || "").toLowerCase().includes(q.toLowerCase());
    });
  }

  // Fallback simple search
  const lower = q.toLowerCase();
  return list.filter(x => (x.name || "").toLowerCase().includes(lower) || (x.shareLabel || "").toLowerCase().includes(lower));
}

export function setPlaylist(kind, items, index) {
  state.playlist.kind = kind;
  state.playlist.items = Array.isArray(items) ? items : [];
  state.playlist.index = Number.isFinite(index) ? index : -1;
  renderPlaylist();
  scheduleAutoFitPlaylistPageSize();
  updateNavButtons();
  updateNavLabels();
  logRemote("info", `Playlist updated: kind=${kind} count=${items?.length} index=${index}`);
}

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

  const pager = document.createElement("div");
  pager.className = "pager";
  const prevBtn = document.createElement("button");
  prevBtn.className = "btn btn--ghost";
  prevBtn.textContent = t("prev");
  const info = document.createElement("div");
  info.className = "small pager__center";
  info.textContent = "1/99";
  const nextBtn = document.createElement("button");
  nextBtn.className = "btn btn--ghost";
  nextBtn.textContent = t("next");
  const left = document.createElement("div");
  left.className = "pager__side";
  left.appendChild(prevBtn);
  const right = document.createElement("div");
  right.className = "pager__side";
  right.appendChild(nextBtn);
  pager.appendChild(left);
  pager.appendChild(info);
  pager.appendChild(right);
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

export function renderPlaylist() {
  const box = el("plList");
  const meta = el("plMeta");
  box.innerHTML = "";

  const items = state.playlist.items || [];
  if (!items.length) {
    meta.textContent = t("not_loaded");
    return;
  }

  const kind = state.playlist.kind || "";
  meta.textContent = `${t("kind_" + kind) || kind} · ${t("item_count", "", items.length).replace(" · ", "")}`;

  const psize = state.plPageSize || 10;
  const total = items.length;
  const totalPages = Math.max(1, Math.ceil(total / psize));
  state.plPage = Math.max(1, Math.min(state.plPage || 1, totalPages));
  const start = (state.plPage - 1) * psize;

  for (let i = start; i < Math.min(total, start + psize); i++) {
    const it = items[i];
    const row = document.createElement("div");
    row.className = "plitem" + (i === state.playlist.index ? " plitem--active" : "");
    row.addEventListener("click", () => playAtIndex(i, true));

    const idx = document.createElement("div");
    idx.className = "plitem__idx";
    idx.textContent = String(i + 1);

    const main = document.createElement("div");
    main.className = "plitem__main";

    const name = document.createElement("div");
    name.className = "plitem__name";
    name.textContent = formatName(it);

    const sub = document.createElement("div");
    sub.className = "plitem__sub";
    sub.textContent = `${it.shareLabel || ""} · ${(it.ext || "").toUpperCase()}`;

    main.appendChild(name);
    main.appendChild(sub);

    row.appendChild(idx);
    row.appendChild(main);
    box.appendChild(row);
  }

  if (totalPages > 1) {
    const pager = document.createElement("div");
    pager.className = "pager";

    const prevBtn = document.createElement("button");
    prevBtn.className = "btn btn--ghost";
    prevBtn.textContent = t("prev");
    prevBtn.disabled = state.plPage <= 1;
    prevBtn.addEventListener("click", () => { state.plPage = Math.max(1, state.plPage - 1); renderPlaylist(); });

    const left = document.createElement("div");
    left.className = "pager__side";
    left.appendChild(prevBtn);

    const info = document.createElement("div");
    info.className = "small pager__center";
    info.textContent = `${state.plPage}/${totalPages}`;

    const nextBtn = document.createElement("button");
    nextBtn.className = "btn btn--ghost";
    nextBtn.textContent = t("next");
    nextBtn.disabled = state.plPage >= totalPages;
    nextBtn.addEventListener("click", () => { state.plPage = Math.min(totalPages, state.plPage + 1); renderPlaylist(); });

    const right = document.createElement("div");
    right.className = "pager__side";
    right.appendChild(nextBtn);

    pager.appendChild(left);
    pager.appendChild(info);
    pager.appendChild(right);
    box.appendChild(pager);
  }
  scheduleAutoFitPlaylistPageSize();
}

export function updateNavButtons() {
  const prev = el("btnPrev");
  const next = el("btnNext");
  const items = state.playlist.items || [];
  const idx = state.playlist.index;
  if (prev) prev.disabled = !(items.length && idx > 0);
  if (next) next.disabled = !(items.length && idx >= 0 && idx < items.length - 1);
  updateNavLabels();
}

export function playAtIndex(i, autoplay, user) {
  const items = state.playlist.items || [];
  if (!items.length) return;
  const idx = Math.max(0, Math.min(items.length - 1, i));
  state.playlist.index = idx;
  renderPlaylist();
  updateNavButtons();
  playItem(items[idx], { fromPlaylist: true, autoplay: !!autoplay, user: !!user });
}

export function buildPlaylist(item, kind) {
  const scope = getCfg(`playback.${kind}.scope`, kind === "audio" ? "all" : "folder");
  const poolMap = { video: "videos", audio: "audios", image: "images" };
  const all = state.media?.[poolMap[kind]] || [];
  if (!all.length) return { items: [], index: -1 };

  let items = [...all];
  if (scope === "folder") {
    const dir = dirOfAbsPath(absPathOfItem(item));
    items = items.filter(x => dirOfAbsPath(absPathOfItem(x)) === dir);
  } else if (scope === "share") {
    items = items.filter(x => x.shareLabel === item.shareLabel);
  }

  // INTUITIVE SORTING LOGIC:
  items.sort((a, b) => String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" }));

  if (kind === "audio" && state.playlist.shuffle) {
    for (let i = items.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [items[i], items[j]] = [items[j], items[i]];
    }
  }

  const index = items.findIndex(x => x.id === item.id);
  return { items, index };
}
