import { state, el, lsSet, lsGet } from './state.js';
import { t } from './i18n.js';
import { getCfg } from './utils.js';

export async function apiRetry(fn, retries = 3, delay = 1000) {
  for (let i = 0; i < retries; i++) {
    try {
      return await fn();
    } catch (e) {
      if (i === retries - 1) throw e;
      await new Promise(r => setTimeout(r, delay));
    }
  }
}

export async function apiGet(url) {
  const res = await fetch(url, { cache: "no-store" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data?.error?.message || `${res.status} ${res.statusText}`);
  if (data?.error?.message) throw new Error(data.error.message);
  return data;
}

export async function apiPost(url, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data?.error?.message || `${res.status} ${res.statusText}`);
  if (data?.error?.message) throw new Error(data.error.message);
  return data;
}

export function logRemote(level, msg) {
  apiPost("/api/log", { level, msg }).catch(() => { });
}

const probeCache = new Map();

export async function probeItem(id) {
  if (!id) return null;
  if (probeCache.has(id)) return probeCache.get(id);
  try {
    const data = await apiGet(`/api/probe?id=${encodeURIComponent(id)}`);
    // Limit cache size to 100 items
    if (probeCache.size > 100) {
      const first = probeCache.keys().next().value;
      probeCache.delete(first);
    }
    probeCache.set(id, data);
    return data;
  } catch {
    return null;
  }
}

export function probeText(p) {
  if (!p) return "";
  const parts = [];
  if (p.container) parts.push(String(p.container).toUpperCase());
  if (p.video) parts.push(String(p.video));
  if (p.audio) parts.push(String(p.audio));
  return parts.length ? `${t("codec_info")}${parts.join(" / ")}` : "";
}

export function probeWarnText(p) {
  const a = String(p?.audio || "");
  if (!a) return "";
  if (a.includes("AC-3") || a.includes("E-AC-3") || a.includes("DTS") || a.includes("TrueHD") || a.includes("FLAC")) {
    return t("audio_warn", a);
  }
  return "";
}

export function mediaErrorText(err) {
  if (!err) return "";
  switch (err.code) {
    case 1: return t("err_aborted");
    case 2: return t("err_network");
    case 3: return t("err_decode");
    case 4: return t("err_src");
    default: return t("err_unknown");
  }
}

export function reportProgress(id, time) {
  if (!id) return;
  apiPost("/api/progress", { id, time }).catch(() => { });
}

export async function getProgress(id) {
  if (!id) return 0;
  try {
    const res = await apiGet(`/api/progress?id=${encodeURIComponent(id)}`);
    return Number(res.time || 0);
  } catch {
    return 0;
  }
}

// Prefs Logic
export async function loadPrefs() {
  try {
    const data = await apiGet("/api/prefs");
    state.prefs = data.prefs || {};
  } catch {
    state.prefs = {};
  }
}

export function gpGet(k) {
  const v = state.prefs?.[k];
  if (v !== undefined && v !== null) return v;
  return lsGet(k);
}

let gpBatchQueue = {};
let gpBatchTimer = 0;

export function gpSet(k, v) {
  state.prefs[k] = v;
  lsSet(k, v);
  gpBatchQueue[k] = v;
  if (gpBatchTimer) return;
  gpBatchTimer = setTimeout(async () => {
    const batch = { ...gpBatchQueue };
    gpBatchQueue = {};
    gpBatchTimer = 0;
    try {
      await apiPost("/api/prefs", { prefs: batch });
    } catch (e) {
      console.warn("Batch preference save failed", e);
    }
  }, 300); // 300ms batching windows
}

export function rememberEnabled(kind) {
  const override = gpGet(`msp.remember.${kind}`);
  if (override === "1") return true;
  if (override === "0") return false;
  if (kind === "audio") return !!getCfg("playback.audio.remember", true);
  if (kind === "video") return !!getCfg("playback.video.remember", true);
  if (kind === "image") return !!getCfg("playback.image.remember", true);
  return true;
}
