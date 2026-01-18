/**
 * Global state and localStorage helpers
 */

export const el = (id) => document.getElementById(id);

export const LS = {
  audioLastID: 'msp.audio.lastId',
  audioLastTime: 'msp.audio.lastTime',
  audioShuffle: 'msp.audio.shuffle',
  audioLoop: 'msp.audio.loop',
  videoLastID: 'msp.video.lastId',
  videoLastTime: 'msp.video.lastTime',
  imageLastID: 'msp.image.lastId',
  lastActiveKind: 'msp.lastActiveKind',
  mediaETag: 'msp.media.etag',
  theme: 'msp.theme',
  lang: 'msp.lang',
  volume: 'msp.volume',
  playlist: 'msp.playlist',
  sortField: 'msp.sort.field',
  sortOrder: 'msp.sort.order',
};

export const state = {
  lang: 'en',
  config: null,
  media: null,
  tab: 'video',
  q: '',
  current: null,
  currentMetaBase: '',
  plyr: null,
  lyrics: null,
  prefs: {},
  plyrPersistTimer: 0,
  selectionToken: 0,
  playlist: {
    kind: null,
    items: [],
    index: -1,
    shuffle: false,
    loop: false,
  },
  listPageSize: 10,
  listPage: 1,
  plPageSize: 10,
  plPage: 1,
  isSwitchingMedia: false,
  sort: {
    field: 'name',
    order: 1,
  },
  scanning: false,
};

// Initialize sort from LS
try {
  const sf = lsGet(LS.sortField);
  if (sf) state.sort.field = sf;
  const so = lsGet(LS.sortOrder);
  if (so) state.sort.order = Number(so) || 1;
} catch {}

export function canStorage() {
  try {
    const k = '__msp__probe__';
    localStorage.setItem(k, '1');
    localStorage.removeItem(k);
    return true;
  } catch {
    return false;
  }
}

export function lsGet(k) {
  try {
    return localStorage.getItem(k);
  } catch {
    return null;
  }
}

export function lsSet(k, v) {
  try {
    localStorage.setItem(k, v);
  } catch {}
}
