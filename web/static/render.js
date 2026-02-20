// render.js
(() => {
    const R = {};
    const U = () => window.NAMNESIS.util;
    const A = () => window.NAMNESIS.api;


    // Parse a number even if the string includes units (e.g. "0 dB", "-3.5", "12.0ms").
    // Returns NaN if it doesn't start with a numeric token.
    function toNumberLoose(v) {
        if (typeof v === 'number') return v;
        if (typeof v !== 'string') return NaN;
        const s = v.trim();
        const m = s.match(/^[-+]?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?/i);
        if (!m) return NaN;
        return Number(m[0]);
    }
    R.tile = function tile(text) {
        const d = document.createElement('div');
        d.className = 'tile';
        d.textContent = text;
        return d;
    };

    function computeDecimalsForParam(step, decimalsFromStep) {
        const s = Number(step);
        const fromStep = decimalsFromStep ? decimalsFromStep(s) : 3;
        // Yo dejaría mínimo 2 o 3; si quieres “menos ruido visual”, pon 2.
        return Math.max(Number.isFinite(fromStep) ? fromStep : 3, 2);
    }

    R.buildDropdown = function buildDropdown(label, options, selectedValue, onChange, state) {
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
        state.isProgrammaticModelUpdate = true;
        sel.value = initial;
        state.isProgrammaticModelUpdate = false;

        if (typeof onChange === 'function') {
            sel.addEventListener('change', async (e) => {
                if (state.isProgrammaticModelUpdate) return;
                const v = e.target.value;
                if (!v) return;
                try { await onChange(v); } catch (err) { console.error(err); }
            });
        }

        wrap.appendChild(sel);
        return wrap;
    };

    R.renderChainsFromProgram = function renderChainsFromProgram(
        elLanes,
        program,
        pluginMetaByName,
        paramMetaByBaseType,
        buildPill,
        opts // NEW: { availablePlugins: [], onAddPlugin: async (chainName, baseType)=>{} }
    ) {
        elLanes.textContent = '';

        const chainOrder = ['Input', 'FxLoop', 'Output'];
        const chains = program?.chains || {};
        const paramsByPlugin = program?.params || {};

        for (const chainName of chainOrder) {
            const lane = document.createElement('div');
            lane.className = 'lane';

            // --- Lane header (title + Add plugin dropdown)
            const head = document.createElement('div');
            head.className = 'lane-head';

            const title = document.createElement('h3');
            title.textContent = chainName;
            head.appendChild(title);

            if (opts?.availablePlugins?.length && typeof opts?.onAddPlugin === 'function') {
                const sel = document.createElement('select');
                sel.className = 'lane-add';
                sel.innerHTML =
                    `<option value="">+ Add plugin...</option>` +
                    opts.availablePlugins.map(p => `<option value="${p}">${p}</option>`).join('');

                sel.addEventListener('change', async (e) => {
                    const v = e.target.value;
                    if (!v) return;
                    sel.disabled = true;
                    try {
                        await opts.onAddPlugin(chainName, v);
                    } catch (err) {
                        console.error(err);
                        alert(err?.message || String(err));
                    } finally {
                        sel.value = '';
                        sel.disabled = false;
                    }
                });
                head.appendChild(sel);
            }

            lane.appendChild(head);

            const items = chains[chainName] || [];
            if (!items.length) {
                const empty = document.createElement('div');
                empty.className = 'muted';
                empty.textContent = '(empty)';
                lane.appendChild(empty);
                elLanes.appendChild(lane);
                continue;
            }

            for (let i = 0; i < items.length; i++) {
                const pluginName = items[i];
                const p = paramsByPlugin[pluginName] || {};
                const baseType = pluginName.replace(/_\d+$/, '');
                const meta = pluginMetaByName?.[baseType] || pluginMetaByName?.[pluginName] || null;
                // NEW: pass chain context to the pill builder
                lane.appendChild(buildPill(pluginName, p, meta, baseType, {
                    chainName,
                    index: i,
                    chainItems: items
                }));
            }

            elLanes.appendChild(lane);
        }
    };
    function parseHexColor(s) {
        // #rgb or #rrggbb
        const h = s.replace('#', '').trim();
        if (h.length === 3) {
            const r = parseInt(h[0] + h[0], 16);
            const g = parseInt(h[1] + h[1], 16);
            const b = parseInt(h[2] + h[2], 16);
            return { r, g, b, a: 1 };
        }
        if (h.length === 6) {
            const r = parseInt(h.slice(0, 2), 16);
            const g = parseInt(h.slice(2, 4), 16);
            const b = parseInt(h.slice(4, 6), 16);
            return { r, g, b, a: 1 };
        }
        return null;
    }

    function parseRgbColor(s) {
        // rgb(r,g,b) or rgba(r,g,b,a)
        const m = s.match(/rgba?\s*\(\s*([0-9.]+)\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)\s*(?:,\s*([0-9.]+)\s*)?\)/i);
        if (!m) return null;
        const r = Number(m[1]), g = Number(m[2]), b = Number(m[3]);
        const a = (m[4] !== undefined) ? Number(m[4]) : 1;
        if (![r, g, b, a].every(Number.isFinite)) return null;
        return { r, g, b, a };
    }

    function relLuminance({ r, g, b }) {
        // sRGB -> linear
        const toLin = (c) => {
            c = c / 255;
            return (c <= 0.04045) ? (c / 12.92) : Math.pow((c + 0.055) / 1.055, 2.4);
        };
        const R = toLin(r), G = toLin(g), B = toLin(b);
        return 0.2126 * R + 0.7152 * G + 0.0722 * B;
    }

    function isDarkBackground(bg) {
        if (!bg) return false;
        const s = String(bg).trim().toLowerCase();

        // If it's a named color we only special-case "black"
        if (s === 'black') return true;

        let c = null;
        if (s.startsWith('#')) c = parseHexColor(s);
        else if (s.startsWith('rgb')) c = parseRgbColor(s);

        if (!c) return false;

        // If alpha < 1, assume it's on dark UI anyway; treat as dark if color itself is dark.
        const L = relLuminance(c);
        return L < 0.35; // threshold: tweak if needed (0.30–0.45 typical)
    }

    R.buildReadOnlyRow = function buildReadOnlyRow(paramName, raw) {
        const row = document.createElement('div');
        row.className = 'kv';

        const kk = document.createElement('div');
        kk.className = 'k';
        kk.textContent = paramName + ':';

        const vv = document.createElement('div');
        vv.className = 'v';
        vv.textContent = U().withUnit(paramName, raw);

        row.appendChild(kk);
        row.appendChild(vv);
        return row;
    };

    R.buildNumericSliderRow = function buildNumericSliderRow(pluginName, paramName, currentValue, meta, onCommit, uiState) {
  const row = document.createElement('div');
  row.className = 'kv';

  const kk = document.createElement('div');
  kk.className = 'k';
  kk.textContent = paramName + ':';

  const vv = document.createElement('div');
  vv.className = 'v';

  // --- slider FIRST (so editor can reference it) ---
  const slider = document.createElement('input');
  slider.type = 'range';

  const min = Number.isFinite(meta?.min) ? meta.min : 0;
  const max = Number.isFinite(meta?.max) ? meta.max : 1;
  const step = Number.isFinite(meta?.step) ? meta.step : 0.01;

  slider.min = String(min);
  slider.max = String(max);
  slider.step = String(step);
  slider.value = String(Number(currentValue));

  slider.style.gridColumn = '1 / -1';
  slider.style.width = '100%';
  slider.style.marginTop = '4px';

  // --- editor AFTER slider ---
  let editor = null;
  if (uiState && typeof uiState === 'object') {
    editor = buildInlineNumericEditor({
      plugin: pluginName,
      param: paramName,
      value: Number(currentValue),
      meta,
      onNudge: (val) => {
        slider.value = String(val);
        slider.dispatchEvent(new Event('input', { bubbles: true }));
      },
      onCommit: async (val) => {
        try { await onCommit(val); }
        catch (err) { console.error(err); alert(`SetParam failed: ${err.message || err}`); }
      },
      uiState,
    });
    vv.appendChild(editor.el);
  } else {
    const step0 = Number.isFinite(meta?.step) ? meta.step : 0.1;
    const dec = computeDecimalsForParam(step0, uiState?.numericEdit?.decimalsFromStep);
    vv.textContent = Number(currentValue).toFixed(dec);
  }

  slider.addEventListener('input', (e) => {
    const val = parseFloat(e.target.value);
    if (editor && editor.el) {
      const key = `${pluginName}::${paramName}`;
      const locked = uiState?.editing?.has?.(key);
      const hasFocus = (document.activeElement === editor.el);
      if (!locked && !hasFocus) {
        editor.el.value = editor.fmt ? editor.fmt(val) : String(val);
      }
    } else {
      const step0 = Number.isFinite(meta?.step) ? meta.step : 0.1;
      const dec = computeDecimalsForParam(step0, uiState?.numericEdit?.decimalsFromStep);
      vv.textContent = val.toFixed(dec);
    }
  });

  slider.addEventListener('change', async (e) => {
    const val = parseFloat(e.target.value);
    try { await onCommit(val); }
    catch (err) { console.error(err); alert(`SetParam failed: ${err.message || err}`); }
  });

  row.appendChild(kk);
  row.appendChild(vv);
  row.appendChild(slider);
  return row;
};

    R.buildBooleanToggleRow = function buildBooleanToggleRow(pluginName, paramName, n, onCommit) {
        const row = document.createElement('div');
        row.className = 'kv';

        const kk = document.createElement('div');
        kk.className = 'k';
        kk.textContent = paramName + ':';

        const wrap = document.createElement('div');
        wrap.className = 'v';

        const btn = document.createElement('button');
        btn.type = 'button';

        const initialOn = Number(n) >= 0.5;
        btn.className = 'pill-state pill-param-toggle ' + (initialOn ? 'on' : 'off');
        btn.innerHTML = `
      <div class="switch-container">
        <div class="switch-track"></div>
        <div class="switch-thumb"></div>
      </div>
    `;
        btn.setAttribute('aria-label', `Toggle ${pluginName} ${paramName}`);
        btn.setAttribute('aria-pressed', initialOn ? 'true' : 'false');

        btn.addEventListener('click', async () => {
            const currentOn = (btn.getAttribute('aria-pressed') === 'true');
            const nextOn = !currentOn;
            const nextVal = nextOn ? 1 : 0;

            // optimistic UI
            btn.classList.toggle('on', nextOn);
            btn.classList.toggle('off', !nextOn);
            btn.setAttribute('aria-pressed', nextOn ? 'true' : 'false');

            try {
                await onCommit(nextVal);
            } catch (err) {
                // rollback
                btn.classList.toggle('on', currentOn);
                btn.classList.toggle('off', !currentOn);
                btn.setAttribute('aria-pressed', currentOn ? 'true' : 'false');
                console.error(err);
                alert(`SetParam failed: ${err.message || err}`);
            }
        });

        wrap.appendChild(btn);
        row.appendChild(kk);
        row.appendChild(wrap);
        return row;
    };

    R.setToggleUI = function setToggleUI(el, enabled) {
        el.classList.toggle("on", enabled);
        el.classList.toggle("off", !enabled);
        el.setAttribute("aria-pressed", enabled ? "true" : "false");
    };

    R.wirePluginToggle = function wirePluginToggle(el, name, onResync) {
        el.addEventListener("click", async () => {
            if (el.dataset.busy === "1") return;
            el.dataset.busy = "1";

            const current = (el.getAttribute("aria-pressed") === "true");
            const next = !current;

            R.setToggleUI(el, next);
            el.classList.add("is-busy");

            try {
                await A().setPluginEnabled(name, next);
                await onResync(); // authoritative re-sync
            } catch (e) {
                R.setToggleUI(el, current);
                console.error(e);
                alert(`Toggle failed: ${e.message}`);
            } finally {
                el.classList.remove("is-busy");
                el.dataset.busy = "0";
            }
        });

        el.addEventListener("keydown", (ev) => {
            if (ev.key === " " || ev.key === "Enter") {
                ev.preventDefault();
                el.click();
            }
        });
    };
    function buildInlineNumericEditor({ plugin, param, value, meta, uiState, onCommit, onNudge }) {
        const key = `${plugin}::${param}`;
        const { decimalsFromStep, parseUserNumber } = uiState.numericEdit || {};

        const min = Number.isFinite(meta?.min) ? meta.min : undefined;
        const max = Number.isFinite(meta?.max) ? meta.max : undefined;
        const step0 = Number.isFinite(meta?.step) ? meta.step : 0.1;

        // Decimals: based on step, but show extra for fine-feel params
        const dec = computeDecimalsForParam(step0, decimalsFromStep);

        // IMPORTANT: Do not quantize “fine-feel” params on commit,
        // or ArrowUp/Down will look like it “does nothing” until x10.
        const doSnap = false;

        const clampBounds = (x) => {
            let y = x;
            if (Number.isFinite(min)) y = Math.max(min, y);
            if (Number.isFinite(max)) y = Math.min(max, y);
            return y;
        };

        // Optional: snap to step0 exactly (avoid global quantization)
        const snapToStep = (x) => {
            const s = Number(step0);
            if (!Number.isFinite(s) || s <= 0) return x;
            return Math.round(x / s) * s;
        };

        const fmt = (n) => {
            const nn = Number(n);
            if (!Number.isFinite(nn)) return String(n ?? '');
            return nn.toFixed(dec);
        };

        const input = document.createElement('input');
        input.type = 'text';
        input.inputMode = 'decimal';
        input.autocomplete = 'off';
        input.spellcheck = false;
        input.className = 'pill-param-input'; // ponle CSS acorde (abajo te digo)

        input.value = fmt(value);

        const commit = async () => {
            const raw = input.value;
            const n = parseUserNumber ? parseUserNumber(raw) : Number(raw);
            if (!Number.isFinite(n)) {
                input.value = fmt(value);
                return;
            }
            const nn0 = doSnap ? snapToStep(n) : n;
            const nn = clampBounds(nn0);
            input.value = fmt(nn);
            await onCommit(nn);
        };

        input.addEventListener('focus', () => uiState.editing.add(key));
        input.addEventListener('blur', async () => {
            try { await commit(); } finally { uiState.editing.delete(key); }
        });

        input.addEventListener('keydown', async (e) => {
            if (e.key === 'Enter') { e.preventDefault(); await commit(); input.blur(); return; }
            if (e.key === 'Escape') { e.preventDefault(); input.value = fmt(value); input.blur(); return; }

            if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
                e.preventDefault();
                const cur = parseUserNumber ? parseUserNumber(input.value) : Number(input.value);
                const base = Number.isFinite(cur) ? cur : (Number.isFinite(value) ? value : 0);

               // Keyboard nudge step (fixed): 0.10 per keystroke
                // Modifiers:
                //  - Shift: x10 (1.0)
                //  - Alt/Ctrl: /10 (0.01)
                let s = 0.10;
                if (e.shiftKey) s *= 10;
                if (e.altKey || e.ctrlKey) s /= 10;

                const rawNext = base + (e.key === 'ArrowUp' ? s : -s);
                // For keyboard nudges, don't snap back to coarse step0; just clamp bounds.
                const next = clampBounds(rawNext);
                input.value = fmt(next);
                if (typeof onNudge === 'function') onNudge(next);
                await onCommit(next);
            }
        });

        return { el: input, key, fmt };
    }
    R.buildPluginPill = function buildPluginPill(opts) {
        const {
            pluginName, pluginParams, bgColor, fgColor,
            paramMetaForThisPlugin,
            fileTrees,
            pickKeys,
            isBooleanParam,
            WRITABLE_NUMERIC_PARAMS,
            onParamCommit,
            onPluginToggleResync,
            uiState,
            // NEW: chain context (provided by renderChainsFromProgram via buildPill wrapper)
            chainName,
            chainIndex,
            chainLength,

            // NEW: callbacks (provided by app.js)
            onUnload,   // async ({chainName, pluginName, index}) => {}
            onMoveUp,   // async ({chainName, pluginName, from}) => {}
            onMoveDown  // async ({chainName, pluginName, from}) => {}
        } = opts;

        const el = document.createElement('div');
        el.className = 'pill';
        if (bgColor) el.style.background = bgColor;

        // Apply dark/light class for param input contrast
        if (isDarkBackground(bgColor)) {
            el.classList.add('pill-dark');
        }
        // Contrast class for editable numeric input (and optionally other text)
        const dark = isDarkBackground(bgColor);
        el.classList.toggle('pill-dark', dark);
        el.classList.toggle('pill-light', !dark);

        if (fgColor) el.style.color = fgColor;


        const enabledRaw = pluginParams?.Enabled;
        const isOn = String(enabledRaw) === '1';
        if (!isOn) el.classList.add('is-off');

        const head = document.createElement('div');
        head.className = 'pill-head';

        const title = document.createElement('div');
        title.className = 'pill-title';
        title.textContent = pluginName;

        const state = document.createElement('button');
        state.type = "button";
        state.className = 'pill-state ' + (isOn ? 'on' : 'off');
        state.innerHTML = `
    <div class="switch-container">
      <div class="switch-track"></div>
      <div class="switch-thumb"></div>
    </div>
  `;
        state.setAttribute("aria-label", `Toggle ${pluginName}`);
        state.setAttribute("aria-pressed", isOn ? "true" : "false");

        if (enabledRaw === undefined) state.disabled = true;
        else R.wirePluginToggle(state, pluginName, onPluginToggleResync);

        head.appendChild(title);
        head.appendChild(state);
        el.appendChild(head);

        // --- NEW: corner actions (Unload / Move Up / Move Down)
        // Only render if callbacks provided.
        if (typeof onUnload === 'function') {
            const x = document.createElement('button');
            x.type = 'button';
            x.className = 'pill-corner pill-x';
            x.textContent = '×';
            x.title = 'Unload';
            x.addEventListener('click', async (ev) => {
                ev.stopPropagation();
                const ok = confirm(`Unload ${pluginName} from ${chainName}?`);
                if (!ok) return;
                try {
                    x.disabled = true;
                    await onUnload({ chainName, pluginName, index: chainIndex });
                } catch (err) {
                    console.error(err);
                    alert(err?.message || String(err));
                } finally {
                    x.disabled = false;
                }
            });
            el.appendChild(x);
        }

        if (typeof onMoveUp === 'function') {
            const up = document.createElement('button');
            up.type = 'button';
            up.className = 'pill-corner pill-up';
            up.textContent = '↑';
            up.title = 'Move up';
            if (!Number.isFinite(chainIndex) || chainIndex <= 0) up.disabled = true;
            up.addEventListener('click', async (ev) => {
                ev.stopPropagation();
                try {
                    up.disabled = true;
                    await onMoveUp({ chainName, pluginName, from: chainIndex });
                } catch (err) {
                    console.error(err);
                    alert(err?.message || String(err));
                }
            });
            el.appendChild(up);
        }

        if (typeof onMoveDown === 'function') {
            const down = document.createElement('button');
            down.type = 'button';
            down.className = 'pill-corner pill-down';
            down.textContent = '↓';
            down.title = 'Move down';
            if (!Number.isFinite(chainIndex) || (Number.isFinite(chainLength) && chainIndex >= chainLength - 1)) down.disabled = true;
            down.addEventListener('click', async (ev) => {
                ev.stopPropagation();
                try {
                    down.disabled = true;
                    await onMoveDown({ chainName, pluginName, from: chainIndex });
                } catch (err) {
                    console.error(err);
                    alert(err?.message || String(err));
                }
            });
            el.appendChild(down);
        }


        const body = document.createElement('div');
        body.className = 'pill-body';

        const keys = pickKeys(pluginName, pluginParams);

        const baseType = pluginName.replace(/_\d+$/, '');

        for (const k of keys) {
            const raw = pluginParams?.[k];
            const n = toNumberLoose(raw);
            const meta = paramMetaForThisPlugin?.[k] || null;
            const metaType = String(meta?.type || '').toLowerCase();
            const lname = String(k || '').toLowerCase();

            const isLevelish =
                lname === 'level' ||
                lname === 'gain' ||
                lname === 'vol' ||
                lname === 'volume' ||
                lname === 'threshold';



            // Note: meta is declared as const above; we create a new reference for downstream logic.
            const meta2 = (!meta || !Number.isFinite(meta.min) || !Number.isFinite(meta.max))
                ? (
                    (lname === 'volume' || lname === 'vol' || lname === 'gain')
                        ? { min: -40, max: 40, step: 0.1, def: 0, unit: 'dB' }
                        : (isLevelish ? { min: -24, max: 24, step: 0.1, def: 0, unit: 'dB' } : meta)
                )
                : meta;
            // --- 2.5) HARDEN: "Level" plugin semantics
            // Stompbox "Level" plugin exposes:
            //   - Volume : writable trim (persisted)
            //   - Level  : read-only meter/output (always returns ~0 / runtime value, not persisted)
            // So: force Volume to slider and force Level to read-only.
            if (baseType === 'Level') {
                if (lname === 'level') {
                    body.appendChild(R.buildReadOnlyRow(k, raw));
                    continue;
                }
                if (lname === 'volume') {
                    const v = Number.isFinite(n) ? n : 0;
                    const m = (meta2 && Number.isFinite(meta2.min) && Number.isFinite(meta2.max))
                        ? meta2
                        : { min: -40, max: 40, step: 0.1, def: 0, unit: 'dB' };
                    body.appendChild(
                        R.buildNumericSliderRow(pluginName, k, v, m, (val) => onParamCommit(pluginName, k, val), uiState)
                    );
                    continue;
                }
            }

            // --- 1) Output-only params are always read-only
            if (meta?.isOutput && !isLevelish) {
                body.appendChild(R.buildReadOnlyRow(k, raw));
                continue;
            }

            // --- 2) File params (Model / Impulse) => dropdown if we have tree
            // In DumpConfig trees are keyed by BASE type (e.g. "ConvoReverb.Impulse"),
            // while program instances are "ConvoReverb_2", "ConvoReverb_3", etc.

            const isFileParam = (k === 'Model' || k === 'Impulse') && metaType === 'file';

            if (isFileParam) {
                const keyBase = `${baseType}.${k}`;       // e.g. "ConvoReverb.Impulse"
                const keyExact = `${pluginName}.${k}`;    // fallback if ever provided
                const options = (fileTrees && (fileTrees[keyBase] || fileTrees[keyExact])) || [];

                if (options.length) {
                    // buildDropdown(label, options, current, onChange, state)
                    const ddState = { isProgrammaticModelUpdate: false };
                    const row = R.buildDropdown(k, options, raw, async (v) => {
                        try {
                            // NOTE: setFileParam is usually the correct endpoint for file params,
                            // but your onParamCommit currently targets numeric only.
                            // So we call a dedicated hook if present, else fallback to onParamCommit.
                            if (opts.onFileParamCommit) {
                                await opts.onFileParamCommit(pluginName, k, v);  // instancia: ConvoReverb_2
                            } else {
                                // last-resort fallback (won't work if backend expects /api/param/file)
                                await onParamCommit(pluginName, k, v);
                            }
                            await onPluginToggleResync();
                        } catch (err) {
                            console.error(err);
                            alert(err?.message || String(err));
                        }
                    }, ddState);

                    body.appendChild(row);
                } else {
                    body.appendChild(R.buildReadOnlyRow(k, raw));
                }
                continue;
            }

            const allow =
                (meta?.isOutput ? isLevelish : true) &&
                meta2 &&
                metaType !== 'bool' &&
                Number.isFinite(meta2.min) &&
                Number.isFinite(meta2.max) &&
                k !== 'Model' &&
                k !== 'Impulse';

            // NOTE: your current boolean block is unreachable because allow excludes bool.
            // We'll keep your intended behaviour: if bool -> toggle.
            const isBool = isBooleanParam(meta, k);
            if (isBool && Number.isFinite(n)) {
                body.appendChild(R.buildBooleanToggleRow(pluginName, k, n, (val) => onParamCommit(pluginName, k, val)));
                continue;
            }

            const canSlider =
                allow &&
                Number.isFinite(n) &&
                meta2 &&
                metaType !== 'bool' &&
                Number.isFinite(meta2.min) &&
                Number.isFinite(meta2.max);

            if (canSlider) {
                body.appendChild(R.buildNumericSliderRow(pluginName, k, n, meta2, (val) => onParamCommit(pluginName, k, val), uiState));
                continue;
            }

            body.appendChild(R.buildReadOnlyRow(k, raw));
        }

        el.appendChild(body);
        return el;
    };


    R.renderSlotsFromDumpProgram = function renderSlotsFromDumpProgram(elSlots, program) {
        const entries = Object.entries(program?.slots || {});
        elSlots.innerHTML = '';
        if (!entries.length) {
            elSlots.appendChild(R.tile('(no slots found)'));
            return;
        }
        for (const [slot, plugin] of entries) {
            elSlots.appendChild(R.tile(`${slot} → ${plugin}`));
        }
    };

    window.NAMNESIS = window.NAMNESIS || {};
    window.NAMNESIS.render = R;
})();
