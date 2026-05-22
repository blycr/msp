import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { logRemote } from '../api.js';
import { getCfg, formatName, dirOfAbsPath, absPathOfItem } from '../utils.js';
import { bus } from '../eventbus.js';

export function navLabelsForKind(kind) {
  if (kind === "video") return { prev: t("prev_video"), next: t("next_video") };
  if (kind === "image") return { prev: t("prev_image"), next: t("next_image") };
  if (kind === "audio") return { prev: t("prev_audio"), next: t("next_audio") };
  return { prev: t("prev_item"), next: t("next_item") };
}

export function updateNavLabels() {
  const kind = state.current?.kind || state.playlist.kind || "";
  const { prev, next } = navLabelsForKind(kind);
  const prevBtn = el("btnPrev");
  const nextBtn = el("btnNext");
  if (prevBtn) prevBtn.textContent = prev;
  if (nextBtn) nextBtn.textContent = next;
}

export function updateNavButtons() {
  const prev = el("btnPrev");
  const next = el("btnNext");
  const pl = state.playlist;
  const items = pl.items || [];
  const playOrder = pl.playOrder || [];
  const playIndex = pl.playIndex;

  const hasItems = items.length > 0 && playOrder.length > 0;
  const canPrev = hasItems && playIndex > 0;
  const canNext = hasItems && playIndex < playOrder.length - 1;

  if (prev) prev.disabled = !canPrev;
  if (next) next.disabled = !canNext;
  updateNavLabels();
}

export function playAtIndex(i, autoplay, user) {
  const items = state.playlist.items || [];
  if (!items.length) return;

  const playOrder = state.playlist.playOrder;
  const playIndex = Math.max(0, Math.min(playOrder.length - 1, i));
  const actualIndex = playOrder[playIndex];

  if (actualIndex === undefined || actualIndex < 0 || actualIndex >= items.length) return;

  state.playlist.playIndex = playIndex;
  state.playlist.index = actualIndex;
  bus.emit('playlist:updated');
  updateNavButtons();
  bus.emit('play:request', items[actualIndex], { fromPlaylist: true, autoplay: !!autoplay, user: !!user, autoSwitch: true });
}

export function playPrev(autoplay = true) {
  const pl = state.playlist;
  if (!pl.items.length || pl.playIndex < 0) return;

  if (pl.playIndex > 0) {
    playAtIndex(pl.playIndex - 1, autoplay, true);
  } else if (pl.loop) {
    playAtIndex(pl.playOrder.length - 1, autoplay, true);
  }
}

export function playNext(autoplay = true) {
  const pl = state.playlist;
  if (!pl.items.length || pl.playIndex < 0) return;

  if (pl.playIndex < pl.playOrder.length - 1) {
    playAtIndex(pl.playIndex + 1, autoplay, true);
  } else if (pl.loop) {
    playAtIndex(0, autoplay, true);
  }
}

export function generatePlayOrder(itemCount, startIndex, shuffle) {
  const order = Array.from({ length: itemCount }, (_, i) => i);
  if (!shuffle || itemCount <= 1) return order;

  if (startIndex > 0 && startIndex < itemCount) {
    [order[0], order[startIndex]] = [order[startIndex], order[0]];
  }

  for (let i = itemCount - 1; i > 1; i--) {
    const j = Math.floor(Math.random() * i) + 1;
    [order[i], order[j]] = [order[j], order[i]];
  }

  return order;
}

export function buildPlaylist(item, kind, shuffle = null) {
  const scope = getCfg(`playback.${kind}.scope`, kind === "audio" ? "all" : "folder");
  const poolMap = { video: "videos", audio: "audios", image: "images" };
  const all = state.media?.[poolMap[kind]] || [];
  if (!all.length) return { items: [], index: -1, playOrder: [], playIndex: -1 };

  let items = [...all];
  if (scope === "folder") {
    const dir = dirOfAbsPath(absPathOfItem(item));
    items = items.filter(x => dirOfAbsPath(absPathOfItem(x)) === dir);
  } else if (scope === "share") {
    items = items.filter(x => x.shareLabel === item.shareLabel);
  }

  items.sort((a, b) => String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" }));

  const index = items.findIndex(x => x.id === item.id);
  if (index < 0) return { items: [], index: -1, playOrder: [], playIndex: -1 };

  const isShuffle = shuffle !== null ? shuffle : state.playlist.shuffle;
  const playOrder = generatePlayOrder(items.length, index, isShuffle);
  const playIndex = playOrder.findIndex(idx => idx === index);

  return { items, index, playOrder, playIndex };
}

export function rebuildPlayOrderFromCurrent(shuffle) {
  const pl = state.playlist;
  if (!pl.items.length) return;

  const currentPlayIndex = pl.playIndex;
  const currentItemIndex = pl.playOrder[currentPlayIndex] ?? pl.index;

  const newOrder = Array.from({ length: pl.items.length }, (_, i) => i);

  if (shuffle && pl.items.length > 1) {
    if (currentItemIndex >= 0 && currentItemIndex < pl.items.length) {
      newOrder[currentPlayIndex] = currentItemIndex;

      const remaining = newOrder.filter((_, i) => i !== currentPlayIndex);
      for (let i = remaining.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [remaining[i], remaining[j]] = [remaining[j], remaining[i]];
      }

      let ri = 0;
      for (let i = 0; i < newOrder.length; i++) {
        if (i !== currentPlayIndex) {
          newOrder[i] = remaining[ri++];
        }
      }
    }
  }

  pl.playOrder = newOrder;
}

export function setPlaylist(kind, items, index, playOrder = null, playIndex = -1) {
  state.playlist.kind = kind;
  state.playlist.items = Array.isArray(items) ? items : [];
  state.playlist.index = Number.isFinite(index) ? index : -1;
  state.playlist.playOrder = Array.isArray(playOrder) ? playOrder : [];
  state.playlist.playIndex = Number.isFinite(playIndex) ? playIndex : -1;
  updateNavButtons();
  updateNavLabels();
  logRemote("info", `Playlist updated: kind=${kind} count=${items?.length} index=${index} playIndex=${playIndex}`);
  bus.emit('playlist:updated');
}
