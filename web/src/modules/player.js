import { state, el, LS } from './state.js';
import { t } from './i18n.js';
import { gpGet, gpSet, logRemote, probeItem, probeText, probeWarnText, rememberEnabled, reportProgress, getProgress } from './api.js';
import { canPlayMedia, streamUrl, formatName, formatBytes, formatTime, getCfg } from './utils.js';
import { resetLyrics, renderLyrics, parseLrc, updateLyricsByTime } from './lyrics.js';
import { setPlaylist, updateNavLabels, updateNavButtons, playAtIndex, playNext, buildPlaylist, generatePlayOrder } from './playlist.js';
import { bus } from './eventbus.js';
import {
  destroyPlyr,
  resetMediaEl,
  hideAllMedia,
  showPreviewError,
  setFitBtnVisible,
  updateFitBtnFromVideo,
  setTracks,
  getActivePlyr,
  applyPlyr,
  needsCompatibilityVideoTranscode,
  switchToTranscodeSource,
  setupErrorHandler,
  getActiveMedia,
  saveProgress,
  hasResumeCandidate,
  updateResumeButton,
  restorePlaybackTime,
  setupAudioTrackHandling,
  switchAudioTrack,
  getAudioTracks
} from './player/index.js';

export {
  destroyPlyr,
  resetMediaEl,
  hideAllMedia,
  showPreviewError,
  setFitBtnVisible,
  updateFitBtnFromVideo,
  setTracks,
  getActiveMedia,
  saveProgress,
  hasResumeCandidate,
  updateResumeButton,
  switchAudioTrack,
  getAudioTracks
};

bus.on('play:request', (item, opts) => playItem(item, opts));

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

  if (state.playlist.playIndex < 0) return;
  playNext(true);
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

  if (getCfg("features.playlist", true)) {
    let restored = false;
    const savedPlRaw = gpGet(LS.playlist);
    if (savedPlRaw) {
      try {
        const plData = JSON.parse(savedPlRaw);
        if (plData.kind === kind && Array.isArray(plData.ids)) {
          const items = plData.ids.map(id => pool.find(x => x.id === id)).filter(Boolean);
          if (items.length > 0) {
            const index = Math.max(0, Math.min(plData.index || 0, items.length - 1));
            const playOrder = generatePlayOrder(items.length, index, state.playlist.shuffle);
            const playIndex = playOrder.findIndex(idx => idx === index);
            setPlaylist(kind, items, index, playOrder, playIndex);
            restored = true;
          }
        }
      } catch { }
    }
    if (!restored) {
      const pl = buildPlaylist(item, kind);
      setPlaylist(kind, pl.items, pl.index, pl.playOrder, pl.playIndex);
    }
    playItem(item, { fromPlaylist: true, autoplay: false, resume: true });
  } else {
    playItem(item, { autoplay: false, resume: true });
  }

  if (kind === "image") return;

  const savedVol = gpGet(LS.volume);
  const elId = kind === "audio" ? "audioEl" : "videoEl";
  if (savedVol) {
    const mediaEl = el(elId);
    if (mediaEl) mediaEl.volume = Number(savedVol);
  }

  const mediaEl = el(elId);
  if (mediaEl) {
    await restorePlaybackTime(kind, id, mediaEl);
  }
}

export function playItem(item, opts) {
  const options = opts || {};
  if (!item) return;

  const prevKind = state.current?.kind;
  const token = ++state.selectionToken;
  state.current = item;
  state.tab = item.kind;
  logRemote("info", `Playing item: ${item.name} (${item.id})`);

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

  if (options.user && !options.fromPlaylist && getCfg("features.playlist", true)) {
    const pl = buildPlaylist(item, item.kind);
    if (pl.items.length) {
      setPlaylist(item.kind, pl.items, pl.index, pl.playOrder, pl.playIndex);
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
    try { delete audio.dataset.mspDirectRetryDone; } catch { }
    audio.src = streamUrl(item.id);
    audio.style.opacity = "0";
    audio.style.display = "block";
    requestAnimationFrame(() => {
      audio.style.transition = "opacity 0.25s ease";
      audio.style.opacity = "1";
    });

    audio.removeEventListener("ended", onMediaEnded);
    audio.addEventListener("ended", onMediaEnded);
    setupErrorHandler(audio, onMediaEnded);

    applyPlyr(audio, onMediaEnded);
    setupAudioTrackHandling(audio);
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

      const t = audio.currentTime || 0;
      const d = audio.duration || 0;
      if (d > 0 && d - t < 0.5 && !audio.paused) {
        onMediaEnded();
      }

      if (!state.lyrics) return;
      updateLyricsByTime(t, ev.type === "seeked");
    };
    audio.ontimeupdate = onTimeUpdate;
    audio.onseeked = onTimeUpdate;

    const meta = el("audioMeta");
    const cover = el("audioCover");
    const placeholder = el("audioCoverPlaceholder");

    if (item.coverId) {
      cover.src = streamUrl(item.coverId);
      cover.hidden = false;
      if (placeholder) placeholder.hidden = true;
    } else {
      cover.removeAttribute("src");
      cover.hidden = true;
      if (placeholder) placeholder.hidden = false;
    }

    meta.style.opacity = "0";
    meta.style.display = "flex";
    requestAnimationFrame(() => {
      meta.style.transition = "opacity 0.25s ease";
      meta.style.opacity = "1";
      if (typeof navigator !== "undefined" && navigator.userAgent.includes("Firefox")) {
        meta.style.transform = "translateZ(0)";
      }
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
    let video = el("videoEl");

    if (isVideoSwitch && state.plyr && state.plyr.media) {
      video = state.plyr.media;
    }

    if (!video) {
      showPreviewError(t("err_video_format", item.ext || ""));
      console.error("Video element not found in DOM or Plyr");
      return;
    }
    if (!canPlayMedia("video", item.ext, item.name, video)) {
      showPreviewError(t("err_video_format", item.ext || ""));
      return;
    }
    try { delete video.dataset.mspDirectRetryDone; } catch { }

    if (isVideoSwitch) {
      if (state.plyr) {
        state.isSwitchingMedia = true;
        state.plyr.off("ended", onMediaEnded);

        try {
          const needsPreemptiveTranscode = needsCompatibilityVideoTranscode(item) && getCfg("playback.video.transcode", false);

          let src = streamUrl(item.id);
          if (needsPreemptiveTranscode) {
            src += "&transcode=1";
            logRemote("info", `Pre-emptive transcode for AVI/WMV compatibility`);
          }

          state.plyr.source = {
            type: "video",
            title: item.name || "",
            sources: [{ src: src }],
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

    const needsPreemptiveTranscode = needsCompatibilityVideoTranscode(item) && getCfg("playback.video.transcode", false);

    let src = streamUrl(item.id);
    if (needsPreemptiveTranscode) {
      src += "&transcode=1";
      logRemote("info", `Pre-emptive transcode for AVI/WMV compatibility`);
    }

    video.src = src;
    setTracks(video, item.subtitles || []);
    video.style.display = "block";
    updateFitBtnFromVideo(video);
    setupErrorHandler(video, onMediaEnded);
    applyPlyr(video, onMediaEnded);
    setupAudioTrackHandling(video);
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
  const onKey = async (ev) => {
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

    if (k === "ArrowLeft" || k === "ArrowRight") {
      if (media && (act.kind === "video" || act.kind === "audio")) {
        handled();
        try {
          const step = 10;
          media.currentTime = k === "ArrowLeft" ? Math.max(0, media.currentTime - step) : Math.min(media.duration || 999999, media.currentTime + step);
        } catch { }
      }
      return;
    }

    if (k === "ArrowUp" || k === "ArrowDown") {
      if (media && (act.kind === "video" || act.kind === "audio")) {
        handled();
        try {
          const step = 0.1;
          let v = media.volume + (k === "ArrowUp" ? step : -step);
          if (v < 0) v = 0;
          if (v > 1) v = 1;
          media.volume = v;
        } catch { }
      }
      return;
    }

    if (k.toLowerCase() === "m") {
      if (media && (act.kind === "video" || act.kind === "audio")) {
        handled();
        try { media.muted = !media.muted; } catch { }
      }
      return;
    }

    if (k === "[" || k === "]") {
      const pl = state.playlist;
      if (pl && pl.items && pl.items.length > 0 && pl.playOrder.length > 0) {
        handled();
        if (k === "[") {
          if (pl.playIndex > 0) playAtIndex(pl.playIndex - 1, true, true);
          else if (pl.loop) playAtIndex(pl.playOrder.length - 1, true, true);
        } else {
          if (pl.playIndex < pl.playOrder.length - 1) playAtIndex(pl.playIndex + 1, true, true);
          else if (pl.loop) playAtIndex(0, true, true);
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
