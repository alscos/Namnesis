(() => {
  "use strict";

  const A = window.NAMNESIS.api;
  const P = window.NAMNESIS.parse;

  const els = {
    app: document.getElementById("liveApp"),
    presetNumber: document.getElementById("presetNumber"),
    presetName: document.getElementById("presetName"),
    namModel: document.getElementById("namModel"),
    cabinetIR: document.getElementById("cabinetIR"),
    namPickerButton: document.getElementById("namPickerButton"),
    cabPickerButton: document.getElementById("cabPickerButton"),
    savePresetButton: document.getElementById("savePresetButton"),
    saveAsPresetButton: document.getElementById("saveAsPresetButton"),
    streamText: document.getElementById("streamText"),
    stompRack: document.getElementById("stompRack"),
    editorTitle: document.getElementById("editorTitle"),
    editorToggle: document.getElementById("editorToggle"),
    editorToggleText: document.getElementById("editorToggleText"),
    parameterGrid: document.getElementById("parameterGrid"),
    statusPanel: document.querySelector(".status-panel"),
    readyText: document.getElementById("readyText"),
    xrunValue: document.getElementById("xrunValue"),
    midiValue: document.getElementById("midiValue"),
    audioValue: document.getElementById("audioValue"),
    revisionValue: document.getElementById("revisionValue"),
    inputValue: document.getElementById("inputValue"),
    masterValue: document.getElementById("masterValue"),
    clockValue: document.getElementById("clockValue"),
    libraryOverlay: document.getElementById("libraryOverlay"),
    libraryEyebrow: document.getElementById("libraryEyebrow"),
    libraryTitle: document.getElementById("libraryTitle"),
    libraryCurrent: document.getElementById("libraryCurrent"),
    libraryCount: document.getElementById("libraryCount"),
    libraryClose: document.getElementById("libraryClose"),
    libraryQuery: document.getElementById("libraryQuery"),
    libraryKeyboard: document.getElementById("libraryKeyboard"),
    libraryResults: document.getElementById("libraryResults"),
  };

  const state = {
    snapshot: null,
    program: null,
    config: null,
    paramMeta: null,
    fileTrees: {},
    resolved: null,
    selectedController: null,
    stompPage: 0,
    stompPresetKey: null,
    pendingParams: new Map(),
    activeParamKey: null,
    library: {
      kind: null,
      query: "",
      busy: false,
    },
    refreshPromise: null,
    eventSource: null,
    pendingFrame: null,
    systemTimer: null,
  };

  const PREFERRED_PARAMS = {
    Phase90Script: ["Speed", "Depth", "Center", "FrqWidth", "Spread", "FBack"],
    Delay: ["Delay", "Mix", "FBack", "Warm", "HiLo"],
    ConvoReverb: ["Wet", "Dry"],
    Reverb: ["Blend", "Decay", "Size"],
    Chorus: ["Rate", "Depth"],
    Flanger: ["Rate", "Depth", "FBack"],
    Vibrato: ["Speed", "Depth", "FBack", "Ratio"],
    Tremolo: ["Speed", "Depth", "Shape"],
    Compressor: ["Threshold", "Ratio", "Attack", "Release", "Blend", "Comp"],
    NoiseGate: ["Threshold", "Strength", "Attack", "Release", "Soft"],
    Boost: ["Gain", "Level"],
    Screamer: ["Drive", "Tone", "Level"],
    Fuzz: ["Fuzz", "Level", "Bias", "Octave"],
    Level: ["Volume"],
  };

  const STOMP_PAGE_SIZE = 12;

  const STOMP_STYLES = {
    Boost:         { name: "BOOST",   color: "#42d667", knobs: 2 },
    Screamer:      { name: "DRIVE",   color: "#ff4454", knobs: 3 },
    Delay:         { name: "DELAY",   color: "#438cff", knobs: 4 },
    ConvoReverb:   { name: "REVERB",  color: "#2ed6d3", knobs: 6 },
    Reverb:        { name: "REVERB",  color: "#2ed6d3", knobs: 5 },
    Compressor:    { name: "COMP",    color: "#ffb51b", knobs: 4 },
    NoiseGate:     { name: "GATE",    color: "#f59e0b", knobs: 4 },
    SimpleGate:    { name: "GATE",    color: "#f59e0b", knobs: 3 },
    Level:         { name: "LEVEL",   color: "#e5e7eb", knobs: 1 },
    Fuzz:          { name: "FUZZ",    color: "#ef5ee8", knobs: 3 },
    Phase90Script: { name: "PHASE",   color: "#ff8a2d", knobs: 3 },
    Phaser:        { name: "PHASE",   color: "#ff8a2d", knobs: 3 },
    Chorus:        { name: "CHORUS",  color: "#27c6c7", knobs: 3 },
    Tremolo:       { name: "TREMOLO", color: "#b45cff", knobs: 3 },
    Vibrato:       { name: "VIBRATO", color: "#ff68bd", knobs: 3 },
  };

  const KEYBOARD_ROWS = [
    "1234567890-_",
    "QWERTYUIOP",
    "ASDFGHJKL",
    "ZXCVBNM",
  ];

  async function postJSON(url, body = {}) {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    const text = await response.text().catch(() => "");
    let payload = {};

    if (text) {
      try {
        payload = JSON.parse(text);
      } catch {
        payload = { message: text };
      }
    }

    if (!response.ok) {
      throw new Error(
        payload?.error ||
        payload?.message ||
        text ||
        `HTTP ${response.status}`,
      );
    }

    return payload;
  }

  function pluginBase(name) {
    return String(name || "").replace(/_\d+$/, "");
  }

  function friendlyPluginName(name) {
    const base = pluginBase(name);
    const aliases = {
      Phase90Script: "Phase 90",
      ConvoReverb: "Conv Rev",
      Screamer: "Drive",
      NoiseGate: "Gate",
    };
    return aliases[base] || base.replace(/([a-z])([A-Z])/g, "$1 $2");
  }

  function splitPreset(value) {
    const raw = String(value || "").trim();
    const match = raw.match(/^(\d+)[\s._-]*(.*)$/);
    if (!match) {
      return { number: "—", name: raw.replace(/[_-]+/g, " ") || "No preset loaded" };
    }
    return {
      number: match[1],
      name: (match[2] || raw).replace(/[_-]+/g, " ").trim(),
    };
  }

  function isEnabled(plugin) {
    const raw = state.program?.params?.[plugin]?.Enabled;
    return raw !== undefined && Number(raw) !== 0;
  }

  function resolvedValue(plugin, param) {
    const live = String(state.program?.params?.[plugin]?.[param] ?? "").trim();
    if (live) return live;
    return String(state.resolved?.[plugin]?.[param] ?? "").trim();
  }

  function findPlugin(baseName) {
    const names = new Set([
      ...Object.keys(state.program?.params || {}),
      ...Object.values(state.program?.slots || {}),
      ...Object.keys(state.resolved || {}),
    ]);
    return Array.from(names).find(name => {
      const base = pluginBase(name);
      return name === baseName || base === baseName;
    }) || baseName;
  }

  function parseMappedStomps(raw) {
    const mappings = new Map();

    for (const line of String(raw || "").split(/\r?\n/)) {
      const match = line.match(
        /^\s*MapController\s+(\d+)\s+(\S+)\s+(\S+)\s*$/,
      );
      if (!match) continue;

      const controller = Number(match[1]);
      const plugin = match[2];
      const param = match[3];

      if (
        !Number.isInteger(controller) ||
        controller < 1 ||
        controller > 127 ||
        param.toLowerCase() !== "enabled"
      ) {
        continue;
      }

      // A controller number has one effective mapping. Keep the final line,
      // matching the way a later mapping supersedes an earlier one.
      mappings.set(controller, { controller, plugin, param });
    }

    return Array.from(mappings.values())
      .sort((left, right) => left.controller - right.controller);
  }

  function loadedPluginNames() {
    return new Set(
      Object.values(state.program?.chains || {})
        .flat()
        .filter(Boolean),
    );
  }

  function inferredKnobCount(plugin) {
    const params = state.program?.params?.[plugin] || {};
    const numeric = Object.entries(params).filter(([name, value]) => (
      name !== "Enabled" &&
      name !== "Model" &&
      name !== "Impulse" &&
      Number.isFinite(Number(value))
    )).length;

    return Math.max(1, Math.min(6, numeric || 3));
  }

  function stompStyle(plugin) {
    const base = pluginBase(plugin);
    const known = STOMP_STYLES[base];

    return {
      name: known?.name || friendlyPluginName(plugin).toUpperCase(),
      color: known?.color || pluginColor(plugin),
      knobs: known?.knobs || inferredKnobCount(plugin),
    };
  }

  function getMappedStomps() {
    const loaded = loadedPluginNames();

    return parseMappedStomps(state.snapshot?.program?.raw || "")
      .map(mapping => {
        const style = stompStyle(mapping.plugin);
        const isLoaded = loaded.has(mapping.plugin);

        return {
          ...mapping,
          ...style,
          loaded: isLoaded,
          enabled: isLoaded && isEnabled(mapping.plugin),
        };
      });
  }

  function selectedStomp(stomps = getMappedStomps()) {
    return stomps.find(
      stomp => stomp.controller === state.selectedController,
    ) || null;
  }

  function pluginColor(plugin) {
    const base = pluginBase(plugin);
    return state.config?.[plugin]?.bg || state.config?.[base]?.bg || "#8b5cf6";
  }

  function formatNumber(value, meta) {
    const number = Number(value);
    if (!Number.isFinite(number)) return String(value ?? "—");

    const step = Number(meta?.step);
    let digits = 2;
    if (Number.isFinite(step)) {
      if (step >= 1) digits = 0;
      else if (step >= 0.1) digits = 1;
      else if (step >= 0.01) digits = 2;
      else digits = 3;
    }
    return number.toFixed(digits);
  }

  function parameterKey(plugin, param) {
    return `${plugin}::${param}`;
  }

  function valuesMatch(actual, expected, step) {
    const left = Number(actual);
    const right = Number(expected);
    if (!Number.isFinite(left) || !Number.isFinite(right)) return false;

    const numericStep = Number(step);
    const tolerance = Number.isFinite(numericStep) && numericStep > 0
      ? Math.max(numericStep * 0.51, 1e-6)
      : 1e-6;

    return Math.abs(left - right) <= tolerance;
  }

  function holdPendingParam(plugin, param, value, step) {
    state.pendingParams.set(parameterKey(plugin, param), {
      plugin,
      param,
      value: Number(value),
      step: Number(step),
      expiresAt: Date.now() + 5000,
    });
  }

  function effectiveParamValue(plugin, param, fallback) {
    const pending = state.pendingParams.get(parameterKey(plugin, param));
    return pending ? pending.value : fallback;
  }

  function reconcilePendingParams() {
    const now = Date.now();

    for (const [key, pending] of state.pendingParams) {
      const actual = state.program?.params?.[pending.plugin]?.[pending.param];

      if (valuesMatch(actual, pending.value, pending.step) || now >= pending.expiresAt) {
        state.pendingParams.delete(key);
      }
    }
  }

  function selectDefaultController(stomps) {
    if (
      state.selectedController !== null &&
      stomps.some(stomp => stomp.controller === state.selectedController)
    ) {
      return;
    }

    const selected =
      stomps.find(stomp => (
        stomp.loaded &&
        pluginBase(stomp.plugin) === "Phase90Script"
      )) ||
      stomps.find(stomp => stomp.loaded && stomp.enabled) ||
      stomps.find(stomp => stomp.loaded) ||
      stomps[0] ||
      null;

    state.selectedController = selected?.controller ?? null;
  }

  function setStreamStatus(status) {
    els.app.dataset.stream = status;
    els.streamText.textContent =
      status === "online" ? "Synchronized" :
      status === "offline" ? "Reconnecting" :
      "Connecting";
  }

  function renderIdentity() {
    const preset = splitPreset(state.program?.preset);
    els.presetNumber.textContent = preset.number;
    els.presetName.textContent = preset.name;

    const nam = findPlugin("NAM");
    const cabinet = findPlugin("Cabinet");
    els.namModel.textContent =
      isEnabled(nam) ? resolvedValue(nam, "Model") || "—" : "NONE — BYPASSED";
    els.cabinetIR.textContent =
      isEnabled(cabinet) ? resolvedValue(cabinet, "Impulse") || "—" : "NONE — BYPASSED";

    const input = Number(state.program?.params?.Input?.Gain);
    const master = Number(state.program?.params?.Master?.Volume);
    els.inputValue.textContent = Number.isFinite(input) ? `${input.toFixed(1)} dB` : "—";
    els.masterValue.textContent = Number.isFinite(master) ? `${master.toFixed(1)} dB` : "—";
    els.revisionValue.textContent = String(state.snapshot?.meta?.revision ?? "—");
  }

  async function togglePlugin(plugin, enabled) {
    await A.setPluginEnabled(plugin, enabled);
    scheduleRefresh();
  }

  function renderStomps() {
    const stomps = getMappedStomps();
    const presetKey = String(state.program?.preset || "");

    if (presetKey !== state.stompPresetKey) {
      state.stompPresetKey = presetKey;
      state.stompPage = 0;
      state.selectedController = null;
    }

    const pageCount = Math.max(
      1,
      Math.ceil(stomps.length / STOMP_PAGE_SIZE),
    );
    state.stompPage = Math.min(
      Math.max(0, state.stompPage),
      pageCount - 1,
    );

    selectDefaultController(stomps);
    els.stompRack.replaceChildren();

    if (!stomps.length) {
      state.selectedController = null;
      els.stompRack.dataset.paged = "false";
      els.stompRack.style.setProperty("--stomp-columns", "1");
      els.stompRack.style.setProperty("--stomp-max-width", "720px");

      const empty = document.createElement("div");
      empty.className = "stomp-rack-empty";
      empty.innerHTML = `
        <strong>NO STOMPS MAPPED</strong>
        <span>This preset has no MapController … Enabled entries.</span>
      `;
      els.stompRack.appendChild(empty);
      return;
    }

    const start = state.stompPage * STOMP_PAGE_SIZE;
    const page = stomps.slice(start, start + STOMP_PAGE_SIZE);
    const columnCount = Math.max(1, page.length);
    const baseWidth = columnCount * 104 + Math.max(0, columnCount - 1) * 5;
    const paged = pageCount > 1;

    els.stompRack.dataset.paged = String(paged);
    els.stompRack.style.setProperty(
      "--stomp-columns",
      String(columnCount),
    );
    els.stompRack.style.setProperty(
      "--stomp-max-width",
      `${baseWidth + (paged ? 64 : 0)}px`,
    );

    for (const stomp of page) {
      const plugin = stomp.plugin;
      const loaded = stomp.loaded;
      const enabled = stomp.enabled;
      const selected = stomp.controller === state.selectedController;
      const knobs = Array.from(
        { length: stomp.knobs },
        () => "<i></i>",
      ).join("");

      const tile = document.createElement("article");
      tile.className = "stomp";
      tile.dataset.enabled = String(enabled);
      tile.dataset.present = String(loaded);
      tile.dataset.selected = String(selected);
      tile.dataset.controller = String(stomp.controller);
      tile.style.setProperty("--plugin-color", stomp.color);
      tile.setAttribute(
        "aria-label",
        loaded
          ? `Controller ${stomp.controller}, ${stomp.name}: ${enabled ? "on" : "off"}`
          : `Controller ${stomp.controller}, ${plugin}: mapped but not loaded`,
      );

      const selectButton = document.createElement("button");
      selectButton.type = "button";
      selectButton.className = "stomp-select";
      selectButton.disabled = !loaded;
      selectButton.setAttribute(
        "aria-label",
        loaded
          ? `Open ${stomp.name} editor`
          : `${plugin} is mapped but not loaded`,
      );
      selectButton.innerHTML = `
        <span class="stomp-label">${stomp.name}</span>
        <span class="pedal-face" aria-hidden="true">
          <span class="controller-number">${stomp.controller}</span>
          <span class="pedal-knobs knobs-${stomp.knobs}">${knobs}</span>
          <span class="pedal-footswitch"></span>
          ${loaded ? "" : '<span class="pedal-warning">!</span>'}
        </span>
      `;

      const toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "state-toggle";
      toggle.disabled = !loaded;
      toggle.setAttribute("aria-pressed", String(enabled));
      toggle.setAttribute(
        "aria-label",
        loaded
          ? `Turn ${stomp.name} ${enabled ? "off" : "on"}`
          : `${plugin} is not loaded in this preset`,
      );
      toggle.innerHTML = loaded
        ? '<span class="state-hex" aria-hidden="true"></span>'
        : `
          <span class="stomp-unloaded-state" aria-hidden="true">
            <strong>!</strong>
            <small>NOT LOADED</small>
          </span>
        `;

      selectButton.addEventListener("click", () => {
        if (!loaded) return;
        state.selectedController = stomp.controller;
        renderStomps();
        renderEditor();
      });

      toggle.addEventListener("click", async () => {
        if (!loaded) return;
        toggle.disabled = true;

        try {
          await togglePlugin(plugin, !enabled);
        } catch (error) {
          console.error(error);
          setStreamStatus("offline");
        } finally {
          toggle.disabled = false;
        }
      });

      tile.append(selectButton, toggle);
      els.stompRack.appendChild(tile);
    }

    const changePage = direction => {
      const next = Math.min(
        Math.max(0, state.stompPage + direction),
        pageCount - 1,
      );
      if (next === state.stompPage) return;

      state.stompPage = next;
      const nextStart = next * STOMP_PAGE_SIZE;
      const nextPage = stomps.slice(
        nextStart,
        nextStart + STOMP_PAGE_SIZE,
      );
      const nextSelected =
        nextPage.find(stomp => stomp.loaded && stomp.enabled) ||
        nextPage.find(stomp => stomp.loaded) ||
        nextPage[0] ||
        null;

      state.selectedController = nextSelected?.controller ?? null;
      renderStomps();
      renderEditor();
    };

    if (state.stompPage > 0) {
      const previous = document.createElement("button");
      previous.type = "button";
      previous.className = "stomp-page-button is-previous";
      previous.setAttribute(
        "aria-label",
        `Show stomp page ${state.stompPage}`,
      );
      previous.textContent = "‹";
      previous.addEventListener("click", () => changePage(-1));
      els.stompRack.appendChild(previous);
    }

    if (state.stompPage < pageCount - 1) {
      const next = document.createElement("button");
      next.type = "button";
      next.className = "stomp-page-button is-next";
      next.setAttribute(
        "aria-label",
        `Show stomp page ${state.stompPage + 2}`,
      );
      next.textContent = "›";
      next.addEventListener("click", () => changePage(1));
      els.stompRack.appendChild(next);
    }

    if (paged) {
      const indicator = document.createElement("span");
      indicator.className = "stomp-page-indicator";
      indicator.textContent = `${state.stompPage + 1}/${pageCount}`;
      els.stompRack.appendChild(indicator);
    }
  }

  function parameterOrder(plugin, params) {
    const base = pluginBase(plugin);
    const preferred = PREFERRED_PARAMS[base] || [];
    const available = Object.keys(params || {}).filter(name => name !== "Enabled");
    const ordered = [
      ...preferred.filter(name => available.includes(name)),
      ...available.filter(name => !preferred.includes(name)),
    ];

    return ordered.filter(name => {
      const value = Number(params[name]);
      const meta = state.paramMeta?.[plugin]?.[name] || state.paramMeta?.[base]?.[name];
      return (
        Number.isFinite(value) &&
        name !== "Model" &&
        name !== "Impulse" &&
        meta?.isOutput !== true &&
        String(meta?.type || "").toLowerCase() !== "bool"
      );
    }).slice(0, 6);
  }

  function buildParameter(plugin, name, rawValue) {
    const base = pluginBase(plugin);
    const meta =
      state.paramMeta?.[plugin]?.[name] ||
      state.paramMeta?.[base]?.[name] ||
      {};

    const stateValue = Number(rawValue);
    const value = Number(effectiveParamValue(plugin, name, stateValue));
    const min = Number.isFinite(meta.min) ? meta.min : Math.min(0, value);
    const max = Number.isFinite(meta.max) ? meta.max : Math.max(1, value);
    const step = Number.isFinite(meta.step) && meta.step > 0 ? meta.step : 0.01;
    const key = parameterKey(plugin, name);

    const row = document.createElement("div");
    row.className = "parameter";
    row.style.setProperty("--parameter-color", pluginColor(plugin));

    const label = document.createElement("label");
    label.textContent = name;

    const slider = document.createElement("input");
    slider.type = "range";
    slider.min = String(min);
    slider.max = String(max);
    slider.step = String(step);
    slider.value = String(value);
    slider.setAttribute("aria-label", `${friendlyPluginName(plugin)} ${name}`);

    const output = document.createElement("output");
    output.value = formatNumber(value, meta);
    output.textContent = output.value;

    slider.addEventListener("pointerdown", () => {
      state.activeParamKey = key;
    });

    slider.addEventListener("input", () => {
      const nextValue = Number(slider.value);
      holdPendingParam(plugin, name, nextValue, step);
      output.value = formatNumber(nextValue, meta);
      output.textContent = output.value;
    });

    let lastSent = String(value);

    const commit = async () => {
      const nextRaw = slider.value;
      const nextValue = Number(nextRaw);
      state.activeParamKey = null;

      if (nextRaw === lastSent) {
        scheduleRefresh();
        return;
      }

      lastSent = nextRaw;
      holdPendingParam(plugin, name, nextValue, step);

      try {
        await A.setNumericParamQueued(plugin, name, nextValue);
        scheduleRefresh();
      } catch (error) {
        console.error(error);
        state.pendingParams.delete(key);
        renderEditor();
        setStreamStatus("offline");
      }
    };

    slider.addEventListener("change", commit);
    slider.addEventListener("pointerup", commit);
    slider.addEventListener("pointercancel", () => {
      state.activeParamKey = null;
      scheduleRefresh();
    });

    row.append(label, slider, output);
    return row;
  }

  function renderEditor() {
    const stomps = getMappedStomps();
    selectDefaultController(stomps);

    const stomp = selectedStomp(stomps);
    const plugin = stomp?.plugin || null;
    els.parameterGrid.replaceChildren();

    if (!stomp || !plugin) {
      els.editorTitle.textContent = "Select an effect";
      els.editorToggle.disabled = true;
      els.editorToggle.setAttribute("aria-pressed", "false");
      els.editorToggleText.textContent = "OFF";

      const empty = document.createElement("div");
      empty.className = "editor-empty";
      empty.textContent =
        stomps.length
          ? "Touch a loaded stomp above to expose its live controls."
          : "This preset has no mapped stomp controls.";
      els.parameterGrid.appendChild(empty);
      return;
    }

    if (!stomp.loaded) {
      els.editorTitle.textContent = friendlyPluginName(plugin);
      els.editorToggle.disabled = true;
      els.editorToggle.setAttribute("aria-pressed", "false");
      els.editorToggleText.textContent = "—";

      const empty = document.createElement("div");
      empty.className = "editor-empty is-warning";
      empty.textContent =
        `${plugin} is mapped to controller ${stomp.controller}, ` +
        "but it is not loaded in any SetChain of this preset.";
      els.parameterGrid.appendChild(empty);
      return;
    }

    const enabled = isEnabled(plugin);
    els.editorTitle.textContent = friendlyPluginName(plugin);
    els.editorToggle.disabled = false;
    els.editorToggle.setAttribute("aria-pressed", String(enabled));
    els.editorToggleText.textContent = enabled ? "ON" : "OFF";

    const params = state.program?.params?.[plugin] || {};
    const names = parameterOrder(plugin, params);

    if (!names.length) {
      const empty = document.createElement("div");
      empty.className = "editor-empty";
      empty.textContent = "This effect has no live numeric controls.";
      els.parameterGrid.appendChild(empty);
      return;
    }

    for (const name of names) {
      els.parameterGrid.appendChild(
        buildParameter(plugin, name, params[name]),
      );
    }
  }

  function renderAll() {
    if (!state.snapshot) return;
    renderIdentity();
    renderStomps();

    // Replacing the slider DOM while a finger is down would terminate the
    // gesture. Pending values also protect the editor from stale cache echoes.
    if (!state.activeParamKey) {
      renderEditor();
    }
  }

  async function refreshState() {
    if (state.refreshPromise) return state.refreshPromise;

    state.refreshPromise = (async () => {
      const { data } = await A.fetchState();
      state.snapshot = data;
      state.program = P.parseDumpProgram(data.program?.raw || "");
      state.config = P.parseDumpConfig(data.dumpConfig?.raw || "");
      state.paramMeta = P.parseParameterConfig(data.dumpConfig?.raw || "");
      state.fileTrees = P.parseFileTrees(data.dumpConfig?.raw || "");
      state.resolved = data.program?.resolved || {};
      reconcilePendingParams();
      renderAll();
      return data;
    })();

    try {
      return await state.refreshPromise;
    } finally {
      state.refreshPromise = null;
    }
  }

  function scheduleRefresh() {
    if (state.pendingFrame) return;
    state.pendingFrame = requestAnimationFrame(async () => {
      state.pendingFrame = null;
      try {
        await refreshState();
        setStreamStatus("online");
      } catch (error) {
        console.error(error);
        setStreamStatus("offline");
      }
    });
  }

  function extractAudioName(system) {
    const lines = system.audioif?.asound_cards;
    if (!Array.isArray(lines) || !lines.length) return "—";

    const device = String(system.jack?.device || "");
    const cardMatch = device.match(/CARD=([^,]+)/);
    const card = cardMatch?.[1];
    const line =
      (card && lines.find(item => String(item).includes(`[${card}]`))) ||
      lines[0];

    const name = String(line).match(/-\s*(.+)$/)?.[1] || String(line);
    return name.replace(/USB Audio/i, "").trim().split(/\s+/)[0] || "—";
  }

  async function refreshSystem() {
    try {
      const response = await fetch("/api/system", { cache: "no-store" });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const system = await response.json();

      const jackRunning = system.jack?.running === true;
      const midiConnected = system.midi?.connected === true;
      const xrunDelta = Number(system.jack?.xruns_delta || 0);
      const routingWarn =
        system.routing?.probe_ok === true &&
        system.routing?.ok === false;

      const health =
        !jackRunning ? "fail" :
        xrunDelta > 0 || !midiConnected || routingWarn ? "warn" :
        "ready";

      els.statusPanel.dataset.health = health;
      els.readyText.textContent =
        health === "ready" ? "READY" :
        health === "warn" ? "CHECK" :
        "FAIL";

      els.xrunValue.textContent =
        `${system.jack?.xruns ?? "—"} +${system.jack?.xruns_delta ?? 0}`;
      els.midiValue.textContent = midiConnected ? "OK" : "—";
      els.audioValue.textContent = extractAudioName(system);
    } catch (error) {
      console.error(error);
      els.statusPanel.dataset.health = "warn";
      els.readyText.textContent = "CHECK";
    }
  }

  function libraryDefinition(kind) {
    if (kind === "save-as") {
      return {
        title: "Save preset as",
        current: String(state.program?.preset || "").trim(),
        options: [],
      };
    }

    if (kind === "nam") {
      const plugin = findPlugin("NAM");
      const enabled = isEnabled(plugin);
      const currentFile = resolvedValue(plugin, "Model");

      return {
        title: "Select NAM model",
        plugin,
        param: "Model",
        enabled,
        currentFile,
        current: enabled ? currentFile : "NONE — BYPASSED",
        bypassTitle: "NONE / BYPASS NAM",
        bypassNote: "Disable amplifier modelling and keep the selected model ready.",
        options:
          state.fileTrees[`${plugin}.Model`] ||
          state.fileTrees["NAM.Model"] ||
          [],
      };
    }

    const plugin = findPlugin("Cabinet");
    const enabled = isEnabled(plugin);
    const currentFile = resolvedValue(plugin, "Impulse");

    return {
      title: "Select cabinet IR",
      plugin,
      param: "Impulse",
      enabled,
      currentFile,
      current: enabled ? currentFile : "NONE — BYPASSED",
      bypassTitle: "NONE / BYPASS CABINET",
      bypassNote: "Disable cabinet convolution and keep the selected IR ready.",
      options:
        state.fileTrees[`${plugin}.Impulse`] ||
        state.fileTrees["Cabinet.Impulse"] ||
        [],
    };
  }

  function normalizedSearch(value) {
    return String(value || "")
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .toLowerCase();
  }

  function filteredLibraryOptions(definition) {
    const terms = normalizedSearch(state.library.query)
      .split(/\s+/)
      .filter(Boolean);

    if (!terms.length) return definition.options;

    return definition.options.filter(option => {
      const haystack = normalizedSearch(option);
      return terms.every(term => haystack.includes(term));
    });
  }

  function renderLibraryQuery() {
    els.libraryQuery.replaceChildren();

    if (!state.library.query) {
      const placeholder = document.createElement("span");
      placeholder.className = "library-query-placeholder";
      placeholder.textContent =
        state.library.kind === "save-as"
          ? "Enter a new preset name"
          : "Touch keys to filter";
      els.libraryQuery.appendChild(placeholder);
      return;
    }

    els.libraryQuery.textContent = state.library.query;
  }

  function renderLibraryKeyboard() {
    els.libraryKeyboard.replaceChildren();

    const addKey = (row, label, value, classes = "") => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = `keyboard-key ${classes}`.trim();
      button.textContent = label;
      button.addEventListener("click", () => {
        if (state.library.busy) return;

        if (value === "BACKSPACE") {
          state.library.query = state.library.query.slice(0, -1);
        } else if (value === "CLEAR") {
          state.library.query = "";
        } else {
          state.library.query += value;
        }

        renderLibrary();
      });
      row.appendChild(button);
    };

    for (let index = 0; index < KEYBOARD_ROWS.length; index += 1) {
      const row = document.createElement("div");
      row.className = "keyboard-row";

      for (const character of KEYBOARD_ROWS[index]) {
        addKey(row, character, character);
      }

      if (index === 3) {
        addKey(row, "SPACE", " ", "is-wider");
        addKey(row, "⌫", "BACKSPACE", "is-wide");
        addKey(row, "CLEAR", "CLEAR", "is-wide is-danger");
      }

      els.libraryKeyboard.appendChild(row);
    }
  }

  async function chooseLibraryBypass(definition, button) {
    if (state.library.busy) return;

    if (!definition.enabled) {
      closeLibrary();
      return;
    }

    state.library.busy = true;
    button.disabled = true;
    els.libraryCount.textContent = "Bypassing…";

    try {
      await A.setPluginEnabled(definition.plugin, false);
      await refreshState();
      closeLibrary();
    } catch (error) {
      console.error(error);
      state.library.busy = false;
      button.disabled = false;
      els.libraryCount.textContent = "Bypass failed";
      setStreamStatus("offline");
    }
  }

  async function chooseLibraryValue(definition, value, button) {
    if (state.library.busy) return;

    const sameFile = value === definition.currentFile;
    if (sameFile && definition.enabled) {
      closeLibrary();
      return;
    }

    state.library.busy = true;
    button.disabled = true;
    els.libraryCount.textContent = definition.enabled ? "Loading…" : "Enabling…";

    try {
      if (!sameFile) {
        await A.setFileParam(definition.plugin, definition.param, value);
      }

      if (!definition.enabled) {
        await A.setPluginEnabled(definition.plugin, true);
      }

      await refreshState();
      closeLibrary();
    } catch (error) {
      console.error(error);
      state.library.busy = false;
      button.disabled = false;
      els.libraryCount.textContent = "Selection failed";
      setStreamStatus("offline");
    }
  }

  function knownPresetNames() {
    return P.parsePresets(state.snapshot?.presets?.raw || "");
  }

  function validPresetName(value) {
    const name = String(value || "").trim();
    if (!name || name.length > 96) return false;
    return !/[\\/\r\n\t]/.test(name);
  }

  async function savePresetAs(name, button) {
    if (state.library.busy || !validPresetName(name)) return;

    state.library.busy = true;
    button.disabled = true;
    els.libraryCount.textContent = "Saving…";

    try {
      await postJSON("/api/preset/save-as", { name });
      await refreshState();
      closeLibrary();
      showPresetActionFeedback(els.saveAsPresetButton, "Saved");
    } catch (error) {
      console.error(error);
      state.library.busy = false;
      button.disabled = false;
      els.libraryCount.textContent = "Save failed";
      setStreamStatus("offline");
    }
  }

  function renderSaveAsResult(definition) {
    const name = String(state.library.query || "").trim();
    const existing = knownPresetNames().includes(name);
    const valid = validPresetName(name);

    els.libraryResults.replaceChildren();
    els.libraryResults.classList.add("is-save-as");
    els.libraryCount.textContent =
      !name ? "Enter name" :
      !valid ? "Invalid name" :
      existing ? "Existing preset" :
      "New preset";

    const card = document.createElement("section");
    card.className = "save-as-card";

    const label = document.createElement("span");
    label.className = "micro-label";
    label.textContent = existing ? "This name already exists" : "New preset name";

    const value = document.createElement("strong");
    value.textContent = name || "—";

    const note = document.createElement("p");
    note.textContent =
      !name
        ? "Use the touch keyboard to enter a name."
        : !valid
          ? "Use 1–96 characters. Slashes and line breaks are not allowed."
          : existing
            ? "Saving will replace the preset with this name."
            : "The current sound and effect state will be stored under this name.";

    const button = document.createElement("button");
    button.type = "button";
    button.className = "save-as-confirm";
    button.disabled = !valid || state.library.busy;
    button.textContent = existing ? "Replace preset" : "Save new preset";
    button.addEventListener("click", () => savePresetAs(name, button));

    card.append(label, value, note, button);
    els.libraryResults.appendChild(card);
  }

  function renderLibraryResults(definition) {
    const options = filteredLibraryOptions(definition);
    els.libraryResults.classList.remove("is-save-as");
    els.libraryResults.replaceChildren();
    els.libraryCount.textContent =
      `${options.length} model${options.length === 1 ? "" : "s"} + bypass`;

    const bypass = document.createElement("button");
    bypass.type = "button";
    bypass.className = "library-result is-bypass";
    bypass.dataset.current = String(!definition.enabled);
    bypass.innerHTML = `
      <span class="library-bypass-copy">
        <strong>${definition.bypassTitle}</strong>
        <small>${definition.bypassNote}</small>
      </span>
      <span class="library-result-marker">✓</span>
    `;
    bypass.addEventListener(
      "click",
      () => chooseLibraryBypass(definition, bypass),
    );
    els.libraryResults.appendChild(bypass);

    if (!options.length) {
      const empty = document.createElement("div");
      empty.className = "library-empty-results";
      empty.textContent = "No model matches. Bypass remains available above.";
      els.libraryResults.appendChild(empty);
      return;
    }

    const fragment = document.createDocumentFragment();

    for (const option of options) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "library-result";
      button.dataset.current = String(
        definition.enabled && option === definition.currentFile,
      );
      button.innerHTML = `
        <span>${option}</span>
        <span class="library-result-marker">✓</span>
      `;
      button.addEventListener(
        "click",
        () => chooseLibraryValue(definition, option, button),
      );
      fragment.appendChild(button);
    }

    els.libraryResults.appendChild(fragment);

    const current = els.libraryResults.querySelector('[data-current="true"]');
    if (current && !state.library.query) {
      requestAnimationFrame(() => current.scrollIntoView({ block: "center" }));
    }
  }

  function renderLibrary() {
    if (!state.library.kind) return;
    const definition = libraryDefinition(state.library.kind);
    const saveMode = state.library.kind === "save-as";

    els.libraryEyebrow.textContent = saveMode ? "Preset management" : "Sound library";
    els.libraryTitle.textContent = definition.title;
    els.libraryCurrent.textContent = definition.current || "—";

    renderLibraryQuery();
    renderLibraryKeyboard();

    if (saveMode) {
      renderSaveAsResult(definition);
    } else {
      renderLibraryResults(definition);
    }
  }

  function openLibrary(kind) {
    state.library.kind = kind;
    state.library.query =
      kind === "save-as"
        ? String(state.program?.preset || "").trim()
        : "";
    state.library.busy = false;
    els.libraryOverlay.hidden = false;
    renderLibrary();
  }

  function closeLibrary() {
    state.library.kind = null;
    state.library.query = "";
    state.library.busy = false;
    els.libraryOverlay.hidden = true;
  }

  function showPresetActionFeedback(button, label) {
    if (!button) return;

    const original = button.dataset.defaultLabel || button.textContent;
    button.dataset.defaultLabel = original;
    button.textContent = label;
    button.dataset.feedback = "true";

    window.setTimeout(() => {
      button.textContent = button.dataset.defaultLabel || original;
      delete button.dataset.feedback;
    }, 1200);
  }

  els.savePresetButton.addEventListener("click", async () => {
    const name = String(state.program?.preset || "").trim();
    if (!name || els.savePresetButton.disabled) return;

    els.savePresetButton.disabled = true;

    try {
      await A.savePreset(name);
      await refreshState();
      showPresetActionFeedback(els.savePresetButton, "Saved");
    } catch (error) {
      console.error(error);
      showPresetActionFeedback(els.savePresetButton, "Failed");
      setStreamStatus("offline");
    } finally {
      els.savePresetButton.disabled = false;
    }
  });

  els.saveAsPresetButton.addEventListener("click", () => openLibrary("save-as"));

  els.namPickerButton.addEventListener("click", () => openLibrary("nam"));
  els.cabPickerButton.addEventListener("click", () => openLibrary("cab"));
  els.libraryClose.addEventListener("click", closeLibrary);

  window.addEventListener("keydown", event => {
    if (els.libraryOverlay.hidden) return;

    if (event.key === "Escape") {
      event.preventDefault();
      closeLibrary();
      return;
    }

    if (event.key === "Backspace") {
      event.preventDefault();
      state.library.query = state.library.query.slice(0, -1);
      renderLibrary();
      return;
    }

    if (event.key.length === 1 && /[a-z0-9 ._-]/i.test(event.key)) {
      event.preventDefault();
      state.library.query += event.key.toUpperCase();
      renderLibrary();
    }
  });

  els.editorToggle.addEventListener("click", async () => {
    const stomp = selectedStomp();
    const plugin = stomp?.loaded ? stomp.plugin : null;
    if (!plugin) return;

    const enabled = isEnabled(plugin);
    els.editorToggle.disabled = true;
    try {
      await togglePlugin(plugin, !enabled);
    } catch (error) {
      console.error(error);
      setStreamStatus("offline");
    } finally {
      els.editorToggle.disabled = false;
    }
  });

  function updateClock() {
    els.clockValue.textContent = new Intl.DateTimeFormat(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date());
  }

  async function start() {
    updateClock();
    setInterval(updateClock, 15000);

    try {
      await refreshState();
      await refreshSystem();
    } catch (error) {
      console.error(error);
      setStreamStatus("offline");
    }

    state.eventSource = A.openEvents({
      open: () => setStreamStatus("online"),
      disconnect: () => setStreamStatus("offline"),
      snapshot: scheduleRefresh,
      program: scheduleRefresh,
      config: scheduleRefresh,
      presets: scheduleRefresh,
      error: error => console.error(error),
    });

    state.systemTimer = setInterval(refreshSystem, 2000);
  }

  window.addEventListener("beforeunload", () => {
    state.eventSource?.close();
    if (state.systemTimer) clearInterval(state.systemTimer);
  });

  start();
})();
