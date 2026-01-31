// api.js
(() => {
  const A = {};
  let setParamBusy = false;
  let queuedSet = null; // { plugin, param, value }

  A.setNumericParam = async function setNumericParam(plugin, param, value) {
    const res = await fetch('/api/param/set', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugin, param, value })
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || `HTTP ${res.status}`);
    }
  };

  // Serialize param writes: keep only latest while in-flight
  A.setNumericParamQueued = async function setNumericParamQueued(plugin, param, value) {
    queuedSet = { plugin, param, value };
    if (setParamBusy) return;

    setParamBusy = true;
    try {
      while (queuedSet) {
        const req = queuedSet;
        queuedSet = null;
        await A.setNumericParam(req.plugin, req.param, req.value);
      }
    } finally {
      setParamBusy = false;
    }
  };

  A.setFileParam = async function setFileParam(plugin, param, value) {
    const res = await fetch('/api/param/file', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugin, param, value })
    });
    if (!res.ok) {
      const t = await res.text();
      throw new Error(t || `HTTP ${res.status}`);
    }
    return await res.json().catch(() => ({}));
  };

  A.setPluginEnabled = async function setPluginEnabled(pluginName, enabled) {
    const res = await fetch(`/api/plugins/${encodeURIComponent(pluginName)}/enabled`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled })
    });
    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || `HTTP ${res.status}`);
    }
  };

  A.loadPreset = async function loadPreset(name) {
    const res = await fetch('/api/preset/load', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    });
    if (!res.ok) {
      const t = await res.text();
      throw new Error(t || `HTTP ${res.status}`);
    }
    return await res.json().catch(() => ({}));
  };

  A.savePreset = async function savePreset(name) {
    const res = await fetch("/api/preset/save", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data?.error || `HTTP ${res.status}`);
    return data;
  };

  A.fetchState = async function fetchState() {
    const res = await fetch('/api/state', { cache: 'no-store' });
    const data = await res.json();
    return { res, data };
  };

  window.NAMNESIS = window.NAMNESIS || {};
  window.NAMNESIS.api = A;
})();
