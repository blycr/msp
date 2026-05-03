import { state, el } from './state.js';
import { t } from './i18n.js';
import { getCfg, formatName, dirOfAbsPath, absPathOfItem } from './utils.js';
import { logRemote } from './api.js';
import { bus } from './eventbus.js';
import {
  currentList,
  sortFiles,
  filterFiles,
  navLabelsForKind,
  updateNavLabels as _updateNavLabels,
  updateNavButtons as _updateNavButtons,
  playAtIndex as _playAtIndex,
  playPrev as _playPrev,
  playNext as _playNext,
  generatePlayOrder,
  buildPlaylist as _buildPlaylist,
  rebuildPlayOrderFromCurrent,
  scheduleAutoFitPlaylistPageSize,
  getAutoFitState,
  autoFitPlaylistPageSize as _autoFitPlaylistPageSize,
  renderPlaylist as _renderPlaylist
} from './playlist/index.js';

export {
  currentList,
  sortFiles,
  filterFiles,
  navLabelsForKind,
  generatePlayOrder,
  rebuildPlayOrderFromCurrent,
  scheduleAutoFitPlaylistPageSize,
  getAutoFitState
};

export function setPlaylist(kind, items, index, playOrder = null, playIndex = -1) {
  state.playlist.kind = kind;
  state.playlist.items = Array.isArray(items) ? items : [];
  state.playlist.index = Number.isFinite(index) ? index : -1;
  state.playlist.playOrder = Array.isArray(playOrder) ? playOrder : [];
  state.playlist.playIndex = Number.isFinite(playIndex) ? playIndex : -1;
  renderPlaylist();
  scheduleAutoFitPlaylistPageSize();
  updateNavButtons();
  updateNavLabels();
  logRemote("info", `Playlist updated: kind=${kind} count=${items?.length} index=${index} playIndex=${playIndex}`);
}

export function updateNavLabels() {
  _updateNavLabels();
}

export function updateNavButtons() {
  _updateNavButtons();
}

export function playAtIndex(i, autoplay, user) {
  const items = state.playlist.items || [];
  if (!items.length) return;

  const playOrder = state.playlist.playOrder;
  const playIndex = Math.max(0, Math.min(playOrder.length - 1, i));
  const actualIndex = playOrder[playIndex];

  if (actualIndex === undefined || actualIndex < 0 || actualIndex >= items.length) return;

  state.playlist.playIndex = playIndex;
  state.playlist.index = actualIndex;
  renderPlaylist();
  updateNavButtons();
  bus.emit('play:request', items[actualIndex], { fromPlaylist: true, autoplay: !!autoplay, user: !!user });
}

export function playPrev(autoplay = true) {
  _playPrev(autoplay);
}

export function playNext(autoplay = true) {
  _playNext(autoplay);
}

export function buildPlaylist(item, kind, shuffle = null) {
  return _buildPlaylist(item, kind, shuffle);
}

export function autoFitPlaylistPageSize() {
  _autoFitPlaylistPageSize();
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
    const isActive = state.playlist.playOrder[state.playlist.playIndex] === i;
    row.className = "plitem" + (isActive ? " plitem--active" : "");
    const playPos = state.playlist.playOrder.findIndex(idx => idx === i);
    row.addEventListener("click", () => playAtIndex(playPos >= 0 ? playPos : i, true));

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
