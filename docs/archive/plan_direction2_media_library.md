# 方向二：媒体库体验增强 — 实施计划

> **目标读者**：执行此计划的 AI。
> **兜底原则**：如果具体实现方案行不通，自由调整，只要最终效果一致。

---

## 全局规则

1. 构建验证：`cd web && bun run build && cd .. && go test ./... && go vet ./...`
2. Go 风格：遵循 handler 模式（writeJSON/writeError）
3. i18n：所有文案同时添加 en/zh 到 `web/src/modules/i18n.js`
4. 测试：后端改动必须有单元测试

---

## 特性 2A：文件夹层级浏览

**场景**：500+ 视频的扁平列表效率低。电视剧按季/集分目录存放，需要按目录树浏览。
**复杂度**：Standard — 纯前端路径分组 + 最小后端改动。

### 2A.1 前端：状态扩展

**文件**：`web/src/modules/state.js`

在 `state` 对象中添加：
```js
browseMode: 'flat',    // 'flat' | 'folder'
currentFolder: null,   // 当前浏览的文件夹路径前缀（如 "Movies/Action"）
```

### 2A.2 前端：从 MediaItem 提取文件夹结构

**文件**：新建 `web/src/modules/folder.js`

```js
/**
 * 从扁平的 MediaItem 列表中提取文件夹树。
 * 每个 MediaItem 都有 shareLabel 和 name。
 * name 的格式是 "SubDir/SubDir2/filename.ext"（由 scanner 生成的相对路径）。
 *
 * 返回当前 currentFolder 下的子文件夹列表和文件列表。
 */
export function getFolderContents(items, currentFolder) {
  // currentFolder 为 null 时，显示顶级：各 shareLabel 作为根文件夹
  // currentFolder 格式: "shareLabel" 或 "shareLabel/subdir/subdir2"

  if (!currentFolder) {
    // 按 shareLabel 分组作为顶级文件夹
    const labels = new Set(items.map(i => i.shareLabel).filter(Boolean));
    return {
      folders: [...labels].sort().map(l => ({ name: l, path: l, count: items.filter(i => i.shareLabel === l).length })),
      files: [],
    };
  }

  const parts = currentFolder.split('/');
  const shareLabel = parts[0];
  const subPath = parts.slice(1).join('/');

  // 筛选属于该 share 的项目
  const shareItems = items.filter(i => i.shareLabel === shareLabel);

  // name 字段包含相对于 share root 的路径
  // 需要检查实际 name 格式 —— 可能是 "filename.ext" 或 "subdir/filename.ext"
  // **兜底**：如果 name 不含路径分隔符，说明 scanner 只返回文件名。
  // 此时需要检查 MediaItem 是否有其他字段携带目录信息。
  // 如果没有，需要后端改动（见 2A.3）。

  const folderSet = new Set();
  const files = [];

  for (const item of shareItems) {
    // 假设 item 有 relPath 字段（见 2A.3）
    const rel = item.relPath || item.name;
    if (!subPath) {
      // 在 share 根目录
      const slash = rel.indexOf('/');
      if (slash > 0) {
        folderSet.add(rel.substring(0, slash));
      } else {
        files.push(item);
      }
    } else {
      if (!rel.startsWith(subPath + '/')) continue;
      const rest = rel.substring(subPath.length + 1);
      const slash = rest.indexOf('/');
      if (slash > 0) {
        folderSet.add(rest.substring(0, slash));
      } else {
        files.push(item);
      }
    }
  }

  const folders = [...folderSet].sort().map(name => ({
    name,
    path: currentFolder + '/' + name,
    count: 0, // 可选：递归计数
  }));

  return { folders, files };
}
```

> **兜底**：如果 `MediaItem.name` 只是文件名而非相对路径，则需要后端在扫描时增加 `relPath` 字段（见 2A.3）。先检查实际数据。

### 2A.3 后端：MediaItem 增加 relPath 字段

**文件**：`internal/domain/types.go`

检查 `MediaItem` 的 `Name` 字段是否包含目录路径。查看 `internal/scanner/scanner.go` 中 `buildMediaItem` 是如何设置 `Name` 的。

**如果 Name 只是文件名**（无目录路径），在 `MediaItem` 中添加：
```go
RelPath string `json:"relPath,omitempty" gorm:"column:rel_path"`
```

**文件**：`internal/scanner/scanner.go`

在 `buildMediaItem` 或 `shareWalker` 中，计算相对于 share root 的路径：
```go
// relPath = filepath.Rel(shareRoot, filePath) 得到的目录部分
relDir, _ := filepath.Rel(shareRoot, filepath.Dir(absPath))
if relDir == "." {
    item.RelPath = item.Name // 根目录下的文件
} else {
    item.RelPath = filepath.ToSlash(relDir) + "/" + item.Name
}
```

> **兜底**：如果 Name 已经包含了相对路径（检查 `buildMediaItem` 逻辑），则跳过此步，前端直接使用 `item.name`。GORM AutoMigrate 会自动添加新列。

### 2A.4 前端：文件夹浏览渲染

**文件**：`web/src/modules/ui/render.js`

在 `renderList()` 函数中，`renderContinueWatching(box)` 之后，当前文件列表渲染之前，增加分支：

```js
// 在 renderList 中，构建 pageItems 之前：
if (state.browseMode === 'folder') {
  renderFolderView(box);
  return; // 文件夹模式单独渲染，不走分页列表逻辑
}
```

新增函数：
```js
import { getFolderContents } from '../folder.js';

function renderFolderView(box) {
  const allItems = [
    ...(state.media?.videos || []),
    ...(state.media?.audios || []),
    ...(state.media?.images || []),
    ...(state.media?.others || []),
  ];
  const { folders, files } = getFolderContents(allItems, state.currentFolder);

  // 面包屑导航
  if (state.currentFolder) {
    const breadcrumb = document.createElement('div');
    breadcrumb.className = 'folder-breadcrumb';
    const parts = state.currentFolder.split('/');
    // "← 返回" 按钮
    const backBtn = document.createElement('button');
    backBtn.className = 'btn btn--ghost folder-back';
    backBtn.textContent = '← ' + t('folder_back');
    backBtn.addEventListener('click', () => {
      if (parts.length <= 1) {
        state.currentFolder = null;
      } else {
        state.currentFolder = parts.slice(0, -1).join('/');
      }
      renderList();
    });
    breadcrumb.appendChild(backBtn);
    // 当前路径
    const pathSpan = document.createElement('span');
    pathSpan.className = 'folder-breadcrumb__path';
    pathSpan.textContent = state.currentFolder;
    breadcrumb.appendChild(pathSpan);
    box.appendChild(breadcrumb);
  }

  // 渲染文件夹
  for (const folder of folders) {
    const row = document.createElement('div');
    row.className = 'item item--folder';
    row.addEventListener('click', () => {
      state.currentFolder = folder.path;
      renderList();
    });
    const icon = document.createElement('span');
    icon.className = 'folder-icon';
    icon.textContent = '📁';
    const name = document.createElement('div');
    name.className = 'item__name';
    name.textContent = folder.name;
    const count = document.createElement('div');
    count.className = 'item__sub';
    count.textContent = folder.count > 0 ? `${folder.count} ${t('folder_items')}` : '';
    const main = document.createElement('div');
    main.className = 'item__main';
    main.appendChild(name);
    main.appendChild(count);
    row.appendChild(icon);
    row.appendChild(main);
    box.appendChild(row);
  }

  // 渲染当前目录下的文件（复用现有文件项渲染逻辑）
  for (const item of files) {
    // 复用 renderList 中的文件项 DOM 构建逻辑
    const row = document.createElement("div");
    row.className = "item";
    row.addEventListener("click", () => bus.emit('play:request', item, { user: true, autoplay: true }));
    if (item.kind === "video") {
      const thumb = document.createElement("img");
      thumb.className = "file-thumb";
      thumb.src = `/api/thumbnail?id=${encodeURIComponent(item.id)}`;
      thumb.loading = "lazy";
      thumb.alt = "";
      row.appendChild(thumb);
    }
    const main = document.createElement("div");
    main.className = "item__main";
    const nameEl = document.createElement("div");
    nameEl.className = "item__name";
    nameEl.textContent = formatName(item);
    const sub = document.createElement("div");
    sub.className = "item__sub";
    sub.textContent = `${formatBytes(item.size)}  ·  ${formatTime(item.modTime)}`;
    main.appendChild(nameEl);
    main.appendChild(sub);
    const badge = document.createElement("div");
    badge.className = "badge";
    badge.textContent = (item.ext || "").replace(".", "").toUpperCase();
    row.appendChild(main);
    row.appendChild(badge);
    box.appendChild(row);
  }

  // 更新 hint
  const hint = el("hint");
  hint.textContent = `${folders.length} ${t('folder_folders')} · ${files.length} ${t('folder_files')}`;
}
```

### 2A.5 前端：模式切换按钮

**文件**：`web/src/modules/ui/bindings.js`

在 `bindUI()` 中添加模式切换逻辑。在排序控件附近添加：

```js
// 文件夹/扁平模式切换
const browseModeBtn = el('browseMode');
if (browseModeBtn) {
  browseModeBtn.addEventListener('click', () => {
    state.browseMode = state.browseMode === 'flat' ? 'folder' : 'flat';
    state.currentFolder = null;
    browseModeBtn.textContent = state.browseMode === 'flat' ? t('mode_folder') : t('mode_flat');
    renderList();
  });
}
```

**文件**：`web/index.html`

在排序区域（`sortField` dropdown 附近）增加按钮：
```html
<button id="browseMode" class="btn btn--ghost btn--sm" data-i18n="mode_folder">Folder</button>
```

> **兜底**：如果 HTML 结构中没有合适的位置，在排序控件的同一行 `.controls` 容器中追加。

### 2A.6 前端：样式

**文件**：`web/src/styles/components/list.css`（追加）

```css
/* Folder Browse */
.item--folder { cursor: pointer; }
.folder-icon { font-size: 20px; margin-right: 8px; flex-shrink: 0; }
.folder-breadcrumb {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 12px; font-size: 12px;
  color: var(--md-on-surface-variant);
  border-bottom: 1px solid var(--md-border);
  margin-bottom: 4px;
}
.folder-breadcrumb__path {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.folder-back { font-size: 12px; padding: 2px 8px; }
```

### 2A.7 前端：i18n

**文件**：`web/src/modules/i18n.js`

en:
```js
mode_folder: "Folder",
mode_flat: "Flat",
folder_back: "Back",
folder_items: "items",
folder_folders: "folders",
folder_files: "files",
```

zh:
```js
mode_folder: "文件夹",
mode_flat: "平铺",
folder_back: "返回",
folder_items: "项",
folder_folders: "个文件夹",
folder_files: "个文件",
```

### 2A.8 验证清单

- [ ] `go test ./...` 通过（如有后端改动）
- [ ] `cd web && bun run build` 通过
- [ ] 点击 "Folder" 按钮切换到文件夹模式
- [ ] 按 shareLabel 分组显示顶级文件夹
- [ ] 点击文件夹进入子目录，面包屑显示路径
- [ ] 点击"返回"回到上级目录
- [ ] 搜索在文件夹模式下仍然可用
- [ ] 再次点击切回平铺模式

---

## 特性 2B：收藏/标记

**场景**：用户想"记住想看的"。
**复杂度**：Light — SQLite 新表 + 前端星标按钮。

### 2B.1 后端：新增 Favorite 领域类型

**文件**：`internal/domain/types.go`

追加：
```go
type Favorite struct {
    MediaID   string    `json:"mediaId" gorm:"primaryKey"`
    CreatedAt time.Time `json:"createdAt"`
}
```

### 2B.2 后端：存储层

**文件**：`internal/storage/interface.go`

新增接口：
```go
type FavoriteStore interface {
    ListFavorites(ctx context.Context) ([]domain.Favorite, error)
    AddFavorite(ctx context.Context, mediaID string) error
    RemoveFavorite(ctx context.Context, mediaID string) error
    IsFavorite(ctx context.Context, mediaID string) (bool, error)
}
```

**文件**：`internal/storage/sqlite.go`

1. 在 `InitSQLite` 的 `AutoMigrate` 中添加 `&domain.Favorite{}`
2. 实现四个方法：

```go
func (s *SQLite) ListFavorites(ctx context.Context) ([]domain.Favorite, error) {
    dbConn, ok := s.guard("ListFavorites")
    if !ok { return nil, nil }
    var items []domain.Favorite
    err := dbConn.WithContext(ctx).Order("created_at DESC").Find(&items).Error
    return items, err
}

func (s *SQLite) AddFavorite(ctx context.Context, mediaID string) error {
    dbConn, ok := s.guard("AddFavorite")
    if !ok { return nil }
    return dbConn.WithContext(ctx).
        Where(domain.Favorite{MediaID: mediaID}).
        FirstOrCreate(&domain.Favorite{MediaID: mediaID}).Error
}

func (s *SQLite) RemoveFavorite(ctx context.Context, mediaID string) error {
    dbConn, ok := s.guard("RemoveFavorite")
    if !ok { return nil }
    return dbConn.WithContext(ctx).Delete(&domain.Favorite{}, "media_id = ?", mediaID).Error
}

func (s *SQLite) IsFavorite(ctx context.Context, mediaID string) (bool, error) {
    dbConn, ok := s.guard("IsFavorite")
    if !ok { return false, nil }
    var count int64
    err := dbConn.WithContext(ctx).Model(&domain.Favorite{}).Where("media_id = ?", mediaID).Count(&count).Error
    return count > 0, err
}
```

### 2B.3 后端：Handler

**文件**：新建 `internal/handler/favorite.go`

```go
package handler

import (
    "net/http"
)

// HandleFavorites handles GET/POST/DELETE /api/favorites
func (h *Handler) HandleFavorites(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        items, err := h.favorites.ListFavorites(r.Context())
        if err != nil {
            writeError(w, http.StatusInternalServerError, "failed to list favorites")
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"items": items})

    case http.MethodPost:
        var req struct {
            MediaID string `json:"mediaId"`
        }
        if err := decodeJSONBody(w, r, &req, defaultJSONBodyLimit); err != nil {
            writeJSONDecodeError(w, err)
            return
        }
        if req.MediaID == "" {
            writeError(w, http.StatusBadRequest, "missing mediaId")
            return
        }
        if err := h.favorites.AddFavorite(r.Context(), req.MediaID); err != nil {
            writeError(w, http.StatusInternalServerError, "failed to add favorite")
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})

    case http.MethodDelete:
        id := r.URL.Query().Get("id")
        if id == "" {
            writeError(w, http.StatusBadRequest, "missing id")
            return
        }
        if err := h.favorites.RemoveFavorite(r.Context(), id); err != nil {
            writeError(w, http.StatusInternalServerError, "failed to remove favorite")
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})

    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}
```

### 2B.4 后端：注入依赖

**文件**：`internal/handler/handler.go`

在 `Deps` 和 `Handler` 结构体中添加：
```go
// Deps:
Favorites storage.FavoriteStore

// Handler:
favorites storage.FavoriteStore
```

在 `New()` 中赋值：`favorites: deps.Favorites`

**文件**：`cmd/msp/main.go`

注册路由和注入：
```go
// registerRoutes 中：
mux.Handle("/api/favorites", http.HandlerFunc(h.HandleFavorites))

// Deps 中：
Favorites: store,
```

> **兜底**：`Store` 结构体（`storage.NewStore(sq)`）可能需要让 `SQLite` 同时实现 `FavoriteStore`。检查 `Store` 是否是 `SQLite` 的包装，如果是，确保接口断言 `_ storage.FavoriteStore = (*storage.SQLite)(nil)` 在 main.go 中添加。

### 2B.5 前端：API

**文件**：`web/src/modules/api.js`

追加：
```js
export async function loadFavorites() {
  return apiGet('/api/favorites');
}
export async function addFavorite(mediaId) {
  return apiPost('/api/favorites', { mediaId });
}
export async function removeFavorite(mediaId) {
  const res = await fetch(`/api/favorites?id=${encodeURIComponent(mediaId)}`, {
    method: 'DELETE', credentials: 'include',
  });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
}
```

### 2B.6 前端：列表项添加星标

**文件**：`web/src/modules/ui/render.js`

在 `renderList` 中，每个文件项的 badge 之后，追加收藏按钮：

```js
const favBtn = document.createElement('button');
favBtn.className = 'fav-btn' + (state.favoriteIds?.has(item.id) ? ' fav-btn--active' : '');
favBtn.textContent = state.favoriteIds?.has(item.id) ? '★' : '☆';
favBtn.addEventListener('click', async (e) => {
  e.stopPropagation();
  if (state.favoriteIds?.has(item.id)) {
    await removeFavorite(item.id);
    state.favoriteIds.delete(item.id);
  } else {
    await addFavorite(item.id);
    if (!state.favoriteIds) state.favoriteIds = new Set();
    state.favoriteIds.add(item.id);
  }
  renderList();
});
row.appendChild(favBtn);
```

import `addFavorite`, `removeFavorite` at top of file.

### 2B.7 前端：状态 + 启动加载

**文件**：`web/src/modules/state.js` — 添加 `favoriteIds: null`

**文件**：`web/src/modules/actions.js` — 在 boot 中加载收藏：
```js
import { loadFavorites } from './api.js';
// 在 loadRecentProgress 之后：
try {
  const favData = await loadFavorites();
  state.favoriteIds = new Set((favData?.items || []).map(f => f.mediaId));
} catch {}
```

### 2B.8 前端：样式 + i18n

**文件**：`web/src/styles/components/list.css`
```css
.fav-btn {
  background: none; border: none; cursor: pointer;
  font-size: 16px; color: var(--md-on-surface-variant);
  padding: 4px; flex-shrink: 0; transition: color 0.15s;
}
.fav-btn:hover { color: var(--md-primary); }
.fav-btn--active { color: var(--md-primary); }
```

### 2B.9 后端测试

**文件**：新建 `internal/handler/favorite_test.go`
```go
func TestHandleFavorites_Get(t *testing.T) { /* GET 返回 200 */ }
func TestHandleFavorites_PostMissingID(t *testing.T) { /* POST 无 mediaId 返回 400 */ }
func TestHandleFavorites_InvalidMethod(t *testing.T) { /* PUT 返回 405 */ }
```

### 2B.10 验证清单

- [ ] `go test ./...` 通过
- [ ] `cd web && bun run build` 通过
- [ ] 列表项右侧显示 ☆ 按钮
- [ ] 点击变为 ★，刷新后仍然保持
- [ ] 再次点击取消收藏

---

## 特性 2B 补充：收藏聚合视图（Favorites Tab）

**场景**：仅实现"星标标记"是不够的——用户收藏了 20 个文件后，这些文件仍分散在 Video/Audio/Image/Other 各分类中，无法一键找到。收藏功能必须提供一个**跨类型的聚合入口**。
**复杂度**：Light — 纯前端改动，利用已有的 `favoriteIds` 状态。

### 2B补.1 前端：新增 Favorites Tab

**文件**：`web/index.html`

在现有的 tabs 区域（Video / Audio / Image / Other）中插入 Favorites tab：

```html
<div class="tabs" role="tablist">
  <button class="tab tab--active" data-tab="video" data-i18n="tab_video">Video</button>
  <button class="tab" data-tab="audio" data-i18n="tab_audio">Audio</button>
  <button class="tab" data-tab="image" data-i18n="tab_image">Image</button>
  <button class="tab" data-tab="favorites" data-i18n="tab_favorites">Favorites</button>
  <button class="tab" data-tab="other" id="tabOther" hidden data-i18n="tab_other">Other</button>
</div>
```

> **兜底**：如果 HTML 结构已有变化，找到 `.tabs` 容器，在 Image 和 Other 之间追加一个 `data-tab="favorites"` 的按钮即可。

### 2B补.2 前端：currentList 支持跨类型聚合

**文件**：`web/src/modules/playlist/sort-filter.js`

在 `currentList()` 函数中，为 `state.tab === "favorites"` 增加独立分支。该分支聚合所有 kind 的媒体，并过滤出已收藏项：

```js
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
```

> **兜底**：如果 `favoriteIds` 尚未加载（为 `null` 或 `undefined`），直接返回空数组，避免渲染异常。

### 2B补.3 前端：收藏 Tab 下隐藏无关组件

**文件**：`web/src/modules/ui/render.js`

在 `renderList()` 中做两处调整：

1. **隐藏"继续观看"**：收藏 tab 是聚合视图，不需要显示 Continue Watching 区域。
2. **禁用文件夹浏览**：收藏模式下不应进入 folder view，否则收藏的文件会被目录树再次分散。

```js
export function renderList() {
  const box = el("list");
  const hint = el("hint");
  box.innerHTML = "";

  // 收藏 tab 不显示 continue watching
  if (state.tab !== 'favorites') {
    renderContinueWatching(box);
  }
  // ...
  // 收藏 tab 不走文件夹浏览
  if (state.browseMode === 'folder' && state.tab !== 'favorites') {
    renderFolderView(box, hint);
    return;
  }
  // ...
}
```

> **兜底**：如果后续还有其他视图模式（如网格模式），同样应在收藏 tab 下 fallback 到最基础的列表渲染。

### 2B补.4 前端：Tab 点击逻辑适配

**文件**：`web/src/modules/ui/bindings.js`

tab 点击事件中，当切换到 favorites 时，隐藏 Shuffle/Loop 开关（因为跨 kind 的收藏列表不适合播放列表的 shuffle/loop 语义）：

```js
const tabs = Array.from(document.querySelectorAll(".tab"));
for (const tab of tabs) {
  tab.addEventListener("click", () => {
    for (const x of tabs) x.classList.remove("tab--active");
    tab.classList.add("tab--active");
    state.tab = tab.getAttribute("data-tab");
    renderList();
    setFitBtnVisible(state.tab === "video" && state.current?.kind === "video");
    // ...
    // 收藏 tab 下隐藏 shuffle/loop
    const shuffleWrap = el("shuffleWrap");
    if (shuffleWrap) {
      shuffleWrap.hidden = state.tab === "favorites" || state.tab === "other";
    }
  });
}
```

> **兜底**：如果 `shuffleWrap` 元素不存在，跳过即可。

### 2B补.5 前端：i18n

**文件**：`web/src/modules/i18n.js`

在 `en` 和 `zh` 中分别添加：

```js
// en
tab_favorites: "Favorites",

// zh
tab_favorites: "收藏",
```

### 2B补.6 验证清单

- [ ] `cd web && bun run build` 通过
- [ ] 侧边栏出现 Favorites / 收藏 tab
- [ ] 收藏若干文件后，点击 Favorites tab，列表中仅显示已收藏的文件
- [ ] Favorites tab 中支持搜索、排序、分页
- [ ] Favorites tab 中不显示"继续观看"区域
- [ ] Favorites tab 中点击文件夹/平铺切换按钮无效果（或保持平铺列表）
- [ ] 在 Favorites tab 中取消收藏（点击 ★），文件应从列表中消失（因为列表已重新渲染）
- [ ] 切换语言后，Favorites tab 文案正确切换
