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

/** Parent directory of a media item for folder-scoped playlists. Uses relPath
 *  (and shareLabel), never item.id — IDs are opaque AES-GCM, not paths. */
export function playlistFolderKey(item) {
  const rel = item?.relPath || item?.name || "";
  return `${item?.shareLabel || ""}\n${dirOfAbsPath(rel)}`;
}

export function dirOfAbsPath(p) {
  if (!p) return "";
  const s = String(p);
  const idx = Math.max(s.lastIndexOf("\\"), s.lastIndexOf("/"));
  return idx >= 0 ? s.slice(0, idx) : "";
}

export function streamUrl(id, start) {
  const ts = Date.now();
  let url = `/api/stream?id=${encodeURIComponent(id)}&ts=${ts}`;
  if (start && start > 0) {
    url += `&start=${start}`;
  }
  return url;
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
    if (e === ".mov" || e === ".qt") return "video/quicktime";
    if (e === ".mkv") return "video/x-matroska";
    if (e === ".avi") return "video/x-msvideo";
    if (e === ".wmv") return "video/x-ms-wmv";
    if (e === ".flv") return "video/x-flv";
    if (e === ".ts" || e === ".m2ts" || e === ".mts") return "video/mp2t";
    if (e === ".mpg" || e === ".mpeg") return "video/mpeg";
    if (e === ".3gp" || e === ".3g2") return "video/3gpp";
  }
  if (kind === "audio") {
    if (e === ".mp3") return "audio/mpeg";
    if (e === ".m4a" || e === ".f4a") return "audio/mp4";
    if (e === ".aac") return "audio/aac";
    if (e === ".wav" || e === ".wave") return "audio/wav";
    if (e === ".flac") return "audio/flac";
    if (e === ".ogg" || e === ".oga" || e === ".spx") return "audio/ogg";
    if (e === ".opus") return "audio/ogg; codecs=opus";
    if (e === ".wma") return "audio/x-ms-wma";
    if (e === ".ape") return "audio/ape";
    if (e === ".mka") return "audio/x-matroska";
    if (e === ".weba") return "audio/webm";
    if (e === ".mid" || e === ".midi") return "audio/midi";
  }
  return "";
}

export function canPlayMedia(kind, ext, name, mediaEl) {
  const e = (ext || "").toLowerCase();

  // 只要配置中允许转码，前端就放行，让后端去决定是否需要真正的转码
  if (kind === "video" && getCfg("playback.video.transcode", false)) {
    return true;
  }
  if (kind === "audio" && getCfg("playback.audio.transcode", false)) {
    return true;
  }

  // 对未知/未识别格式也允许尝试直接播放，让浏览器的错误处理机制去兜底
  const knownAudioExts = new Set([".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus", ".oga", ".spx", ".wma", ".ape", ".mka"]);
  const knownVideoExts = new Set([".mp4", ".m4v", ".webm", ".ogg", ".ogv", ".mov", ".mkv", ".avi", ".wmv", ".flv", ".ts", ".m2ts", ".mts", ".mpg", ".mpeg"]);

  if (kind === "audio") {
    // 对已知音频格式进行 canPlayType 检测；对未知格式允许尝试播放
    if (!knownAudioExts.has(e)) return true;
    const mime = mimeFor("audio", e);
    if (mime && mediaEl && typeof mediaEl.canPlayType === "function") {
      const res = mediaEl.canPlayType(mime);
      // Firefox 对某些格式返回空字符串，但可能仍支持容器内的解码；
      // 只要没有明确返回空字符串，就允许尝试。对明确不支持的再拦截。
      if (res === "") return false;
    }
    return true;
  }
  if (kind === "video") {
    if (!knownVideoExts.has(e)) return true;
    const mime = mimeFor("video", e);
    if (mime && mediaEl && typeof mediaEl.canPlayType === "function") {
      const res = mediaEl.canPlayType(mime);
      if (res === "") return false;
    }
    return true;
  }
  return true;
}
