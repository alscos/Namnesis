// render.js
(() => {
    const R = {};
    const U = () => window.NAMNESIS.util;
    const A = () => window.NAMNESIS.api;

    R.tile = function tile(text) {
        const d = document.createElement('div');
        d.className = 'tile';
        d.textContent = text;
        return d;
    };

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

    R.buildNumericSliderRow = function buildNumericSliderRow(pluginName, paramName, currentValue, meta, onCommit) {
        const row = document.createElement('div');
        row.className = 'kv';

        const kk = document.createElement('div');
        kk.className = 'k';
        kk.textContent = paramName + ':';

        const vv = document.createElement('div');
        vv.className = 'v';
        vv.textContent = Number(currentValue).toFixed(3);

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

        slider.addEventListener('input', (e) => {
            const val = parseFloat(e.target.value);
            vv.textContent = val.toFixed(3);
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
            const n = Number(raw);
            const meta = paramMetaForThisPlugin?.[k] || null;

            // --- 1) Output-only params are always read-only
            if (meta?.isOutput) {
                body.appendChild(R.buildReadOnlyRow(k, raw));
                continue;
            }

            // --- 2) File params (Model / Impulse) => dropdown if we have tree
            // In DumpConfig trees are keyed by BASE type (e.g. "ConvoReverb.Impulse"),
            // while program instances are "ConvoReverb_2", "ConvoReverb_3", etc.
            const metaType = String(meta?.type || '').toLowerCase();
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

            // --- 3) Decide if numeric slider/toggle is allowed
            const allow =
                !meta?.isOutput &&
                meta &&
                metaType !== 'bool' &&
                Number.isFinite(meta.min) &&
                Number.isFinite(meta.max) &&
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
                meta &&
                metaType !== 'bool' &&
                Number.isFinite(meta.min) &&
                Number.isFinite(meta.max);

            if (canSlider) {
                body.appendChild(R.buildNumericSliderRow(pluginName, k, n, meta, (val) => onParamCommit(pluginName, k, val)));
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
