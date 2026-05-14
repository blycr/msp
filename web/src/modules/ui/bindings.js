import { state, el, lsSet, LS } from '../state.js';
import { t } from '../i18n.js';
import { getCfg } from '../utils.js';
import { apiPost, gpSet } from '../api.js';
import { bus } from '../eventbus.js';
import { renderPlaylist, updateNavButtons, rebuildPlayOrderFromCurrent, playPrev, playNext } from '../playlist.js';
import { updateResumeButton, resumeLast, setFitBtnVisible } from '../player.js';
import { createArrowDownIcon, createArrowUpIcon } from '../icons.js';
import { showDlg, updateUIForLang, renderList } from './render.js';
import { renderShares, updateBlacklistUI } from './shares.js';
import { applyConfigToUI } from './settings.js';

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

  el("btnRefresh").addEventListener("click", () => {
    bus.emit('config:reload');
    bus.emit('media:refresh');
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
      bus.emit('media:refresh');
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
      bus.emit('media:refresh');
    } catch (e) {
      alert(String(e?.message || e));
    }
  });

  let searchDebounceTimer = 0;
  el("q").addEventListener("input", (ev) => {
    state.q = ev.target.value || "";
    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => renderList(), 200);
  });

  el("sortField").addEventListener("change", (ev) => {
    state.sort.field = ev.target.value;
    lsSet(LS.sortField, state.sort.field);
    renderList();
  });

  if (state.sort.field) {
    try { el("sortField").value = state.sort.field; } catch {}
  }

  const sortBtn = el("sortOrder");
  if (sortBtn) {
    sortBtn.innerHTML = state.sort.order === 1 ? createArrowDownIcon() : createArrowUpIcon();
  }

  el("sortOrder").addEventListener("click", () => {
    state.sort.order *= -1;
    lsSet(LS.sortOrder, String(state.sort.order));
    const sortBtn = el("sortOrder");
    if (sortBtn) {
      sortBtn.innerHTML = state.sort.order === 1 ? createArrowDownIcon() : createArrowUpIcon();
    }
    renderList();
  });

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const tab of tabs) {
    tab.addEventListener("click", () => {
      for (const x of tabs) x.classList.remove("tab--active");
      tab.classList.add("tab--active");
      state.tab = tab.getAttribute("data-tab");
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
      rebuildPlayOrderFromCurrent(on);
      renderPlaylist();
      updateNavButtons();
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

  el("btnPrev").addEventListener("click", () => playPrev(true));
  el("btnNext").addEventListener("click", () => playNext(true));
}
