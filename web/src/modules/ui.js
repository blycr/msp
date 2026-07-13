import { bus } from './eventbus.js';
import { resumeLast } from './player.js';
import { bindUI } from './ui/bindings.js';
import { setMeta, showDlg, updateUIForLang, renderList } from './ui/render.js';
import { renderShares, updateBlacklistUI } from './ui/shares.js';
import { applyConfigToUI } from './ui/settings.js';
import { bindMobileNav, updateMobileNav } from './ui/mobile.js';

export { bindUI } from './ui/bindings.js';
export { setMeta, showDlg, updateUIForLang, renderList } from './ui/render.js';
export { renderShares, updateBlacklistUI } from './ui/shares.js';
export { applyConfigToUI } from './ui/settings.js';

bus.on('meta:update', (text) => setMeta(text));
bus.on('config:loaded', () => {
  applyConfigToUI();
  updateMobileNav();
  renderShares();
  updateBlacklistUI();
});
bus.on('media:loaded', () => {
  renderList();
});
bus.on('media:resume', () => {
  resumeLast();
});
bus.on('boot:init', () => {
  bindUI();
  bindMobileNav();
  updateUIForLang();
});
