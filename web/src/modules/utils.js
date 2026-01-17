import { state, el } from './state.js';
import { t } from './i18n.js';

export function formatBytes(n) {
  if (!Number.isFinite(n)) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024;
    u++;
  }
  return `${v.toFixed(v >= 10 || u === 0 ? 0 : 1)} ${units[u]}`;
}

export function formatTime(ts) {
  if (!ts) return "";
  const d = new Date(ts * 1000);
  const locale = state.lang === "zh" ? "zh-CN" : "en-US";
  return d.toLocaleString(locale);
}

export function getCfg(path, fallback) {
  const parts = String(path || "").split(".");
  let cur = state.config;
  for (const p of parts) {
    if (!cur || typeof cur !== "object") return fallback;
    cur = cur[p];
  }
  return cur === undefined || cur === null ? fallback : cur;
}

export function base64UrlDecodeToString(b64url) {
  const s = String(b64url || "").replace(/-/g, "+").replace(/_/g, "/");
  const pad = s.length % 4 ? "=".repeat(4 - (s.length % 4)) : "";
  const bin = atob(s + pad);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder("utf-8").decode(bytes);
}

export function absPathOfItem(item) {
  try { return base64UrlDecodeToString(item?.id || ""); } catch { return ""; }
}

export function dirOfAbsPath(p) {
  if (!p) return "";
  const s = String(p);
  const idx = Math.max(s.lastIndexOf("\\"), s.lastIndexOf("/"));
  return idx >= 0 ? s.slice(0, idx) : "";
}

export function streamUrl(id) {
  const ts = Date.now();
  return `/api/stream?id=${encodeURIComponent(id)}&ts=${ts}`;
}

export function formatName(item) {
  if (!item || !item.name) return "";
  const name = item.name;
  const ext = item.ext || "";
  if (ext && name.toLowerCase().endsWith(ext.toLowerCase())) {
    return name.slice(0, -ext.length);
  }
  return name;
}

export function mimeFor(kind, ext) {
  const e = (ext || "").toLowerCase();
  if (kind === "video") {
    if (e === ".mp4" || e === ".m4v") return "video/mp4";
    if (e === ".webm") return "video/webm";
    if (e === ".ogg" || e === ".ogv") return "video/ogg";
    if (e === ".mov") return "video/quicktime";
    if (e === ".mkv") return "video/x-matroska";
    if (e === ".avi") return "video/x-msvideo";
  }
  if (kind === "audio") {
    if (e === ".mp3") return "audio/mpeg";
    if (e === ".m4a") return "audio/mp4";
    if (e === ".aac") return "audio/aac";
    if (e === ".wav") return "audio/wav";
    if (e === ".flac") return "audio/flac";
    if (e === ".ogg") return "audio/ogg";
    if (e === ".opus") return "audio/ogg; codecs=opus";
  }
  return "";
}

export function canPlayMedia(kind, ext, name, mediaEl) {
  const e = (ext || "").toLowerCase();
  if (kind === "audio") {
    const mime = mimeFor("audio", e);
    if (mime && mediaEl && typeof mediaEl.canPlayType === "function") {
      const res = mediaEl.canPlayType(mime);
      if (!res) return false;
    }
    return true;
  }
  if (kind === "video") {
    const mime = mimeFor("video", e);
    if (mime && mediaEl && typeof mediaEl.canPlayType === "function") {
      const res = mediaEl.canPlayType(mime);
      if (!res && e !== ".mkv" && e !== ".avi") return false;
    }
    return true;
  }
  return true;
}
