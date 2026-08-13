import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { logRemote } from '../api.js';
import { getCfg, playlistFolderKey } from '../utils.js';
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
    return;
  }

  // 已播完一整轮。
  if (pl.loop) {
    if (pl.shuffle && pl.items.length > 1) {
      // 随机循环：重新洗牌生成新一轮（洗牌包耗尽即重洗），
      // 并避免上一轮末尾立即成为新一轮首位。
      const lastPlayedIndex = pl.playOrder[pl.playOrder.length - 1];
      pl.playOrder = reshuffleForNextPass(pl.items.length, lastPlayedIndex, true);
      playAtIndex(0, autoplay, true);
    } else {
      // 顺序循环：回到第一项。
      playAtIndex(0, autoplay, true);
    }
  }
  // loop=false 且已到末尾：保持停在最后一首，不自动续播。
}

// generateShuffledOrder 构造一个洗牌包（shuffle bag）：一个全量随机排列，
// 每个下标在一轮内恰好出现一次、绝不重复。这是“假随机”的标准实现——
// 比纯 Math.random() 抽样更能保证“一整轮内不重复听到同一首”，正好满足
// 用户“随机播放但不想重复”的预期。
//
// startIndex 项被钉在 order[0]（当前歌先播），其余用 Fisher-Yates 洗牌。
function generateShuffledOrder(itemCount, startIndex) {
  const order = Array.from({ length: itemCount }, (_, i) => i);
  if (itemCount <= 1) return order;

  // 把起始项放到最前，保证从当前歌开始播。
  if (startIndex > 0 && startIndex < itemCount) {
    [order[0], order[startIndex]] = [order[startIndex], order[0]];
  }

  // 对 order[1..] 做 Fisher-Yates 洗牌；order[0] 钉住不动。
  for (let i = itemCount - 1; i > 1; i--) {
    const j = Math.floor(Math.random() * i) + 1;
    [order[i], order[j]] = [order[j], order[i]];
  }

  return order;
}

// reshuffleForNextPass 在一整轮播放结束、需要续播时生成新一轮随机排列。
// 关键：避免上一轮末尾项立即成为新一轮首位（常见的“刚刚听过又来”体验缺陷）。
// lastPlayedIndex 是上一轮最后播放的 items 下标。
function reshuffleForNextPass(itemCount, lastPlayedIndex, shuffle) {
  if (!shuffle || itemCount <= 1) {
   return Array.from({ length: itemCount }, (_, i) => i);
  }
  let order = generateShuffledOrder(itemCount, 0);
  // 若首位恰好是上一轮末尾，与第二位交换，避免立即重复（count>2 才有意义）。
  if (itemCount > 2 && lastPlayedIndex >= 0 && lastPlayedIndex < itemCount && order[0] === lastPlayedIndex) {
    [order[0], order[1]] = [order[1], order[0]];
  }
  return order;
}

export function generatePlayOrder(itemCount, startIndex, shuffle) {
  if (!shuffle) return Array.from({ length: itemCount }, (_, i) => i);
  return generateShuffledOrder(itemCount, startIndex);
}

export function buildPlaylist(item, kind, shuffle = null) {
  const scope = getCfg(`playback.${kind}.scope`, kind === "audio" ? "all" : "folder");
  const poolMap = { video: "videos", audio: "audios", image: "images" };
  const all = state.media?.[poolMap[kind]] || [];
  if (!all.length) return { items: [], index: -1, playOrder: [], playIndex: -1 };

  let items = [...all];
  if (scope === "folder") {
    const key = playlistFolderKey(item);
    items = items.filter(x => playlistFolderKey(x) === key);
  } else if (scope === "share") {
    items = items.filter(x => x.shareLabel === item.shareLabel);
  }

  items.sort((a, b) => String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" }));

  const index = items.findIndex(x => x.id === item.id);
  if (index < 0) return { items: [], index: -1, playOrder: [], playIndex: -1 };

  // 随机播放仅对 audio 生效（视频/图片通常按目录顺序观看）。这从源头消除原
  // "state.playlist.shuffle 为全局态、buildPlaylist 对所有类型都读它、但 UI
  // 只在 audio 暴露开关" 导致的状态泄漏——video/image 会被悄悄随机化又无法切换。
  // 显式传入 shuffle 参数时以参数为准（resume 等场景按当前类型决定）。
  const isShuffle = shuffle !== null ? shuffle : (kind === "audio" && state.playlist.shuffle);
  const playOrder = generatePlayOrder(items.length, index, isShuffle);
  const playIndex = playOrder.findIndex(idx => idx === index);

  return { items, index, playOrder, playIndex };
}

export function rebuildPlayOrderFromCurrent(shuffle) {
  const pl = state.playlist;
  if (!pl.items.length) return;

  const currentPlayIndex = pl.playIndex;
  const currentItemIndex = pl.playOrder[currentPlayIndex] ?? pl.index;

  if (!shuffle) {
    // 关闭随机：恢复顺序播放，游标定位到当前项。
    pl.playOrder = Array.from({ length: pl.items.length }, (_, i) => i);
    if (currentItemIndex >= 0 && currentItemIndex < pl.items.length) {
      pl.playIndex = currentItemIndex;
    }
    return;
  }

  // 开启随机：用洗牌包生成全量随机排列，当前项钉在当前游标位置不打断播放。
  const newOrder = Array.from({ length: pl.items.length }, (_, i) => i);
  if (pl.items.length > 1 && currentItemIndex >= 0 && currentItemIndex < pl.items.length) {
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
