import './app.css';
import { registerSW } from 'virtual:pwa-register'

registerSW({ immediate: true })

const el = (id) => document.getElementById(id);

const I18N = {
  en: {
    title: "MSP Media Share",
    theme: "Switch Theme",
    settings: "Settings",
    refresh: "Refresh",
    tab_video: "Video",
    tab_audio: "Audio",
    tab_image: "Image",
    tab_other: "Other",
    search_ph: "Search filename (Pinyin/Regex)...",
    sort_name: "Name",
    sort_size: "Size",
    sort_date: "Date",
    sort_order: "Order",
    hint_noshare: "Unconfigured. Click 'Settings' to add shares.",
    preview_none: "No Selection",
    prev: "Prev",
    next: "Next",
    shuffle: "Shuffle",
    loop: "Loop",
    open_raw: "Open Raw",
    fit_mode: "Fit Mode: Adapt",
    empty_tip: "Select a file to preview",
    playlist: "Playlist",
    not_loaded: "Not Loaded",
    footer_text: "A simple media server for personal use.",
    dlg_title: "Share Settings",
    path_ph: "e.g. D:\\Media",
    label_ph: "Alias (Optional)",
    add: "Add",
    bl_title: "Blacklist Settings",
    bl_exts_ph: "Block Exts (.log; Regex /log$/)",
    bl_files_ph: "Block Files (thumb.db; Regex /^tmp_/)",
    bl_folders_ph: "Block Folders ($RECYCLE.BIN; Regex /^\\./)",
    bl_size_ph: "Block Size (>100MB, 10KB-1MB)",
    bl_hint: `Usage:<br>1. <strong>Regex</strong>: /pattern/<br>2. <strong>Size</strong>: >100MB, 10MB-1GB<br>3. <strong>Units</strong>: B, KB, MB, GB`,
    save_bl: "Save Blacklist",
    dlg_note: "Note: Browser cannot open the system folder picker due to security limits. Please input the path manually.",
    close: "Close",

    // JS Dynamic
    kind_video: "Video",
    kind_audio: "Audio",
    kind_image: "Image",
    kind_other: "Other",
    prev_video: "Prev Video",
    next_video: "Next Video",
    prev_image: "Prev Image",
    next_image: "Next Image",
    prev_audio: "Prev Audio",
    next_audio: "Next Audio",
    prev_item: "Prev Item",
    next_item: "Next Item",

    codec_info: " · Codec: ",
    audio_warn: " · Note: Audio is {0}, browser may not support.",
    err_aborted: "Aborted",
    err_network: "Network Error",
    err_decode: "Decode Failed",
    err_src: "Source Not Supported",
    err_unknown: "Unknown Error",

    meta_urls: "Available: {0}",
    meta_noip: "No LAN IP detected (127.0.0.1 available)",
    hint_stats: "Current: {0}, Total {1}",
    item_count: "{0} · {1} Items",
  },
  zh: {
    title: "MSP 媒体分享预览",
    theme: "切换主题",
    settings: "共享目录设置",
    refresh: "刷新",
    tab_video: "视频",
    tab_audio: "音频",
    tab_image: "图片",
    tab_other: "其他",
    search_ph: "搜索文件名 (支持拼音/正则/模糊)…",
    sort_name: "按名称排序",
    sort_size: "按大小排序",
    sort_date: "按时间排序",
    sort_order: "切换正序/倒序",
    hint_noshare: "未配置共享目录。点击右上角“共享目录设置”添加。",
    preview_none: "未选择",
    prev: "上一个",
    next: "下一个",
    shuffle: "随机",
    loop: "循环",
    open_raw: "在新标签打开",
    fit_mode: "填充模式：适配",
    empty_tip: "从左侧选择一个媒体文件进行预览",
    playlist: "播放列表",
    not_loaded: "未加载",
    footer_text: "一个适合个人使用的简易媒体服务器。",
    dlg_title: "共享目录设置",
    path_ph: "例如：D:\\Media 或 D:/Media（会自动兼容斜杠）",
    label_ph: "别名（可选）",
    add: "添加",
    bl_title: "文件黑名单设置",
    bl_exts_ph: "屏蔽扩展名 (如 .log, .txt; 支持正则 /log$/)",
    bl_files_ph: "屏蔽文件名 (如 thumb.db; 支持正则 /^tmp_/)",
    bl_folders_ph: "屏蔽文件夹 (如 $RECYCLE.BIN; 支持正则 /^\\./)",
    bl_size_ph: "大小屏蔽 (如: >100MB, 10KB-1MB, 500B)",
    bl_hint: `用法提示：<br>1. <strong>正则匹配</strong>：使用 <code>/</code> 包裹<br>2. <strong>大小范围</strong>：支持 <code>10MB-1GB</code>, <code>&gt;500MB</code><br>3. <strong>单位支持</strong>：B, KB, MB, GB, TB`,
    save_bl: "保存黑名单设置",
    dlg_note: "提示：由于浏览器安全限制，网页无法直接弹出系统文件夹选择器，请手动输入路径。",
    close: "关闭",

    kind_video: "视频",
    kind_audio: "音频",
    kind_image: "图片",
    kind_other: "其他",
    prev_video: "上一个视频",
    next_video: "下一个视频",
    prev_image: "上一张",
    next_image: "下一张",
    prev_audio: "上一首",
    next_audio: "下一首",
    prev_item: "上一个",
    next_item: "下一个",

    codec_info: " · 编码/容器：",
    audio_warn: " · 提示：音频为 {0}，浏览器常不支持",
    err_aborted: "播放被中止",
    err_network: "网络/读取失败",
    err_decode: "解码失败（常见于编码不支持）",
    err_src: "媒体源不支持",
    err_unknown: "未知错误",

    meta_urls: "可用地址：{0}",
    meta_noip: "未检测到局域网 IP（仍可用 127.0.0.1 访问）",
    hint_stats: "当前分类：{0}，共 {1} 个",
    item_count: "{0} · {1} 项",

    // New additions
    fit_cover: "填充模式：铺满",
    fit_contain: "填充模式：适配",
    err_audio_format: "该音频格式浏览器可能不支持（{0}）。请用“在新标签打开”。",
    err_video_format: "该视频格式浏览器可能不支持（{0}）。请用“在新标签打开”。",
    err_unsupported: "该文件类型暂不支持预览（可用“在新标签打开”下载/查看）。",
    shares_empty: "当前没有共享目录。",
    remove: "移除",
    msg_bl_saved: "黑名单已保存，刷新媒体库后生效。",
    err_audio_load: "音频加载/解码失败（{0}）。可能是浏览器不支持该编码，建议用“在新标签打开”下载后本地播放器播放。",
    err_video_load: "视频加载/解码失败（{0}，{1}）。同为 mp4/mkv 也可能因编码不同而无法播放。{2}建议用“在新标签打开”，或转码为 H.264/AAC（或仅转音频为 AAC）再播放。",
    err_img_load: "图片加载失败（{0}）。可用“在新标签打开”查看原文件。",
    meta_fail: "服务连接失败或初始化失败",
  }
};

const state = {
  lang: "en",
  config: null,
  media: null,
  tab: "video",
  q: "",
  current: null,
  currentMetaBase: "",
  plyr: null,
  lyrics: null,
  selectionToken: 0,
  playlist: {
    kind: null,
    items: [],
    index: -1,
    shuffle: false,
    loop: false,
  },
  // 列表分页
  listPageSize: 10,
  listPage: 1,
  // 播放列表分页
  plPageSize: 10,
  plPage: 1,
  sort: {
    field: "name",
    order: 1, // 1 for asc, -1 for desc
  },
};

const LS = {
  audioLastID: "msp.audio.lastId",
  audioLastTime: "msp.audio.lastTime",
  audioShuffle: "msp.audio.shuffle",
  audioLoop: "msp.audio.loop",
  videoLastID: "msp.video.lastId",
  videoLastTime: "msp.video.lastTime",
  imageLastID: "msp.image.lastId",
  lastActiveKind: "msp.lastActiveKind", // "audio" | "video" | "image"
  mediaETag: "msp.media.etag",
  theme: "msp.theme",
  lang: "msp.lang",
};

function t(key, ...args) {
  const dict = I18N[state.lang] || I18N.en;
  let val = dict[key] || I18N.en[key] || key;
  for (let i = 0; i < args.length; i++) {
    val = val.replace(`{${i}}`, args[i]);
  }
  return val;
}

function setLang(lang) {
  if (lang !== "en" && lang !== "zh") return;
  state.lang = lang;
  localStorage.setItem(LS.lang, lang);
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";

  // Update button text
  const btn = el("langBtn");
  if (btn) btn.textContent = lang === "en" ? "CN" : "EN"; // Toggle text

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

  // Update HTML content (like blacklist hint)
  const blHint = el("blHint");
  if (blHint) blHint.innerHTML = t("bl_hint");

  // Re-render dynamic content
  renderList();
  renderPlaylist();
  try {
    plAutoFit.last.itemH = 0;
    plAutoFit.last.pagerH = 0;
  } catch { }
  scheduleAutoFitPlaylistPageSize();
  updateNavLabels();


  // Update previewSub for current item
  if (state.current) {
    const item = state.current;
    state.currentMetaBase = `${item.shareLabel || ""} · ${(item.ext || "").toUpperCase()} · ${formatBytes(item.size)} · ${formatTime(item.modTime)}`;
    if (item.kind === "video") {
      probeItem(item.id).then(p => {
        el("previewSub").textContent = state.currentMetaBase + probeText(p) + probeWarnText(p);
      });
    } else {
      el("previewSub").textContent = state.currentMetaBase;
    }
  }

  // Update specific dynamic texts if needed (meta, etc)
  if (state.config) loadConfig();
}

function initLang() {
  const saved = localStorage.getItem(LS.lang);
  const lang = saved === "zh" ? "zh" : "en"; // Default en
  setLang(lang);

  const btn = el("langBtn");
  if (btn) {
    btn.addEventListener("click", () => {
      const next = state.lang === "en" ? "zh" : "en";
      setLang(next);
    });
  }
}

// SVG Icon helpers
function createSunIcon() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="12" cy="12" r="5"></circle>
    <line x1="12" y1="1" x2="12" y2="3"></line>
    <line x1="12" y1="21" x2="12" y2="23"></line>
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
    <line x1="1" y1="12" x2="3" y2="12"></line>
    <line x1="21" y1="12" x2="23" y2="12"></line>
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
  </svg>`;
}

function createMoonIcon() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
  </svg>`;
}

function createArrowDownIcon() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <line x1="12" y1="5" x2="12" y2="19"></line>
    <polyline points="19 12 12 19 5 12"></polyline>
  </svg>`;
}

function createArrowUpIcon() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <line x1="12" y1="19" x2="12" y2="5"></line>
    <polyline points="5 12 12 5 19 12"></polyline>
  </svg>`;
}

function initTheme() {
  const btn = el("themeBtn");
  if (!btn) return;

  const saved = localStorage.getItem(LS.theme);
  const systemDark = window.matchMedia("(prefers-color-scheme: dark)");

  const updateTheme = (isDark) => {
    document.documentElement.setAttribute("data-theme", isDark ? "dark" : "light");
    btn.innerHTML = isDark ? createMoonIcon() : createSunIcon();
  };

  const getAutoTheme = () => {
    const hour = new Date().getHours();
    const isNight = hour < 6 || hour >= 18;
    return isNight || systemDark.matches;
  };

  // Initial set
  if (saved === "dark") {
    updateTheme(true);
  } else if (saved === "light") {
    updateTheme(false);
  } else {
    updateTheme(getAutoTheme());
  }

  // Toggle handler
  btn.addEventListener("click", () => {
    const isDark = document.documentElement.getAttribute("data-theme") === "dark";
    const next = !isDark;
    const apply = () => {
      updateTheme(next);
      localStorage.setItem(LS.theme, next ? "dark" : "light");
    };
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduce) { apply(); return; }
    if (document.startViewTransition) {
      document.startViewTransition(apply);
    } else {
      document.documentElement.classList.add("theme-fade");
      apply();
      setTimeout(() => document.documentElement.classList.remove("theme-fade"), 300);
    }
  });

  // System preference listener (only if no manual override)
  systemDark.addEventListener("change", (e) => {
    if (!localStorage.getItem(LS.theme)) {
      const next = e.matches || (new Date().getHours() < 6 || new Date().getHours() >= 18);
      const apply = () => updateTheme(next);
      const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (reduce) { apply(); return; }
      if (document.startViewTransition) {
        document.startViewTransition(apply);
      } else {
        document.documentElement.classList.add("theme-fade");
        apply();
        setTimeout(() => document.documentElement.classList.remove("theme-fade"), 300);
      }
    }
  });
}

function setFitBtnVisible(visible) {
  const btn = el("btnToggleFit");
  if (!btn) return;
  btn.hidden = !visible;
  if (!visible) btn.disabled = true;
}

document.addEventListener("fullscreenchange", () => {
  const isFull = !!document.fullscreenElement;
  document.documentElement.style.overflow = isFull ? "hidden" : "";
  try {
    const el = document.fullscreenElement;
    console.log(el && (el.id || el.className || el.tagName));
  } catch { }
});

function formatBytes(n) {
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

function formatTime(ts) {
  if (!ts) return "";
  const d = new Date(ts * 1000);
  const locale = state.lang === "zh" ? "zh-CN" : "en-US";
  return d.toLocaleString(locale);
}

function formatName(item) {
  if (!item || !item.name) return "";
  const name = item.name;
  const ext = item.ext || "";
  if (ext && name.toLowerCase().endsWith(ext.toLowerCase())) {
    return name.slice(0, -ext.length);
  }
  return name;
}

function getCfg(path, fallback) {
  const parts = String(path || "").split(".");
  let cur = state.config;
  for (const p of parts) {
    if (!cur || typeof cur !== "object") return fallback;
    cur = cur[p];
  }
  return cur === undefined || cur === null ? fallback : cur;
}

function base64UrlDecodeToString(b64url) {
  const s = String(b64url || "").replace(/-/g, "+").replace(/_/g, "/");
  const pad = s.length % 4 ? "=".repeat(4 - (s.length % 4)) : "";
  const bin = atob(s + pad);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder("utf-8").decode(bytes);
}

function absPathOfItem(item) {
  try { return base64UrlDecodeToString(item?.id || ""); } catch { return ""; }
}

function dirOfAbsPath(p) {
  if (!p) return "";
  const s = String(p);
  const idx = Math.max(s.lastIndexOf("\\"), s.lastIndexOf("/"));
  return idx >= 0 ? s.slice(0, idx) : "";
}

function streamUrl(id) {
  return `/api/stream?id=${encodeURIComponent(id)}`;
}

function setMeta(text) {
  el("meta").textContent = text;
}

function showDlg(show) {
  el("dlgBackdrop").hidden = !show;
  el("dlg").hidden = !show;
}

async function apiGet(url) {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

const probeCache = new Map();

async function probeItem(id) {
  if (!id) return null;
  if (probeCache.has(id)) return probeCache.get(id);
  try {
    const data = await apiGet(`/api/probe?id=${encodeURIComponent(id)}`);
    probeCache.set(id, data);
    return data;
  } catch {
    return null;
  }
}

function probeText(p) {
  if (!p) return "";
  const parts = [];
  if (p.container) parts.push(String(p.container).toUpperCase());
  if (p.video) parts.push(String(p.video));
  if (p.audio) parts.push(String(p.audio));
  return parts.length ? `${t("codec_info")}${parts.join(" / ")}` : "";
}

function probeWarnText(p) {
  const a = String(p?.audio || "");
  if (!a) return "";
  if (a.includes("AC-3") || a.includes("E-AC-3") || a.includes("DTS") || a.includes("TrueHD") || a.includes("FLAC")) {
    return t("audio_warn", a);
  }
  return "";
}

function mediaErrorText(err) {
  if (!err) return "";
  switch (err.code) {
    case 1: return t("err_aborted");
    case 2: return t("err_network");
    case 3: return t("err_decode");
    case 4: return t("err_src");
    default: return t("err_unknown");
  }
}

async function apiPost(url, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data?.error?.message || `${res.status} ${res.statusText}`);
  if (data?.error?.message) throw new Error(data.error.message);
  return data;
}

async function loadConfig() {
  const data = await apiGet("/api/config");
  state.config = data.config;
  const urls = (data.urls || []).slice(0, 3).join("  ");
  setMeta(urls ? t("meta_urls", urls) : t("meta_noip"));
  applyConfigToUI();
  renderShares();

  const bl = state.config.blacklist || {};
  el("blExts").value = (bl.extensions || []).join(", ");
  el("blFiles").value = (bl.filenames || []).join(", ");
  el("blFolders").value = (bl.folders || []).join(", ");
  el("blMinSize").value = bl.sizeRule || "";
}

async function loadMedia(refresh, limit) {
  const isLimitedRequest = Number(limit || 0) > 0;

  const headers = {};
  if (!refresh && !isLimitedRequest && !state.media?.limited) {
    try {
      const etag = localStorage.getItem(LS.mediaETag);
      if (etag) headers["If-None-Match"] = etag;
    } catch { }
  }

  const params = new URLSearchParams();
  if (refresh) params.set("refresh", "1");
  if (isLimitedRequest) params.set("limit", String(Number(limit) || 0));
  let url = "/api/media";
  const qs = params.toString();
  if (qs) url += `?${qs}`;

  const res = await fetch(url, { cache: "no-store", headers });

  if (res.status === 304) {
    if (state.config) {
      applyConfigToUI();
      // Switch to the last active tab if possible, defaults to video
      const lastKind = localStorage.getItem(LS.lastActiveKind);
      if (lastKind && ["video", "audio", "image"].includes(lastKind)) {
        state.tab = lastKind;
      } else {
        state.tab = "video";
      }
      renderList();
      tryResumeLastMedia();
      return;
    }
  }

  async function tryResumeLastMedia() {
    const kind = localStorage.getItem(LS.lastActiveKind);
    if (!kind) {
      // Fallback: try resume audio if no kind set (migration)
      tryResumeAudioCompat();
      return;
    }

    if (kind === "audio" && getCfg("playback.audio.remember", true)) {
      resumeMedia("audio", LS.audioLastID, LS.audioLastTime, "audioEl");
    } else if (kind === "video" && getCfg("playback.video.remember", true)) {
      resumeMedia("video", LS.videoLastID, LS.videoLastTime, "videoEl");
    } else if (kind === "image" && getCfg("playback.image.remember", true)) {
      const lastID = localStorage.getItem(LS.imageLastID);
      if (lastID) resumeMedia("image", lastID, null, "imgEl");
    }
  }

  function tryResumeAudioCompat() {
    if (!getCfg("playback.audio.remember", true)) return;
    const lastID = localStorage.getItem(LS.audioLastID);
    if (lastID) resumeMedia("audio", LS.audioLastID, LS.audioLastTime, "audioEl");
  }

  function resumeMedia(kind, idKey, timeKey, elId) {
    if (!state.media) return;
    let pool = [];
    if (kind === "audio") pool = state.media.audios || [];
    if (kind === "video") pool = state.media.videos || [];
    if (kind === "image") pool = state.media.images || [];
    if (!pool.length) return;

    const id = idKey.startsWith("msp.") ? (localStorage.getItem(idKey) || "") : idKey;
    if (!id) return;

    const item = pool.find(x => x.id === id);
    if (!item) return;

    // Setup playlist
    if (getCfg("features.playlist", true)) {
      let pl = { items: [], index: -1 };
      if (kind === "audio") pl = buildAudioPlaylist(item);
      if (kind === "video") pl = buildVideoPlaylist(item);
      if (kind === "image") pl = buildImagePlaylist(item);
      setPlaylist(kind, pl.items, pl.index);
      playItem(item, { fromPlaylist: true, autoplay: false, resume: true });
    } else {
      playItem(item, { autoplay: false, resume: true });
    }

    if (!timeKey) return; // Image doesn't need seek

    const timeVal = Number(localStorage.getItem(timeKey) || "0") || 0;
    if (timeVal <= 0) return;

    const mediaEl = el(elId);
    if (!mediaEl) return;

    const seek = () => {
      try { mediaEl.currentTime = timeVal; } catch { }
    };
    if (mediaEl.readyState >= 1) {
      queueMicrotask(seek);
    } else {
      const onLoaded = () => {
        seek();
        mediaEl.removeEventListener("loadedmetadata", onLoaded);
      };
      mediaEl.addEventListener("loadedmetadata", onLoaded);
    }
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);

  if (!isLimitedRequest) {
    const newETag = res.headers.get("ETag");
    if (newETag) {
      try { localStorage.setItem(LS.mediaETag, newETag); } catch { }
    }
  }

  const data = await res.json();
  state.media = data;
  renderList();
}

function applyConfigToUI() {
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
  try {
    const saved = localStorage.getItem(LS.audioShuffle);
    if (saved === "1") shuffle = true;
    else if (saved === "0") shuffle = false;
    else shuffle = !!getCfg("playback.audio.shuffle", false);
  } catch {
    shuffle = !!getCfg("playback.audio.shuffle", false);
  }
  state.playlist.shuffle = shuffle;
  const t = el("toggleShuffle");
  if (t) t.checked = shuffle;

  let loop = false;
  try {
    const saved = localStorage.getItem(LS.audioLoop);
    loop = saved === "1";
  } catch { }
  state.playlist.loop = loop;
  const tl = el("toggleLoop");
  if (tl) tl.checked = loop;

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const x of tabs) x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
}


function currentList() {
  if (!state.media) return [];
  switch (state.tab) {
    case "video": return state.media.videos || [];
    case "audio": return state.media.audios || [];
    case "image": return state.media.images || [];
    default: return state.media.others || [];
  }
}

function navLabelsForKind(kind) {
  if (kind === "video") return { prev: t("prev_video"), next: t("next_video") };
  if (kind === "image") return { prev: t("prev_image"), next: t("next_image") };
  if (kind === "audio") return { prev: t("prev_audio"), next: t("next_audio") };
  return { prev: t("prev_item"), next: t("next_item") };
}

function updateNavLabels() {
  const kind = state.current?.kind || state.playlist.kind || "";
  const { prev, next } = navLabelsForKind(kind);
  const prevBtn = el("btnPrev");
  const nextBtn = el("btnNext");
  if (prevBtn) prevBtn.textContent = prev;
  if (nextBtn) nextBtn.textContent = next;
}

function getSortVal(item, field) {
  if (field === "size") return item.size || 0;
  if (field === "date") return item.modTime || 0;
  return String(item.name || "").toLowerCase();
}

function sortFiles(list) {
  const field = state.sort?.field || "name";
  const order = state.sort?.order || 1;
  return list.sort((a, b) => {
    const va = getSortVal(a, field);
    const vb = getSortVal(b, field);
    if (va < vb) return -1 * order;
    if (va > vb) return 1 * order;
    return 0;
  });
}

function filterFiles(list) {
  const q = (state.q || "").trim();
  if (!q) return list;

  // Regex search
  if (q.startsWith("/") && q.length > 2) {
    // Check if it ends with / or has flags
    const match = q.match(/^\/(.+)\/([a-z]*)$/);
    if (match) {
      try {
        const re = new RegExp(match[1], match[2] || "i");
        return list.filter(x => re.test(x.name));
      } catch { }
    }
  }

  // Pinyin/Fuzzy search
  const { pinyinPro } = window;
  if (pinyinPro) {
    return list.filter(x => {
      const name = x.name || "";
      // Check pinyin match
      const m = pinyinPro.match(name, q);
      if (m) return true;
      // Fallback to standard include
      return name.toLowerCase().includes(q.toLowerCase()) || (x.shareLabel || "").toLowerCase().includes(q.toLowerCase());
    });
  }

  // Fallback simple search
  const lower = q.toLowerCase();
  return list.filter(x => (x.name || "").toLowerCase().includes(lower) || (x.shareLabel || "").toLowerCase().includes(lower));
}

function renderList() {
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

function setPlaylist(kind, items, index) {
  state.playlist.kind = kind;
  state.playlist.items = Array.isArray(items) ? items : [];
  state.playlist.index = Number.isFinite(index) ? index : -1;
  renderPlaylist();
  scheduleAutoFitPlaylistPageSize();
  updateNavButtons();
  updateNavLabels();
}

const plAutoFit = {
  raf: 0,
  inUpdate: false,
  last: { boxH: 0, boxW: 0, itemH: 0, pagerH: 0 },
  ro: null,
};

function scheduleAutoFitPlaylistPageSize() {
  if (plAutoFit.raf) return;
  plAutoFit.raf = requestAnimationFrame(() => {
    plAutoFit.raf = 0;
    autoFitPlaylistPageSize();
  });
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

function autoFitPlaylistPageSize() {
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
  target = Math.max(1, Math.min(200, target));

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

function renderPlaylist() {
  const box = el("plList");
  const meta = el("plMeta");
  box.innerHTML = "";

  const items = state.playlist.items || [];
  if (!items.length) {
    meta.textContent = t("not_loaded");
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
    row.className = "plitem" + (i === state.playlist.index ? " plitem--active" : "");
    row.addEventListener("click", () => playAtIndex(i, true, true));

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

function updateNavButtons() {
  const prev = el("btnPrev");
  const next = el("btnNext");
  const items = state.playlist.items || [];
  const idx = state.playlist.index;
  prev.disabled = !(items.length && idx > 0);
  next.disabled = !(items.length && idx >= 0 && idx < items.length - 1);
  updateNavLabels();
}

function playAtIndex(i, autoplay, user) {
  const items = state.playlist.items || [];
  if (!items.length) return;
  const idx = Math.max(0, Math.min(items.length - 1, i));
  state.playlist.index = idx;
  renderPlaylist();
  updateNavButtons();
  playItem(items[idx], { fromPlaylist: true, autoplay: !!autoplay, user: !!user });
}

function buildVideoPlaylist(item) {
  const scope = getCfg("playback.video.scope", "folder");
  const all = state.media?.videos || [];
  if (!all.length) return { items: [], index: -1 };

  if (scope !== "folder") {
    const index = all.findIndex(x => x.id === item.id);
    return { items: all, index };
  }

  const p = absPathOfItem(item);
  const dir = dirOfAbsPath(p);
  // Robust check: if dir is empty, ensure we match others with empty dir
  const items = all.filter(x => dirOfAbsPath(absPathOfItem(x)) === dir);
  items.sort((a, b) => String(a.name || "").localeCompare(String(b.name || ""), "zh"));
  let index = items.findIndex(x => x.id === item.id);
  if (index < 0 && items.length > 0) {
    // Fallback: try matching by name/path if ID lookup fails
    index = items.findIndex(x => absPathOfItem(x) === p);
  }
  return { items, index };
}

function buildAudioPlaylist(item) {
  const scope = getCfg("playback.audio.scope", "all");
  const all = state.media?.audios || [];
  if (!all.length) return { items: [], index: -1 };

  let items = all;
  if (scope === "share") {
    items = all.filter(x => x.shareLabel === item.shareLabel);
  } else if (scope === "folder") {
    const dir = dirOfAbsPath(absPathOfItem(item));
    items = all.filter(x => dirOfAbsPath(absPathOfItem(x)) === dir);
  }

  const shuffle = !!state.playlist.shuffle;
  if (shuffle) {
    // Shuffle all items randomly
    for (let i = items.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [items[i], items[j]] = [items[j], items[i]];
    }
    // Find the current item in the shuffled list
    const index = items.findIndex(x => x.id === item.id);
    return { items, index };
  }

  const index = items.findIndex(x => x.id === item.id);
  return { items, index };
}

function buildImagePlaylist(item) {
  const scope = getCfg("playback.image.scope", "folder");
  const all = state.media?.images || [];
  if (!all.length) return { items: [], index: -1 };

  if (scope !== "folder") {
    const index = all.findIndex(x => x.id === item.id);
    return { items: all, index };
  }

  const dir = dirOfAbsPath(absPathOfItem(item));
  const items = all.filter(x => dirOfAbsPath(absPathOfItem(x)) === dir);
  items.sort((a, b) => String(a.name || "").localeCompare(String(b.name || ""), "zh"));
  const index = items.findIndex(x => x.id === item.id);
  return { items, index };
}

function destroyPlyr() {
  if (state.plyr) {
    try { state.plyr.destroy(); } catch { }
    state.plyr = null;
  }
}

function hideAllMedia() {
  destroyPlyr();
  const box = el("playerBox");
  if (box) {
    const plyrs = Array.from(box.querySelectorAll(".plyr"));
    for (const p of plyrs) p.style.display = "none";
  }
  resetMediaEl(el("videoEl"));
  resetMediaEl(el("audioEl"));
  try { el("imgEl").removeAttribute("src"); } catch { }
  try { el("audioCover").removeAttribute("src"); } catch { }
  el("videoEl").style.display = "none";
  el("audioEl").style.display = "none";
  el("audioMeta").style.display = "none";
  el("imgEl").style.display = "none";
  el("emptyEl").style.display = "none";
}

function showPreviewError(text) {
  destroyPlyr();
  el("videoEl").style.display = "none";
  el("audioEl").style.display = "none";
  el("audioMeta").style.display = "none";
  el("imgEl").style.display = "none";
  el("emptyEl").textContent = text;
  el("emptyEl").style.display = "block";
}

function resetMediaEl(mediaEl) {
  if (!mediaEl) return;
  try { mediaEl.pause(); } catch { }
  try { mediaEl.currentTime = 0; } catch { }
  try { mediaEl.srcObject = null; } catch { }
  try { mediaEl.removeAttribute("src"); } catch { }
  try {
    const sources = Array.from(mediaEl.querySelectorAll("source"));
    for (const s of sources) s.remove();
  } catch { }
  try { mediaEl.load(); } catch { }
}

function mimeFor(kind, ext) {
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

function canPlayMedia(kind, ext, name, mediaEl) {
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

let lastMediaEndedAt = 0;
function onMediaEnded() {
  const now = Date.now();
  if (now - lastMediaEndedAt < 500) return;
  lastMediaEndedAt = now;

  if (!state.current) return;
  const k = state.current.kind;
  if (k !== "audio" && k !== "video") return;
  if (state.playlist.kind !== k) return;

  if (state.playlist.index < 0) return;
  if (state.playlist.index >= (state.playlist.items?.length || 0) - 1) {
    if (state.playlist.loop) playAtIndex(0, true);
    return;
  }
  playAtIndex(state.playlist.index + 1, true);
}

function applyPlyr(element) {
  destroyPlyr();

  const isTouch = (() => {
    try {
      if (window.matchMedia && window.matchMedia("(pointer: coarse)").matches) return true;
      if (window.matchMedia && window.matchMedia("(max-width: 980px)").matches) return true;
    } catch { }
    return false;
  })();

  if (isTouch) {
    try { element.controls = true; } catch { }
    try {
      if (String(element?.tagName || "").toUpperCase() === "VIDEO") element.playsInline = true;
    } catch { }
    try {
      element.removeEventListener("ended", onMediaEnded);
      element.addEventListener("ended", onMediaEnded);
    } catch { }
    try {
      const wrap = element.closest?.(".plyr");
      if (wrap) wrap.style.display = "block";
    } catch { }
    return;
  }

  const features = state.config?.features || {};
  const opts = {};

  if (features.speed) {
    opts.speed = { selected: 1, options: Array.isArray(features.speedOptions) && features.speedOptions.length ? features.speedOptions : [0.5, 0.75, 1, 1.25, 1.5, 2] };
  }

  if (features.captions && String(element?.tagName || "").toUpperCase() === "VIDEO") {
    opts.captions = { active: true, update: true, language: "auto" };
  }

  opts.fullscreen = { enabled: true, fallback: true };
  state.plyr = new Plyr(element, opts);
  state.plyr.on("ended", onMediaEnded);
  try {
    const wrap = element.closest?.(".plyr");
    if (wrap) wrap.style.display = "block";
  } catch { }
  try {
    if (String(element?.tagName || "").toUpperCase() === "VIDEO") {
      state.plyr.on("enterfullscreen", () => {
        try { element.dataset.fit = "cover"; } catch { }
        try { console.log(document.fullscreenElement); } catch { }
        try {
          const fitBtn = el("btnToggleFit");
          fitBtn.textContent = t("fit_cover");
        } catch { }
      });
      state.plyr.on("exitfullscreen", () => {
        try { element.dataset.fit = "contain"; } catch { }
        try {
          const fitBtn = el("btnToggleFit");
          fitBtn.textContent = t("fit_contain");
        } catch { }
      });
    }
  } catch { }
  window.plyrPlayer = state.plyr;
  window.callPlyr = (method, ...args) => {
    if (!state.plyr) throw new Error("Plyr 未初始化");
    const fn = state.plyr[method];
    if (typeof fn !== "function") throw new Error("不支持的 Plyr 方法: " + method);
    return fn.apply(state.plyr, args);
  };
}

function setTracks(videoEl, subtitles) {
  const tracks = Array.from(videoEl.querySelectorAll("track"));
  for (const t of tracks) t.remove();

  if (!Array.isArray(subtitles) || subtitles.length === 0) return;

  const features = state.config?.features || {};
  if (!features.captions) return;

  for (const s of subtitles) {
    const tr = document.createElement("track");
    tr.kind = "subtitles";
    tr.label = s.label || "字幕";
    tr.srclang = s.lang || "zh";
    tr.src = s.src || streamUrl(s.id);
    if (s.default) tr.default = true;
    videoEl.appendChild(tr);
  }

  queueMicrotask(() => {
    try {
      const tt = videoEl.textTracks;
      if (!tt || tt.length === 0) return;
      for (let i = 0; i < tt.length; i++) tt[i].mode = "disabled";
      tt[0].mode = "showing";
    } catch { }
  });
}

function resetLyrics() {
  state.lyrics = null;
  el("lyrics").innerHTML = "";
}

function parseLrc(text) {
  const s = String(text || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const out = [];
  for (const line of s.split("\n")) {
    const matches = [...line.matchAll(/\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g)];
    if (matches.length === 0) continue;
    const content = line.replace(/\[[^\]]+\]/g, "").trim();
    for (const m of matches) {
      const mm = Number(m[1] || 0);
      const ss = Number(m[2] || 0);
      const frac = m[3] ? Number(String(m[3]).padEnd(3, "0")) : 0;
      const t = mm * 60 + ss + frac / 1000;
      if (Number.isFinite(t)) out.push({ t, text: content });
    }
  }
  out.sort((a, b) => a.t - b.t);
  return out;
}

function renderLyrics(lines) {
  const box = el("lyrics");
  box.innerHTML = "";
  if (!Array.isArray(lines) || lines.length === 0) return;
  const frag = document.createDocumentFragment();
  for (const ln of lines) {
    const div = document.createElement("div");
    div.className = "ly";
    div.dataset.t = String(ln.t);
    div.textContent = ln.text || "";
    frag.appendChild(div);
  }
  box.appendChild(frag);
}

function updateLyricsByTime(t, force) {
  const lines = state.lyrics?.lines || [];
  if (lines.length === 0) return;
  const box = el("lyrics");
  const nodes = Array.from(box.querySelectorAll(".ly"));
  if (nodes.length === 0) return;

  let idx = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].t <= t + 0.05) idx = i;
    else break;
  }
  if (!force && state.lyrics?.activeIndex === idx) return;
  state.lyrics.activeIndex = idx;

  for (let i = 0; i < nodes.length; i++) {
    nodes[i].classList.toggle("ly--active", i === idx);
  }

  const active = nodes[idx];
  if (active) {
    const top = active.offsetTop - box.clientHeight * 0.35;
    box.scrollTo({ top: Math.max(0, top), behavior: "smooth" });
  }
}

function playItem(item, opts) {
  const options = opts || {};
  if (!item) return;

  const prevKind = state.current?.kind;
  const token = ++state.selectionToken;
  state.current = item;
  updateNavLabels();

  setFitBtnVisible(state.tab === "video" && item.kind === "video");

  el("previewTitle").textContent = formatName(item);
  state.currentMetaBase = `${item.shareLabel || ""} · ${(item.ext || "").toUpperCase()} · ${formatBytes(item.size)} · ${formatTime(item.modTime)}`;
  el("previewSub").textContent = state.currentMetaBase;

  if (item.kind === "video") {
    probeItem(item.id).then((p) => {
      if (token !== state.selectionToken) return;
      if (!state.current || state.current.id !== item.id) return;
      el("previewSub").textContent = state.currentMetaBase + probeText(p) + probeWarnText(p);
    }).catch(() => { });
  }

  const openBtn = el("btnOpenRaw");
  openBtn.disabled = false;
  openBtn.onclick = () => {
    try { state.plyr?.pause?.(); } catch { }
    try { el("videoEl")?.pause?.(); } catch { }
    try { el("audioEl")?.pause?.(); } catch { }
    if (item.kind === "video" && Array.isArray(item.subtitles) && item.subtitles.length > 0) {
      const base = String(window.location.origin || "");
      const toAbs = (u) => {
        if (!u) return u;
        return u.startsWith("/") ? (base + u) : u;
      };
      const src = toAbs(streamUrl(item.id));
      const tr = (item.subtitles || []).map(s => {
        const label = s.label || "字幕";
        const lang = s.lang || "zh";
        const tsrc = toAbs(s.src || streamUrl(s.id));
        const def = s.default ? " default" : "";
        return `<track kind="subtitles" label="${label}" srclang="${lang}" src="${tsrc}"${def}>`;
      }).join("");
      const html =
        `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
        `<title>${formatName(item)}</title>` +
        `<style>html,body{height:100%;margin:0;background:#000}body{display:flex;align-items:center;justify-content:center}` +
        `video{max-width:100%;max-height:100vh;background:#000}</style></head>` +
        `<body><video controls preload="metadata" src="${src}">${tr}</video></body></html>`;
      const blob = new Blob([html], { type: "text/html;charset=utf-8" });
      const url = URL.createObjectURL(blob);
      window.open(url, "_blank", "noopener,noreferrer");
      return;
    }
    window.open(streamUrl(item.id), "_blank", "noopener,noreferrer");
  };

  const shuffleWrap = el("shuffleWrap");
  shuffleWrap.hidden = !getCfg("features.playlist", true) || item.kind !== "audio";

  const isVideoSwitch = prevKind === "video" && item.kind === "video";
  if (!isVideoSwitch) {
    hideAllMedia();
  }
  resetLyrics();

  if (options.user && window.matchMedia && window.matchMedia("(max-width: 980px)").matches) {
    try {
      document.querySelector(".stage")?.scrollIntoView({ behavior: "smooth", block: "start" });
    } catch { }
  }

  if (options.user && getCfg("features.playlist", true)) {
    if (item.kind === "video" && getCfg("playback.video.enabled", true)) {
      const pl = buildVideoPlaylist(item);
      setPlaylist("video", pl.items, pl.index);
    } else if (item.kind === "audio" && getCfg("playback.audio.enabled", true)) {
      const pl = buildAudioPlaylist(item);
      setPlaylist("audio", pl.items, pl.index);
    } else if (item.kind === "image" && getCfg("playback.image.enabled", true)) {
      const pl = buildImagePlaylist(item);
      setPlaylist("image", pl.items, pl.index);
    }
  }

  if (options.fromPlaylist) {
    state.playlist.kind = item.kind;
  }

  if (item.kind === "image") {
    const img = el("imgEl");
    img.src = streamUrl(item.id);
    img.style.opacity = "0";
    img.style.display = "block";
    requestAnimationFrame(() => {
      img.style.transition = "opacity 0.25s ease";
      img.style.opacity = "1";
    });
    if (options.autoplay) {
      try { img.decode?.(); } catch { }
    }
    if (getCfg("playback.image.remember", true)) {
      saveProgress("image", item.id);
    }
    return;
  }

  if (item.kind === "audio") {
    const audio = el("audioEl");
    if (!canPlayMedia("audio", item.ext, item.name, audio)) {
      showPreviewError(t("err_audio_format", item.ext || ""));
      return;
    }
    resetMediaEl(audio);
    audio.src = streamUrl(item.id);
    audio.style.opacity = "0";
    audio.style.display = "block";
    requestAnimationFrame(() => {
      audio.style.transition = "opacity 0.25s ease";
      audio.style.opacity = "1";
    });

    // Ensure event listener is attached even if DOM or Plyr changes
    audio.removeEventListener("ended", onMediaEnded);
    audio.addEventListener("ended", onMediaEnded);

    applyPlyr(audio);
    try { audio.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", () => state.plyr.play().catch(() => { }));
      } else {
        audio.play().catch(() => { });
      }
    }

    // Re-bind lyrics sync events to ensure they work for every song
    const onTimeUpdate = (ev) => {
      if (!state.current || state.current.kind !== "audio") return;
      if (!state.lyrics) return;
      updateLyricsByTime(audio.currentTime || 0, ev.type === "seeked");
    };
    audio.ontimeupdate = onTimeUpdate;
    audio.onseeked = onTimeUpdate;

    const meta = el("audioMeta");
    const cover = el("audioCover");
    cover.removeAttribute("src");
    if (item.coverId) {
      cover.src = streamUrl(item.coverId);
    }
    meta.style.opacity = "0";
    meta.style.display = "flex";
    requestAnimationFrame(() => {
      meta.style.transition = "opacity 0.25s ease";
      meta.style.opacity = "1";
    });

    if (getCfg("playback.audio.remember", true)) {
      if (options.user && !options.resume) {
        saveProgress("audio", item.id, 0);
      }
    }

    if (item.lyricsId) {
      fetch(streamUrl(item.lyricsId))
        .then(r => r.ok ? r.text() : "")
        .then(txt => {
          if (token !== state.selectionToken) return;
          const lines = parseLrc(txt);
          state.lyrics = { lines, activeIndex: -1 };
          renderLyrics(lines);
          // Use a loop to check time update more aggressively for lyrics
          requestAnimationFrame(() => updateLyricsByTime(audio.currentTime || 0, true));
        })
        .catch(() => { });
    }

    return;
  }

  if (item.kind === "video") {
    const video = el("videoEl");
    if (!canPlayMedia("video", item.ext, item.name, video)) {
      showPreviewError(`该视频格式浏览器可能不支持（${item.ext || ""}）。请用“在新标签打开”。`);
      return;
    }

    if (isVideoSwitch) {
      if (state.plyr) {
        state.plyr.source = {
          type: "video",
          title: item.name || "",
          sources: [{ src: streamUrl(item.id), type: mimeFor("video", item.ext) }],
          tracks: (item.subtitles || []).map(s => ({
            kind: "captions",
            label: s.label || "字幕",
            srclang: s.lang || "zh",
            src: s.src || streamUrl(s.id),
            default: !!s.default
          })),
          poster: item.coverId ? streamUrl(item.coverId) : undefined
        };
        if (options.autoplay) {
          state.plyr.once("ready", () => state.plyr.play().catch(() => { }));
        }
        // Ensure fit button is visible/active
        try {
          const fitBtn = el("btnToggleFit");
          fitBtn.hidden = false;
          fitBtn.disabled = false;
        } catch { }
        return;
      } else {
        // Raw video switch (touch devices mostly)
        video.src = streamUrl(item.id);
        setTracks(video, item.subtitles || []);
        try { video.load(); } catch { }
        if (options.autoplay) video.play().catch(() => { });
        return;
      }
    }

    resetMediaEl(video);
    video.src = streamUrl(item.id);
    setTracks(video, item.subtitles || []);
    video.style.display = "block";
    try {
      const fitBtn = el("btnToggleFit");
      fitBtn.hidden = false;
      fitBtn.disabled = false;
      const fit = video.dataset.fit || "contain";
      fitBtn.textContent = fit === "cover" ? t("fit_cover") : t("fit_contain");
    } catch { }
    applyPlyr(video);
    try { video.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", () => state.plyr.play().catch(() => { }));
      } else {
        video.play().catch(() => { });
      }
    }
    return;
  }

  el("emptyEl").textContent = t("err_unsupported");
  el("emptyEl").style.display = "block";
}

function renderShares() {
  const list = el("shareList");
  list.innerHTML = "";

  const shares = state.config?.shares || [];
  if (shares.length === 0) {
    const empty = document.createElement("div");
    empty.className = "small";
    empty.textContent = "当前没有共享目录。";
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
        await loadMedia(false);
      } catch (e) {
        alert(String(e?.message || e));
      }
    });

    row.appendChild(main);
    row.appendChild(btn);
    list.appendChild(row);
  }
}

function bindUI() {
  el("btnSettings").addEventListener("click", () => showDlg(true));
  el("btnCloseDlg").addEventListener("click", () => showDlg(false));
  el("dlgBackdrop").addEventListener("click", () => showDlg(false));

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
      await loadMedia(false);
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
      alert("黑名单已保存，刷新媒体库后生效。");
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

  // Initialize sort order button icon
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
          fitBtn.textContent = fit === "cover" ? "填充模式：铺满" : "填充模式：适配";
        } catch { }
      }
    });
  }

  hideAllMedia();
  el("emptyEl").style.display = "block";
  el("btnOpenRaw").disabled = true;
  el("btnPrev").disabled = true;
  el("btnNext").disabled = true;
  el("previewSub").textContent = "";
  setFitBtnVisible(false);

  el("btnPrev").addEventListener("click", () => {
    if (state.playlist.index > 0) playAtIndex(state.playlist.index - 1, true, true);
  });
  el("btnNext").addEventListener("click", () => {
    if (state.playlist.items?.length && state.playlist.index < state.playlist.items.length - 1) playAtIndex(state.playlist.index + 1, true, true);
  });

  el("toggleShuffle").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.shuffle = on;
    try { localStorage.setItem(LS.audioShuffle, on ? "1" : "0"); } catch { }
    if (state.current?.kind === "audio" && getCfg("playback.audio.enabled", true)) {
      const pl = buildAudioPlaylist(state.current);
      setPlaylist("audio", pl.items, pl.index);
    }
  });

  el("toggleLoop").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.loop = on;
    try { localStorage.setItem(LS.audioLoop, on ? "1" : "0"); } catch { }
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
    });
  } catch { }

  const audio = el("audioEl");

  let lastSaveAt = 0;
  function saveProgress(kind, id, t) {
    try { localStorage.setItem(LS.lastActiveKind, kind); } catch { }
    if (kind === "audio") {
      try { localStorage.setItem(LS.audioLastID, id); } catch { }
      if (t !== undefined) try { localStorage.setItem(LS.audioLastTime, String(t)); } catch { }
    } else if (kind === "video") {
      try { localStorage.setItem(LS.videoLastID, id); } catch { }
      if (t !== undefined) try { localStorage.setItem(LS.videoLastTime, String(t)); } catch { }
    } else if (kind === "image") {
      try { localStorage.setItem(LS.imageLastID, id); } catch { }
    }
  }

  audio.addEventListener("timeupdate", () => {
    if (!state.current || state.current.kind !== "audio") return;
    if (!getCfg("playback.audio.remember", true)) return;
    const now = Date.now();
    if (now - lastSaveAt < 1500) return;
    lastSaveAt = now;
    saveProgress("audio", state.current.id, Math.max(0, audio.currentTime || 0));
  });

  const video = el("videoEl");
  video.addEventListener("timeupdate", () => {
    if (!state.current || state.current.kind !== "video") return;
    if (!getCfg("playback.video.remember", true)) return;
    const now = Date.now();
    if (now - lastSaveAt < 1500) return;
    lastSaveAt = now;
    saveProgress("video", state.current.id, Math.max(0, video.currentTime || 0));
  });
  const img = el("imgEl");

  try {
    if (plAutoFit.ro) plAutoFit.ro.disconnect();
    if ("ResizeObserver" in window) {
      plAutoFit.ro = new ResizeObserver(() => scheduleAutoFitPlaylistPageSize());
      plAutoFit.ro.observe(el("plList"));
    }
    window.addEventListener("resize", () => scheduleAutoFitPlaylistPageSize());
    document.addEventListener("fullscreenchange", () => scheduleAutoFitPlaylistPageSize());
  } catch { }

  audio.addEventListener("error", () => {
    const ext = state.current?.ext || "";
    showPreviewError(t("err_audio_load", ext));
  });
  video.addEventListener("error", () => {
    if (!state.current || state.current.kind !== "video") return;
    const item = state.current;
    const token = state.selectionToken;
    const ext = item.ext || "";
    const err = mediaErrorText(video.error);
    probeItem(item.id).then((p) => {
      if (token !== state.selectionToken) return;
      if (!state.current || state.current.id !== item.id) return;
      const hint = probeText(p) + probeWarnText(p);
      showPreviewError(t("err_video_load", ext, err, hint));
    }).catch(() => {
      showPreviewError(t("err_video_load", ext, err, ""));
    });
  });
  img.addEventListener("error", () => {
    const ext = state.current?.ext || "";
    showPreviewError(t("err_img_load", ext));
  });

  video.addEventListener("ended", () => {
    if (!state.current || state.current.kind !== "video") return;
    if (state.playlist.kind !== "video") return;
    if (state.playlist.index < 0) return;
    if (state.playlist.index >= (state.playlist.items?.length || 0) - 1) {
      if (state.playlist.loop) playAtIndex(0, true);
      return;
    }
    playAtIndex(state.playlist.index + 1, true);
  });
}

async function boot() {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(err => console.log('SW fail:', err));
  }

  initLang();
  initTheme();
  bindUI();
  try {
    await loadConfig();
    await loadMedia(false, 200);
    setTimeout(() => loadMedia(false).catch(() => { }), 0);
    if (state.tab === "audio") {
      tryResumeAudio();
    }
  } catch (e) {
    setMeta(t("meta_fail"));
    alert(String(e?.message || e));
  }
}

boot();
