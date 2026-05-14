import { state, el, canStorage } from '../state.js';
import { t } from '../i18n.js';
import { gpGet, gpSet, logRemote } from '../api.js';
import { streamUrl, getCfg } from '../utils.js';
import { cleanupAudioTrackHandling } from './audio-track.js';

// Module-level refs for symmetric cleanup
let plyrStallTimer = 0;
let volumeChangeHandler = null;
let rateChangeHandler = null;

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
  try {
    if (plyrStallTimer) {
      clearInterval(plyrStallTimer);
      plyrStallTimer = 0;
    }
  } catch { }
  try { delete window.plyrPlayer; } catch { }
  try { delete window.callPlyr; } catch { }
  try {
    if (volumeChangeHandler) {
      el("videoEl")?.removeEventListener("volumechange", volumeChangeHandler);
      el("audioEl")?.removeEventListener("volumechange", volumeChangeHandler);
      volumeChangeHandler = null;
    }
  } catch { }
  try {
    if (rateChangeHandler) {
      el("videoEl")?.removeEventListener("ratechange", rateChangeHandler);
      el("audioEl")?.removeEventListener("ratechange", rateChangeHandler);
      rateChangeHandler = null;
    }
  } catch { }
  cleanupAudioTrackHandling();
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

export function setTracks(videoEl, subtitles) {
  const tracks = Array.from(videoEl.querySelectorAll("track"));
  for (const t of tracks) t.remove();

  if (!Array.isArray(subtitles) || subtitles.length === 0) return;

  const features = state.config?.features || {};
  if (!features.captions) return;

  for (const s of subtitles) {
    const tr = document.createElement("track");
    tr.kind = "subtitles";
    tr.label = s.label || t("label_subtitle");
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

export function getActivePlyr() {
  const p = state.plyr;
  if (!p || typeof p !== "object") return null;
  if (typeof p.play !== "function") return null;
  return p;
}

export function applyPlyr(element, onMediaEnded) {
  destroyPlyr();
  const isVideoElement = String(element?.tagName || "").toUpperCase() === "VIDEO";

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

  opts.controls = [
    'play-large',
    'play',
    'progress',
    'current-time',
    'duration',
    'mute',
    'volume',
    'captions',
    'settings',
    'fullscreen'
  ];

  if (features.speed) {
    opts.speed = { selected: 1, options: Array.isArray(features.speedOptions) && features.speedOptions.length ? features.speedOptions : [0.5, 0.75, 1, 1.25, 1.5, 2] };
  }

  if (isVideoElement) {
    opts.captions = { active: true, update: true, language: "auto" };
  }

  opts.settings = ['captions', 'quality', 'speed', 'loop'];
  opts.fullscreen = { enabled: true, fallback: true };
  opts.storage = { enabled: !!canStorage() };
  opts.tooltips = { controls: true, seek: true };
  try { opts.keyboard = { focused: true, global: true }; } catch { }
  state.plyr = new Plyr(element, opts);
  state.plyr.on("ended", onMediaEnded);

  let lastProgressTime = Date.now();
  state.plyr.on("timeupdate", (event) => {
    const instance = event.detail.plyr;
    if (!instance.paused && !instance.seeking) {
      lastProgressTime = Date.now();
    }
  });

  plyrStallTimer = setInterval(() => {
    if (!state.plyr || state.plyr.media !== element) {
      clearInterval(plyrStallTimer);
      plyrStallTimer = 0;
      return;
    }

    if (!element.paused && !element.seeking && element.readyState >= 3) {
      const now = Date.now();
      if (now - lastProgressTime > 15000) {
        console.warn(`Playback heartbeat stopped for 15s (Time: ${element.currentTime.toFixed(2)}) - forcing end state`);
        onMediaEnded();
      }
    } else {
      lastProgressTime = Date.now();
    }
  }, 1000);

  {
    let lastSeekTime = 0;
    state.plyr.on("seeking", () => {
      const currentSrc = element.currentSrc || element.src || "";
      if (!currentSrc.includes("transcode=1")) return;

      const now = Date.now();
      if (now - lastSeekTime < 1000) return;
      const targetTime = element.currentTime;
      if (targetTime < 0.1) return;

      logRemote("info", `Transcode seek detected: target=${targetTime}`);
      lastSeekTime = now;

      const url = streamUrl(state.current.id, targetTime) + "&transcode=1";
      const isPaused = element.paused;

      const newSource = {
        type: isVideoElement ? "video" : "audio",
        title: state.current.name || "",
        sources: [{ src: url, type: isVideoElement ? "video/mp4" : "audio/mpeg" }],
      };

      if (isVideoElement) {
        newSource.poster = state.current.coverId ? streamUrl(state.current.coverId) : undefined;
        newSource.tracks = (state.current.subtitles || []).map(s => ({
          kind: "subtitles",
          label: s.label || t("label_subtitle"),
          srclang: s.lang || "zh",
          src: s.src || streamUrl(s.id),
          default: !!s.default
        }));
      }

      state.plyr.source = newSource;

      state.plyr.once("ready", () => {
        element.currentTime = targetTime;
        if (!isPaused) state.plyr.play().catch(() => { });
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
    if (!state.plyr) throw new Error(t("err_plyr_init"));
    const fn = state.plyr[method];
    if (typeof fn !== "function") throw new Error(t("err_plyr_method", method));
    return fn.apply(state.plyr, args);
  };
  try {
    volumeChangeHandler = () => {
      gpSet("msp.volume", String(element.volume || 1), 500);
      gpSet("msp.muted", element.muted ? "1" : "0", 500);
    };
    element.addEventListener("volumechange", volumeChangeHandler);
  } catch { }
  try {
    rateChangeHandler = () => {
      try { gpSet("msp.rate", String(element.playbackRate || 1)); } catch { }
    };
    element.addEventListener("ratechange", rateChangeHandler);
  } catch { }
}
