// api.js
(() => {
  const A = {};

  const paramQueues = new Map();

  async function requestJSON(url, options = {}) {
    const res = await fetch(url, options);
    const text = await res.text().catch(() => '');
    let data = {};
    if (text) {
      try { data = JSON.parse(text); }
      catch { data = { message: text }; }
    }
    if (!res.ok) {
      throw new Error(data?.error || data?.message || text || `HTTP ${res.status}`);
    }
    return data;
  }

  A.setNumericParam = function setNumericParam(plugin, param, value) {
    return requestJSON('/api/param/set', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugin, param, value }),
    });
  };

  // Serialize writes independently per parameter and keep only the newest pending value.
  // The previous global queue could silently discard a write to parameter B while A was in flight.
  A.setNumericParamQueued = function setNumericParamQueued(plugin, param, value) {
    const key = `${plugin}::${param}`;
    let queue = paramQueues.get(key);
    if (!queue) {
      queue = { busy: false, pending: null, waiters: [] };
      paramQueues.set(key, queue);
    }

    queue.pending = { plugin, param, value };

    return new Promise((resolve, reject) => {
      queue.waiters.push({ resolve, reject });
      if (queue.busy) return;

      queue.busy = true;
      (async () => {
        try {
          while (queue.pending) {
            const req = queue.pending;
            queue.pending = null;
            await A.setNumericParam(req.plugin, req.param, req.value);
          }
          const waiters = queue.waiters.splice(0);
          waiters.forEach(w => w.resolve());
        } catch (err) {
          const waiters = queue.waiters.splice(0);
          waiters.forEach(w => w.reject(err));
          throw err;
        } finally {
          queue.busy = false;
          if (!queue.pending && !queue.waiters.length) paramQueues.delete(key);
        }
      })().catch(err => console.error('SetParam queue failed:', err));
    });
  };

  A.setFileParam = function setFileParam(plugin, param, value) {
    return requestJSON('/api/param/file', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugin, param, value }),
    });
  };

  A.setPluginEnabled = function setPluginEnabled(pluginName, enabled) {
    return requestJSON(`/api/plugins/${encodeURIComponent(pluginName)}/enabled`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    });
  };

  A.setChain = function setChain(chain, plugins) {
    return requestJSON(`/api/chains/${encodeURIComponent(chain)}/set`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugins: plugins || [] }),
    });
  };

  A.releasePlugin = function releasePlugin(pluginName) {
    return requestJSON(`/api/plugins/${encodeURIComponent(pluginName)}/release`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
  };

  A.loadPreset = function loadPreset(name) {
    return requestJSON('/api/preset/load', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
  };

  A.savePreset = function savePreset(name) {
    return requestJSON('/api/preset/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
  };

  A.fetchState = async function fetchState() {
    const started = performance.now();
    const res = await fetch('/api/state', { cache: 'no-store' });
    const data = await res.json();
    return { res, data, clientDurationMs: performance.now() - started };
  };

  A.refreshState = function refreshState(scope = 'all') {
    return requestJSON('/api/state/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scope }),
    });
  };

  A.openEvents = function openEvents(handlers = {}) {
    const source = new EventSource('/api/events');
    const names = ['snapshot', 'program', 'config', 'presets'];

    for (const name of names) {
      source.addEventListener(name, event => {
        try {
          const payload = JSON.parse(event.data);
          handlers[name]?.(payload, event);
        } catch (err) {
          handlers.error?.(err);
        }
      });
    }
    source.onopen = event => handlers.open?.(event);
    source.onerror = event => handlers.disconnect?.(event);
    return source;
  };

  window.NAMNESIS = window.NAMNESIS || {};
  window.NAMNESIS.api = A;
})();
