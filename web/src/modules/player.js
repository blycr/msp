import { state, el, lsGet, LS } from './state.js';
import { t } from './i18n.js';
import { gpGet, gpSet, logRemote, apiPost, probeItem, probeText, probeWarnText, mediaErrorText, rememberEnabled, reportProgress, getProgress } from './api.js';
import { mimeFor, canPlayMedia, streamUrl, formatName, formatBytes, formatTime, getCfg } from './utils.js';
import { resetLyrics, renderLyrics, parseLrc, updateLyricsByTime } from './lyrics.js';
import { setPlaylist, renderPlaylist, buildPlaylist, updateNavLabels, updateNavButtons, playAtIndex } from './playlist.js';

export function getActiveMedia() {
  const kind = state.current?.kind;
  if (kind === "video") return { el: el("videoEl"), kind: "video" };
  if (kind === "audio") return { el: el("audioEl"), kind: "audio" };
  return { el: null, kind: "" };
}

export function saveProgress(kind, id, t) {
  gpSet(LS.lastActiveKind, kind);
  if (kind === "audio") {
    gpSet(LS.audioLastID, id);
    if (t !== undefined) {
      gpSet(LS.audioLastTime, String(t));
      reportProgress(id, t);
    }
  } else if (kind === "video") {
    gpSet(LS.videoLastID, id);
    if (t !== undefined) {
      gpSet(LS.videoLastTime, String(t));
      reportProgress(id, t);
    }
  } else if (kind === "image") {
    gpSet(LS.imageLastID, id);
  }

  // Save full playlist state
  if (state.playlist && state.playlist.items && state.playlist.items.length > 0) {
    const plData = {
      kind: state.playlist.kind,
      index: state.playlist.index,
      ids: state.playlist.items.map(x => x.id),
    };
    gpSet(LS.playlist, JSON.stringify(plData));
  }

  // Save volume
  const act = getActiveMedia();
  if (act && act.el && act.el.volume !== undefined) {
    gpSet(LS.volume, String(act.el.volume));
  }

  logRemote("info", `Playback progress saved: kind=${kind} id=${id} time=${t}`);
}

export function hasResumeCandidate() {
  const kind = gpGet(LS.lastActiveKind);
  if (!kind) return false;
  if (kind === "audio" && !rememberEnabled("audio")) return false;
  if (kind === "video" && !rememberEnabled("video")) return false;
  if (kind === "image" && !rememberEnabled("image")) return false;
  if (kind === "audio") return !!gpGet(LS.audioLastID);
  if (kind === "video") return !!gpGet(LS.videoLastID);
  if (kind === "image") return !!gpGet(LS.imageLastID);
  return false;
}

export function updateResumeButton() {
  const btn = el("btnResume");
  if (!btn) return;
  const show = !state.current && hasResumeCandidate();
  btn.hidden = !show;
  btn.disabled = !show;
}

export async function resumeLast() {
  if (!state.media) return;
  const kind = gpGet(LS.lastActiveKind);
  if (!kind) return;
  if (kind !== "audio" && kind !== "video" && kind !== "image") return;
  let pool = [];
  if (kind === "audio") pool = state.media.audios || [];
  if (kind === "video") pool = state.media.videos || [];
  if (kind === "image") pool = state.media.images || [];
  if (!pool.length) return;
  const id = kind === "audio" ? gpGet(LS.audioLastID)
    : kind === "video" ? gpGet(LS.videoLastID)
      : gpGet(LS.imageLastID);
  if (!id) return;
  const item = pool.find(x => x.id === id);
  if (!item) return;

  state.tab = kind;

  // Restore playlist if available
  if (getCfg("features.playlist", true)) {
    let restored = false;
    const savedPlRaw = gpGet(LS.playlist);
    if (savedPlRaw) {
      try {
        const plData = JSON.parse(savedPlRaw);
        if (plData.kind === kind && Array.isArray(plData.ids)) {
          const items = plData.ids.map(id => pool.find(x => x.id === id)).filter(Boolean);
          if (items.length > 0) {
            setPlaylist(kind, items, plData.index);
            restored = true;
          }
        }
      } catch { }
    }
    if (!restored) {
      const pl = buildPlaylist(item, kind);
      setPlaylist(kind, pl.items, pl.index);
    }
    playItem(item, { fromPlaylist: true, autoplay: false, resume: true });
  } else {
    playItem(item, { autoplay: false, resume: true });
  }

  if (kind === "image") return;

  // Restore volume
  const savedVol = gpGet(LS.volume);
  const elId = kind === "audio" ? "audioEl" : "videoEl";
  if (savedVol) {
    const mediaEl = el(elId);
    if (mediaEl) mediaEl.volume = Number(savedVol);
  }

  // Per-file time has priority over global last time in the Resume Last flow
  // Refactored to use API
  let timeVal = 0;
  try {
    const apiTime = await getProgress(id);
    if (apiTime > 0) {
      timeVal = apiTime;
    } else {
      // Fallback to local storage global last time
      timeVal = Number((kind === "audio" ? gpGet(LS.audioLastTime) : gpGet(LS.videoLastTime)) || "0") || 0;
    }
  } catch {
    timeVal = Number((kind === "audio" ? gpGet(LS.audioLastTime) : gpGet(LS.videoLastTime)) || "0") || 0;
  }
  
  if (timeVal <= 0) return;
  const mediaEl = el(elId);
  if (!mediaEl) return;
  const seek = () => { try { mediaEl.currentTime = timeVal; } catch { } };
  if (mediaEl.readyState >= 1) {
    queueMicrotask(seek);
  } else {
    const onLoaded = () => { seek(); mediaEl.removeEventListener("loadedmetadata", onLoaded); };
    mediaEl.addEventListener("loadedmetadata", onLoaded);
  }
}

export function destroyPlyr() {
  if (state.plyr) {
    try { state.plyr.destroy(); } catch { }
    state.plyr = null;
  }
  try {
    if (state.plyrPersistTimer) {
      clearInterval(state.plyrPersistTimer);
      state.plyrPersistTimer = 0;
    }
  } catch { }
}

export function resetMediaEl(mediaEl) {
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

export function hideAllMedia() {
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

export function showPreviewError(text) {
  destroyPlyr();
  el("videoEl").style.display = "none";
  el("audioEl").style.display = "none";
  el("audioMeta").style.display = "none";
  el("imgEl").style.display = "none";
  el("emptyEl").textContent = text;
  el("emptyEl").style.display = "block";
}

export function setFitBtnVisible(visible) {
  const btn = el("btnToggleFit");
  if (!btn) return;
  btn.hidden = !visible;
  if (!visible) btn.disabled = true;
}

export function updateFitBtnFromVideo(videoEl) {
  const btn = el("btnToggleFit");
  if (!btn || !videoEl) return;
  btn.hidden = false;
  btn.disabled = false;
  let fit = videoEl.dataset.fit || gpGet("msp.video.fit") || "contain";
  try { videoEl.dataset.fit = fit; } catch { }
  btn.textContent = fit === "cover" ? t("fit_cover") : t("fit_contain");
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

export function canStorage() {
  try {
    const k = "__msp__probe__";
    localStorage.setItem(k, "1");
    localStorage.removeItem(k);
    return true;
  } catch {
    return false;
  }
}

let lastMediaEndedAt = 0;
function onMediaEnded() {
  const now = Date.now();
  if (now - lastMediaEndedAt < 500) return;
  if (state.isSwitchingMedia) return;
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

export function applyPlyr(element) {
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
  try {
    const vol = Number(gpGet("msp.volume") || "");
    if (!Number.isNaN(vol) && vol >= 0 && vol <= 1) element.volume = vol;
    const muted = gpGet("msp.muted");
    if (muted === "1") element.muted = true;
    const rate = Number(gpGet("msp.rate") || "");
    const opts = Array.isArray((state.config?.features || {}).speedOptions) ? state.config.features.speedOptions : null;
    if (!Number.isNaN(rate) && rate > 0.1 && rate <= 4) {
      if (opts && opts.length) {
        const has = opts.some(x => Number(x) === rate);
        if (has) element.playbackRate = rate;
      } else {
        element.playbackRate = rate;
      }
    }
  } catch { }
  const opts = {};

  if (features.speed) {
    opts.speed = { selected: 1, options: Array.isArray(features.speedOptions) && features.speedOptions.length ? features.speedOptions : [0.5, 0.75, 1, 1.25, 1.5, 2] };
  }

  if (features.captions && String(element?.tagName || "").toUpperCase() === "VIDEO") {
    opts.captions = { active: true, update: true, language: "auto" };
  }

  opts.fullscreen = { enabled: true, fallback: true };
  opts.storage = { enabled: !!canStorage() };
  opts.tooltips = { controls: true, seek: true };
  try { opts.keyboard = { focused: true, global: true }; } catch { }
  state.plyr = new Plyr(element, opts);
  state.plyr.on("ended", onMediaEnded);

  // Smart Seeking for Transcoded Streams
  const ext = (state.current?.ext || "").toLowerCase();
  const isVideo = state.current?.kind === "video";
  const isAudio = state.current?.kind === "audio";
  const transVideo = isVideo && ext !== ".mp4" && ext !== ".m4v" && ext !== ".webm" && getCfg("playback.video.transcode", false);
  const transAudio = isAudio && ext !== ".mp3" && ext !== ".m4a" && ext !== ".aac" && ext !== ".wav" && getCfg("playback.audio.transcode", false);

  if (transVideo || transAudio) {
    let lastSeekTime = 0;
    state.plyr.on("seeking", () => {
      const now = Date.now();
      if (now - lastSeekTime < 1000) return; // Debounce
      const targetTime = element.currentTime;
      if (targetTime < 0.1) return; // Ignore reset to 0

      logRemote("info", `Transcode seek detected: target=${targetTime}`);
      lastSeekTime = now;

      // Reload source with start parameter
      const url = streamUrl(state.current.id, targetTime);
      
      // We need to pause, change src, and resume.
      // For Plyr, we can update source.
      const isPaused = element.paused;
      
      if (isVideo) {
        state.plyr.source = {
          type: "video",
          sources: [{ src: url }],
          poster: state.current.coverId ? streamUrl(state.current.coverId) : undefined,
          tracks: (state.current.subtitles || []).map(s => ({
            kind: "subtitles",
            label: s.label || "字幕",
            srclang: s.lang || "zh",
            src: s.src || streamUrl(s.id),
            default: !!s.default
          }))
        };
      } else {
        state.plyr.source = {
          type: "audio",
          sources: [{ src: url }]
        };
      }
      
      state.plyr.once("ready", () => {
        element.currentTime = targetTime;
        if (!isPaused) state.plyr.play().catch(() => {});
      });
    });
  }

  try {
    const wrap = element.closest?.(".plyr");
    if (wrap) wrap.style.display = "block";
  } catch { }
  try {
    if (String(element?.tagName || "").toUpperCase() === "VIDEO") {
      state.plyr.on("enterfullscreen", () => {
        try { element.dataset.fit = "cover"; } catch { }
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
  try {
    element.addEventListener("volumechange", () => {
      gpSet("msp.volume", String(element.volume || 1), 500); // Debounce volume
      gpSet("msp.muted", element.muted ? "1" : "0", 500);
    });
  } catch { }
  try {
    element.addEventListener("ratechange", () => {
      try { gpSet("msp.rate", String(element.playbackRate || 1)); } catch { }
    });
  } catch { }
  try {
    if (String(element?.tagName || "").toUpperCase() === "VIDEO") {
      const applyCaptionPref = () => {
        try {
          const tt = element.textTracks;
          const n = tt ? tt.length : 0;
          const mid = String(state.current?.id || "");
          const idxPref = Number(gpGet(mid ? ("msp.subTrack." + mid) : "msp.subTrack") || "");
          const activePref = gpGet(mid ? ("msp.subActive." + mid) : "msp.subActive");
          const idx = (!Number.isNaN(idxPref) && idxPref >= 0 && idxPref < n) ? idxPref : 0;
          if (state.plyr && typeof state.plyr.currentTrack === "number") state.plyr.currentTrack = idx;
          if (tt && n > 0) {
            for (let i = 0; i < n; i++) tt[i].mode = "disabled";
            tt[idx].mode = (activePref === "0") ? "disabled" : "showing";
            if (state.plyr && state.plyr.captions) state.plyr.captions.active = activePref !== "0";
          }
        } catch { }
      };
      setTimeout(applyCaptionPref, 150);
      let lastIdx = -1;
      let lastActive = "";
      state.plyrPersistTimer = setInterval(() => {
        try {
          let idx = -1;
          let active = "";
          if (state.plyr && typeof state.plyr.currentTrack === "number") idx = Number(state.plyr.currentTrack);
          if (state.plyr && state.plyr.captions) active = state.plyr.captions.active ? "1" : "0";
          const mid = String(state.current?.id || "");
          const tKey = mid ? ("msp.subTrack." + mid) : "msp.subTrack";
          const aKey = mid ? ("msp.subActive." + mid) : "msp.subActive";
          if (idx !== lastIdx && idx >= 0) { gpSet(tKey, String(idx)); lastIdx = idx; }
          if (active !== lastActive && active) { gpSet(aKey, active); lastActive = active; }
        } catch { }
      }, 2000);
    }
  } catch { }
}

export function playItem(item, opts) {
  const options = opts || {};
  if (!item) return;

  const prevKind = state.current?.kind;
  const token = ++state.selectionToken;
  state.current = item;
  state.tab = item.kind;
  logRemote("info", `Playing item: ${item.name} (${item.id})`);

  // Restore volume
  const savedVol = gpGet(LS.volume);
  if (savedVol && (item.kind === "audio" || item.kind === "video")) {
    const mediaEl = el(item.kind === "audio" ? "audioEl" : "videoEl");
    if (mediaEl) mediaEl.volume = Number(savedVol);
  }
  updateNavLabels();
  updateResumeButton();

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

  // ONLY build a new playlist if the user clicked from the main list (not from playlist panel/nav)
  if (options.user && !options.fromPlaylist && getCfg("features.playlist", true)) {
    const pl = buildPlaylist(item, item.kind);
    if (pl.items.length) {
      setPlaylist(item.kind, pl.items, pl.index);
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
    if (rememberEnabled("image")) {
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

    audio.removeEventListener("ended", onMediaEnded);
    audio.addEventListener("ended", onMediaEnded);

    applyPlyr(audio);
    try { audio.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", async () => {
          let perFileTime = 0;
          try { perFileTime = await getProgress(item.id); } catch { }
          if (perFileTime > 0) state.plyr.currentTime = perFileTime;
          state.plyr.play().catch(() => { });
        });
      } else {
        getProgress(item.id).then(t => {
          if (t > 0) audio.currentTime = t;
          audio.play().catch(() => { });
        }).catch(() => {
          audio.play().catch(() => { });
        });
      }
    }

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

    if (rememberEnabled("audio")) {
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
          requestAnimationFrame(() => updateLyricsByTime(audio.currentTime || 0, true));
        })
        .catch(() => { });
    }

    return;
  }

  if (item.kind === "video") {
    const video = el("videoEl");
    if (!canPlayMedia("video", item.ext, item.name, video)) {
      showPreviewError(t("err_video_format", item.ext || ""));
      return;
    }

    if (isVideoSwitch) {
      if (state.plyr) {
        state.isSwitchingMedia = true;
        state.plyr.off("ended", onMediaEnded);

        try {
          state.plyr.source = {
            type: "video",
            title: item.name || "",
            sources: [{ src: streamUrl(item.id) }],
            tracks: (item.subtitles || []).map(s => ({
              kind: "subtitles",
              label: s.label || "字幕",
              srclang: s.lang || "zh",
              src: s.src || streamUrl(s.id),
              default: !!s.default
            })),
            poster: item.coverId ? streamUrl(item.coverId) : undefined
          };

          try { video.currentTime = 0; } catch (e) { }

          const forceCaptions = () => {
            try {
              if (state.plyr.captions) {
                state.plyr.currentTrack = 0;
                state.plyr.captions.active = true;
              }
              const tt = video.textTracks;
              if (tt && tt.length > 0) {
                for (let i = 0; i < tt.length; i++) tt[i].mode = "disabled";
                tt[0].mode = "showing";
              }
            } catch (e) { }
          };

          if (options.autoplay) {
            setTimeout(() => {
              state.plyr.play().catch(() => { });
              forceCaptions();
              state.plyr.on("ended", onMediaEnded);
              state.isSwitchingMedia = false;
            }, 150);
          } else {
            setTimeout(() => {
              forceCaptions();
              state.plyr.on("ended", onMediaEnded);
              state.isSwitchingMedia = false;
            }, 150);
          }
        } catch (e) {
          console.error("Plyr source switch failed", e);
          state.plyr.on("ended", onMediaEnded);
          state.isSwitchingMedia = false;
        }

        updateFitBtnFromVideo(video);
        return;
      } else {
        state.isSwitchingMedia = true;
        video.src = streamUrl(item.id);
        setTracks(video, item.subtitles || []);
        try { video.load(); } catch { }
        if (options.autoplay) {
          video.play().then(() => {
            state.isSwitchingMedia = false;
          }).catch(() => {
            state.isSwitchingMedia = false;
          });
        } else {
          state.isSwitchingMedia = false;
        }
        return;
      }
    }

    resetMediaEl(video);
    video.src = streamUrl(item.id);
    setTracks(video, item.subtitles || []);
    video.style.display = "block";
    updateFitBtnFromVideo(video);
    applyPlyr(video);
    try { video.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", async () => {
          let perFileTime = 0;
          try { perFileTime = await getProgress(item.id); } catch { }
          if (perFileTime > 0) state.plyr.currentTime = perFileTime;
          state.plyr.play().catch(() => { });
        });
      } else {
        getProgress(item.id).then(t => {
          if (t > 0) video.currentTime = t;
          video.play().catch(() => { });
        }).catch(() => {
          video.play().catch(() => { });
        });
      }
    }
    return;
  }

  el("emptyEl").textContent = t("err_unsupported");
  el("emptyEl").style.display = "block";
}

export function bindGlobalHotkeys() {
  const onKey = (ev) => {
    const active = document.activeElement;
    if (active && (active.tagName === "INPUT" || active.tagName === "TEXTAREA" || active.isContentEditable)) return;
    if (!state.current) return;

    const k = ev.key;
    if (!k) return;
    const act = getActiveMedia();
    const media = act.el;

    const handled = () => {
      ev.preventDefault();
      ev.stopPropagation();
      ev.stopImmediatePropagation();
    };

    if (k === " " || k === "Spacebar") {
      if (media && (act.kind === "video" || act.kind === "audio")) {
        handled();
        if (media.paused) {
          media.play().catch(() => { });
        } else {
          media.pause();
        }
      }
      return;
    }

    if (k === "[" || k === "]") {
      const pl = state.playlist;
      if (pl && pl.items && pl.items.length > 0) {
        handled();
        if (k === "[") {
          if (pl.index > 0) playAtIndex(pl.index - 1, true, true);
        } else {
          if (pl.index < pl.items.length - 1) playAtIndex(pl.index + 1, true, true);
        }
      }
      return;
    }

    if (k.toLowerCase() === "f") {
      if (act.kind === "video") {
        handled();
        if (state.plyr && state.plyr.fullscreen) {
          state.plyr.fullscreen.toggle();
        } else if (media) {
          if (document.fullscreenElement) {
            document.exitFullscreen().catch(() => { });
          } else {
            media.requestFullscreen?.().catch(() => { });
          }
        }
      }
      return;
    }
  };

  document.addEventListener("keydown", onKey, true);
}
