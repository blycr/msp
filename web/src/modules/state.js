/**
 * Global state and localStorage helpers
 */

/**
 * @typedef {Object} PlaylistState
 * @property {?string} kind - Current playlist type (video, audio, etc)
 * @property {Array<Object>} items - List of media items
 * @property {number} index - Current index in items
 * @property {boolean} shuffle - Shuffle mode enabled
 * @property {boolean} loop - Loop mode enabled
 */

/**
 * @typedef {Object} AppState
 * @property {string} lang - Current language code (e.g. 'en')
 * @property {?Object} config - Server configuration object
 * @property {?Object} media - Media library response
 * @property {string} tab - Current active sidebar tab
 * @property {string} q - Search query
 * @property {?Object} current - Currently playing media item
 * @property {string} currentMetaBase - Base metadata string
 * @property {?Object} plyr - Plyr instance
 * @property {?Object} lyrics - Lyrics object
 * @property {Object} prefs - User preferences
 * @property {number} plyrPersistTimer - Timer ID for persistence
 * @property {number} selectionToken - Selection consistency token
 * @property {PlaylistState} playlist - Playlist state management
 * @property {number} listPageSize - Items per page in file list
 * @property {number} listPage - Current file list page
 * @property {number} plPageSize - Items per page in playlist
 * @property {number} plPage - Current playlist page
 * @property {boolean} isSwitchingMedia - Lock flag during transitions
 * @property {{field: string, order: number}} sort - Sort settings
 * @property {boolean} scanning - Whether a scan is in progress
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

/** @type {AppState} */
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
