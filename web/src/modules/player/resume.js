import { state, el, LS } from '../state.js';
import { gpGet } from '../api.js';
import { getCfg } from '../utils.js';
import { setPlaylist, buildPlaylist, generatePlayOrder, playNext, playAtIndex } from '../playlist.js';
import { updateNavLabels } from '../playlist.js';
import { restorePlaybackTime, getActiveMedia } from './seek.js';
import { playItem } from '../player.js';

function rememberEnabled(kind) {
  const cfg = state.config?.playback?.[kind];
  if (!cfg) return true;
  return cfg.remember !== false;
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
          if (pl.playIndex > 0) {
            playAtIndex(pl.playIndex - 1, true, true);
          }
          else if (pl.loop) {
            playAtIndex(pl.playOrder.length - 1, true, true);
          }
        } else {
          if (pl.playIndex < pl.playOrder.length - 1) {
            playAtIndex(pl.playIndex + 1, true, true);
          }
          else if (pl.loop) {
            playAtIndex(0, true, true);
          }
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
