import { el, lsGet, lsSet, LS } from './state.js';
import { createSunIcon, createMoonIcon } from './icons.js';

export function initTheme() {
  const btn = el("themeBtn");
  if (!btn) return;

  const saved = lsGet(LS.theme);
  const systemDark = window.matchMedia("(prefers-color-scheme: dark)");

  const updateTheme = (isDark) => {
    document.documentElement.setAttribute("data-theme", isDark ? "dark" : "light");
    btn.innerHTML = isDark ? createMoonIcon() : createSunIcon();
  };

  const getAutoTheme = () => {
    const hour = new Date().getHours();
    const isNight = hour < 6 || hour >= 18;
    return isNight || systemDark.matches;
  };

  // Initial set
  if (saved === "dark") {
    updateTheme(true);
  } else if (saved === "light") {
    updateTheme(false);
  } else {
    updateTheme(getAutoTheme());
  }

  // Toggle handler
  btn.addEventListener("click", () => {
    const isDark = document.documentElement.getAttribute("data-theme") === "dark";
    const next = !isDark;
    const apply = () => {
      updateTheme(next);
      lsSet(LS.theme, next ? "dark" : "light");
    };
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduce) { apply(); return; }
    if (document.startViewTransition) {
      document.startViewTransition(apply);
    } else {
      document.documentElement.classList.add("theme-fade");
      apply();
      setTimeout(() => document.documentElement.classList.remove("theme-fade"), 300);
    }
  });

  // System preference listener (only if no manual override)
  systemDark.addEventListener("change", (e) => {
    if (!lsGet(LS.theme)) {
      const next = e.matches || (new Date().getHours() < 6 || new Date().getHours() >= 18);
      const apply = () => updateTheme(next);
      const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (reduce) { apply(); return; }
      if (document.startViewTransition) {
        document.startViewTransition(apply);
      } else {
        document.documentElement.classList.add("theme-fade");
        apply();
        setTimeout(() => document.documentElement.classList.remove("theme-fade"), 300);
      }
    }
  });
}
