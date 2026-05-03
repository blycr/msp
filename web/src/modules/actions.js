import { registerSW } from 'virtual:pwa-register';
import { state, el, lsSet, LS, lsGet } from './state.js';
import { t, initLang } from './i18n.js';
import { apiGet, apiRetry, loadPrefs, gpGet } from './api.js';
import { updateResumeButton, hideAllMedia, bindGlobalHotkeys, resumeLast } from './player.js';
import { initTheme } from './theme.js';
import { bindPinDialog, checkPinRequired, showPinDialog } from './pin.js';
import { bus } from './eventbus.js';

export async function loadConfig() {
  try {
    const data = await apiGet("/api/config");
    state.config = data.config;
    const urls = (data.urls || []).slice(0, 3).join("  ");
    bus.emit('meta:update', urls ? t("meta_urls", urls) : t("meta_noip"));
    bus.emit('config:loaded');
  } catch (e) {
    console.error("Failed to load config:", e);
    bus.emit('meta:update', t("meta_fail"));
    state.config = {};
    bus.emit('config:loaded');
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

  const res = await fetch(url, { cache: "no-store", headers, credentials: "include" });

  if (res.status === 304) {
    const hadLimited = !!state.media?.limited;
    if (state.config && state.media && !hadLimited) {
      bus.emit('config:loaded');
      const lastKind = gpGet(LS.lastActiveKind);
      if (lastKind && ["video", "audio", "image"].includes(lastKind)) {
        state.tab = lastKind;
      } else {
        state.tab = "video";
      }
      bus.emit('media:loaded');
      bus.emit('media:resume');
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
  bus.emit('media:loaded');
  updateResumeButton();

  if (!refresh && !isLimitedRequest) {
    bus.emit('media:resume');
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
  bindGlobalHotkeys();
  bindPinDialog();
  bus.emit('boot:init');

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
        bus.emit('media:loaded');
      }, 2000);
    }, 50);
  } catch (e) {
    bus.emit('meta:update', t("meta_fail"));
    alert(String(e?.message || e));
  }
}