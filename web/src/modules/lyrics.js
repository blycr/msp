import { state, el } from './state.js';
import { t } from './i18n.js';

export function resetLyrics() {
  state.lyrics = null;
  el("lyrics").innerHTML = "";
}

export function parseLrc(text) {
  const s = String(text || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const out = [];
  for (const line of s.split("\n")) {
    const matches = [...line.matchAll(/\[(\d{1,2}):(\d{2})(?:[.,](\d{1,3}))?\]/g)];
    if (matches.length === 0) continue;
    const content = line.replace(/\[[^\]]+\]/g, "").trim();
    for (const m of matches) {
      const mm = Number(m[1] || 0);
      const ss = Number(m[2] || 0);
      const frac = m[3] ? Number(String(m[3]).padEnd(3, "0")) : 0;
      const t = mm * 60 + ss + frac / 1000;
      if (Number.isFinite(t)) out.push({ t, text: content });
    }
  }
  out.sort((a, b) => a.t - b.t);
  return out;
}

export function renderLyrics(lines) {
  const box = el("lyrics");
  box.innerHTML = "";
  if (!Array.isArray(lines) || lines.length === 0) {
    const emptyDiv = document.createElement("div");
    emptyDiv.className = "lyrics-empty";
    emptyDiv.textContent = t("lyrics_pure_music");
    box.appendChild(emptyDiv);
    if (state.lyrics) state.lyrics.nodes = [];
    return;
  }
  const frag = document.createDocumentFragment();
  const nodes = [];
  for (const ln of lines) {
    const div = document.createElement("div");
    div.className = "ly";
    div.dataset.t = String(ln.t);
    div.textContent = ln.text || "";
    frag.appendChild(div);
    nodes.push(div);
  }
  box.appendChild(frag);
  if (state.lyrics) state.lyrics.nodes = nodes;
}

export function updateLyricsByTime(t, force) {
  const lines = state.lyrics?.lines || [];
  if (lines.length === 0) return;
  const box = el("lyrics");
  const nodes = state.lyrics?.nodes || Array.from(box.querySelectorAll(".ly"));
  if (nodes.length === 0) return;

  let idx = 0;
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].t <= t + 0.05) idx = i;
    else break;
  }
  if (!force && state.lyrics?.activeIndex === idx) return;
  state.lyrics.activeIndex = idx;

  for (let i = 0; i < nodes.length; i++) {
    nodes[i].classList.toggle("ly--active", i === idx);
  }

  const active = nodes[idx];
  if (active) {
    // offsetTop is measured from the nearest positioned ancestor (.playerbox),
    // not from the lyrics box, so use rects for a layout-independent position.
    const lineTop = active.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop;
    const top = lineTop - (box.clientHeight - active.offsetHeight) / 2;
    box.scrollTo({ top: Math.max(0, top), behavior: "smooth" });
  }
}
