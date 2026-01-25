(async function () {
  const elNow = document.getElementById('now');
  const elStatus = document.getElementById('status');
  const elPreset = document.getElementById('presetSelect');
  const elSlots = document.getElementById('slotsRow');
  const elLanes = document.getElementById('lanesRow');
  const elDebug = document.getElementById('debug');
  const elModelSelectors = document.getElementById('modelSelectors');



  
  function parsePresets(raw) {
    const toks = raw.split(/\s+/).filter(Boolean);
    return toks.filter(t => t !== 'Presets' && t !== 'Ok');
  }

  function parseCurrentPreset(programRaw) {
    const lines = programRaw.split(/\r?\n/);
    for (const line of lines) {
      const f = line.trim().split(/\s+/).filter(Boolean);
      if (f[0] === 'SetPreset') {
        if (f.length >= 2) return f[1]; // SetPreset <name>
        return null; // SetPreset (no name)
      }
    }
    return null;
  }
  function tile(text) {
    const d = document.createElement('div');
    d.className = 'tile';
    d.textContent = text;
    return d;
  }
let isProgrammaticModelUpdate = false;

async function setFileParam(plugin, param, value) {
  elStatus.textContent = 'setting...';

  const res = await fetch('/api/param/file', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ plugin, param, value })
  });

  if (!res.ok) {
    const t = await res.text();
    elStatus.textContent = 'error';
    elDebug.innerHTML = '<pre>' + t + '</pre>';
    throw new Error('setFileParam failed: ' + res.status);
  }

  const data = await res.json().catch(() => ({}));
  elStatus.textContent = 'ok';
  return data;
}

async function refreshAfterFileParamChange(plugin, param, expectedValue) {
  for (let i = 0; i < 12; i++) {
    const program = await refreshUI();

    const got = program?.params?.[plugin]?.[param];
    if (got === expectedValue) return;

    await new Promise(r => setTimeout(r, 120));
  }

  // If it never catches up, don’t block; just leave UI refreshed.
}

  function renderErrorBox(title, err) {
    const box = document.createElement('div');
    box.className = 'tile error';

    const t = document.createElement('div');
    t.style.fontWeight = '600';
    t.textContent = title;

    const m = document.createElement('div');
    m.className = 'muted';
    m.textContent = err || 'unknown error';

    box.appendChild(t);
    box.appendChild(m);
    return box;
  }

  let presetChangeHandlerBound = false;
let isProgrammaticPresetUpdate = false;

function setPresetDropdown(presetList, currentPreset) {
  // assume elPreset is your <select>
  elPreset.innerHTML = '';

  // add default option
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

  isProgrammaticPresetUpdate = true;
  elPreset.value = currentPreset || '';
  isProgrammaticPresetUpdate = false;

  if (!presetChangeHandlerBound) {
    elPreset.addEventListener('change', onPresetChanged);
    presetChangeHandlerBound = true;
  }
}

async function onPresetChanged(e) {
  if (isProgrammaticPresetUpdate) return;

  const name = e.target.value;
  if (!name) return;

  // optional: show "loading"
  elStatus.textContent = 'loading...';

  // call backend to load preset
  const res = await fetch('/api/preset/load', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name })
  });

  if (!res.ok) {
    const t = await res.text();
    elStatus.textContent = 'error';
    elDebug.innerHTML = '<pre>' + t + '</pre>';
    return;
  }

  // Now refresh state. We may need a short settle/retry loop.
  await refreshAfterPresetChange(name);
}

async function refreshAfterPresetChange(expectedName) {
  for (let i = 0; i < 10; i++) {
    const program = await refreshUI();

    if (program && program.preset === expectedName) {
      return; // backend has caught up; UI is now rendered from the new program
    }

    await new Promise(r => setTimeout(r, 150));
  }

  // Optional: if it never caught up, show something useful
  elStatus.textContent = 'loaded (UI not yet confirmed)';
}


  function parseQuotedTokens(s) {
    // returns array of strings inside "..."
    const out = [];
    const re = /"([^"]*)"/g;
    let m;
    while ((m = re.exec(s)) !== null) out.push(m[1]);
    return out;
  }
  
  function parseFileTrees(dumpConfigRaw) {
    // returns: { "<Plugin>.<Param>": ["opt1","opt2", ...] }
    const trees = {};
    const lines = dumpConfigRaw.split(/\r?\n/);
  
    for (const line of lines) {
      const l = line.trim();
      if (!l.startsWith('ParameterFileTree ')) continue;
  
      // Format: ParameterFileTree <Plugin> <Param> <Root>  "label" "id" ...
      // We only trust the first 4 space-separated tokens, then parse quotes.
      const parts = l.split(/\s+/);
      if (parts.length < 4) continue;
  
      const plugin = parts[1];
      const param = parts[2];
  
      const opts = parseQuotedTokens(l);
      if (!opts.length) continue;
  
      trees[plugin + '.' + param] = opts;
    }
  
    return trees;
  }
  function parseCurrentFileParam(programRaw, pluginName, paramName) {
  let found = null;
  const lines = programRaw.split(/\r?\n/);

  for (const line of lines) {
    const l = line.trim();
    if (!l.startsWith('SetParam ')) continue;

    const parts = l.split(/\s+/);
    if (parts.length < 4) continue;
    if (parts[1] !== pluginName) continue;
    if (parts[2] !== paramName) continue;

    let val = l.split(/\s+/).slice(3).join(' ').trim();
    if (val.startsWith('"') && val.endsWith('"') && val.length >= 2) val = val.slice(1, -1);

    found = val || null; // keep updating; last match wins
  }

  return found;
}

  function buildDropdown(label, options, selectedValue, onChange) {
  const wrap = document.createElement('div');
  wrap.className = 'selector';

  const lab = document.createElement('div');
  lab.className = 'muted';
  lab.textContent = label;
  wrap.appendChild(lab);

  const sel = document.createElement('select');

  const ph = document.createElement('option');
  ph.value = '';
  ph.textContent = '---';
  sel.appendChild(ph);

  for (const opt of options) {
    const o = document.createElement('option');
    o.value = opt;
    o.textContent = opt;
    sel.appendChild(o);
  }

  const initial = (selectedValue && options.includes(selectedValue)) ? selectedValue : '';
  isProgrammaticModelUpdate = true;
  sel.value = initial;
  isProgrammaticModelUpdate = false;

  if (typeof onChange === 'function') {
    sel.addEventListener('change', async (e) => {
      if (isProgrammaticModelUpdate) return;

      const v = e.target.value;
      if (!v) return;

      try {
        await onChange(v);
      } catch (err) {
        // setFileParam already writes debug/status; keep this minimal
        console.error(err);
      }
    });
  }

  wrap.appendChild(sel);
  return wrap;
}

  
  function pluginTile(label, meta) {
  const el = tile(label);

  if (meta?.bg) el.style.backgroundColor = meta.bg;
  if (meta?.fg) el.style.color = meta.fg;

  // Optional: keep borders readable if bg is dark/light
  el.style.borderColor = 'rgba(0,0,0,0.15)';

  // Tooltip
  if (meta?.desc) el.title = meta.desc;

  return el;
}
function renderChainsFromProgram(elLanes, program, pluginMetaByName) {
  elLanes.textContent = '';

  const chainOrder = ['Input', 'FxLoop', 'Output']; // fixed UI order
  const chains = program?.chains || {};
  const paramsByPlugin = program?.params || {};

  for (const chainName of chainOrder) {
    const lane = document.createElement('div');
    lane.className = 'lane';

    const title = document.createElement('h3');
    title.textContent = chainName;
    lane.appendChild(title);

    const items = chains[chainName] || [];
    if (!items.length) {
      const empty = document.createElement('div');
      empty.className = 'muted';
      empty.textContent = '(empty)';
      lane.appendChild(empty);
      elLanes.appendChild(lane);
      continue;
    }

    for (const pluginName of items) {
      // params are keyed by instance name (e.g., Reverb_4)
      const p = paramsByPlugin[pluginName] || {};

      // meta is keyed by base type name (e.g., Reverb), but sometimes equals pluginName (NAM, Cabinet, etc.)
      const baseType = pluginName.replace(/_\d+$/, '');
      const meta = pluginMetaByName?.[baseType] || pluginMetaByName?.[pluginName] || null;

      lane.appendChild(buildPluginPill(pluginName, p, meta?.bg || null, meta?.fg || null));
    }

    elLanes.appendChild(lane);
  }
}

function buildPluginCard(label, meta, paramObj) {
  const card = document.createElement('div');
  card.className = 'pill';

  if (meta?.bg) card.style.backgroundColor = meta.bg;
  if (meta?.fg) card.style.color = meta.fg;
  if (meta?.desc) card.title = meta.desc;

  // Header: title (left) + ON/OFF (right)
  const head = document.createElement('div');
  head.className = 'pill-head';

  const t = document.createElement('div');
  t.className = 'pill-title';
  t.textContent = label;

  const enabled = paramObj?.Enabled;
  const st = document.createElement('div');
  st.className = 'pill-state ' + ((String(enabled) === '1') ? 'is-on' : 'is-off');
  st.textContent = (enabled === undefined) ? '' : ((String(enabled) === '1') ? 'ON' : 'OFF');

  head.appendChild(t);
  head.appendChild(st);
  card.appendChild(head);

  // Body: key params as “Label: Value” rows
  const rows = buildParamRows(label, paramObj);
  if (rows.length) {
    const body = document.createElement('div');
    body.className = 'pill-body';
    for (const row of rows) body.appendChild(row);
    card.appendChild(body);
  }

  return card;
}

function buildParamRows(pluginLabel, p) {
  if (!p) return [];

  // Pick a reasonable set, in priority order.
  // (You can keep expanding this per-plugin later.)
  const keys = [];

  // Most important “file” params
  if (p.Model !== undefined) keys.push('Model');
  if (p.Impulse !== undefined) keys.push('Impulse');

  // Common mix params
  if (p.Wet !== undefined) keys.push('Wet');
  if (p.Dry !== undefined) keys.push('Dry');

  // Common controls
  if (p.Gain !== undefined) keys.push('Gain');
  if (p.Volume !== undefined) keys.push('Volume');
  if (p.Level !== undefined) keys.push('Level');

  // If still empty, show up to 3 non-Enabled params
  if (!keys.length) {
    for (const k of Object.keys(p)) {
      if (k === 'Enabled') continue;
      keys.push(k);
      if (keys.length >= 3) break;
    }
  }

  // Build DOM rows
  return keys.slice(0, 4).map((k) => {
    const row = document.createElement('div');
    row.className = 'pill-row';

    const kk = document.createElement('strong');
    kk.className = 'pill-key';
    kk.textContent = prettifyKey(k) + ':';

    const vv = document.createElement('span');
    vv.className = 'pill-val';
    vv.textContent = formatValueWithUnits(k, p[k]);

    row.appendChild(kk);
    row.appendChild(vv);
    return row;
  });
}

function prettifyKey(k) {
  // Minor cosmetics
  if (k === 'Impulse') return 'IR';
  return k;
}

function formatValueWithUnits(paramName, raw) {
  // Reduce “too many zeros”
  const n = Number(raw);
  if (!Number.isFinite(n)) return String(raw);

  // Heuristic units:
  // - If param name looks like frequency-ish, show Hz/kHz
  const freqish = /freq|tone|high|low|hz/i.test(paramName);

  if (freqish) {
    if (Math.abs(n) >= 1000) return (n / 1000).toFixed(2) + ' kHz';
    return n.toFixed(0) + ' Hz';
  }

  // Generic numeric formatting
  // keep integers clean; otherwise 3 decimals is plenty
  if (Math.abs(n - Math.round(n)) < 1e-9) return String(Math.round(n));
  return n.toFixed(3);
}

function pluginTileWithState(label, meta, paramObj) {
  const el = tile(label); // your existing tile() helper

  if (meta?.bg) el.style.backgroundColor = meta.bg;
  if (meta?.fg) el.style.color = meta.fg;
  if (meta?.desc) el.title = meta.desc;

  // Enabled badge (if present)
  const enabled = paramObj?.Enabled;
  if (enabled !== undefined) {
    const badge = document.createElement('span');
    badge.className = 'badge';
    badge.textContent = (String(enabled) === '1') ? 'ON' : 'OFF';
    el.appendChild(badge);
  }

  // Show key params (minimal, pragmatic)
  const summary = pickKeyParams(label, paramObj);
  if (summary.length) {
    const small = document.createElement('div');
    small.className = 'tile-sub';
    small.textContent = summary.join('   ');
    el.appendChild(small);
  }

  return el;
}

function pickKeyParams(label, p) {
  if (!p) return [];

  // Prefer these when present
  const preferred = [];

  // “File” params are the most valuable: Model / Impulse
  if (p.Model) preferred.push(`Model: ${p.Model}`);
  if (p.Impulse) preferred.push(`IR: ${p.Impulse}`);

  // Common mix params
  if (p.Wet !== undefined) preferred.push(`Wet: ${fmt(p.Wet)}`);
  if (p.Dry !== undefined) preferred.push(`Dry: ${fmt(p.Dry)}`);

  // Common gain params
  if (p.Gain !== undefined) preferred.push(`Gain: ${fmt(p.Gain)}`);
  if (p.Volume !== undefined) preferred.push(`Vol: ${fmt(p.Volume)}`);
  if (p.Level !== undefined) preferred.push(`Lvl: ${fmt(p.Level)}`);

  // If nothing matched, just show first 2 non-Enabled params
  if (!preferred.length) {
    const keys = Object.keys(p).filter(k => k !== 'Enabled').slice(0, 2);
    for (const k of keys) preferred.push(`${k}: ${String(p[k])}`);
  }

  return preferred.slice(0, 3);
}

function fmt(v) {
  const n = Number(v);
  if (Number.isFinite(n)) return n.toFixed(3);
  return String(v);
}

function fmtNumber(n) {
  if (!Number.isFinite(n)) return String(n);

  const abs = Math.abs(n);
  let s;
  if (abs >= 1000) s = n.toFixed(0);
  else if (abs >= 100) s = n.toFixed(1);
  else if (abs >= 10) s = n.toFixed(2);
  else s = n.toFixed(3);

  // trim trailing zeros
  s = s.replace(/\.?0+$/, '');
  return s;
}

// Minimal “unit guessing” (safe default). Later we can drive this from DumpConfig ValueFormat.
function withUnit(paramName, rawValue) {
  // If the value is quoted (Model/Impulse), keep it as-is.
  if (typeof rawValue === 'string' && rawValue.startsWith('"')) return rawValue.replace(/^"|"$/g, '');

  const n = Number(rawValue);
  if (!Number.isFinite(n)) return String(rawValue);

  // Very conservative heuristics:
  if (/Thresh|Gain|Level|Volume/i.test(paramName)) return `${fmtNumber(n)} dB`;
  if (/Freq|High|Low|Tone/i.test(paramName)) {
    if (n >= 1000) return `${fmtNumber(n / 1000)} kHz`;
    return `${fmtNumber(n)} Hz`;
  }
  return fmtNumber(n);
}

function parseDumpProgram(raw) {
  const out = {
    preset: null,
    chains: {},     // e.g. { Input:[...], FxLoop:[...], Output:[...] }
    params: {},     // e.g. { Reverb_4:{Enabled:"0", Size:"0.5"} , NAM:{Model:"..."} }
    slots: {},      // e.g. { Amp:"NAM", Tonestack:"EQ-7", Cabinet:"Cabinet" }
  };
  if (!raw) return out;

  const lines = raw.split(/\r?\n/);

  for (const line of lines) {
    if (!line) continue;

    // SetPreset 00_twicked_tutti_frutti
    {
      const m = line.match(/^SetPreset\s+(.+)$/);
      if (m) {
        out.preset = m[1].trim();
        continue;
      }
    }

    // SetChain Input Reverb_4 Boost Screamer ...
    {
      const m = line.match(/^SetChain\s+(\S+)\s+(.*)$/);
      if (m) {
        const chain = m[1];
        const rest = (m[2] || "").trim();
        const items = rest ? rest.split(/\s+/).filter(Boolean) : [];
        out.chains[chain] = items;
        continue;
      }
    }

    // SetPluginSlot Amp NAM
    {
      const m = line.match(/^SetPluginSlot\s+(\S+)\s+(\S+)/);
      if (m) {
        const slot = m[1];
        const plugin = m[2];
        out.slots[slot] = plugin;
        continue;
      }
    }

    // SetParam NAM Model "fender_bassman_..."
    // SetParam EQ-7 3.2k 0.000000
    {
      const m = line.match(/^SetParam\s+(\S+)\s+(\S+)\s+(.+)$/);
      if (m) {
        const plugin = m[1];
        const param = m[2];
        let value = m[3].trim();

        // Strip quotes when present: "..." -> ...
        if (value.startsWith('"') && value.endsWith('"')) {
          value = value.slice(1, -1);
        }

        if (!out.params[plugin]) out.params[plugin] = {};
        out.params[plugin][param] = value;
        continue;
      }
    }
  }

  return out;
}

  function parseProgram(programRaw) {
    const lines = programRaw.split(/\r?\n/);
  
    const chains = [];
    const seen = new Set();
  
    const globalSlots = {};            // slotName -> pluginName
    const chainItems = {};             // chainName -> array of {kind, label}
    let currentChain = null;
  
    for (const line of lines) {
      const l = line.trim();
      if (!l) continue;
  
      const f = l.split(/\s+/);
  
      if (f[0] === 'SetChain' && f[1]) {
        currentChain = f[1];
  
        if (!seen.has(currentChain)) {
          seen.add(currentChain);
          chains.push(currentChain);
        }
        if (!chainItems[currentChain]) chainItems[currentChain] = [];
  
        // IMPORTANT: Inline chain membership: SetChain <ChainName> <Plugin1> <Plugin2> ...
        // If plugins are present, use them as authoritative membership for this chain.
        if (f.length > 2) {
          chainItems[currentChain] = []; // reset (authoritative list)
          for (const pluginName of f.slice(2)) {
            chainItems[currentChain].push({ kind: 'plugin', label: pluginName });
          }
        }
  
        continue;
      }
  
      if (f[0] === 'SetPluginSlot' && f[1] && f[2]) {
        const slot = f[1];
        const plugin = f[2];
  
        globalSlots[slot] = plugin;
  
        // Fallback membership: only add slot tiles if this chain has no inline list
        if (currentChain) {
          const hasInline = (chainItems[currentChain] || []).some(x => x.kind === 'plugin');
          if (!hasInline) {
            chainItems[currentChain].push({ kind: 'slot', label: slot + ' → ' + plugin });
          }
        }
  
        continue;
      }
    }
  
    return { chains, slots: globalSlots, chainItems };
  }
  
  function buildPluginPill(pluginName, pluginParams, bgColor, fgColor) {
  const el = document.createElement('div');
  el.className = 'pill';

  // background / foreground
  if (bgColor) el.style.background = bgColor;
  if (fgColor) el.style.color = fgColor;

  const enabledRaw = pluginParams?.Enabled;
  const isOn = String(enabledRaw) === '1';

  if (!isOn) el.classList.add('is-off');

  // Header
  const head = document.createElement('div');
  head.className = 'pill-head';

  const title = document.createElement('div');
  title.className = 'pill-title';
  title.textContent = pluginName;

  const state = document.createElement('div');
  state.className = 'pill-state ' + (isOn ? 'on' : 'off');
  state.textContent = isOn ? 'ON' : 'OFF';

  head.appendChild(title);
  head.appendChild(state);
  el.appendChild(head);

  // Body (key/value lines)
  const body = document.createElement('div');
  body.className = 'pill-body';

  const keys = Object.keys(pluginParams || {})
    .filter(k => k !== 'Enabled')
    // optional: keep Model/Impulse near the top
    .sort((a, b) => {
      const prio = (k) => (k === 'Model' || k === 'Impulse') ? 0 : 1;
      return prio(a) - prio(b) || a.localeCompare(b);
    });

  // Avoid dumping 20 EQ bands in early UI. Keep it compact for now:
  const MAX_LINES = 4;

  for (const k of keys.slice(0, MAX_LINES)) {
    const row = document.createElement('div');
    row.className = 'kv';

    const kk = document.createElement('div');
    kk.className = 'k';
    kk.textContent = k + ':';

    const vv = document.createElement('div');
    vv.className = 'v';
    vv.textContent = withUnit(k, pluginParams[k]);

    row.appendChild(kk);
    row.appendChild(vv);
    body.appendChild(row);
  }

  el.appendChild(body);
  return el;
}

  function parseDumpConfig(raw) {
  // Returns: { [pluginName]: { bg: "#rrggbb", fg: "#rrggbb", desc: "..." } }
  const map = {};
  if (!raw) return map;

  const lines = raw.split(/\r?\n/);
  for (const line of lines) {
    if (!line.startsWith('PluginConfig ')) continue;

    // Example:
    // PluginConfig Boost BackgroundColor #e31b00 ForegroundColor #ffffff IsUserSelectable 1 Description "Clean boost effect"
    const mName = line.match(/^PluginConfig\s+(\S+)/);
    if (!mName) continue;
    const name = mName[1];

    const mBg = line.match(/\bBackgroundColor\s+(#[0-9a-fA-F]{6})\b/);
    const mFg = line.match(/\bForegroundColor\s+(#[0-9a-fA-F]{6})\b/);

    // Description may contain spaces and is quoted
    const mDesc = line.match(/\bDescription\s+"([^"]*)"/);

    map[name] = {
      bg: mBg ? mBg[1] : null,
      fg: mFg ? mFg[1] : null,
      desc: mDesc ? mDesc[1] : '',
    };
  }

  return map;
}
function renderSlotsFromDumpProgram(elSlots, program) {
  const entries = Object.entries(program?.slots || {});
  if (!entries.length) {
    elSlots.appendChild(tile('(no slots found)'));
    return;
  }
  for (const [slot, plugin] of entries) {
    elSlots.appendChild(tile(`${slot} → ${plugin}`));
  }
}

async function refreshUI() {
  try {
    const res = await fetch('/api/state', { cache: 'no-store' });
    const data = await res.json();

    elNow.textContent = data?.meta?.now || '(no time)';
    elStatus.textContent = res.ok ? 'ok' : ('http ' + res.status);

    const presetList = data?.presets?.error ? [] : parsePresets(data?.presets?.raw || '');

    const pluginMetaMap = data?.dumpConfig?.error ? {} : parseDumpConfig(data?.dumpConfig?.raw || '');
    const trees = data?.dumpConfig?.error ? {} : parseFileTrees(data?.dumpConfig?.raw || '');

    const program = data?.program?.error
      ? parseDumpProgram('')
      : parseDumpProgram(data?.program?.raw || '');

    // Preset dropdown reflects *program.preset*
    setPresetDropdown(presetList, program.preset);

    // NAM/Cab read-only selectors (just display)
    const namCurrent = program?.params?.NAM?.Model ?? null;
    const cabCurrent = program?.params?.Cabinet?.Impulse ?? null;
    const namOpts = trees['NAM.Model'] || [];
    const cabOpts = trees['Cabinet.Impulse'] || [];

    elModelSelectors.innerHTML = '';

    elModelSelectors.appendChild(
      buildDropdown('NAM Model', namOpts, namCurrent, async (v) => {
        await setFileParam('NAM', 'Model', v);
        await refreshAfterFileParamChange('NAM', 'Model', v);
      })
    );

    elModelSelectors.appendChild(
      buildDropdown('Cab IR', cabOpts, cabCurrent, async (v) => {
        await setFileParam('Cabinet', 'Impulse', v);
        await refreshAfterFileParamChange('Cabinet', 'Impulse', v);
      })
    );


    // Render lanes + slots ONCE (and do not clear after)
    elLanes.innerHTML = '';
    renderChainsFromProgram(elLanes, program, pluginMetaMap);

    elSlots.innerHTML = '';
    renderSlotsFromDumpProgram(elSlots, program);

    // Debug
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
  await refreshUI();

})();
