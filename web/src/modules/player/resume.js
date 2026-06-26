import { state, el, LS } from '../state.js';
import { gpGet, rememberEnabled } from '../api.js';
import { getCfg } from '../utils.js';
import { setPlaylist, buildPlaylist, generatePlayOrder, playNext, playAtIndex } from '../playlist.js';
import { updateNavLabels } from '../playlist.js';
import { restorePlaybackTime, getActiveMedia } from './seek.js';
import { playItem } from '../player.js';

let hotkeyHandler = null;

// resumeLast 只应在首次启动恢复时沿用上次的 tab；一旦恢复过一次（或用户已
// 主动切换 tab），后续后台 media:refresh / loadMedia(304) 触发的 media:resume
// 不再强行覆盖 state.tab，否则会把用户已选的 tab 切回上次播放类型，与 renderList
// 的 tab 高亮自同步配合后会出现"点翻页后列表变成另一种类型"的脱节。
let resumeDone = false;

export async function resumeLast() {
  if (resumeDone) return;
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
  // 恢复成功后置位守卫：后续后台触发的 resumeLast 直接返回，不再压制用户已选 tab。
  resumeDone = true;

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

export function unbindGlobalHotkeys() {
  if (hotkeyHandler) {
    document.removeEventListener("keydown", hotkeyHandler, true);
    hotkeyHandler = null;
  }
}

export function bindGlobalHotkeys() {
  unbindGlobalHotkeys();
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

  hotkeyHandler = onKey;
  document.addEventListener("keydown", onKey, true);
}
