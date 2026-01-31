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
  const savePresetBtn = document.getElementById("savePresetBtn");

  const state = {
    presetChangeHandlerBound: false,
    isProgrammaticPresetUpdate: false,
    isProgrammaticModelUpdate: false,
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
      'EQ-7': ['10.0k', '120', '4.5k', '400'],
    };

    const preferred = PREFERRED[base] || [];
    for (const k of preferred) if (all.includes(k) && !out.includes(k)) out.push(k);

    for (const k of all) {
      if (out.includes(k)) continue;
      const n = Number(pluginParams[k]);
      if (!Number.isFinite(n)) continue;
      if (!WRITABLE_NUMERIC_PARAMS.has(k)) continue;
      out.push(k);
    }
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

      const buildPill = (pluginName, pluginParams, meta, baseType) => {
        return R.buildPluginPill({
          pluginName,
          pluginParams,
          bgColor: meta?.bg || null,
          fgColor: meta?.fg || null,
          paramMetaForThisPlugin: paramMetaMap?.[baseType] || paramMetaMap?.[pluginName] || {},
          pickKeys: pickPluginKeys,
          isBooleanParam,
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
