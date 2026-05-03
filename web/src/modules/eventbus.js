const handlers = {};

const bus = {
  on(event, fn) {
    if (!handlers[event]) handlers[event] = [];
    handlers[event].push(fn);
  },

  off(event, fn) {
    if (!handlers[event]) return;
    handlers[event] = handlers[event].filter(f => f !== fn);
  },

  emit(event, ...args) {
    if (!handlers[event]) return;
    for (const fn of handlers[event]) {
      try { fn(...args); } catch (e) { console.error(`[bus] Error in handler for "${event}":`, e); }
    }
  },
};

export { bus };