// app.js (entrypoint)
(() => {
  const { api: A, parse: P, render: R } = window.NAMNESIS;

  const elNow = document.getElementById('now');
  const elStatus = document.getElementById('status');
  const elPreset = document.getElementById('presetSelect');
  const elSlots = document.getElementById('slotsRow') || document.getElementById('slotsRowMobile');
  const elLanes = document.getElementById('lanesRow') || document.getElementById('lanesRowMobile');
  const elDebug = document.getElementById('debug') || document.getElementById('debugMobile');
  const elModelSelectors = document.getElementById('modelSelectors');
  const elCore = document.getElementById('coreRow');
  const savePresetBtn = document.getElementById("savePresetBtn");

  const state = {
    presetChangeHandlerBound: false,
    isProgrammaticPresetUpdate: false,
    isProgrammaticModelUpdate: false,
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
    'Gain', 'Level', 'Tone',
    'Bias',
    'Soft', 'Blend', 'Comp', 'Wah', 'Fuzz', 'Octave',
    'FrqWidth', 'Shape', 'Delay', 'HiLo', 'High', 'Low',
    'Drive', 'Decay', 'Size'
  ]);

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

    for (const p of presetList) {
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
        savePresetBtn.disabled = true;
        await A.savePreset(preset);
        await refreshUI();
        elStatus.textContent = `Saved ${preset || '(active)'}`;
      } catch (err) {
        console.error(err);
        elStatus.textContent = `Save failed: ${err.message || err}`;
      } finally {
        savePresetBtn.disabled = false;
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

  async function refreshUI() {
    try {
      const { res, data } = await A.fetchState();

      elNow.textContent = data?.meta?.now || '(no time)';
      elStatus.textContent = res.ok ? 'ok' : ('http ' + res.status);

      const presetList = data?.presets?.error ? [] : P.parsePresets(data?.presets?.raw || '');
      const pluginMetaMap = data?.dumpConfig?.error ? {} : P.parseDumpConfig(data?.dumpConfig?.raw || '');
      const trees = data?.dumpConfig?.error ? {} : P.parseFileTrees(data?.dumpConfig?.raw || '');
      const paramMetaMap = data?.dumpConfig?.error ? {} : P.parseParameterConfig(data?.dumpConfig?.raw || '');

      const program = data?.program?.error ? P.parseDumpProgram('') : P.parseDumpProgram(data?.program?.raw || '');

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

      const buildPill = (pluginName, pluginParams, meta, baseType) => {
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
        });
      };

      elLanes.innerHTML = '';
      R.renderChainsFromProgram(elLanes, program, pluginMetaMap, paramMetaMap, buildPill);
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
    }
  }

  refreshUI();
})();
