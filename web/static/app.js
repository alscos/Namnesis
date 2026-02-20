// app.js (entrypoint)
(() => {
  const { api: A, parse: P, render: R } = window.NAMNESIS;
  const FALLBACK_PLUGIN_TYPES = ['NAM', 'Cabinet', 'EQ-7', 'BEQ-7', 'NoiseGate', 'Compressor', 'Delay', 'Reverb', 'ConvoReverb', 'Chorus', 'Flanger', 'Phaser', 'Tremolo', 'Vibrato', 'Boost', 'Screamer', 'Fuzz', 'AutoWah', 'Wah', 'HighLow'];
  const elNow = document.getElementById('now');
  const elStatus = document.getElementById('status');
  const elPreset = document.getElementById('presetSelect');
  const elSlots = document.getElementById('slotsRow') || document.getElementById('slotsRowMobile');
  const elLanes = document.getElementById('lanesRow') || document.getElementById('lanesRowMobile');
  const elDebug = document.getElementById('debug') || document.getElementById('debugMobile');
  const elModelSelectors = document.getElementById('modelSelectors');
  const elCore = document.getElementById('coreRow');
  const savePresetBtn = document.getElementById("savePresetBtn");
  const savePresetAsBtn = document.getElementById("savePresetAsBtn");
  const deletePresetBtn = document.getElementById("deletePresetBtn");

  const state = {
    presetChangeHandlerBound: false,
    isProgrammaticPresetUpdate: false,
    isProgrammaticModeUpdate: false,

    // --- LIVE vs RESEARCH ---
    refreshMode: localStorage.getItem("namnesis.refreshMode") || "live", // "live" | "research"
    systemPollTimer: null,
    presetWatchTimer: null,
    lastPresetSeen: null, // used later for midi-triggered preset sync
    isRefreshingUI: false,
    isCheckingPreset: false,

    core: {
      inputGain: null,
      inputMuted: false,
      lastInputGain: null,
      masterVolume: null,
      masterMuted: false,
      lastMasterVolume: null,
    },
  };

  const WRITABLE_NUMERIC_PARAMS = new Set([
    'Attack', 'Release', 'Threshold', 'Strength',
    'Depth', 'Rate', 'FBack', 'Ratio', 'Speed',
    'Dry', 'Wet', 'Mix',
    'Gain', 'Volume', 'Vol', 'Level', 'Tone',
    'Bias',
    'Soft', 'Blend', 'Comp', 'Wah', 'Fuzz', 'Octave',
    'FrqWidth', 'Shape', 'Delay', 'HiLo', 'High', 'Low',
    'Drive', 'Decay', 'Size'
  ]);
  async function postJSON(url, body) {
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
    if (!res.ok) {
      const msg = await res.text().catch(() => '');
      throw new Error(msg || `HTTP ${res.status}`);
    }
    const txt = await res.text().catch(() => '');
    if (!txt) return {};
    try { return JSON.parse(txt); } catch { return { ok: true }; }
  }
  function disableBtn(btn) {
    if (!btn) return;
    btn.disabled = true;
    btn.classList.add("opacity-50", "cursor-not-allowed");
  }

  // Prefer the ALSA "CARD=" that JACK is actually using (stable across hotplug).
  function getJackCardShortname(jackDevice) {
    if (!jackDevice) return null;
    // examples:
    //  - "hw:CARD=Audio,DEV=0"
    //  - "plughw:CARD=Audio,DEV=0"
    const m = String(jackDevice).match(/CARD=([^,]+)/);
    return m ? m[1] : null;
  }

  function pickActiveAsoundCardLine(asoundCards, jackDevice) {
    const cards = Array.isArray(asoundCards) ? asoundCards : [];
    const card = getJackCardShortname(jackDevice);
    if (card) {
      const hit = cards.find(line => String(line).includes(`[${card}]`));
      if (hit) return hit;
    }
    return cards[0] || null; // fallback
  }
  function enableBtn(btn) {
    if (!btn) return;
    btn.disabled = false;
    btn.classList.remove("opacity-50", "cursor-not-allowed");
  }

  function pickPluginKeys(pluginName, pluginParams) {
    const base = pluginName.replace(/_\d+$/, '');
    const all = Object.keys(pluginParams || {}).filter(k => k !== 'Enabled');
    const out = [];

    if (all.includes('Model')) out.push('Model');
    if (all.includes('Impulse')) out.push('Impulse');

    const PREFERRED = {
      NoiseGate: ['Threshold', 'Attack', 'Release', 'Soft', 'Strength'],
      Compressor: ['Attack', 'Blend', 'Comp', 'Ratio'],
      Phaser: ['Depth', 'FBack', 'FrqWidth', 'Ratio'],
      Chorus: ['Depth', 'Rate'],
      Flanger: ['Depth', 'FBack', 'Rate'],
      Vibrato: ['Depth', 'FBack', 'Ratio', 'Speed'],
      Tremolo: ['Depth', 'Shape', 'Speed'],
      Delay: ['Delay', 'FBack', 'HiLo', 'Mix'],
      ConvoReverb: ['Dry', 'Wet'],
      Reverb: ['Blend', 'Decay', 'Size'],
      Screamer: ['Drive', 'Level', 'Tone'],
      Boost: ['Gain', 'Level'],
      Fuzz: ['Bias', 'Fuzz', 'Level', 'Octave'],
      AutoWah: ['Level', 'Wah'],
      Wah: ['Wah'],
      HighLow: ['High', 'Low'],
      Level: ['Volume'],
      Master: ['Volume'],
      Input: ['Gain'],
      // (optional) keep a fallback preferred for EQ-7 if you ever render it in pills
      'EQ-7': ['100', '200', '400', '800', '1.6k', '3.2k', '6.4k', 'Vol'],
      'BEQ-7': ['50', '120', '400', '800', '1.6k', '4.5k', '10.0k', 'Vol'],
    };

    const preferred = PREFERRED[base] || [];
    for (const k of preferred) if (all.includes(k) && !out.includes(k)) out.push(k);

    // Helper: recognize EQ band-ish keys
    const looksEqBand = (k) => /^[0-9.]+k?$|^Vol$/i.test(k);

    // Helper: sort EQ bands numerically, keep Vol last
    const eqBandSortKey = (k) => {
      if (/^Vol$/i.test(k)) return Number.POSITIVE_INFINITY;
      const m = /^([0-9.]+)(k)?$/i.exec(k);
      if (!m) return Number.POSITIVE_INFINITY - 1;
      const num = parseFloat(m[1]);
      const mult = m[2] ? 1000 : 1;
      return num * mult;
    };

    // Collect additional numeric params not already included
    const extraEqBands = [];
    const extraKnobs = [];

    for (const k of all) {
      if (out.includes(k)) continue;
      const n = Number(pluginParams[k]);
      if (!Number.isFinite(n)) continue;

      if (k === 'Model' || k === 'Impulse') continue;

      if (looksEqBand(k)) {
        extraEqBands.push(k);
        continue;
      }

      // keep previous allowlist for standard knobs
      if (!WRITABLE_NUMERIC_PARAMS.has(k)) continue;
      extraKnobs.push(k);
    }

    // If it's an EQ plugin, prioritize its band list (sorted) before other knobs
    if (base === 'BEQ-7' || base === 'EQ-7') {
      extraEqBands.sort((a, b) => eqBandSortKey(a) - eqBandSortKey(b));
      for (const k of extraEqBands) if (!out.includes(k)) out.push(k);
      for (const k of extraKnobs) if (!out.includes(k)) out.push(k);
    } else {
      // non-EQ: just append extras in discovery order
      for (const k of extraKnobs) if (!out.includes(k)) out.push(k);
    }

    // Hard cap to keep pills compact
    return out.slice(0, 8);
  }

  function isBooleanParam(meta, paramName) {
    if (paramName === 'Enabled') return true;
    return !!meta && String(meta.type || '').toLowerCase() === 'bool';
  }

  function setPresetDropdown(presetList, currentPreset) {
    elPreset.innerHTML = '';
    const opt0 = document.createElement('option');
    opt0.value = '';
    opt0.textContent = '---';
    elPreset.appendChild(opt0);

    // ---- sort presets: numeric prefix first, then alphabetic ----
    const sortedPresets = [...presetList].sort((a, b) => {
      const rx = /^(\d+)[\s._-]*/;
      const ma = a.match(rx);
      const mb = b.match(rx);

      if (ma && mb) {
        // both have numeric prefix → numeric compare
        return Number(ma[1]) - Number(mb[1]);
      }
      if (ma) return -1; // a has number, b doesn't
      if (mb) return 1;  // b has number, a doesn't

      // fallback: alphabetical
      return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
    });

    for (const p of sortedPresets) {
      const opt = document.createElement('option');
      opt.value = p;
      opt.textContent = p;
      elPreset.appendChild(opt);
    }

    state.isProgrammaticPresetUpdate = true;
    elPreset.value = currentPreset || '';
    state.isProgrammaticPresetUpdate = false;

    if (!state.presetChangeHandlerBound) {
      elPreset.addEventListener('change', onPresetChanged);
      state.presetChangeHandlerBound = true;
    }
  }

  async function refreshAfterPresetChange(expectedName) {
    for (let i = 0; i < 10; i++) {
      const program = await refreshUI();
      if (program && program.preset === expectedName) return;
      await new Promise(r => setTimeout(r, 150));
    }
    elStatus.textContent = 'loaded (UI not yet confirmed)';
  }

  async function refreshAfterFileParamChange(plugin, param, expectedValue) {
    for (let i = 0; i < 12; i++) {
      const program = await refreshUI();
      const got = program?.params?.[plugin]?.[param];
      if (got === expectedValue) return;
      await new Promise(r => setTimeout(r, 120));
    }
  }

  async function onPresetChanged(e) {
    if (state.isProgrammaticPresetUpdate) return;
    const name = e.target.value;
    if (!name) return;

    elStatus.textContent = 'loading...';
    try {
      await A.loadPreset(name);
    } catch (err) {
      elStatus.textContent = 'error';
      elDebug.innerHTML = '<pre>' + String(err) + '</pre>';
      return;
    }
    await refreshAfterPresetChange(name);
  }

  if (savePresetBtn) {
    savePresetBtn.addEventListener("click", async () => {
      const preset = elPreset?.value || "";
      try {
        disableBtn(savePresetBtn);

        await A.savePreset(preset);
        await refreshUI();

        elStatus.textContent = `Saved ${preset || "(active)"}`;
      } catch (err) {
        console.error(err);
        elStatus.textContent = `Save failed: ${err.message || err}`;
      } finally {
        enableBtn(savePresetBtn);
      }
    });

  }

  if (savePresetAsBtn) {
    savePresetAsBtn.addEventListener("click", async () => {
      const current = elPreset?.value || "";
      const suggested = current && current !== "---" ? current : "";
      const name = window.prompt("Save As preset name:", suggested);
      if (!name) return;

      try {
        disableBtn(savePresetAsBtn);

        await postJSON("/api/preset/save-as", { name });
        await refreshUI();

        if (elPreset) elPreset.value = name;
        elStatus.textContent = `Saved As ${name}`;
      } catch (err) {
        console.error(err);
        elStatus.textContent = `Save As failed: ${err.message || err}`;
      } finally {
        enableBtn(savePresetAsBtn);
      }
    });

  }

  if (deletePresetBtn) {
    deletePresetBtn.addEventListener("click", async () => {
      const preset = elPreset?.value || "";
      if (!preset || preset === "---") {
        elStatus.textContent = "Delete failed: no preset selected";
        return;
      }

      if (!window.confirm(`Delete preset "${preset}"? This cannot be undone.`)) {
        return;
      }

      try {
        disableBtn(deletePresetBtn);

        await postJSON("/api/preset/delete", { name: preset });
        await refreshUI();

        elStatus.textContent = `Deleted ${preset}`;
      } catch (err) {
        console.error(err);
        elStatus.textContent = `Delete failed: ${err.message || err}`;
      } finally {
        enableBtn(deletePresetBtn);
      }
    });

  }


  function fmtDb(x) {
    const n = Number(x);
    if (!Number.isFinite(n)) return String(x);
    return (n >= 0 ? '+' : '') + n.toFixed(1) + ' dB';
  }

  function buildCoreCard(title) {
    const card = document.createElement('section');
    card.className = 'rounded-xl border border-neutral-800 bg-neutral-950/40 p-4 flex flex-col gap-3';

    const h = document.createElement('div');
    h.className = 'text-[10px] uppercase font-black tracking-[0.3em] text-neutral-600';
    h.textContent = title;

    card.appendChild(h);
    return card;
  }

  function buildMuteToggle(labelText, initialOn, onToggle) {
    const wrap = document.createElement('div');
    wrap.className = 'flex items-center justify-between gap-3';

    const lab = document.createElement('div');
    lab.className = 'text-[10px] uppercase font-bold tracking-widest text-neutral-500';
    lab.textContent = labelText;

    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'pill-state pill-param-toggle ' + (initialOn ? 'on' : 'off');
    btn.innerHTML = `
      <div class="switch-container">
        <div class="switch-track"></div>
        <div class="switch-thumb"></div>
      </div>
    `;
    btn.setAttribute('aria-label', labelText);
    btn.setAttribute('aria-pressed', initialOn ? 'true' : 'false');

    btn.addEventListener('click', async () => {
      const current = (btn.getAttribute('aria-pressed') === 'true');
      const next = !current;

      // optimistic
      btn.classList.toggle('on', next);
      btn.classList.toggle('off', !next);
      btn.setAttribute('aria-pressed', next ? 'true' : 'false');

      try {
        await onToggle(next);
      } catch (e) {
        // rollback
        btn.classList.toggle('on', current);
        btn.classList.toggle('off', !current);
        btn.setAttribute('aria-pressed', current ? 'true' : 'false');
        console.error(e);
        alert(e?.message || String(e));
      }
    });

    wrap.appendChild(lab);
    wrap.appendChild(btn);
    return wrap;
  }
  function buildCommitVSlider(labelText, value, meta, onCommit) {
    const wrap = document.createElement('div');
    wrap.className = 'eq7-fader';

    const lab = document.createElement('div');
    lab.className = 'eq7-label text-[10px] uppercase font-bold tracking-widest text-neutral-500';
    lab.textContent = labelText;

    const vv = document.createElement('div');
    vv.className = 'eq7-value text-[11px] font-mono text-neutral-300';
    vv.textContent = fmtDb(value);

    const slider = document.createElement('input');
    slider.type = 'range';
    slider.className = 'eq7-vslider';

    const min = Number.isFinite(meta?.min) ? meta.min : -15;
    const max = Number.isFinite(meta?.max) ? meta.max : 15;
    const step = Number.isFinite(meta?.step) ? meta.step : 0.1;

    slider.min = String(min);
    slider.max = String(max);
    slider.step = String(step);
    slider.value = String(Number(value ?? 0));

    // UI-only while dragging
    slider.addEventListener('input', (e) => {
      const v = parseFloat(e.target.value);
      vv.textContent = fmtDb(v);
    });

    let lastCommitted = slider.value;
    const commit = async () => {
      if (slider.value === lastCommitted) return;
      lastCommitted = slider.value;
      const v = parseFloat(slider.value);
      await onCommit(v);
    };

    // commit only on release / confirm
    slider.addEventListener('change', () => commit());
    slider.addEventListener('pointerup', () => commit());
    slider.addEventListener('keyup', (ev) => { if (ev.key === 'Enter') commit(); });

    // Slot to constrain the rotated range input so it doesn't cover labels/values
    const sliderSlot = document.createElement('div');
    sliderSlot.className = 'eq7-slider-slot';
    sliderSlot.appendChild(slider);

    wrap.appendChild(lab);
    wrap.appendChild(sliderSlot);
    wrap.appendChild(vv);
    return wrap;
  }

  function extractAudioIFName(asoundCards) {
    if (!Array.isArray(asoundCards) || asoundCards.length === 0) return "—";

    // Primera línea de la primera tarjeta
    const line = asoundCards[0];

    // Caso típico: "...: USB-Audio - Jogg USB Audio"
    const m = line.match(/-\s(.+)$/);
    if (m) {
      // Ej: "Jogg USB Audio" → "Jogg"
      return m[1].replace(/USB Audio/i, "").trim();
    }

    // Fallback: lo que haya tras el colon
    const m2 = line.match(/:\s(.+)$/);
    if (m2) return m2[1].trim();

    return "—";
  }


  function buildCommitFaderRow(labelText, value, meta, onCommit) {
    const row = document.createElement('div');
    row.className = 'flex flex-col gap-2';

    const top = document.createElement('div');
    top.className = 'flex items-baseline justify-between';

    const lab = document.createElement('div');
    lab.className = 'text-[10px] uppercase font-bold tracking-widest text-neutral-500';
    lab.textContent = labelText;

    const vv = document.createElement('div');
    vv.className = 'text-[11px] font-mono text-neutral-300';
    vv.textContent = fmtDb(value);

    top.appendChild(lab);
    top.appendChild(vv);

    const slider = document.createElement('input');
    slider.type = 'range';
    slider.className = 'w-full accent-indigo-500';

    const min = Number.isFinite(meta?.min) ? meta.min : -40;
    const max = Number.isFinite(meta?.max) ? meta.max : 40;
    const step = Number.isFinite(meta?.step) ? meta.step : 0.1;

    slider.min = String(min);
    slider.max = String(max);
    slider.step = String(step);
    slider.value = String(Number(value ?? 0));

    // UI only while dragging
    slider.addEventListener('input', (e) => {
      const v = parseFloat(e.target.value);
      vv.textContent = fmtDb(v);
    });

    let lastCommitted = slider.value;
    const commit = async () => {
      if (slider.value === lastCommitted) return;
      lastCommitted = slider.value;
      const v = parseFloat(slider.value);
      await onCommit(v);
    };
    // LIVE / RESEARCH toggle
    const refreshToggle = document.getElementById("refreshModeToggle");
    const manualBtn = document.getElementById("manualRefreshBtn");

    if (refreshToggle) {
      refreshToggle.addEventListener("change", async (e) => {
        const on = !!e.target.checked;
        await setRefreshMode(on ? "live" : "research");
      });
    }

    if (manualBtn) {
      manualBtn.addEventListener("click", async () => {
        await refreshUI();
        await refreshSystemStrip();
      });
    }

    // Apply mode on startup
    applyRefreshModeUI();
    if (state.refreshMode === "live") startLivePolling();

    // commit only on release / confirm
    slider.addEventListener('change', () => commit());
    slider.addEventListener('pointerup', () => commit());
    slider.addEventListener('keyup', (ev) => {
      if (ev.key === 'Enter') commit();
    });

    row.appendChild(top);
    row.appendChild(slider);
    return row;
  }

  function renderCore(el, program, paramMetaMap) {
    if (!el) return;
    el.innerHTML = '';

    // Input/Master are fixed engine modules and are NOT serialized in DumpProgram.
    // We keep a local shadow state and only write-back on user action.

    // --- INPUT ---
    const inputCard = buildCoreCard('INPUT');
    const metaGain = paramMetaMap?.Input?.Gain || null;
    const minGain = Number.isFinite(metaGain?.min) ? metaGain.min : -40;

    inputCard.appendChild(buildMuteToggle('Mute', state.core.inputMuted, async (muted) => {
      if (muted) {
        state.core.lastInputGain = state.core.inputGain;
        state.core.inputMuted = true;
        state.core.inputGain = minGain;
        await A.setNumericParamQueued('Input', 'Gain', minGain);
      } else {
        state.core.inputMuted = false;
        const restore = Number.isFinite(state.core.lastInputGain) ? state.core.lastInputGain : 0;
        state.core.inputGain = restore;
        await A.setNumericParamQueued('Input', 'Gain', restore);
      }
      await refreshUI();
    }));

    inputCard.appendChild(buildCommitFaderRow('Gain', state.core.inputGain ?? 0, metaGain, async (v) => {
      state.core.inputMuted = false;
      state.core.inputGain = v;
      state.core.lastInputGain = v;
      await A.setNumericParamQueued('Input', 'Gain', v);
      await refreshUI();
    }));

    // --- EQ-7 (CORE) ---
    const eqCard = buildCoreCard('EQ-7');

    const eqParams = program?.params?.['EQ-7'] || null;
    if (!eqParams) {
      const msg = document.createElement('div');
      msg.className = 'text-xs text-neutral-500';
      msg.textContent = 'EQ-7 not present (Tonestack slot not loaded).';
      eqCard.appendChild(msg);
    } else {
      const grid = document.createElement('div');
      grid.className = 'eq7-grid';


      const bands = ['100', '200', '400', '800', '1.6k', '3.2k', '6.4k', 'Vol'];

      for (const b of bands) {
        const meta = paramMetaMap?.['EQ-7']?.[b] || null;
        const cur = Number(eqParams[b]);
        grid.appendChild(buildCommitVSlider(b, Number.isFinite(cur) ? cur : (meta?.def ?? 0), meta, async (v) => {
          await A.setNumericParamQueued('EQ-7', b, v);
          await refreshUI();
        }));
      }

      eqCard.appendChild(grid);
    }


    // --- MASTER ---
    const masterCard = buildCoreCard('MASTER');
    const metaVol = paramMetaMap?.Master?.Volume || null;
    const minVol = Number.isFinite(metaVol?.min) ? metaVol.min : -40;

    masterCard.appendChild(buildMuteToggle('Mute', state.core.masterMuted, async (muted) => {
      if (muted) {
        state.core.lastMasterVolume = state.core.masterVolume;
        state.core.masterMuted = true;
        state.core.masterVolume = minVol;
        await A.setNumericParamQueued('Master', 'Volume', minVol);
      } else {
        state.core.masterMuted = false;
        const restore = Number.isFinite(state.core.lastMasterVolume) ? state.core.lastMasterVolume : 0;
        state.core.masterVolume = restore;
        await A.setNumericParamQueued('Master', 'Volume', restore);
      }
      await refreshUI();
    }));

    masterCard.appendChild(buildCommitFaderRow('Volume', state.core.masterVolume ?? 0, metaVol, async (v) => {
      state.core.masterMuted = false;
      state.core.masterVolume = v;
      state.core.lastMasterVolume = v;
      await A.setNumericParamQueued('Master', 'Volume', v);
      await refreshUI();
    }));

    el.appendChild(inputCard);
    el.appendChild(eqCard);
    el.appendChild(masterCard);
  }

  async function fetchCurrentPresetName() {
    // Preferred (future): lightweight endpoint
    try {
      const r = await fetch("/api/preset/current", { cache: "no-store" });
      if (r.ok) {
        const j = await r.json();
        // accept a few possible keys to be resilient
        return j.currentPreset || j.preset || j.name || "";
      }
    } catch (_) { }

    // Fallback (beta): ask /api/state and extract preset
    try {
      const r = await fetch("/api/state", { cache: "no-store" });
      if (!r.ok) return "";
      const j = await r.json();

      // Try multiple shapes:
      // - j.currentPreset
      // - j.preset.current
      // - j.program.currentPreset
      // Adjust once you confirm actual payload.
      return (
        j.currentPreset ||
        j.preset?.current ||
        j.program?.currentPreset ||
        j.program?.preset ||
        ""
      );
    } catch (_) {
      return "";
    }
  }

  async function checkPresetChangedLive() {
    if (state.refreshMode !== "live") return;
    if (state.isCheckingPreset) return;
    state.isCheckingPreset = true;
    try {
      const p = await fetchCurrentPresetName();
      if (!p) return;

      if (state.lastPresetSeen === null) {
        state.lastPresetSeen = p;
        return;
      }

      if (p !== state.lastPresetSeen) {
        state.lastPresetSeen = p;

        // Update dropdown without triggering onPresetChanged
        const elPreset = document.getElementById("presetSelect");
        if (elPreset) {
          state.isProgrammaticPresetUpdate = true;
          elPreset.value = p;
          state.isProgrammaticPresetUpdate = false;
        }

        // Full UI refresh (chains/params reflect new preset)
        await refreshUI();
      }
    } finally {
      state.isCheckingPreset = false;
    }
  }

  async function refreshSystemStrip() {
    try {
      const r = await fetch("/api/system");
      const s = await r.json();

      // READY / WARN / FAIL
      let state = "READY";
      let cls = "text-emerald-400";

      if (!s.jack?.running) {
        state = "FAIL"; cls = "text-rose-500";
      } else if (
        s.jack?.xruns_delta > 0 ||
        s.routing?.ok === false ||
        s.midi?.connected === false
      ) {
        state = "WARN"; cls = "text-amber-400";
      }

      const readyEl = document.getElementById("sysReady");
      readyEl.textContent = state;
      readyEl.className = `font-black ${cls}`;

      // XRuns
      document.getElementById("sysXruns").textContent =
        `XRUNS ${s.jack.xruns} (+${s.jack.xruns_delta})`;

      // Latency (play-feel estimate): (buf * periods) / sr
      const sr = s.jack?.sr || 48000;
      const buf = s.jack?.buf || 0;
      const per = s.jack?.periods || 0;

      let latMs = 0;
      if (sr > 0 && buf > 0 && per > 0) {
        latMs = (buf * per / sr) * 1000;
      }

      document.getElementById("sysLatency").textContent =
        `LAT ${latMs.toFixed(1)}ms`;


      // MIDI
      const midiEl = document.getElementById("sysMidi");

      let midiState = "fail";
      let midiText = "🎹✖";

      if (s.midi?.connected) {
        midiState = "ok";
        midiText = "🎹✓";
      } else if ((s.midi?.alsa?.length || 0) > 0) {
        // MIDI devices exist, but none are routed into stompbox:midi_in
        midiState = "warn";
        midiText = "🎹!";
      }

      midiEl.textContent = midiText;
      midiEl.className = "font-black text-[11px] uppercase tracking-widest";

      if (midiState === "ok") {
        midiEl.classList.add("text-emerald-400");
      } else if (midiState === "warn") {
        midiEl.classList.add("text-amber-400");
      } else {
        midiEl.classList.add("text-rose-500");
      }

      // Audio IF
      const line = pickActiveAsoundCardLine(s.audioif?.asound_cards, s.jack?.device);
      const ifName = line?.match(/\]:\s(.+)/)?.[1] || "—";
      // keep your current "first token" behavior, but now it's the active card
      const short = (ifName === "—") ? "—" : ifName.split(" ")[0];
      document.getElementById("sysIF").textContent = `IF ${short}`;


    } catch (e) {
      document.getElementById("sysReady").textContent = "FAIL";
    }
  }
  function stopLivePolling() {
    if (state.systemPollTimer) {
      clearInterval(state.systemPollTimer);
      state.systemPollTimer = null;
    }
    if (state.presetWatchTimer) {
      clearInterval(state.presetWatchTimer);
      state.presetWatchTimer = null;
    }
  }
  function applyRefreshModeUI() {
    const isLive = state.refreshMode === "live";
    const el = document.getElementById("refreshModeToggle");
    const label = document.getElementById("refreshModeLabel");
    const btn = document.getElementById("manualRefreshBtn");

    if (label) label.textContent = isLive ? "LIVE" : "RESEARCH";
    if (btn) btn.classList.toggle("hidden", isLive);
    if (el) el.checked = isLive;
  }

  async function setRefreshMode(mode) {
    state.refreshMode = (mode === "research") ? "research" : "live";
    localStorage.setItem("namnesis.refreshMode", state.refreshMode);

    if (state.refreshMode === "live") {
      // Prime watcher so first tick doesn't force refresh
      state.lastPresetSeen = await fetchCurrentPresetName() || state.lastPresetSeen;
      await refreshSystemStrip();
      startLivePolling();
    } else {
      stopLivePolling();
    }

    applyRefreshModeUI();
  }

  function startLivePolling() {
    stopLivePolling();
    state.systemPollTimer = setInterval(() => {
      refreshSystemStrip();
    }, 750);
    // Watch preset changes (e.g. MIDI) and refresh UI when it changes
    state.presetWatchTimer = setInterval(() => {
      checkPresetChangedLive();
    }, 300);
  }
  async function refreshUI() {
    if (state.isRefreshingUI) return;
    state.isRefreshingUI = true;
    try {
      const { res, data } = await A.fetchState();

      elNow.textContent = data?.meta?.now || '(no time)';
      elStatus.textContent = res.ok ? 'ok' : ('http ' + res.status);

      const presetList = data?.presets?.error ? [] : P.parsePresets(data?.presets?.raw || '');
      const pluginMetaMap = data?.dumpConfig?.error ? {} : P.parseDumpConfig(data?.dumpConfig?.raw || '');
      const trees = data?.dumpConfig?.error ? {} : P.parseFileTrees(data?.dumpConfig?.raw || '');
      const paramMetaMap = data?.dumpConfig?.error ? {} : P.parseParameterConfig(data?.dumpConfig?.raw || '');

      const program = data?.program?.error ? P.parseDumpProgram('') : P.parseDumpProgram(data?.program?.raw || '');

      // ---- AVAILABLE PLUGINS FOR "+ Add plugin..." DROPDOWN ----
      // Source of truth MUST be DumpConfig (PluginConfig ... IsUserSelectable ...)
      // because DumpProgram only lists currently-instantiated plugins.
      let availablePlugins = [];

      // 1) Preferred: DumpConfig → all user-selectable plugin *types*
      if (pluginMetaMap && Object.keys(pluginMetaMap).length) {
        availablePlugins = Object.entries(pluginMetaMap)
          .filter(([name, meta]) => !!meta?.selectable)
          .map(([name]) => name)
          // Optional: keep UI tidy by hiding fixed engine modules if they ever appear selectable
          .filter((name) => !['Input', 'Master'].includes(name))
          .sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
      }

      // 2) Fallback: derive from current program instances (better than nothing)
      if (!availablePlugins.length) {
        availablePlugins = Array.from(
          new Set(Object.keys(program?.params || {}).map(p => p.replace(/_\d+$/, '')))
        ).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
      }

      // 3) Last-resort fallback: hardcoded minimal palette (keeps UI usable if DumpConfig fails)
      if (!availablePlugins.length) {
        availablePlugins = FALLBACK_PLUGIN_TYPES.slice().sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
      }

      // --- CORE fixed params (Input / Master) ---
      // These fixed engine modules are not serialized in DumpProgram.
      // Initialize once from DumpConfig defaults (or 0 dB fallback).
      if (state.core.inputGain === null) {
        const defIn = paramMetaMap?.Input?.Gain?.def;
        state.core.inputGain = Number.isFinite(defIn) ? defIn : 0;
      }
      if (state.core.masterVolume === null) {
        const defOut = paramMetaMap?.Master?.Volume?.def;
        state.core.masterVolume = Number.isFinite(defOut) ? defOut : 0;
      }

      setPresetDropdown(presetList, program.preset);

      const namCurrent = program?.params?.NAM?.Model ?? null;
      const cabCurrent = program?.params?.Cabinet?.Impulse ?? null;
      const namOpts = trees['NAM.Model'] || [];
      const cabOpts = trees['Cabinet.Impulse'] || [];

      elModelSelectors.innerHTML = '';
      elModelSelectors.appendChild(
        R.buildDropdown('NAM Model', namOpts, namCurrent, async (v) => {
          await A.setFileParam('NAM', 'Model', v);
          await refreshAfterFileParamChange('NAM', 'Model', v);
        }, state)
      );
      elModelSelectors.appendChild(
        R.buildDropdown('Cab IR', cabOpts, cabCurrent, async (v) => {
          await A.setFileParam('Cabinet', 'Impulse', v);
          await refreshAfterFileParamChange('Cabinet', 'Impulse', v);
        }, state)
      );

      renderCore(elCore, program, paramMetaMap);
      // ---- ADD PLUGIN HANDLER (for lane dropdown) ----
      const handleAddPlugin = async (chainName, baseType) => {
        const items = (program?.chains?.[chainName] || []).slice();

        // Push baseType; Stompbox will instantiate (e.g. Delay -> Delay_3)
        items.push(baseType);

        await A.setChain(chainName, items);
        await refreshUI();
      };
      const buildPill = (pluginName, pluginParams, meta, baseType, ctx) => {
        return R.buildPluginPill({
          pluginName,
          pluginParams,
          bgColor: meta?.bg || null,
          fgColor: meta?.fg || null,
          paramMetaForThisPlugin: paramMetaMap?.[baseType] || paramMetaMap?.[pluginName] || {},
          pickKeys: pickPluginKeys,
          isBooleanParam,
          fileTrees: trees,
          onFileParamCommit: (pl, pa, val) => A.setFileParam(pl, pa, val),
          WRITABLE_NUMERIC_PARAMS,
          onParamCommit: (pl, pa, val) => A.setNumericParamQueued(pl, pa, val),
          onPluginToggleResync: () => refreshUI(),
          // NEW: chain context so render.js can enable/disable arrows
          chainName: ctx?.chainName,
          chainIndex: ctx?.index,
          chainLength: ctx?.chainItems?.length || 0,

          onUnload: async ({ chainName, pluginName, index }) => {
            // Get latest chain from the *current* program object you render from.
            const items = (program?.chains?.[chainName] || []).slice();

            // Remove by position first (safest), fallback by name
            let next = items.slice();
            if (Number.isFinite(index) && index >= 0 && index < next.length) {
              next.splice(index, 1);
            } else {
              next = next.filter(x => x !== pluginName);
            }

            await A.setChain(chainName, next);

            // Optional best-effort cleanup
            try { await A.releasePlugin(pluginName); } catch (e) { console.warn("ReleasePlugin failed:", e); }

            await refreshUI();
          },

          onMoveUp: async ({ chainName, from }) => {
            const items = (program?.chains?.[chainName] || []).slice();
            if (!Number.isFinite(from) || from <= 0 || from >= items.length) return;

            const next = items.slice();
            const t = next[from - 1];
            next[from - 1] = next[from];
            next[from] = t;

            await A.setChain(chainName, next);
            await refreshUI();
          },

          onMoveDown: async ({ chainName, from }) => {
            const items = (program?.chains?.[chainName] || []).slice();
            if (!Number.isFinite(from) || from < 0 || from >= items.length - 1) return;

            const next = items.slice();
            const t = next[from + 1];
            next[from + 1] = next[from];
            next[from] = t;

            await A.setChain(chainName, next);
            await refreshUI();
          },

        });
      };

      elLanes.innerHTML = '';
      R.renderChainsFromProgram(
        elLanes,
        program,
        pluginMetaMap,
        paramMetaMap,
        buildPill,
        {
          availablePlugins,
          onAddPlugin: handleAddPlugin
        }
      );
      R.renderSlotsFromDumpProgram(elSlots, program);

      const dbg = [];
      dbg.push('Durations:');
      dbg.push('  dumpConfig: ' + (data?.dumpConfig?.duration || 'n/a'));
      dbg.push('  program:    ' + (data?.program?.duration || 'n/a'));
      dbg.push('  presets:    ' + (data?.presets?.duration || 'n/a'));
      elDebug.innerHTML = '<pre>' + dbg.join('\n') + '</pre>';

      return program;
    } catch (e) {
      elStatus.textContent = 'error';
      elDebug.innerHTML = '<pre>' + String(e) + '</pre>';
    } finally {
      state.isRefreshingUI = false;
    }
  }
  // ---- wire LIVE/RESEARCH controls once ----
  const refreshToggle = document.getElementById("refreshModeToggle");
  const manualBtn = document.getElementById("manualRefreshBtn");

  if (refreshToggle) {
    refreshToggle.addEventListener("change", async (e) => {
      const on = !!e.target.checked;
      await setRefreshMode(on ? "live" : "research");
    });
  }

  if (manualBtn) {
    manualBtn.addEventListener("click", async () => {
      await refreshUI();
      await refreshSystemStrip();
    });
  }

  // ---- startup ----
  applyRefreshModeUI();
  refreshUI();
  if (state.refreshMode === "live") startLivePolling();

})();

