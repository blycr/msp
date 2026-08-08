import { bus } from '../eventbus.js';

const VIEW_IDS = {
  stage: 'stagePanel',
  playlist: 'playlistPanel',
  sidebar: 'filesPanel',
};

let activeView = 'stage';
let bound = false;
let mobileQuery = null;

function isMobile() {
  return mobileQuery?.matches ?? window.matchMedia('(max-width: 1024px)').matches;
}

function getPanel(view) {
  const id = VIEW_IDS[view];
  return id ? document.getElementById(id) : null;
}

export function updateMobileNav() {
  const nav = document.querySelector('.mobile-nav');
  if (!nav) return;

  const mobile = isMobile();
  const buttons = Array.from(nav.querySelectorAll('[data-mobile-view]'));

  for (const button of buttons) {
    const view = button.dataset.mobileView;
    const panel = getPanel(view);
    const available = !!panel && !panel.hidden;
    button.hidden = !available;

    const active = available && (!mobile || view === activeView);
    button.classList.toggle('mobile-nav__item--active', active && mobile);
    button.setAttribute('aria-selected', String(active && mobile));

    if (panel) {
      panel.classList.toggle('mobile-panel--active', activeView === view && mobile && available);
      panel.setAttribute('aria-hidden', String(!active));
    }
  }

  const activePanel = getPanel(activeView);
  if (mobile && (!activePanel || activePanel.hidden)) {
    activeView = 'stage';
    updateMobileNav();
  }
}

export function setMobileView(view) {
  if (!VIEW_IDS[view] || getPanel(view)?.hidden) return;
  activeView = view;
  updateMobileNav();
}

export function bindMobileNav() {
  if (bound) return;
  bound = true;

  mobileQuery = window.matchMedia('(max-width: 1024px)');
  const onViewportChange = () => updateMobileNav();
  if (mobileQuery.addEventListener) {
    mobileQuery.addEventListener('change', onViewportChange);
  } else {
    mobileQuery.addListener(onViewportChange);
  }

  for (const button of document.querySelectorAll('.mobile-nav [data-mobile-view]')) {
    button.addEventListener('click', () => setMobileView(button.dataset.mobileView));
  }

  // Selecting a file should take the user back to the result they just chose.
  bus.on('play:request', () => {
    if (isMobile()) setMobileView('stage');
  });

  updateMobileNav();
}
