import { state } from '../state.js';
import { t } from '../i18n.js';

export function currentList() {
  if (!state.media) return [];
  if (state.tab === "favorites") {
    if (!state.favoriteIds?.size) return [];
    const all = [
      ...(state.media.videos || []),
      ...(state.media.audios || []),
      ...(state.media.images || []),
      ...(state.media.others || []),
    ];
    return all.filter(x => state.favoriteIds.has(x.id));
  }
  switch (state.tab) {
    case "video": return state.media.videos || [];
    case "audio": return state.media.audios || [];
    case "image": return state.media.images || [];
    default: return state.media.others || [];
  }
}

function getSortVal(item, field) {
  if (field === "size") return item.size || 0;
  if (field === "date") return item.modTime || 0;
  return String(item.name || "").toLowerCase();
}

export function sortFiles(list) {
  const field = state.sort?.field || "name";
  const order = state.sort?.order || 1;
  return [...list].sort((a, b) => {
    const va = getSortVal(a, field);
    const vb = getSortVal(b, field);
    if (field === "name") {
      return String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" }) * order;
    }
    if (va < vb) return -1 * order;
    if (va > vb) return 1 * order;
    return String(a.name || "").localeCompare(String(b.name || ""), "zh", { numeric: true, sensitivity: "base" });
  });
}

// 拼音匹配缓存：name -> { full, first }（无调全拼连写 + 首字母）。
// 缓存的是每个 name 的可匹配形式而非 match 结果（查询词每次都变）。
// state.media 引用变化时整体清空失效；由于 key 是 name 本身，残留条目也不会造成错误匹配。
const pinyinCache = new Map();
let pinyinCacheMedia = null;

function getPinyinForms(name) {
  let entry = pinyinCache.get(name);
  if (entry) return entry;
  try {
    const { pinyinPro } = window;
    const full = pinyinPro.pinyin(name, { toneType: "none", type: "array" }).join("").toLowerCase();
    const first = pinyinPro.pinyin(name, { pattern: "first", toneType: "none" }).replace(/\s+/g, "").toLowerCase();
    entry = { full, first };
  } catch {
    entry = { full: "", first: "" };
  }
  pinyinCache.set(name, entry);
  return entry;
}

export function filterFiles(list) {
  const q = (state.q || "").trim();
  if (!q) return list;

  if (q.startsWith("/") && q.length > 2) {
    const match = q.match(/^\/(.+)\/([a-z]*)$/);
    if (match) {
      try {
        const re = new RegExp(match[1], match[2] || "i");
        return list.filter(x => re.test(x.name));
      } catch { }
    }
  }

  const { pinyinPro } = window;
  if (pinyinPro) {
    if (state.media !== pinyinCacheMedia) {
      pinyinCache.clear();
      pinyinCacheMedia = state.media;
    }
    const lower = q.toLowerCase();
    const qp = lower.replace(/\s+/g, "");
    return list.filter(x => {
      const name = x.name || "";
      if (name.toLowerCase().includes(lower) || (x.shareLabel || "").toLowerCase().includes(lower)) return true;
      if (!qp) return false;
      const { full, first } = getPinyinForms(name);
      return full.includes(qp) || first.includes(qp);
    });
  }

  const lower = q.toLowerCase();
  return list.filter(x => (x.name || "").toLowerCase().includes(lower) || (x.shareLabel || "").toLowerCase().includes(lower));
}
