import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { currentList, filterFiles, sortFiles } from '../playlist.js';
import { formatName, formatBytes, formatTime } from '../utils.js';
import { bus } from '../eventbus.js';

export function setMeta(text) {
  el("meta").textContent = text;
}

export function showDlg(show) {
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
}

export function renderList() {
  const box = el("list");
  const hint = el("hint");
  box.innerHTML = "";

  if (!state.media || (state.media.shares || []).length === 0) {
    hint.textContent = t("hint_noshare");
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
    const row = document.createElement("div");
    row.className = "item";
    row.addEventListener("click", () => bus.emit('play:request', item, { user: true, autoplay: true }));

    const main = document.createElement("div");
    main.className = "item__main";

    const name = document.createElement("div");
    name.className = "item__name";
    name.textContent = formatName(item);

    const sub = document.createElement("div");
    sub.className = "item__sub";
    sub.textContent = `${item.shareLabel || ""}  ·  ${formatBytes(item.size)}  ·  ${formatTime(item.modTime)}`;

    main.appendChild(name);
    main.appendChild(sub);

    const badge = document.createElement("div");
    badge.className = "badge";
    badge.textContent = (item.ext || "").replace(".", "").toUpperCase();

    row.appendChild(main);
    row.appendChild(badge);
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
