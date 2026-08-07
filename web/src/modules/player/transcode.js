import { state } from '../state.js';
import { t } from '../i18n.js';
import { logRemote, startHlsSession } from '../api.js';
import { streamUrl, getCfg } from '../utils.js';
import { getActivePlyr, showPreviewError, applySource } from './core.js';

export function switchToTranscodeSource(element, isVideo, url, currentTime) {
  const player = getActivePlyr();
  const isHLS = isVideo && (url.includes("/api/hls/") || url.includes(".m3u8"));

  if (isHLS) {
    // HLS：Plyr source API 不适用于 hls.js，元素级 attach 后由 Plyr 复用同一元素
    try {
      if (!applySource(element, url, true)) {
        console.error("HLS not supported by this browser");
        return false;
      }
      if (typeof player?.once === "function") {
        player.once("ready", () => {
          try { element.currentTime = currentTime; } catch { }
          player.play().catch(() => { });
        });
      } else {
        setTimeout(() => {
          try { element.currentTime = currentTime; } catch { }
          element.play().catch(() => { });
        }, 120);
      }
      return true;
    } catch (err) {
      console.error("HLS source switch failed", err);
    }
  }

  if (!player) {
    try { element.src = url; } catch { return false; }
    try { element.load(); } catch { }
    try { element.currentTime = currentTime; } catch { }
    try { element.play().catch(() => { }); } catch { }
    return true;
  }

  const newSource = {
    type: isVideo ? "video" : "audio",
    title: state.current?.name || "",
    sources: [{ src: url, type: isVideo ? "video/mp4" : "audio/mpeg" }],
  };

  if (isVideo) {
    newSource.poster = state.current?.coverId ? streamUrl(state.current.coverId) : undefined;
    newSource.tracks = (state.current?.subtitles || []).map(s => ({
      kind: "subtitles",
      label: s.label || t("label_subtitle"),
      srclang: s.lang || "zh",
      src: s.src || streamUrl(s.id),
      default: !!s.default
    }));
  }

  try {
    player.source = newSource;
    if (typeof player.once === "function") {
      player.once("ready", () => {
        try { element.currentTime = currentTime; } catch { }
        player.play().catch(() => { });
      });
    } else {
      setTimeout(() => {
        try { element.currentTime = currentTime; } catch { }
        player.play().catch(() => { });
      }, 120);
    }
    return true;
  } catch (err) {
    console.error("Failed to switch source via player, fallback to native element", err);
    try { element.src = url; } catch { return false; }
    try { element.load(); } catch { }
    try { element.currentTime = currentTime; } catch { }
    try { element.play().catch(() => { }); } catch { }
    return true;
  }
}

export function setupErrorHandler(element, onMediaEnded) {
  if (element._errBound) return;
  element._errBound = true;

  element.addEventListener("error", (e) => {
    try {
      const err = element.error;
      const d = element.duration;
      const currentTime = element.currentTime;
      const currentSrc = element.currentSrc || element.src || "";

      if (!currentSrc) return;

      if (err && (err.code === 3 || err.code === 4)) {
        const isNearEnd = (d > 0 && currentTime / d > 0.9) ||
          (d > 0 && d - currentTime < 10) ||
          (Number.isNaN(d) && currentTime > 5);

        if (isNearEnd) {
          console.warn("Media decoding error near end - suppressing error and skipping to next", err);
          e.preventDefault();
          e.stopPropagation();
          onMediaEnded();
          return;
        }

        const isAlreadyTranscoding = currentSrc.includes("transcode=1");
        const isVideo = state.current?.kind === "video";
        const isAudio = state.current?.kind === "audio";
        const allowVideo = isVideo && getCfg("playback.video.transcode", false);
        const allowAudio = isAudio && getCfg("playback.audio.transcode", false);
        const allowFallback = allowVideo || allowAudio;

        if (err.code === 4 && !isAlreadyTranscoding && element.dataset.mspDirectRetryDone !== "1") {
          element.dataset.mspDirectRetryDone = "1";
          const retryURL = currentSrc.includes("ts=")
            ? currentSrc.replace(/([?&])ts=\d+/, `$1ts=${Date.now()}`)
            : `${currentSrc}${currentSrc.includes("?") ? "&" : "?"}ts=${Date.now()}`;
          console.warn("Playback source error, retrying once with refreshed URL", retryURL);
          try { element.src = retryURL; } catch { }
          try { element.load(); } catch { }
          const activePlayer = getActivePlyr();
          if (activePlayer) activePlayer.play().catch(() => { });
          else try { element.play().catch(() => { }); } catch { }
          return;
        }

        if (!isAlreadyTranscoding && allowFallback && state.current?.id) {
          console.warn("Playback failed, attempting fallback to transcode...", err);
          e.preventDefault();
          e.stopPropagation();

          logRemote("info", `Fallback to transcode: ${state.current.name} @ ${currentTime}s`);
          if (isVideo) {
            // 视频：创建 HLS 会话（原生 seek），完成后异步切换
            startHlsSession(state.current.id).then((hi) => {
              if (!hi?.m3u8) {
                console.error("HLS session creation failed");
                showPreviewError(t("err_unsupported") + " (Transcode Failed)");
                return;
              }
              switchToTranscodeSource(element, true, hi.m3u8, currentTime);
            }).catch(() => {
              showPreviewError(t("err_unsupported") + " (Transcode Failed)");
            });
            return;
          }
          const url = streamUrl(state.current.id, currentTime) + "&transcode=1";
          if (switchToTranscodeSource(element, false, url, currentTime)) return;
        }

        console.error("Playback failed permanently. Transcode enabled:", allowFallback);
        showPreviewError(t("err_unsupported") + (isAlreadyTranscoding ? " (Transcode Failed)" : " (Transcode Disabled)"));
      }
      console.error("Critical Media Error:", err);
    } catch (handlerErr) {
      console.error("Media error handler crashed:", handlerErr);
    }
  }, true);
}
