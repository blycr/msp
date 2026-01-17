import { registerSW } from 'virtual:pwa-register';
import { state, el, lsSet, LS, lsGet } from './state.js';
import { t, initLang } from './i18n.js';
import { apiGet, apiRetry, loadPrefs, gpGet } from './api.js';
import { applyConfigToUI, renderList, renderShares, bindUI, updateUIForLang, setMeta, showDlg } from './ui.js';
import { updateResumeButton, hideAllMedia, bindGlobalHotkeys, resumeLast } from './player.js';
import { initTheme } from './theme.js';
import { bindPinDialog, checkPinRequired, showPinDialog } from './pin.js';

export async function loadConfig() {
  try {
    const data = await apiGet("/api/config");
    state.config = data.config;
    const urls = (data.urls || []).slice(0, 3).join("  ");
    setMeta(urls ? t("meta_urls", urls) : t("meta_noip"));
    applyConfigToUI();
    renderShares();

    const bl = state.config.blacklist || {};
    const blExts = el("blExts");
    const blFiles = el("blFiles");
    const blFolders = el("blFolders");
    const blMinSize = el("blMinSize");
    
    if (blExts) blExts.value = (bl.extensions || []).join(", ");
    if (blFiles) blFiles.value = (bl.filenames || []).join(", ");
    if (blFolders) blFolders.value = (bl.folders || []).join(", ");
    if (blMinSize) blMinSize.value = bl.sizeRule || "";
  } catch (e) {
    console.error("Failed to load config:", e);
    setMeta(t("meta_fail"));
    state.config = {};
    applyConfigToUI();
    renderShares();
  }
}

export async function loadMedia(refresh, limit) {
  const isLimitedRequest = Number(limit || 0) > 0;

  const headers = {};
  if (!refresh && !isLimitedRequest && !state.media?.limited) {
    const etag = lsGet(LS.mediaETag);
    if (etag) headers["If-None-Match"] = etag;
  }

  const params = new URLSearchParams();
  if (refresh) params.set("refresh", "1");
  if (isLimitedRequest) params.set("limit", String(Number(limit) || 0));
  let url = "/api/media";
  const qs = params.toString();
  if (qs) url += `?${qs}`;

  const res = await fetch(url, { cache: "no-store", headers });

  if (res.status === 304) {
    const hadLimited = !!state.media?.limited;
    if (state.config && state.media && !hadLimited) {
      applyConfigToUI();
      const lastKind = gpGet(LS.lastActiveKind);
      if (lastKind && ["video", "audio", "image"].includes(lastKind)) {
        state.tab = lastKind;
      } else {
        state.tab = "video";
      }
      renderList();
      resumeLast();
      return;
    }
    return loadMedia(true, 0);
  }

  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);

  if (!isLimitedRequest) {
    const newETag = res.headers.get("ETag");
    if (newETag) {
      lsSet(LS.mediaETag, newETag);
    }
  }

  const data = await res.json();
  state.media = data;
  state.scanning = !!data.scanning;
  renderList();
  updateResumeButton();

  if (!refresh && !isLimitedRequest) {
    resumeLast();
  }
}

export async function boot() {
  if ('serviceWorker' in navigator) {
    // eslint-disable-next-line
    registerSW({ immediate: true });
  }

  initLang();
  initTheme();
  
  // Setup UI bindings
  bindUI();
  bindGlobalHotkeys();
  bindPinDialog();
  
  // Initial UI Update for Lang (since initLang only sets state)
  updateUIForLang();

  // Reset UI state
  hideAllMedia();
  const emptyEl = el("emptyEl");
  const openRawBtn = el("btnOpenRaw");
  const prevBtn = el("btnPrev");
  const nextBtn = el("btnNext");
  const previewSub = el("previewSub");
  
  if (emptyEl) emptyEl.style.display = "block";
  if (openRawBtn) openRawBtn.disabled = true;
  if (prevBtn) prevBtn.disabled = true;
  if (nextBtn) nextBtn.disabled = true;
  if (previewSub) previewSub.textContent = "";

  // Check if PIN is required
  const pinRequired = await checkPinRequired();
  if (pinRequired) {
    showPinDialog();
    return; // Stop boot process until PIN is verified
  }

  try {
    // Retry initial config/prefs as the server might still be starting
    await apiRetry(loadConfig).catch(e => console.warn("Load config failed", e));
    await apiRetry(loadPrefs).catch(e => console.warn("Load prefs failed", e));

    // Initial fast load (limited items)
    await loadMedia(false, 200).catch(() => { });

    // Call resume logic after we have at least partial media info
    // (Note: loadMedia already calls resumeLast, but we do it here explicitly if needed or if logic differed)
    // loadMedia handles it.

    // Full load in background
    setTimeout(async () => {
      await loadMedia(false).catch(() => { }); // Use non-refresh first to get what's in DB

      // If still scanning or empty, poll for a while to update the list incrementally
      let polls = 0;
      const poll = setInterval(async () => {
        polls++;
        if (polls > 10 || !state.scanning) {
          clearInterval(poll);
          return;
        }
        await loadMedia(false).catch(() => { });
        renderList();
      }, 2000);
    }, 50);
  } catch (e) {
    setMeta(t("meta_fail"));
    alert(String(e?.message || e));
  }
}
