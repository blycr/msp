import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { gpGet, gpSet } from '../api.js';

export function setupAudioTrackHandling(videoEl) {
  if (!videoEl || videoEl.tagName !== "VIDEO") return;

  videoEl.addEventListener("loadedmetadata", () => {
    if (videoEl.audioTracks) {
      const audioTracks = videoEl.audioTracks;
      if (audioTracks && audioTracks.length > 1) {
        console.log(`检测到 ${audioTracks.length} 个音轨:`);
        for (let i = 0; i < audioTracks.length; i++) {
          const track = audioTracks[i];
          console.log(`  [${i}] ${track.label || "未命名"} (${track.language || "未知语言"})${track.enabled ? " [当前]" : ""}`);
        }

        const mid = String(state.current?.id || "");
        const savedTrackIdx = gpGet(mid ? (`msp.audioTrack.${mid}`) : "msp.audioTrack");
        if (savedTrackIdx !== null && savedTrackIdx !== undefined) {
          const idx = Number(savedTrackIdx);
          if (!Number.isNaN(idx) && idx >= 0 && idx < audioTracks.length) {
            for (let i = 0; i < audioTracks.length; i++) {
              audioTracks[i].enabled = (i === idx);
            }
          }
        }
      }
    }

    const textTracks = videoEl.textTracks;
    console.log(`字幕轨道检测: 找到 ${textTracks?.length || 0} 个轨道`);
    if (textTracks && textTracks.length > 0) {
      for (let i = 0; i < textTracks.length; i++) {
        const track = textTracks[i];
        console.log(`  [${i}] ${track.label || "未命名"} (${track.language || "未知语言"}) kind=${track.kind} mode=${track.mode}`);
      }

      if (state.plyr) {
        try {
          state.plyr.emit('captionsenabled');
        } catch (e) {
          console.log("Plyr 字幕刷新失败:", e);
        }
      }
    } else {
      console.log("未检测到内封字幕轨道。注意: 浏览器对 MKV 内封字幕的支持有限，可能需要外挂字幕。");
    }
  });

  if (videoEl.audioTracks) {
    videoEl.audioTracks.addEventListener("change", () => {
      const tracks = Array.from(videoEl.audioTracks);
      const enabledTrack = tracks.find(t => t.enabled);
      if (enabledTrack) {
        console.log(`音轨切换至: ${enabledTrack.label || enabledTrack.language || "未知"}`);
      }
    });
  }

  let lastAudioTrackIdx = -1;
  setInterval(() => {
    try {
      const tracks = videoEl.audioTracks;
      if (!tracks || tracks.length <= 1) return;

      let currentIdx = -1;
      for (let i = 0; i < tracks.length; i++) {
        if (tracks[i].enabled) {
          currentIdx = i;
          break;
        }
      }

      if (currentIdx !== lastAudioTrackIdx && currentIdx >= 0) {
        const mid = String(state.current?.id || "");
        gpSet(mid ? (`msp.audioTrack.${mid}`) : "msp.audioTrack", String(currentIdx));
        lastAudioTrackIdx = currentIdx;
      }
    } catch { }
  }, 3000);
}

export function switchAudioTrack(trackIndex) {
  const videoEl = el("videoEl");
  if (!videoEl || !videoEl.audioTracks) return false;

  const tracks = videoEl.audioTracks;
  if (trackIndex < 0 || trackIndex >= tracks.length) return false;

  for (let i = 0; i < tracks.length; i++) {
    tracks[i].enabled = (i === trackIndex);
  }

  const mid = String(state.current?.id || "");
  gpSet(mid ? (`msp.audioTrack.${mid}`) : "msp.audioTrack", String(trackIndex));

  return true;
}

export function getAudioTracks() {
  const videoEl = el("videoEl");
  if (!videoEl || !videoEl.audioTracks) return [];

  return Array.from(videoEl.audioTracks).map((track, index) => ({
    index,
    label: track.label || t("label_unnamed"),
    language: track.language || t("label_unknown"),
    enabled: track.enabled
  }));
}
