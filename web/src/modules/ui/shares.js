import { state, el } from '../state.js';
import { t } from '../i18n.js';
import { apiPost } from '../api.js';
import { loadMedia } from '../actions.js';

export function renderShares() {
  const list = el("shareList");
  list.innerHTML = "";

  const shares = state.config?.shares || [];
  if (shares.length === 0) {
    const empty = document.createElement("div");
    empty.className = "small";
    empty.textContent = t("shares_empty");
    list.appendChild(empty);
    return;
  }

  for (const sh of shares) {
    const row = document.createElement("div");
    row.className = "share";

    const main = document.createElement("div");
    main.className = "share__main";

    const title = document.createElement("div");
    title.className = "item__name";
    title.textContent = sh.label || "";

    const p = document.createElement("div");
    p.className = "share__path";
    p.textContent = sh.path || "";

    main.appendChild(title);
    main.appendChild(p);

    const btn = document.createElement("button");
    btn.className = "btn btn--ghost";
    btn.textContent = t("remove");
    btn.addEventListener("click", async () => {
      try {
        const data = await apiPost("/api/shares", { op: "remove", path: sh.path });
        state.config = data.config;
        renderShares();
        await loadMedia(true);
      } catch (e) {
        alert(String(e?.message || e));
      }
    });

    row.appendChild(main);
    row.appendChild(btn);
    list.appendChild(row);
  }
}

export function updateBlacklistUI() {
  const bl = state.config?.blacklist || {};
  const blExts = el("blExts");
  const blFiles = el("blFiles");
  const blFolders = el("blFolders");
  const blMinSize = el("blMinSize");
  if (blExts) blExts.value = (bl.extensions || []).join(", ");
  if (blFiles) blFiles.value = (bl.filenames || []).join(", ");
  if (blFolders) blFolders.value = (bl.folders || []).join(", ");
  if (blMinSize) blMinSize.value = bl.sizeRule || "";
}
