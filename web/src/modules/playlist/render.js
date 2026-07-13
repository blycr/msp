import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { formatName } from '../utils.js';
import { updateNavButtons, updateNavLabels, playAtIndex } from './navigation.js';

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

  for (let i = start; i < Math.min(total, start + psize); i++) {
    const it = items[i];
    const row = document.createElement("div");
    const isActive = state.playlist.playOrder[state.playlist.playIndex] === i;
    row.className = "plitem" + (isActive ? " plitem--active" : "");
    row.setAttribute('role', 'button');
    row.tabIndex = 0;
    row.setAttribute('aria-current', String(isActive));
    const playPos = state.playlist.playOrder.findIndex(idx => idx === i);
    const playItem = () => {
      playAtIndex(playPos >= 0 ? playPos : i, true, true);
    };
    row.addEventListener("click", playItem);
    row.addEventListener("keydown", (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      playItem();
    });

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
