import { state, el, LS } from '../state.js';
import { bus } from '../eventbus.js';
import { t } from '../i18n.js';
import { gpGet, logRemote, probeItem, probeText, rememberEnabled, getProgress } from '../api.js';
import { canPlayMedia, streamUrl, formatName, formatBytes, formatTime, getCfg } from '../utils.js';
import { resetLyrics, renderLyrics, parseLrc, updateLyricsByTime } from '../lyrics.js';
import { setPlaylist, updateNavLabels, playNext, buildPlaylist, generatePlayOrder } from '../playlist.js';
import { resetMediaEl, hideAllMedia, showPreviewError, setFitBtnVisible, updateFitBtnFromVideo, setTracks, applyPlyr } from './core.js';
import { setupErrorHandler } from './transcode.js';
import { saveProgress } from './seek.js';
import { setupAudioTrackHandling } from './audio-track.js';

let currentBlobUrl = null;

function revokeCurrentBlob() {
  if (currentBlobUrl) {
    try { URL.revokeObjectURL(currentBlobUrl); } catch { }
    currentBlobUrl = null;
  }
}

async function getPlaybackUrl(item) {
  if (item.kind === "video" || item.kind === "audio") {
    bus.emit('transcode:status', 'checking');
  }

  const base = streamUrl(item.id);
  const kind = item.kind;
  const transcodeCfg = kind === "video" ? "playback.video.transcode" : "playback.audio.transcode";

  if (getCfg(transcodeCfg, false)) {
    const p = await probeItem(item.id);
    if (p?.playback?.mode === "transcode") {
      bus.emit('transcode:status', 'transcoding');
      return { url: base + "&transcode=1", mode: "transcode" };
    }
  }

  bus.emit('transcode:status', null);
  return { url: base, mode: "direct" };
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

  if (state.playlist.playIndex < 0) return;
  playNext(true);
}

export async function playItem(item, opts) {
  const options = opts || {};
  if (!item) return;

  const prevKind = state.current?.kind;
  const token = ++state.selectionToken;
  state.current = item;
  state.tab = item.kind;
  // 通知列表翻转 active 行（只改新旧两行 class，不做全量渲染）
  bus.emit('player:current', item);
  logRemote("info", `Playing item: ${item.name} (${item.id})`);

  const savedVol = gpGet(LS.volume);
  if (savedVol && (item.kind === "audio" || item.kind === "video")) {
    const mediaEl = el(item.kind === "audio" ? "audioEl" : "videoEl");
    if (mediaEl) mediaEl.volume = Number(savedVol);
  }
  updateNavLabels();

  setFitBtnVisible(state.tab === "video" && item.kind === "video");

  el("previewTitle").textContent = formatName(item);
  state.currentMetaBase = `${item.shareLabel || ""} · ${(item.ext || "").toUpperCase()} · ${formatBytes(item.size)} · ${formatTime(item.modTime)}`;
  el("previewSub").textContent = state.currentMetaBase;

  if (item.kind === "video") {
    probeItem(item.id).then((p) => {
      if (token !== state.selectionToken) return;
      if (!state.current || state.current.id !== item.id) return;
      el("previewSub").textContent = state.currentMetaBase + probeText(p);
    }).catch(() => { });
  }

  const openBtn = el("btnOpenRaw");
  openBtn.disabled = false;
  revokeCurrentBlob();
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
        const label = s.label || t("label_subtitle");
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
      currentBlobUrl = URL.createObjectURL(blob);
      window.open(currentBlobUrl, "_blank", "noopener,noreferrer");
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

    const audioPlayback = await getPlaybackUrl(item);
    if (token !== state.selectionToken) return;
    audio.src = audioPlayback.url;
    audio.style.opacity = "0";
    audio.hidden = false;
    audio.style.display = "block";
    requestAnimationFrame(() => {
      audio.style.transition = "opacity 0.25s ease";
      audio.style.opacity = "1";
    });

    audio.removeEventListener("ended", onMediaEnded);
    audio.addEventListener("ended", onMediaEnded);
    audio.addEventListener('canplay', () => {
      bus.emit('transcode:status', null);
    }, { once: true });
    setupErrorHandler(audio, onMediaEnded);

    applyPlyr(audio, onMediaEnded);
    setupAudioTrackHandling(audio);
    try { audio.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", async () => {
          let perFileTime = 0;
          if (!options.autoSwitch) {
            try { perFileTime = await getProgress(item.id); } catch { }
          }
          if (state.plyr) {
            if (perFileTime > 0) state.plyr.currentTime = perFileTime;
            state.plyr.play().catch(() => { });
          }
        });
      } else {
        if (options.autoSwitch) {
          audio.currentTime = 0;
          audio.play().catch(() => { });
        } else {
          getProgress(item.id).then(t => {
            if (t > 0) audio.currentTime = t;
            audio.play().catch(() => { });
          }).catch(() => {
            audio.play().catch(() => { });
          });
        }
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
          const switchPlayback = await getPlaybackUrl(item);
          if (token !== state.selectionToken) return;

          state.plyr.source = {
            type: "video",
            title: item.name || "",
            sources: [{ src: switchPlayback.url }],
            tracks: (item.subtitles || []).map(s => ({
              kind: "subtitles",
              label: s.label || t("label_subtitle"),
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
              if (state.plyr) state.plyr.play().catch(() => { });
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
        const switchPlayback = await getPlaybackUrl(item);
        if (token !== state.selectionToken) return;
        video.src = switchPlayback.url;
        video.addEventListener('canplay', () => {
          bus.emit('transcode:status', null);
        }, { once: true });
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

    const videoPlayback = await getPlaybackUrl(item);
    if (token !== state.selectionToken) return;

    video.src = videoPlayback.url;
    setTracks(video, item.subtitles || []);
    video.style.display = "block";
    video.addEventListener('canplay', () => {
      bus.emit('transcode:status', null);
    }, { once: true });
    updateFitBtnFromVideo(video);
    setupErrorHandler(video, onMediaEnded);
    applyPlyr(video, onMediaEnded);
    setupAudioTrackHandling(video);
    try { video.load(); } catch { }

    if (options.autoplay) {
      if (state.plyr) {
        state.plyr.once("ready", async () => {
          let perFileTime = 0;
          if (!options.autoSwitch) {
            try { perFileTime = await getProgress(item.id); } catch { }
          }
          if (state.plyr) {
            if (perFileTime > 0) state.plyr.currentTime = perFileTime;
            state.plyr.play().catch(() => { });
          }
        });
      } else {
        if (options.autoSwitch) {
          video.currentTime = 0;
          video.play().catch(() => { });
        } else {
          getProgress(item.id).then(t => {
            if (t > 0) video.currentTime = t;
            video.play().catch(() => { });
          }).catch(() => {
            video.play().catch(() => { });
          });
        }
      }
    }
    return;
  }

  el("emptyText").textContent = t("err_unsupported");
  el("emptyEl").style.display = "block";
}
