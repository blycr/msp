import { state, el, lsSet, LS } from './state.js';
import { t } from './i18n.js';
import { currentList, filterFiles, sortFiles, buildPlaylist, setPlaylist, getSortVal, renderPlaylist, updateNavLabels, playAtIndex } from './playlist.js';
import { playItem, updateResumeButton, resumeLast, setFitBtnVisible } from './player.js';
import { formatName, formatBytes, formatTime, getCfg } from './utils.js';
import { createArrowDownIcon, createArrowUpIcon } from './icons.js';
import { apiPost, gpSet, gpGet, logRemote, probeItem, probeText, probeWarnText } from './api.js';
import { loadConfig, loadMedia } from './actions.js';

export function setMeta(text) {
  el("meta").textContent = text;
}

export function showDlg(show) {
  el("dlgBackdrop").hidden = !show;
  el("dlg").hidden = !show;
}

export function updateUIForLang() {
  // Update button text
  const btn = el("langBtn");
  if (btn) btn.textContent = state.lang === "en" ? "CN" : "EN";

  document.documentElement.lang = state.lang === "zh" ? "zh-CN" : "en";

  // Update static elements
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

  // Platform-specific placeholder for share path
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

  // Update HTML content
  const blHint = el("blHint");
  if (blHint) blHint.innerHTML = t("bl_hint");

  renderList();
  
  // Re-render playlist if loaded
  renderPlaylist();
  updateNavLabels();

  // Update previewSub
  if (state.current) {
    const item = state.current;
    state.currentMetaBase = `${item.shareLabel || ""} · ${(item.ext || "").toUpperCase()} · ${formatBytes(item.size)} · ${formatTime(item.modTime)}`;
    el("previewSub").textContent = state.currentMetaBase; // Simplified, probe text async update skipped for simplicity here
  }
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
    row.addEventListener("click", () => playItem(item, { user: true, autoplay: true }));

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

export function renderShares() {
  const list = el("shareList");
  list.innerHTML = "";

  const shares = state.config?.shares || [];
  if (shares.length === 0) {
    const empty = document.createElement("div");
    empty.className = "small";
    empty.textContent = t("shares_empty");
    list.appendChild(empty);
    return;
  }

  for (const sh of shares) {
    const row = document.createElement("div");
    row.className = "share";

    const main = document.createElement("div");
    main.className = "share__main";

    const title = document.createElement("div");
    title.className = "item__name";
    title.textContent = sh.label || "";

    const p = document.createElement("div");
    p.className = "share__path";
    p.textContent = sh.path || "";

    main.appendChild(title);
    main.appendChild(p);

    const btn = document.createElement("button");
    btn.className = "btn btn--ghost";
    btn.textContent = t("remove");
    btn.addEventListener("click", async () => {
      try {
        const data = await apiPost("/api/shares", { op: "remove", path: sh.path });
        state.config = data.config;
        renderShares();
        await loadMedia(true);
      } catch (e) {
        alert(String(e?.message || e));
      }
    });

    row.appendChild(main);
    row.appendChild(btn);
    list.appendChild(row);
  }
}

export function applyConfigToUI() {
  const showOthers = !!getCfg("ui.showOthers", false);
  const otherTab = el("tabOther");
  if (otherTab) {
    otherTab.hidden = !showOthers;
    if (!showOthers && state.tab === "other") {
      state.tab = getCfg("ui.defaultTab", "video");
    }
  }

  const playlistEnabled = !!getCfg("features.playlist", true);
  const playlistPanel = el("playlistPanel");
  if (playlistPanel) playlistPanel.hidden = !playlistEnabled;
  const prev = el("btnPrev");
  const next = el("btnNext");
  const shuffleWrap = el("shuffleWrap");
  if (prev) prev.hidden = !playlistEnabled;
  if (next) next.hidden = !playlistEnabled;
  if (shuffleWrap) shuffleWrap.hidden = !playlistEnabled || state.current?.kind !== "audio";
  if (!playlistEnabled) {
    setPlaylist(null, [], -1);
  }

  const defTab = getCfg("ui.defaultTab", "video");
  if (defTab === "video" || defTab === "audio" || defTab === "image" || (defTab === "other" && showOthers)) {
    state.tab = defTab;
  }

  let shuffle = false;
  {
    const saved = gpGet(LS.audioShuffle);
    if (saved === "1") shuffle = true;
    else if (saved === "0") shuffle = false;
    else shuffle = !!getCfg("playback.audio.shuffle", false);
  }
  state.playlist.shuffle = shuffle;
  const t = el("toggleShuffle");
  if (t) t.checked = shuffle;

  let loop = false;
  {
    const saved = gpGet(LS.audioLoop);
    loop = saved === "1";
  }
  state.playlist.loop = loop;
  const tl = el("toggleLoop");
  if (tl) tl.checked = loop;

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const x of tabs) x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
}

export function bindUI() {
  el("langBtn").addEventListener("click", () => {
    state.lang = state.lang === "en" ? "zh" : "en";
    lsSet(LS.lang, state.lang);
    updateUIForLang();
  });

  el("btnSettings").addEventListener("click", () => showDlg(true));
  el("btnCloseDlg").addEventListener("click", () => showDlg(false));
  el("dlgBackdrop").addEventListener("click", () => showDlg(false));
  el("topbarTitle").addEventListener("click", () => location.reload());

  el("btnRefresh").addEventListener("click", async () => {
    try { await loadConfig(); await loadMedia(true); } catch (e) { alert(String(e?.message || e)); }
  });

  el("btnAddShare").addEventListener("click", async () => {
    const p = el("sharePath").value || "";
    const label = el("shareLabel").value || "";
    try {
      const data = await apiPost("/api/shares", { op: "add", path: p, label });
      state.config = data.config;
      el("sharePath").value = "";
      el("shareLabel").value = "";
      renderShares();
      await loadMedia(true);
    } catch (e) {
      alert(String(e?.message || e));
    }
  });

  el("btnSaveBlacklist").addEventListener("click", async () => {
    const bl = state.config.blacklist || {};
    bl.extensions = el("blExts").value.split(/[,，]/).map(s => s.trim()).filter(Boolean);
    bl.filenames = el("blFiles").value.split(/[,，]/).map(s => s.trim()).filter(Boolean);
    bl.folders = el("blFolders").value.split(/[,，]/).map(s => s.trim()).filter(Boolean);
    bl.sizeRule = el("blMinSize").value.trim();
    state.config.blacklist = bl;

    try {
      const data = await apiPost("/api/config", state.config);
      state.config = data.config;
      alert(t("msg_bl_saved"));
      await loadMedia(true);
    } catch (e) {
      alert(String(e?.message || e));
    }
  });

  el("q").addEventListener("input", (ev) => {
    state.q = ev.target.value || "";
    renderList();
  });

  el("sortField").addEventListener("change", (ev) => {
    state.sort.field = ev.target.value;
    renderList();
  });

  const sortBtn = el("sortOrder");
  if (sortBtn) {
    sortBtn.innerHTML = state.sort.order === 1 ? createArrowDownIcon() : createArrowUpIcon();
  }

  el("sortOrder").addEventListener("click", () => {
    state.sort.order *= -1;
    const sortBtn = el("sortOrder");
    if (sortBtn) {
      sortBtn.innerHTML = state.sort.order === 1 ? createArrowDownIcon() : createArrowUpIcon();
    }
    renderList();
  });

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const t of tabs) {
    t.addEventListener("click", () => {
      for (const x of tabs) x.classList.remove("tab--active");
      t.classList.add("tab--active");
      state.tab = t.getAttribute("data-tab");
      renderList();
      setFitBtnVisible(state.tab === "video" && state.current?.kind === "video");
      if (state.tab === "video" && state.current?.kind === "video") {
        try {
          const v = el("videoEl");
          const fitBtn = el("btnToggleFit");
          fitBtn.disabled = false;
          const fit = v?.dataset?.fit || "contain";
          fitBtn.textContent = fit === "cover" ? t("fit_cover") : t("fit_contain");
        } catch { }
      }
    });
  }

  el("toggleShuffle").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.shuffle = on;
    gpSet(LS.audioShuffle, on ? "1" : "0");
    if (state.current?.kind === "audio" && getCfg("playback.audio.enabled", true)) {
      const pl = buildPlaylist(state.current, "audio");
      setPlaylist("audio", pl.items, pl.index);
    }
  });

  el("toggleLoop").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.loop = on;
    gpSet(LS.audioLoop, on ? "1" : "0");
  });

  try {
    const fitBtn = el("btnToggleFit");
    fitBtn.disabled = true;
    fitBtn.addEventListener("click", () => {
      const v = el("videoEl");
      if (!v) return;
      const cur = v.dataset.fit || "cover";
      const next = cur === "cover" ? "contain" : "cover";
      try { v.dataset.fit = next; } catch { }
      try { fitBtn.textContent = next === "cover" ? t("fit_cover") : t("fit_contain"); } catch { }
      gpSet("msp.video.fit", next);
    });
  } catch { }

  try {
    const btnResume = el("btnResume");
    btnResume.disabled = true;
    btnResume.hidden = true;
    btnResume.addEventListener("click", () => resumeLast());
    updateResumeButton();
  } catch { }
  
  el("btnPrev").addEventListener("click", () => {
    if (state.playlist.index > 0) playAtIndex(state.playlist.index - 1, true);
  });
  el("btnNext").addEventListener("click", () => {
    if (state.playlist.items?.length && state.playlist.index < state.playlist.items.length - 1) playAtIndex(state.playlist.index + 1, true);
  });
}
