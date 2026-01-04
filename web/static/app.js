const el = (id) => document.getElementById(id);

const state = {
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
};

const kinds = {
  video: "视频",
  audio: "音频",
  image: "图片",
  other: "其他",
};

const LS = {
  audioLastID: "msp.audio.lastId",
  audioLastTime: "msp.audio.lastTime",
  audioShuffle: "msp.audio.shuffle",
  audioLoop: "msp.audio.loop",
  theme: "msp.theme",
};

function initTheme() {
  const btn = el("themeBtn");
  if (!btn) return;

  const saved = localStorage.getItem(LS.theme);
  const systemDark = window.matchMedia("(prefers-color-scheme: dark)");
  
  const updateTheme = (isDark) => {
    document.documentElement.setAttribute("data-theme", isDark ? "dark" : "light");
    btn.textContent = isDark ? "🌙" : "🌞";
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
      document.documentElement.classList.add("theme-swap");
      apply();
      setTimeout(() => document.documentElement.classList.remove("theme-swap"), 650);
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
        document.documentElement.classList.add("theme-swap");
        apply();
        setTimeout(() => document.documentElement.classList.remove("theme-swap"), 650);
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
  } catch {}
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
  return d.toLocaleString();
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
  const res = await fetch(url, { cache: "no-cache" });
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
  return parts.length ? ` · 编码/容器：${parts.join(" / ")}` : "";
}

function probeWarnText(p) {
  const a = String(p?.audio || "");
  if (!a) return "";
  if (a.includes("AC-3") || a.includes("E-AC-3") || a.includes("DTS") || a.includes("TrueHD") || a.includes("FLAC")) {
    return ` · 提示：音频为 ${a}，浏览器常不支持`;
  }
  return "";
}

function mediaErrorText(err) {
  if (!err) return "";
  switch (err.code) {
    case 1: return "播放被中止";
    case 2: return "网络/读取失败";
    case 3: return "解码失败（常见于编码不支持）";
    case 4: return "媒体源不支持";
    default: return "未知错误";
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
  setMeta(urls ? `可用地址：${urls}` : "未检测到局域网 IP（仍可用 127.0.0.1 访问）");
  applyConfigToUI();
  renderShares();

  const bl = state.config.blacklist || {};
  el("blExts").value = (bl.extensions || []).join(", ");
  el("blFiles").value = (bl.filenames || []).join(", ");
  el("blFolders").value = (bl.folders || []).join(", ");
  el("blMinSize").value = bl.minSize || "";
}

async function loadMedia(refresh) {
  const data = await apiGet(refresh ? "/api/media?refresh=1" : "/api/media");
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
  } catch {}
  state.playlist.loop = loop;
  const tl = el("toggleLoop");
  if (tl) tl.checked = loop;

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const x of tabs) x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
}

function tryResumeAudio() {
  if (!getCfg("playback.audio.remember", true)) return;
  if (!state.media || !(state.media.audios || []).length) return;
  if (state.current) return;

  let lastID = "";
  let lastTime = 0;
  try { lastID = localStorage.getItem(LS.audioLastID) || ""; } catch {}
  try { lastTime = Number(localStorage.getItem(LS.audioLastTime) || "0") || 0; } catch { lastTime = 0; }
  if (!lastID) return;

  const item = (state.media.audios || []).find(x => x.id === lastID);
  if (!item) return;

  if (getCfg("features.playlist", true)) {
    const pl = buildAudioPlaylist(item);
    setPlaylist("audio", pl.items, pl.index);
    playItem(item, { fromPlaylist: true, autoplay: false, resume: true });
  } else {
    playItem(item, { autoplay: false, resume: true });
  }

  const audio = el("audioEl");
  if (!audio) return;
  const t = Math.max(0, lastTime);
  if (!t) return;
  const seek = () => {
    try { audio.currentTime = t; } catch {}
  };
  if (audio.readyState >= 1) {
    queueMicrotask(seek);
    return;
  }
  audio.addEventListener("loadedmetadata", seek, { once: true });
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
  if (kind === "video") return { prev: "上一个视频", next: "下一个视频" };
  if (kind === "image") return { prev: "上一张", next: "下一张" };
  if (kind === "audio") return { prev: "上一首", next: "下一首" };
  return { prev: "上一个", next: "下一个" };
}

function updateNavLabels() {
  const kind = state.current?.kind || state.playlist.kind || "";
  const { prev, next } = navLabelsForKind(kind);
  const prevBtn = el("btnPrev");
  const nextBtn = el("btnNext");
  if (prevBtn) prevBtn.textContent = prev;
  if (nextBtn) nextBtn.textContent = next;
}

function renderList() {
  const box = el("list");
  const hint = el("hint");
  box.innerHTML = "";

  if (!state.media || (state.media.shares || []).length === 0) {
    hint.textContent = "未配置共享目录。点击右上角“共享目录设置”添加。";
    return;
  }

  const list = currentList();
  const q = (state.q || "").trim().toLowerCase();

  const filtered = q
    ? list.filter(x => (x.name || "").toLowerCase().includes(q) || (x.shareLabel || "").toLowerCase().includes(q))
    : list;

  hint.textContent = `当前分类：${kinds[state.tab] || state.tab}，共 ${filtered.length} 个`;

  for (const item of filtered) {
    const row = document.createElement("div");
    row.className = "item";
    row.addEventListener("click", () => playItem(item, { user: true, autoplay: true }));

    const main = document.createElement("div");
    main.className = "item__main";

    const name = document.createElement("div");
    name.className = "item__name";
    name.textContent = item.name || "";

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
}

function setPlaylist(kind, items, index) {
  state.playlist.kind = kind;
  state.playlist.items = Array.isArray(items) ? items : [];
  state.playlist.index = Number.isFinite(index) ? index : -1;
  renderPlaylist();
  updateNavButtons();
  updateNavLabels();
}

function renderPlaylist() {
  const box = el("plList");
  const meta = el("plMeta");
  box.innerHTML = "";

  const items = state.playlist.items || [];
  if (!items.length) {
    meta.textContent = "未加载";
    return;
  }

  const kind = state.playlist.kind || "";
  meta.textContent = `${kinds[kind] || kind} · ${items.length} 项`;

  for (let i = 0; i < items.length; i++) {
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
    name.textContent = it.name || "";

    const sub = document.createElement("div");
    sub.className = "plitem__sub";
    sub.textContent = `${it.shareLabel || ""} · ${(it.ext || "").toUpperCase()}`;

    main.appendChild(name);
    main.appendChild(sub);

    row.appendChild(idx);
    row.appendChild(main);
    box.appendChild(row);
  }
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
    try { state.plyr.destroy(); } catch {}
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
  try { el("imgEl").removeAttribute("src"); } catch {}
  try { el("audioCover").removeAttribute("src"); } catch {}
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
  try { mediaEl.pause(); } catch {}
  try { mediaEl.currentTime = 0; } catch {}
  try { mediaEl.srcObject = null; } catch {}
  try { mediaEl.removeAttribute("src"); } catch {}
  try {
    const sources = Array.from(mediaEl.querySelectorAll("source"));
    for (const s of sources) s.remove();
  } catch {}
  try { mediaEl.load(); } catch {}
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
      if (!res && e !== ".mkv") return false;
    }
    return true;
  }
  return true;
}

let lastAudioEndedAt = 0;
function onAudioEnded() {
  const now = Date.now();
  if (now - lastAudioEndedAt < 500) return;
  lastAudioEndedAt = now;

  if (!state.current || state.current.kind !== "audio") return;
  if (state.playlist.kind !== "audio") return;
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
    } catch {}
    return false;
  })();

  if (isTouch) {
    try { element.controls = true; } catch {}
    try {
      if (String(element?.tagName || "").toUpperCase() === "VIDEO") element.playsInline = true;
    } catch {}
    try {
      const wrap = element.closest?.(".plyr");
      if (wrap) wrap.style.display = "block";
    } catch {}
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
  if (state.current?.kind === "audio") {
    state.plyr.on("ended", onAudioEnded);
  }
  try {
    const wrap = element.closest?.(".plyr");
    if (wrap) wrap.style.display = "block";
  } catch {}
  try {
    if (String(element?.tagName || "").toUpperCase() === "VIDEO") {
      state.plyr.on("enterfullscreen", () => {
        try { element.dataset.fit = "cover"; } catch {}
        try { console.log(document.fullscreenElement); } catch {}
        try {
          const fitBtn = el("btnToggleFit");
          fitBtn.textContent = "填充模式：铺满";
        } catch {}
      });
      state.plyr.on("exitfullscreen", () => {
        try { element.dataset.fit = "contain"; } catch {}
        try {
          const fitBtn = el("btnToggleFit");
          fitBtn.textContent = "填充模式：适配";
        } catch {}
      });
    }
  } catch {}
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
    } catch {}
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

  const token = ++state.selectionToken;
  state.current = item;
  updateNavLabels();

  setFitBtnVisible(state.tab === "video" && item.kind === "video");

  el("previewTitle").textContent = item.name || "";
  state.currentMetaBase = `${item.shareLabel || ""} · ${(item.ext || "").toUpperCase()} · ${formatBytes(item.size)} · ${formatTime(item.modTime)}`;
  el("previewSub").textContent = state.currentMetaBase;

  if (item.kind === "video") {
    probeItem(item.id).then((p) => {
      if (token !== state.selectionToken) return;
      if (!state.current || state.current.id !== item.id) return;
      el("previewSub").textContent = state.currentMetaBase + probeText(p) + probeWarnText(p);
    }).catch(() => {});
  }

  const openBtn = el("btnOpenRaw");
  openBtn.disabled = false;
  openBtn.onclick = () => window.open(streamUrl(item.id), "_blank", "noopener,noreferrer");

  const shuffleWrap = el("shuffleWrap");
  shuffleWrap.hidden = !getCfg("features.playlist", true) || item.kind !== "audio";

  hideAllMedia();
  resetLyrics();

  if (options.user && window.matchMedia && window.matchMedia("(max-width: 980px)").matches) {
    try {
      document.querySelector(".stage")?.scrollIntoView({ behavior: "smooth", block: "start" });
    } catch {}
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
    img.style.display = "block";
    if (options.autoplay) {
      try { img.decode?.(); } catch {}
    }
    return;
  }

  if (item.kind === "audio") {
    const audio = el("audioEl");
    if (!canPlayMedia("audio", item.ext, item.name, audio)) {
      showPreviewError(`该音频格式浏览器可能不支持（${item.ext || ""}）。请用“在新标签打开”。`);
      return;
    }
    resetMediaEl(audio);
    audio.src = streamUrl(item.id);
    audio.style.display = "block";
    
    // Ensure event listener is attached even if DOM or Plyr changes
    audio.removeEventListener("ended", onAudioEnded);
    audio.addEventListener("ended", onAudioEnded);

    applyPlyr(audio);
    try { audio.load(); } catch {}

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
    meta.style.display = "flex";

    if (getCfg("playback.audio.remember", true)) {
      try { localStorage.setItem(LS.audioLastID, item.id); } catch {}
      if (options.user && !options.resume) {
        try { localStorage.setItem(LS.audioLastTime, "0"); } catch {}
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
        .catch(() => {});
    }

    return;
  }

  if (item.kind === "video") {
    const video = el("videoEl");
    if (!canPlayMedia("video", item.ext, item.name, video)) {
      showPreviewError(`该视频格式浏览器可能不支持（${item.ext || ""}）。请用“在新标签打开”。`);
      return;
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
      fitBtn.textContent = fit === "cover" ? "填充模式：铺满" : "填充模式：适配";
    } catch {}
    applyPlyr(video);
    try { video.load(); } catch {}

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", () => state.plyr.play().catch(() => {}));
      } else {
        video.play().catch(() => {});
      }
    }
    return;
  }

  el("emptyEl").textContent = "该文件类型暂不支持预览（可用“在新标签打开”下载/查看）。";
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
    btn.textContent = "移除";
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
    bl.minSize = parseInt(el("blMinSize").value || "0", 10);
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
        } catch {}
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
    try { localStorage.setItem(LS.audioShuffle, on ? "1" : "0"); } catch {}
    if (state.current?.kind === "audio" && getCfg("playback.audio.enabled", true)) {
      const pl = buildAudioPlaylist(state.current);
      setPlaylist("audio", pl.items, pl.index);
    }
  });

  el("toggleLoop").addEventListener("change", (ev) => {
    const on = !!ev.target.checked;
    state.playlist.loop = on;
    try { localStorage.setItem(LS.audioLoop, on ? "1" : "0"); } catch {}
  });
  try {
    const fitBtn = el("btnToggleFit");
    fitBtn.disabled = true;
    fitBtn.addEventListener("click", () => {
      const v = el("videoEl");
      if (!v) return;
      const cur = v.dataset.fit || "cover";
      const next = cur === "cover" ? "contain" : "cover";
      try { v.dataset.fit = next; } catch {}
      try { fitBtn.textContent = next === "cover" ? "填充模式：铺满" : "填充模式：适配"; } catch {}
    });
  } catch {}

  const audio = el("audioEl");

  let lastSaveAt = 0;
  audio.addEventListener("timeupdate", () => {
    if (!state.current || state.current.kind !== "audio") return;
    if (!getCfg("playback.audio.remember", true)) return;
    const now = Date.now();
    if (now - lastSaveAt < 1500) return;
    lastSaveAt = now;
    try { localStorage.setItem(LS.audioLastID, state.current.id); } catch {}
    try { localStorage.setItem(LS.audioLastTime, String(Math.max(0, audio.currentTime || 0))); } catch {}
  });

  const video = el("videoEl");
  const img = el("imgEl");

  audio.addEventListener("error", () => {
    const ext = state.current?.ext || "";
    showPreviewError(`音频加载/解码失败（${ext}）。可能是浏览器不支持该编码，建议用“在新标签打开”下载后本地播放器播放。`);
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
      showPreviewError(`视频加载/解码失败（${ext}，${err}）。同为 mp4/mkv 也可能因编码不同而无法播放。${hint}建议用“在新标签打开”，或转码为 H.264/AAC（或仅转音频为 AAC）再播放。`);
    }).catch(() => {
      showPreviewError(`视频加载/解码失败（${ext}，${err}）。同为 mp4/mkv 也可能因编码不同而无法播放。建议用“在新标签打开”，或转码为 H.264/AAC（或仅转音频为 AAC）再播放。`);
    });
  });
  img.addEventListener("error", () => {
    const ext = state.current?.ext || "";
    showPreviewError(`图片加载失败（${ext}）。可用“在新标签打开”查看原文件。`);
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
  initTheme();
  bindUI();
  try {
    await loadConfig();
    await loadMedia(false);
    if (state.tab === "audio") {
      tryResumeAudio();
    }
  } catch (e) {
    setMeta("服务连接失败或初始化失败");
    alert(String(e?.message || e));
  }
}

boot();

