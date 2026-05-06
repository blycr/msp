import { bus } from './eventbus.js';
import {
  currentList,
  sortFiles,
  filterFiles,
  navLabelsForKind,
  updateNavLabels,
  updateNavButtons,
  playAtIndex,
  playPrev,
  playNext,
  generatePlayOrder,
  buildPlaylist,
  rebuildPlayOrderFromCurrent,
  setPlaylist,
  scheduleAutoFitPlaylistPageSize,
  getAutoFitState,
  autoFitPlaylistPageSize,
  renderPlaylist,
} from './playlist/index.js';

export {
  currentList,
  sortFiles,
  filterFiles,
  navLabelsForKind,
  updateNavLabels,
  updateNavButtons,
  playAtIndex,
  playPrev,
  playNext,
  generatePlayOrder,
  buildPlaylist,
  rebuildPlayOrderFromCurrent,
  setPlaylist,
  scheduleAutoFitPlaylistPageSize,
  getAutoFitState,
  autoFitPlaylistPageSize,
  renderPlaylist,
};

bus.on('playlist:updated', () => {
  renderPlaylist();
  scheduleAutoFitPlaylistPageSize();
});
