// parse.js
(() => {
    const P = {};

    P.parsePresets = function parsePresets(raw) {
        const toks = (raw || '').split(/\s+/).filter(Boolean);
        return toks.filter(t => t !== 'Presets' && t !== 'Ok');
    };

    P.parseQuotedTokens = function parseQuotedTokens(s) {
        const out = [];
        const re = /"([^"]*)"/g;
        let m;
        while ((m = re.exec(s)) !== null) out.push(m[1]);
        return out;
    };

    P.parseFileTrees = function parseFileTrees(dumpConfigRaw) {
        const trees = {};
        const lines = (dumpConfigRaw || '').split(/\r?\n/);

        for (const line of lines) {
            const l = line.trim();
            if (!l.startsWith('ParameterFileTree ')) continue;

            const parts = l.split(/\s+/);
            if (parts.length < 4) continue;

            const plugin = parts[1];
            const param = parts[2];

            const opts = P.parseQuotedTokens(l);
            if (!opts.length) continue;

            trees[plugin + '.' + param] = opts;
        }
        return trees;
    };

    P.parseDumpProgram = function parseDumpProgram(raw) {
        const out = {
            preset: null,
            chains: {},
            params: {},
            slots: {},
        };
        if (!raw) return out;

        const lines = raw.split(/\r?\n/);

        for (const line of lines) {
            if (!line) continue;

            // SetPreset <name>
            {
                const m = line.match(/^SetPreset\s+(.+)$/);
                if (m) { out.preset = m[1].trim(); continue; }
            }

            // SetChain <Chain> <plugin1> <plugin2>...
            {
                const m = line.match(/^SetChain\s+(\S+)\s+(.*)$/);
                if (m) {
                    const chain = m[1];
                    const rest = (m[2] || "").trim();
                    out.chains[chain] = rest ? rest.split(/\s+/).filter(Boolean) : [];
                    continue;
                }
            }

            // SetPluginSlot <slot> <plugin>
            {
                const m = line.match(/^SetPluginSlot\s+(\S+)\s+(\S+)/);
                if (m) { out.slots[m[1]] = m[2]; continue; }
            }

            // SetParam <plugin> <param> <value...>
            {
                const m = line.match(/^SetParam\s+(\S+)\s+(\S+)\s+(.+)$/);
                if (m) {
                    const plugin = m[1];
                    const param = m[2];
                    let value = m[3].trim();
                    if (value.startsWith('"') && value.endsWith('"')) value = value.slice(1, -1);
                    if (!out.params[plugin]) out.params[plugin] = {};
                    out.params[plugin][param] = value;
                    continue;
                }
            }
        }
        return out;
    };

    P.parseDumpConfig = function parseDumpConfig(raw) {
        const map = {};
        const lines = (raw || '').split(/\r?\n/);

        for (const line of lines) {
            if (!line.startsWith('PluginConfig ')) continue;

            const mName = line.match(/^PluginConfig\s+(\S+)/);
            if (!mName) continue;
            const name = mName[1];

            const mBg = line.match(/\bBackgroundColor\s+(#[0-9a-fA-F]{6})\b/);
            const mFg = line.match(/\bForegroundColor\s+(#[0-9a-fA-F]{6})\b/);
            const mDesc = line.match(/\bDescription\s+"([^"]*)"/);
            const mSel = line.match(/\bIsUserSelectable\s+([01])\b/);

            map[name] = {
                bg: mBg ? mBg[1] : null,
                fg: mFg ? mFg[1] : null,
                desc: mDesc ? mDesc[1] : '',
                selectable: mSel ? (Number(mSel[1]) === 1) : false,
            };
        }
        return map;
    };

    P.parseParameterConfig = function parseParameterConfig(raw) {
        const meta = {};
        const lines = (raw || '').split(/\r?\n/);

        const getTok = (parts, key) => {
            const i = parts.indexOf(key);
            return (i >= 0 && i + 1 < parts.length) ? parts[i + 1] : null;
        };

        for (const line of lines) {
            if (!line.startsWith('ParameterConfig ')) continue;

            const parts = line.trim().split(/\s+/);
            if (parts.length < 4) continue;

            const plugin = parts[1];
            const param = parts[2];

            const type = getTok(parts, 'Type');
            const minS = getTok(parts, 'MinValue');
            const maxS = getTok(parts, 'MaxValue');
            const defS = getTok(parts, 'DefaultValue');
            const isOutputS = getTok(parts, 'IsOutput');

            const min = (minS !== null) ? Number(minS) : null;
            const max = (maxS !== null) ? Number(maxS) : null;
            const def = (defS !== null) ? Number(defS) : null;
            const isOutput = (isOutputS !== null) ? (Number(isOutputS) === 1) : false;

            let valueFormat = null;
            const mf = line.match(/\bValueFormat\s+([^\s]+)\s/);
            if (mf) valueFormat = mf[1];

            let step = 0.01;
            if (Number.isFinite(min) && Number.isFinite(max)) {
                const span = Math.abs(max - min);
                if (span <= 1) step = 0.001;
                else if (span <= 10) step = 0.01;
                else step = 0.1;
            }

            if (!meta[plugin]) meta[plugin] = {};
            meta[plugin][param] = { type, min, max, def, step, isOutput, valueFormat };
        }
        return meta;
    };

    window.NAMNESIS = window.NAMNESIS || {};
    window.NAMNESIS.parse = P;
})();
