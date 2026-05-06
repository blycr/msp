import { state } from '../state.js';
import { t } from '../i18n.js';

export function currentList() {
  if (!state.media) return [];
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
    return list.filter(x => {
      const name = x.name || "";
      const m = pinyinPro.match(name, q);
      if (m) return true;
      return name.toLowerCase().includes(q.toLowerCase()) || (x.shareLabel || "").toLowerCase().includes(q.toLowerCase());
    });
  }

  const lower = q.toLowerCase();
  return list.filter(x => (x.name || "").toLowerCase().includes(lower) || (x.shareLabel || "").toLowerCase().includes(lower));
}
