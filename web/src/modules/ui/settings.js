import { state, el, lsSet, LS } from '../state.js';
import { t } from '../i18n.js';
import { getCfg } from '../utils.js';
import { gpGet, gpSet } from '../api.js';
import { setPlaylist, renderPlaylist, updateNavButtons, rebuildPlayOrderFromCurrent } from '../playlist.js';
import { setFitBtnVisible } from '../player.js';
import { renderList } from './render.js';
import { renderShares, updateBlacklistUI } from './shares.js';

export function applyConfigToUI() {
  const showOthers = !!getCfg("ui.showOthers", false);
  const otherTab = el("tabOther");
  if (otherTab) {
    otherTab.hidden = !showOthers;
    if (!showOthers && state.tab === "other") {
      state.tab = getCfg("ui.defaultTab", "video");
    }
  }

  const playlistEnabled = !!getCfg("features.playlist", true);
  const playlistPanel = el("playlistPanel");
  if (playlistPanel) playlistPanel.hidden = !playlistEnabled;
  const prev = el("btnPrev");
  const next = el("btnNext");
  const shuffleWrap = el("shuffleWrap");
  if (prev) prev.hidden = !playlistEnabled;
  if (next) next.hidden = !playlistEnabled;
  if (shuffleWrap) shuffleWrap.hidden = !playlistEnabled || state.current?.kind !== "audio";
  if (!playlistEnabled) {
    setPlaylist(null, [], -1);
  }

  const defTab = getCfg("ui.defaultTab", "video");
  if (defTab === "video" || defTab === "audio" || defTab === "image" || (defTab === "other" && showOthers)) {
    state.tab = defTab;
  }

  let shuffle = false;
  {
    const saved = gpGet(LS.audioShuffle);
    if (saved === "1") shuffle = true;
    else if (saved === "0") shuffle = false;
    else shuffle = !!getCfg("playback.audio.shuffle", false);
  }
  state.playlist.shuffle = shuffle;
  const toggleShuffle = el("toggleShuffle");
  if (toggleShuffle) toggleShuffle.checked = shuffle;

  let loop = false;
  {
    const saved = gpGet(LS.audioLoop);
    loop = saved === "1";
  }
  state.playlist.loop = loop;
  const toggleLoop = el("toggleLoop");
  if (toggleLoop) toggleLoop.checked = loop;

  const tabs = Array.from(document.querySelectorAll(".tab"));
  for (const x of tabs) x.classList.toggle("tab--active", x.getAttribute("data-tab") === state.tab);
}
