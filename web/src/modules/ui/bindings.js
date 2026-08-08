import { state, el, lsSet, LS } from '../state.js';
import { t } from '../i18n.js';
import { getCfg } from '../utils.js';
import { apiPost, gpSet } from '../api.js';
import { bus } from '../eventbus.js';
import { renderPlaylist, updateNavButtons, rebuildPlayOrderFromCurrent, playPrev, playNext } from '../playlist.js';
import { resumeLast, setFitBtnVisible } from '../player.js';
import { icon } from '../icons.js';
import { showDlg, updateUIForLang, renderList, setDlgMsg } from './render.js';
import { initListDelegation } from './delegate.js';
import { renderShares, updateBlacklistUI } from './shares.js';
import { applyConfigToUI } from './settings.js';
import { bindPanelToggle } from './panel.js';

export function bindUI() {
  // 图标单一来源（icons.js 注册表）：顶栏/下拉箭头/占位图标统一注入，
  // 避免 inline SVG 绕过注册表（见 icons.js 头注释）。
  const iconSettings = el('iconSettings');
  if (iconSettings) iconSettings.innerHTML = icon('settings', 16);
  const iconRefresh = el('iconRefresh');
  if (iconRefresh) iconRefresh.innerHTML = icon('refresh', 16);
  const sortArrow = el('sortArrow');
  if (sortArrow) sortArrow.innerHTML = icon('chevronDown');
  const coverPh = el('audioCoverPlaceholder');
  if (coverPh) coverPh.innerHTML = icon('musicNote', 48);
  const emptyIc = document.querySelector('#emptyEl .empty__icon');
  if (emptyIc) emptyIc.innerHTML = icon('play');

  // 列表行交互：容器级事件委托（#list / #plList 各绑定一次）
  initListDelegation();

  // 桌面端播放列表列折叠（移动端由 mobile-nav 管理，自动失效）
  bindPanelToggle();

  // Hide settings button for non-local access
  const settingsBtn = el("btnSettings");
  function updateSettingsBtn() {
    if (settingsBtn) {
      settingsBtn.hidden = state.accessLevel !== 'local';
    }
  }
  updateSettingsBtn();
  bus.on('config:loaded', updateSettingsBtn);

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
      setDlgMsg(String(e?.message || e), true);
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
      setDlgMsg(t("msg_bl_saved"), false);
      bus.emit('media:refresh');
    } catch (e) {
      setDlgMsg(String(e?.message || e), true);
    }
  });

  let searchDebounceTimer = 0;
  el("q").addEventListener("input", (ev) => {
    state.q = ev.target.value || "";
    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => renderList(), 200);
  });

  // Custom dropdown for sortField
  function initDropdown(containerId, onChange) {
    const container = el(containerId);
    if (!container) return null;
    const trigger = container.querySelector('.dropdown__trigger');
    const menu = container.querySelector('.dropdown__menu');
    const items = Array.from(container.querySelectorAll('.dropdown__item'));

    function open() {
      container.classList.add('dropdown--open');
      trigger.setAttribute('aria-expanded', 'true');
      menu.hidden = false;
    }
    function close() {
      container.classList.remove('dropdown--open');
      trigger.setAttribute('aria-expanded', 'false');
      menu.hidden = true;
    }
    function toggle() { menu.hidden ? open() : close(); }

    function select(value) {
      for (const item of items) {
        const selected = item.dataset.value === value;
        item.classList.toggle('dropdown__item--selected', selected);
        item.setAttribute('aria-selected', String(selected));
      }
      const selectedItem = items.find(i => i.dataset.value === value);
      if (selectedItem) {
        const valueEl = trigger.querySelector('.dropdown__value');
        if (valueEl) {
          valueEl.textContent = selectedItem.textContent;
          const i18nKey = selectedItem.getAttribute('data-i18n');
          if (i18nKey) valueEl.setAttribute('data-i18n', i18nKey);
        }
      }
      container.dataset.value = value;
      if (onChange) onChange(value);
    }

    trigger.addEventListener('click', (e) => { e.stopPropagation(); toggle(); });
    for (const item of items) {
      item.addEventListener('click', (e) => { e.stopPropagation(); select(item.dataset.value); close(); });
    }
    document.addEventListener('click', () => close());

    container.addEventListener('keydown', (e) => {
      if (menu.hidden) {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); }
        return;
      }
      const activeIdx = items.findIndex(i => i.classList.contains('dropdown__item--selected'));
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        const next = Math.min(activeIdx + 1, items.length - 1);
        select(items[next].dataset.value);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        const prev = Math.max(activeIdx - 1, 0);
        select(items[prev].dataset.value);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        close();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        close();
      }
    });

    trigger.setAttribute('tabindex', '0');
    return { select, get value() { return container.dataset.value; } };
  }

  const sortDropdown = initDropdown('sortField', (value) => {
    state.sort.field = value;
    lsSet(LS.sortField, state.sort.field);
    renderList();
  });

  if (state.sort.field && sortDropdown) {
    sortDropdown.select(state.sort.field);
  }

  const sortBtn = el("sortOrder");
  if (sortBtn) {
    sortBtn.innerHTML = icon(state.sort.order === 1 ? 'arrowDown' : 'arrowUp');
  }

  el("sortOrder").addEventListener("click", () => {
    state.sort.order *= -1;
    lsSet(LS.sortOrder, String(state.sort.order));
    const sortBtn = el("sortOrder");
    if (sortBtn) {
      sortBtn.innerHTML = icon(state.sort.order === 1 ? 'arrowDown' : 'arrowUp');
    }
    renderList();
  });

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const tab of tabs) {
    tab.addEventListener("click", () => {
      for (const x of tabs) x.classList.remove("tab--active");
      tab.classList.add("tab--active");
      state.tab = tab.getAttribute("data-tab");
      // 切换类型时重置页码：不同类型列表长度不同，残留页码会导致翻页越界/错位。
      state.listPage = 1;
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
      // Hide shuffle/loop for favorites tab (cross-kind)
      const shuffleWrap = el("shuffleWrap");
      if (shuffleWrap) {
        shuffleWrap.hidden = state.tab === "favorites" || state.tab === "other";
      }
    });
  }

  el("toggleShuffle").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.shuffle = on;
    gpSet(LS.audioShuffle, on ? "1" : "0");
    // shuffle 仅用于 audio（见 buildPlaylist 的 kind 判定）；只有当前在播
    // 音频时才需要重建播放顺序，避免对 video/image 误触发随机化。
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

  el("btnPrev").addEventListener("click", () => playPrev(true));
  el("btnNext").addEventListener("click", () => playNext(true));

  // Browse mode toggle
  const browseModeBtn = el('browseMode');
  if (browseModeBtn) {
    browseModeBtn.addEventListener('click', () => {
      state.browseMode = state.browseMode === 'flat' ? 'folder' : 'flat';
      state.currentFolder = null;
      browseModeBtn.textContent = state.browseMode === 'flat' ? t('mode_folder') : t('mode_flat');
      renderList();
    });
  }
}
