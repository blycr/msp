import { state, el } from '../state.js';
import { bus } from '../eventbus.js';
import { addFavorite, removeFavorite } from '../api.js';
import { playAtIndex } from '../playlist.js';
import { renderList, showDlg, getRowItem, setFavBtnState } from './render.js';

// 容器级事件委托：#list / #plList 各绑定一次 click + keydown（模块初始化时），
// 行不再逐个 addEventListener。行通过 data-id / data-path / data-pl-index
// 标识，点击时从 state 实时解析目标，语义与原先逐行绑定一致。

export function initListDelegation() {
  const list = el('list');
  if (list) {
    list.addEventListener('click', onListClick);
    list.addEventListener('keydown', onListKeydown);
  }
  const plList = el('plList');
  if (plList) {
    plList.addEventListener('click', onPlClick);
    plList.addEventListener('keydown', onPlKeydown);
  }
}

// —— 文件列表（#list）——

function onListClick(e) {
  // 收藏按钮优先（它在行内，原生 button 点击会冒泡到行）
  const favBtn = e.target.closest('.fav-btn');
  if (favBtn) {
    if (favBtn.disabled) return;
    const row = favBtn.closest('[data-id]');
    toggleFavorite(row?.dataset.id);
    return;
  }

  if (e.target.closest('.folder-back')) {
    navigateFolderUp();
    return;
  }

  if (e.target.closest('.list-empty__action')) {
    showDlg(true);
    return;
  }

  const folderRow = e.target.closest('[data-path]');
  if (folderRow) {
    state.currentFolder = folderRow.dataset.path;
    renderList();
    return;
  }

  const row = e.target.closest('[data-id]');
  if (row) playFileRow(row);
}

function onListKeydown(e) {
  if (e.key !== 'Enter' && e.key !== ' ') return;
  // 焦点在按钮上时走原生 click（fav / folder-back / empty action / pager）
  if (e.target.closest('button')) return;

  const folderRow = e.target.closest('[data-path]');
  if (folderRow) {
    e.preventDefault();
    state.currentFolder = folderRow.dataset.path;
    renderList();
    return;
  }

  const row = e.target.closest('[data-id]');
  if (row) {
    e.preventDefault();
    playFileRow(row);
  }
}

function playFileRow(row) {
  const item = getRowItem(row.dataset.id);
  if (!item) return;
  bus.emit('play:request', item, { user: true, autoplay: true });
}

function navigateFolderUp() {
  const parts = (state.currentFolder || '').split('/');
  state.currentFolder = parts.length <= 1 ? null : parts.slice(0, -1).join('/');
  renderList();
}

// 收藏 toggle：只更新 state.favoriteIds 与该行的星标状态，不做全量 renderList。
async function toggleFavorite(id) {
  if (!id || state.dbAvailable === false) return;
  const isFav = !!state.favoriteIds?.has(id);
  try {
    if (isFav) {
      await removeFavorite(id);
      state.favoriteIds.delete(id);
    } else {
      await addFavorite(id);
      if (!state.favoriteIds) state.favoriteIds = new Set();
      state.favoriteIds.add(id);
    }
  } catch {
    // 静默失败：星标不变；503 降级由 api 层置 state.dbAvailable 并触发 UI 刷新
    return;
  }
  refreshFavButtons(id, !isFav);
}

function refreshFavButtons(id, isFav) {
  const list = el('list');
  if (!list) return;
  for (const row of list.querySelectorAll('[data-id]')) {
    if (row.dataset.id !== id) continue;
    const btn = row.querySelector('.fav-btn');
    if (btn) setFavBtnState(btn, isFav);
  }
}

// —— 播放列表（#plList）——

function onPlClick(e) {
  const row = e.target.closest('.plitem[data-id]');
  if (row) playPlRow(row);
}

function onPlKeydown(e) {
  if (e.key !== 'Enter' && e.key !== ' ') return;
  if (e.target.closest('button')) return;
  const row = e.target.closest('.plitem[data-id]');
  if (!row) return;
  e.preventDefault();
  playPlRow(row);
}

// 与原逐行绑定相同：playPos 在点击时从当前 playOrder 解析（shuffle 切换后仍正确）。
function playPlRow(row) {
  const i = Number(row.dataset.plIndex);
  if (!Number.isInteger(i) || i < 0) return;
  const playPos = state.playlist.playOrder.findIndex(idx => idx === i);
  playAtIndex(playPos >= 0 ? playPos : i, true, true);
}
